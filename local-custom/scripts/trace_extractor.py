#!/usr/bin/env python3
"""Extract Langfuse traces to certification-ready JSON via live API.

Two trace types are created per experiment:
  • Agent trace  (UUID with dashes)  – LLM call observations from flash-agent/proxy
  • OTEL trace   (UUID no dashes)    – Control-plane lifecycle spans from GraphQL server
                                       sessionId on OTEL trace == agent trace's id

The extractor:
  1. Fetches both trace types.
  2. Cross-links them: copies scores + OTEL span attributes (namespace, resiliency,
     fault counts, final_phase) onto the agent trace record.
  3. Emits ONE record per experiment (agent trace), OTEL traces are auxiliary.

Usage:
    python3 trace_extractor.py \
        --env-file /mnt/d/Studies/AgentCert/local-custom/config/.env \
        --from-ist "2026-04-28 20:00" \
        --output /mnt/d/Studies/certifier/trace_dump/traces_srs.json
"""
from __future__ import annotations
import argparse, base64, json, os, sys, time
from datetime import datetime, timezone, timedelta
from pathlib import Path
from typing import Any, Dict, List, Optional
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen

IST = timezone(timedelta(hours=5, minutes=30))
LANGFUSE_HOST = LANGFUSE_PUBLIC_KEY = LANGFUSE_SECRET_KEY = LANGFUSE_PROJECT_ID = ""


