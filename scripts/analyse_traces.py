"""Quick analysis of the two Langfuse trace export files."""
import json
import sys
from datetime import datetime, timezone

HEX_TRACE = "/mnt/c/Users/sanjsingh/Downloads/trace-ac3ccadbea4d4ee1ae39f5bb6cf2f0aa.json"
UUID_TRACE = "/mnt/c/Users/sanjsingh/Downloads/trace-ac3ccadb-ea4d-4ee1-ae39-f5bb6cf2f0aa.json"


def parse_ts(ts_str):
    if not ts_str:
        return None
    return datetime.fromisoformat(ts_str.replace("Z", "+00:00"))


def epoch_to_dt(epoch_str):
    if not epoch_str:
        return None
    try:
        return datetime.fromtimestamp(int(epoch_str), tz=timezone.utc)
    except Exception:
        return None


# ── HEX TRACE (OTEL spans) ──────────────────────────────────────────────────
print("=" * 70)
print("OTEL HEX TRACE (chaos experiment spans)")
print("=" * 70)

with open(HEX_TRACE, encoding="utf-8") as f:
    hex_data = json.load(f)

faults = []
experiment_start = None
experiment_end = None

for span in hex_data:
    attrs = json.loads(span.get("metadata", "{}")).get("attributes", {})
    name = span.get("name", "")
    start = parse_ts(span.get("startTime"))
    end = parse_ts(span.get("endTime"))

    if "experiment-run" in name:
        experiment_start = start
        experiment_end = end
        print(f"Experiment run : {attrs.get('experiment.name')}")
        print(f"  Phase        : {attrs.get('experiment.final_phase')}")
        print(f"  Resiliency   : {attrs.get('experiment.resiliency_score')}%")
        print(f"  Total faults : {attrs.get('experiment.total_faults')}")
        print(f"  Passed       : {attrs.get('experiment.faults_passed')}")
        print(f"  Failed       : {attrs.get('experiment.faults_failed')}")
        duration = (end - start).total_seconds() if start and end else None
        print(f"  Start        : {start}")
        print(f"  End          : {end}")
        print(f"  Duration     : {duration:.0f}s" if duration else "  Duration     : N/A")

    if name.startswith("fault:"):
        fault_name = attrs.get("fault.name", name)
        f_start = epoch_to_dt(attrs.get("fault.started_at"))
        f_end = epoch_to_dt(attrs.get("fault.finished_at"))
        f_dur = (f_end - f_start).total_seconds() if f_start and f_end else None
        faults.append({
            "name": fault_name,
            "verdict": attrs.get("fault.verdict"),
            "probe_pct": attrs.get("fault.probe_success_pct"),
            "target": f"{attrs.get('fault.target_label')} in {attrs.get('fault.target_namespace')}",
            "started_at": f_start,
            "finished_at": f_end,
            "duration_s": f_dur,
        })

print(f"\nFault timeline:")
for ft in sorted(faults, key=lambda x: x["started_at"] or datetime.min.replace(tzinfo=timezone.utc)):
    icon = "✅" if ft["verdict"] == "Pass" else "❌"
    offset = (ft["started_at"] - experiment_start).total_seconds() if ft["started_at"] and experiment_start else "?"
    print(f"  {icon} {ft['name']:25s}  verdict={ft['verdict']:4s}  probe={ft['probe_pct']:>3}%  "
          f"target={ft['target']:30s}  start+{offset:.0f}s  dur={ft['duration_s']:.0f}s")

# ── UUID TRACE (LLM generations) ────────────────────────────────────────────
print("\n" + "=" * 70)
print("UUID TRACE (LLM generations)")
print("=" * 70)

with open(UUID_TRACE, encoding="utf-8") as f:
    uuid_data = json.load(f)

print(f"Total generations: {len(uuid_data)}")

by_name = {}
for g in uuid_data:
    n = g.get("name", "?")
    by_name[n] = by_name.get(n, 0) + 1
print("By name:", dict(sorted(by_name.items(), key=lambda x: -x[1])))

# ── Check ground_truth_evaluation ───────────────────────────────────────────
print("\n--- Ground truth evaluation check ---")
gt_found = 0
gt_sample = None
for g in uuid_data:
    output_raw = g.get("output")
    if not output_raw:
        continue
    try:
        output = json.loads(output_raw) if isinstance(output_raw, str) else output_raw
        if isinstance(output, dict):
            # LiteLLM wraps in choices
            choices = output.get("choices", [])
            if choices:
                content = choices[0].get("message", {}).get("content", "")
                if content:
                    try:
                        parsed = json.loads(content)
                        if "ground_truth_evaluation" in parsed and parsed["ground_truth_evaluation"]:
                            gt_found += 1
                            if gt_sample is None:
                                gt_sample = parsed["ground_truth_evaluation"]
                    except Exception:
                        pass
    except Exception:
        pass

print(f"Generations with ground_truth_evaluation: {gt_found}/{len(uuid_data)}")
if gt_sample:
    print("Sample ground_truth_evaluation:")
    print(json.dumps(gt_sample, indent=2)[:1500])
else:
    print("  ⚠ None found — GT not injected into prompt yet (needs build+deploy)")

# ── TTD / TTR computation ────────────────────────────────────────────────────
print("\n--- TTD / TTR feasibility from traces ---")
print("""
From HEX trace (authoritative):
  fault.started_at  = Unix epoch when chaos engine started
  fault.finished_at = Unix epoch when chaos engine finished
  experiment start  = experiment-triggered span startTime

From UUID trace (agent perspective):
  Each llm_analysis generation has a startTime
  → TTD = time of FIRST llm_analysis that names the fault - fault.started_at
  → TTR = time when agent's output shows recovery/remediation - fault.started_at

CURRENT STATUS:""")

# Find first llm_analysis time
llm_analysis_times = []
for g in uuid_data:
    if g.get("name") in ("llm_analysis", "llm-analysis") or \
       g.get("name", "").startswith("llm_analysis") or \
       g.get("name", "").startswith("llm-analysis"):
        t = parse_ts(g.get("startTime"))
        if t:
            llm_analysis_times.append(t)

if llm_analysis_times and faults and experiment_start:
    llm_analysis_times.sort()
    first_analysis = llm_analysis_times[0]
    last_analysis = llm_analysis_times[-1]
    print(f"  llm_analysis generations : {len(llm_analysis_times)}")
    print(f"  First llm_analysis at    : {first_analysis}")
    print(f"  Last  llm_analysis at    : {last_analysis}")
    print(f"\n  Per-fault potential TTD (fault.started_at → first analysis AFTER fault start):")
    for ft in sorted(faults, key=lambda x: x["started_at"] or datetime.min.replace(tzinfo=timezone.utc)):
        if ft["started_at"]:
            first_after = next((t for t in llm_analysis_times if t > ft["started_at"]), None)
            if first_after:
                ttd_s = (first_after - ft["started_at"]).total_seconds()
                print(f"    {ft['name']:25s}  fault_start={ft['started_at'].strftime('%H:%M:%S')}  "
                      f"first_analysis_after={first_after.strftime('%H:%M:%S')}  "
                      f"max_TTD≤{ttd_s:.0f}s")
            else:
                print(f"    {ft['name']:25s}  no analysis after fault start")
    print("""
  NOTE: This is an UPPER BOUND on TTD. To get precise TTD we need to check
  which llm_analysis output actually mentions/detects each fault by name.
  That requires parsing the output JSON of each generation — doable once
  ground_truth_evaluation or chaos_faults[] is populated in the output.
""")
else:
    print("  Insufficient data to compute TTD")