# ── .env loader ───────────────────────────────────────────────────────────────
def _load_env_file(path: str) -> dict:
    env: dict = {}
    try:
        with open(path, encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                key, _, val = line.partition("=")
                key, val = key.strip(), val.strip()
                if len(val) >= 2 and val[0] in ('"', "'") and val[-1] == val[0]:
                    val = val[1:-1]
                env[key] = val
    except FileNotFoundError:
        print(f"[env-file] Not found: {path}", file=sys.stderr)
        sys.exit(1)
    return env


def _init_config(env: dict) -> None:
    global LANGFUSE_HOST, LANGFUSE_PUBLIC_KEY, LANGFUSE_SECRET_KEY, LANGFUSE_PROJECT_ID
    host = env.get("LANGFUSE_HOST", "http://localhost:3005").rstrip("/")
    # Auto-remap cluster-internal URL to localhost port-forward for WSL/local access
    if "svc.cluster.local" in host:
        print(f"[config] LANGFUSE_HOST is cluster-internal ({host}).")
        print("[config] Remapping to http://localhost:3005 for WSL/local access.")
        print("[config] Ensure port-forward: kubectl port-forward -n langfuse svc/langfuse-web 3005:3000")
        host = "http://localhost:3005"
    LANGFUSE_HOST = host
    LANGFUSE_PUBLIC_KEY = env.get("LANGFUSE_PUBLIC_KEY", "")
    LANGFUSE_SECRET_KEY = env.get("LANGFUSE_SECRET_KEY", "")
    LANGFUSE_PROJECT_ID = env.get("LANGFUSE_PROJECT_ID", "")
    if not LANGFUSE_PUBLIC_KEY or not LANGFUSE_SECRET_KEY:
        print("[ERROR] LANGFUSE_PUBLIC_KEY/SECRET_KEY missing. Pass --env-file.", file=sys.stderr)
        sys.exit(1)
    print(f"[config] Host={LANGFUSE_HOST}  Project={LANGFUSE_PROJECT_ID or '(all)'}")


# ── HTTP ──────────────────────────────────────────────────────────────────────
def _auth() -> str:
    return "Basic " + base64.b64encode(
        f"{LANGFUSE_PUBLIC_KEY}:{LANGFUSE_SECRET_KEY}".encode()
    ).decode()


def _get(path: str, params: dict = None, timeout: int = 60) -> dict:
    url = f"{LANGFUSE_HOST}{path}"
    if params:
        url += "?" + urlencode({k: v for k, v in params.items() if v is not None})
    req = Request(url, headers={"Authorization": _auth(), "Accept": "application/json"})
    try:
        with urlopen(req, timeout=timeout) as r:
            return json.loads(r.read().decode())
    except HTTPError as e:
        print(f"  HTTP {e.code}: {e.read().decode(errors='replace')[:200]}", file=sys.stderr)
        raise
    except URLError as e:
        print(f"  URL error: {e.reason}", file=sys.stderr)
        raise


# ── Fetch ─────────────────────────────────────────────────────────────────────
def fetch_traces_page(from_utc: datetime, limit: int, page: int) -> dict:
    return _get("/api/public/traces", {
        "limit": limit,
        "page": page,
        "fromTimestamp": from_utc.strftime("%Y-%m-%dT%H:%M:%SZ"),
    })


def fetch_observations(trace_id: str) -> list:
    page, result = 1, []
    while True:
        resp = _get("/api/public/observations", {"traceId": trace_id, "limit": 100, "page": page})
        items = resp.get("data", [])
        result.extend(items)
        if page >= resp.get("meta", {}).get("totalPages", 1) or not items:
            break
        page += 1
    return result


# ── Helpers ───────────────────────────────────────────────────────────────────
def fmt_ts(val: Any) -> Optional[str]:
    if val is None:
        return None
    if isinstance(val, str):
        if not val:
            return None
        try:
            val = datetime.fromisoformat(val.replace("Z", "+00:00"))
        except ValueError:
            return val
    if isinstance(val, datetime):
        val = val.astimezone(timezone.utc)
        return val.strftime("%Y-%m-%dT%H:%M:%S.") + f"{val.microsecond // 1000:03d}Z"
    return None


def to_ist_str(ts: Optional[str]) -> Optional[str]:
    if not ts:
        return None
    try:
        return datetime.fromisoformat(ts.replace("Z", "+00:00")).astimezone(IST).strftime(
            "%Y-%m-%d %H:%M:%S IST"
        )
    except Exception:
        return None


def to_str(v: Any) -> Optional[str]:
    if v is None:
        return None
    return v if isinstance(v, str) else json.dumps(v, ensure_ascii=False)


def _cv(o: dict, snake: str, camel: str) -> Any:
    v = o.get(camel)
    return v if v is not None else o.get(snake)


# ── Format observations ───────────────────────────────────────────────────────
def format_observations(raw: list) -> list:
    normed = [
        {
            "id": o.get("id"),
            "trace_id": _cv(o, "trace_id", "traceId"),
            "type": o.get("type"),
            "name": o.get("name"),
            "level": o.get("level"),
            "status_message": _cv(o, "status_message", "statusMessage"),
            "start_time": _cv(o, "start_time", "startTime"),
            "end_time": _cv(o, "end_time", "endTime"),
            "parent_observation_id": _cv(o, "parent_observation_id", "parentObservationId"),
            "input": o.get("input"),
            "output": o.get("output"),
            "metadata": o.get("metadata"),
            "model": o.get("model"),
            "usage": o.get("usage"),
        }
        for o in raw
    ]

    pm = {o["id"]: o.get("parent_observation_id") for o in normed if o.get("id")}
    dc: dict = {}

    def depth(oid):
        if oid in dc:
            return dc[oid]
        pid = pm.get(oid)
        dc[oid] = 0 if not pid or pid not in pm else depth(pid) + 1
        return dc[oid]

    dm = {oid: depth(oid) for oid in pm}

    out = []
    for o in normed:
        oid = o.get("id", "")
        mr = o.get("metadata") or {}
        if isinstance(mr, str):
            try:
                mr = json.loads(mr)
            except Exception:
                mr = {}
        rq = mr.get("requester_metadata") or {} if isinstance(mr, dict) else {}
        out.append({
            "id": oid,
            "trace_id": o["trace_id"],
            "type": o["type"],
            "name": o["name"],
            "level": o["level"],
            "status_message": o["status_message"],
            "startTime": fmt_ts(o["start_time"]),
            "endTime": fmt_ts(o["end_time"]),
            "depth": dm.get(oid, 0),
            "parent_observation_id": o["parent_observation_id"],
            "model": o["model"],
            "usage": o["usage"],
            "input": to_str(o["input"]),
            "output": to_str(o["output"]),
            "metadata": to_str(mr),
            # Certification fields from requester_metadata
            "notify_id": rq.get("notify_id") or rq.get("trace_id"),
            "experiment_id": rq.get("experiment_id") or rq.get("session_id"),
            "experiment_run_id": rq.get("experiment_run_id") or rq.get("user_id"),
            "workflow_name": rq.get("workflow_name") or rq.get("trace_name"),
            "generation_name": rq.get("generation_name"),
            "step": rq.get("step"),
            "scan_id": rq.get("scan_id"),
            "scan_number": rq.get("scan_number"),
            "fault_names": rq.get("fault_names"),
            "expected_output": rq.get("expected_output"),
            "is_ground_truth_data": rq.get("is_ground_truth_data", False),
            "agent_name": rq.get("agent") or rq.get("agent_name"),
            "mcp_server_type": rq.get("mcp_server_type"),
            "mcp_duration_sec": rq.get("mcp_duration_sec"),
            "pods_total": rq.get("pods_total"),
            "events_warning": rq.get("events_warning"),
        })
    out.sort(key=lambda x: (x["depth"], x["startTime"] or ""))
    return out


# ── Scores ────────────────────────────────────────────────────────────────────
def flatten_scores(scores: list) -> dict:
    flat: dict = {}
    for s in scores or []:
        if not isinstance(s, dict):
            continue
        n = s.get("name")
        if n:
            flat[n] = s.get("value")
            if s.get("comment"):
                flat[f"{n}__comment"] = s["comment"]
    return flat


# ── Extract OTEL span attributes ─────────────────────────────────────────────
def _extract_otel_attrs(obs_list: list) -> dict:
    """Walk observations from the OTEL/control-plane trace and pull experiment.* / infra.* attrs."""
    result: dict = {}
    for o in obs_list:
        raw_meta = o.get("metadata") or {}
        if isinstance(raw_meta, str):
            try:
                raw_meta = json.loads(raw_meta)
            except Exception:
                continue
        if not isinstance(raw_meta, dict):
            continue
        attrs = raw_meta.get("attributes") or {}
        if not attrs:
            continue
        # namespace — prefer infra.namespace
        if not result.get("namespace"):
            result["namespace"] = attrs.get("infra.namespace") or attrs.get("experiment.namespace")
        # final phase
        if not result.get("final_phase"):
            result["final_phase"] = (
                attrs.get("experiment.final_phase")
                or attrs.get("experiment.phase")
            )
        # resiliency score
        if result.get("resiliency_score") is None:
            v = attrs.get("experiment.resiliency_score")
            if v is not None:
                try:
                    result["resiliency_score"] = float(v)
                except (ValueError, TypeError):
                    pass
        # fault counts
        for key in ("experiment.total_faults", "experiment.faults_passed", "experiment.faults_failed",
                    "experiment.faults_awaited", "experiment.faults_stopped", "experiment.faults_na"):
            short = key.split(".", 1)[1]  # e.g. "total_faults"
            if result.get(short) is None:
                raw = attrs.get(key)
                if raw is not None:
                    # some are plain ints, some are OTEL JSON objects like {"intValue":0}
                    if isinstance(raw, (int, float)):
                        result[short] = int(raw)
                    elif isinstance(raw, str):
                        try:
                            result[short] = int(raw)
                        except ValueError:
                            try:
                                obj = json.loads(raw)
                                result[short] = int(obj.get("intValue", 0))
                            except Exception:
                                pass
                    elif isinstance(raw, dict):
                        result[short] = int(raw.get("intValue", 0))
        # experiment.run_id (may differ from userId on agent trace)
        if not result.get("experiment_run_id"):
            result["experiment_run_id"] = attrs.get("experiment.run_id")
    return result


# ── Trace record ──────────────────────────────────────────────────────────────
def format_trace(trace: dict, obs: list, otel_attrs: dict, extra_scores: list) -> dict:
    meta = trace.get("metadata") or {}
    if isinstance(meta, str):
        try:
            meta = json.loads(meta)
        except Exception:
            meta = {}
    ts = fmt_ts(trace.get("timestamp") or trace.get("createdAt"))

    # Merge scores: trace's own scores first, then cross-linked OTEL scores
    all_scores = list(trace.get("scores") or []) + list(extra_scores or [])

    # namespace: metadata → OTEL span attrs → trace.input dict
    namespace = meta.get("namespace") or otel_attrs.get("namespace")
    if not namespace:
        inp = trace.get("input") or {}
        if isinstance(inp, dict):
            namespace = inp.get("namespace")

    return {
        "trace_id": trace.get("id"),
        "trace_name": trace.get("name"),
        "timestamp_utc": ts,
        "timestamp_ist": to_ist_str(ts),
        "latency_sec": trace.get("latency"),
        "tags": trace.get("tags") or [],
        # ── Experiment identity ──
        "experiment_id": meta.get("experiment_id") or trace.get("sessionId"),
        "experiment_run_id": (
            meta.get("experiment_run_id")
            or otel_attrs.get("experiment_run_id")
            or trace.get("userId")
        ),
        "notify_id": meta.get("notify_id") or trace.get("id"),
        "workflow_name": meta.get("workflow_name") or trace.get("name"),
        "agent_name": meta.get("agent_name"),
        "agent_platform": meta.get("agent_platform"),
        "agent_id": meta.get("agent_id"),
        # ── Infra / context ──
        "namespace": namespace,
        "phase": meta.get("phase"),
        "priority": meta.get("priority"),
        # ── OTEL-sourced experiment outcome ──
        "final_phase": otel_attrs.get("final_phase"),
        "resiliency_score": otel_attrs.get("resiliency_score"),
        "total_faults": otel_attrs.get("total_faults"),
        "faults_passed": otel_attrs.get("faults_passed"),
        "faults_failed": otel_attrs.get("faults_failed"),
        "faults_awaited": otel_attrs.get("faults_awaited"),
        "faults_stopped": otel_attrs.get("faults_stopped"),
        "faults_na": otel_attrs.get("faults_na"),
        # ── Scores (cross-linked from OTEL trace) ──
        "scores": flatten_scores(all_scores),
        # ── Observations ──
        "observations": format_observations(obs),
        "observation_count": len(obs),
    }



# ── Main ──────────────────────────────────────────────────────────────────────
def parse_args():
    p = argparse.ArgumentParser(description="Extract Langfuse traces to JSON")
    p.add_argument("--env-file", required=True, help="Path to .env file")
    p.add_argument("--from-ist", default="", help="Start IST: 'YYYY-MM-DD HH:MM'")
    p.add_argument("--limit", type=int, default=50, help="Traces per page (default 50)")
    p.add_argument("--max-pages", type=int, default=20, help="Max pages to fetch (default 20)")
    p.add_argument(
        "--output",
        default="/mnt/d/Studies/certifier/trace_dump/traces_april1.json",
        help="Output JSON file path",
    )
    p.add_argument("--no-observations", action="store_true", help="Skip fetching observations")
    return p.parse_args()


def main():
    args = parse_args()
    env = _load_env_file(args.env_file)
    _init_config(env)

    if args.from_ist:
        try:
            naive = datetime.strptime(args.from_ist, "%Y-%m-%d %H:%M")
        except ValueError:
            raise SystemExit("--from-ist must be 'YYYY-MM-DD HH:MM'")
        from_ist = naive.replace(tzinfo=IST)
    else:
        n = datetime.now(IST)
        from_ist = n.replace(hour=0, minute=0, second=0, microsecond=0)

    from_utc = from_ist.astimezone(timezone.utc)
    print(f"[INFO] From: {from_ist.strftime('%Y-%m-%d %H:%M IST')} ({from_utc.strftime('%H:%M UTC')})")

    # ── Fetch all trace pages ──────────────────────────────────────────────────
    all_traces, page = [], 1
    while page <= args.max_pages:
        print(f"  Page {page}...", end=" ", flush=True)
        resp = fetch_traces_page(from_utc, args.limit, page)
        items = resp.get("data", [])
        print(f"{len(items)} traces")
        all_traces.extend(items)
        if page >= resp.get("meta", {}).get("totalPages", 1) or not items:
            break
        page += 1
        time.sleep(0.2)
    print(f"[INFO] Total raw traces fetched: {len(all_traces)}")

    # ── Identify agent traces vs OTEL traces ──────────────────────────────────
    # Agent trace:  sessionId = experiment_id (8ffb3138-...)
    #               id has dashes (UUID format)
    # OTEL trace:   sessionId = agent trace's id  (e04bf4bd-...)
    #               id has no dashes (hex concatenated)
    # Cross-link: OTEL trace.sessionId == agent trace.id
    trace_by_id = {t["id"]: t for t in all_traces if t.get("id")}
    # Map: agent_trace_id -> otel_trace
    otel_for_agent: dict = {}
    for t in all_traces:
        sid = t.get("sessionId") or ""
        if sid and sid in trace_by_id and sid != t["id"]:
            # t is an OTEL trace; sid is the agent trace it belongs to
            otel_for_agent[sid] = t
    print(f"[INFO] Agent traces: {len(all_traces) - len(otel_for_agent)}  OTEL-linked: {len(otel_for_agent)}")

    # ── Fetch observations and format ─────────────────────────────────────────
    records = []
    for i, trace in enumerate(all_traces, 1):
        tid = trace.get("id", "?")
        # Skip OTEL traces — they're auxiliary; their data is merged into the agent trace
        is_otel = any(ot.get("id") == tid for ot in otel_for_agent.values())
        if is_otel:
            print(f"  [{i}/{len(all_traces)}] SKIP (OTEL trace) {tid[:12]}...")
            continue

        print(
            f"  [{i}/{len(all_traces)}] {trace.get('name', '?')[:40]} ({tid[:8]}...)",
            end=" ", flush=True,
        )
        if not args.no_observations:
            obs = fetch_observations(tid)
            print(f"-> {len(obs)} obs", end="")
        else:
            obs = []
            print("-> skipped", end="")

        # Cross-link OTEL trace data
        otel_trace = otel_for_agent.get(tid)
        otel_attrs: dict = {}
        extra_scores: list = []
        if otel_trace:
            otel_tid = otel_trace.get("id", "")
            print(f" | OTEL={otel_tid[:8]}...", end="")
            extra_scores = otel_trace.get("scores") or []
            if not args.no_observations:
                otel_obs = fetch_observations(otel_tid)
                otel_attrs = _extract_otel_attrs(otel_obs)
                print(f" ({len(otel_obs)} OTEL obs, ns={otel_attrs.get('namespace')}, "
                      f"score={otel_attrs.get('resiliency_score')})", end="")
        print()

        records.append(format_trace(trace, obs, otel_attrs, extra_scores))
        time.sleep(0.1)

    # ── Summary ────────────────────────────────────────────────────────────────
    print(f"\nProcessed {len(records)} experiment trace(s):")
    for r in records:
        s = r.get("scores", {})
        print(
            f"  [{r.get('timestamp_ist', '?')}] {r.get('trace_name', '?')}"
            f" | ns={r.get('namespace', '?')}"
            f" | faults={r.get('total_faults', '?')} pass={r.get('faults_passed', '?')} fail={r.get('faults_failed', '?')}"
            f" | resiliency={r.get('resiliency_score', s.get('resiliency_score', '?'))}"
            f" | obs={r.get('observation_count', 0)}"
        )

    # ── Write ──────────────────────────────────────────────────────────────────
    out = Path(args.output)
    out.parent.mkdir(parents=True, exist_ok=True)
    with open(out, "w", encoding="utf-8") as f:
        json.dump(records, f, indent=2, ensure_ascii=False)
    total_obs = sum(r.get("observation_count", 0) for r in records)
    print(f"\n[OK] {len(records)} trace(s) / {total_obs} obs -> {out}")


if __name__ == "__main__":
    main()
