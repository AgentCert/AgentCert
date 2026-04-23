"""
fetch_langfuse_traces.py
========================
Programmatically fetch traces from Langfuse using the REST API.
Bypasses the "cache only" limitation of the UI download button.

Usage
-----
  # Requires port-forward to be running first:
  #   kubectl port-forward -n langfuse svc/langfuse-web 3005:3000

  python fetch_langfuse_traces.py                        # fetch all, save to traces/
  python fetch_langfuse_traces.py --session <session_id> # fetch by session
  python fetch_langfuse_traces.py --trace <trace_id>     # fetch single trace with observations
  python fetch_langfuse_traces.py --gt-only              # fetch only traces with GT data
  python fetch_langfuse_traces.py --limit 100            # limit number of traces

  # Supply a .env file (reads LANGFUSE_* keys from it):
  python fetch_langfuse_traces.py --env-file local-custom/config/.env fetch-trace --trace <id>
  python fetch_langfuse_traces.py --env-file local-custom/config/.env gt-only

Config priority: --env-file > environment variables > built-in defaults
------
  LANGFUSE_HOST       - e.g. http://localhost:3005 (port-forwarded) or cluster URL
  LANGFUSE_PUBLIC_KEY - pk-lf-...
  LANGFUSE_SECRET_KEY - sk-lf-...
  LANGFUSE_PROJECT_ID - optional, filters to specific project

Note: LANGFUSE_HOST in .env is typically the cluster-internal URL.
      When running from WSL/local machine use localhost:3005 with port-forward,
      or override via:  LANGFUSE_HOST=http://localhost:3005 python fetch_langfuse_traces.py ...
"""

import argparse
import base64
import json
import os
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen


# ── .env loader ──────────────────────────────────────────────────────────────

def _load_env_file(path: str) -> dict:
    """Parse a .env file and return a dict of key=value pairs.

    - Strips surrounding quotes from values (single or double)
    - Skips blank lines and lines starting with #
    - Does NOT override keys already set in os.environ
    """
    env = {}
    try:
        with open(path, encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                key, _, value = line.partition("=")
                key = key.strip()
                value = value.strip()
                # Strip surrounding quotes
                if len(value) >= 2 and value[0] in ('"', "'") and value[-1] == value[0]:
                    value = value[1:-1]
                env[key] = value
    except FileNotFoundError:
        print(f"[env-file] File not found: {path}", file=sys.stderr)
        sys.exit(1)
    except OSError as exc:
        print(f"[env-file] Cannot read {path}: {exc}", file=sys.stderr)
        sys.exit(1)
    return env


def _apply_env_file(path: str) -> None:
    """Load .env file and inject LANGFUSE_* keys into os.environ (does not overwrite)."""
    env = _load_env_file(path)
    loaded = []
    for key in ("LANGFUSE_HOST", "LANGFUSE_PUBLIC_KEY", "LANGFUSE_SECRET_KEY",
                "LANGFUSE_PROJECT_ID", "LANGFUSE_ORG_ID"):
        if key in env and key not in os.environ:
            os.environ[key] = env[key]
            loaded.append(key)
    if loaded:
        print(f"[env-file] Loaded from {path}: {', '.join(loaded)}")
    else:
        print(f"[env-file] No new LANGFUSE_* keys loaded from {path} (all already set in env)")


# ── Config ──────────────────────────────────────────────────────────────────
# NOTE: these globals are intentionally set AFTER argument parsing calls
# _apply_env_file() so that .env values take effect before they are read.

DEFAULT_HOST       = "http://localhost:3005"
DEFAULT_PUBLIC_KEY = "pk-lf-83b869a5-10d9-4701-95f5-6d17d5809f83"
DEFAULT_SECRET_KEY = "sk-lf-65878e36-c75e-4072-8f41-4e2e94d21b07"
DEFAULT_PROJECT_ID = "cmoaan8zk0006ye07grwfio3o"


def _init_config() -> None:
    """Populate module-level config globals from environment (call after env-file load)."""
    global LANGFUSE_HOST, LANGFUSE_PUBLIC_KEY, LANGFUSE_SECRET_KEY, LANGFUSE_PROJECT_ID
    host = os.environ.get("LANGFUSE_HOST", DEFAULT_HOST).rstrip("/")
    # Auto-remap cluster-internal URL to localhost port-forward for WSL/local access
    if "svc.cluster.local" in host:
        print(f"[config] LANGFUSE_HOST is cluster-internal ({host}).")
        print("[config] Remapping to http://localhost:3005 for WSL/local access.")
        print("[config] Ensure port-forward is running: kubectl port-forward -n langfuse svc/langfuse-web 3005:3000")
        host = "http://localhost:3005"
    LANGFUSE_HOST       = host
    LANGFUSE_PUBLIC_KEY = os.environ.get("LANGFUSE_PUBLIC_KEY", DEFAULT_PUBLIC_KEY)
    LANGFUSE_SECRET_KEY = os.environ.get("LANGFUSE_SECRET_KEY", DEFAULT_SECRET_KEY)
    LANGFUSE_PROJECT_ID = os.environ.get("LANGFUSE_PROJECT_ID", DEFAULT_PROJECT_ID)
    print(f"[config] Host={LANGFUSE_HOST}  Project={LANGFUSE_PROJECT_ID}")


# Initialise with current env (may be overridden after --env-file is parsed)
LANGFUSE_HOST       = DEFAULT_HOST
LANGFUSE_PUBLIC_KEY = DEFAULT_PUBLIC_KEY
LANGFUSE_SECRET_KEY = DEFAULT_SECRET_KEY
LANGFUSE_PROJECT_ID = DEFAULT_PROJECT_ID

OUTPUT_DIR = Path("traces")


# ── HTTP helpers ─────────────────────────────────────────────────────────────

def _auth_header() -> str:
    token = base64.b64encode(f"{LANGFUSE_PUBLIC_KEY}:{LANGFUSE_SECRET_KEY}".encode()).decode()
    return f"Basic {token}"


def _get(path: str, params: dict | None = None, timeout: int = 60) -> dict:
    url = f"{LANGFUSE_HOST}{path}"
    if params:
        url = f"{url}?{urlencode({k: v for k, v in params.items() if v is not None})}"
    req = Request(url, headers={"Authorization": _auth_header(), "Accept": "application/json"})
    try:
        with urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")
        print(f"  HTTP {e.code} on {path}: {body[:200]}", file=sys.stderr)
        raise
    except URLError as e:
        print(f"  URL error on {path}: {e.reason}", file=sys.stderr)
        print("  Is port-forward running?  kubectl port-forward -n langfuse svc/langfuse-web 3005:3000", file=sys.stderr)
        raise


# ── Fetch functions ──────────────────────────────────────────────────────────

def fetch_traces(session_id: str | None = None,
                 limit: int = 50,
                 page: int = 1) -> dict:
    """Fetch trace list with optional session filter."""
    params = {
        "limit": limit,
        "page": page,
    }
    if session_id:
        params["sessionId"] = session_id
    return _get("/api/public/traces", params)


def fetch_trace_observations(trace_id: str) -> list:
    """Fetch all observations (generations + spans) for a single trace."""
    page = 1
    all_obs = []
    while True:
        resp = _get("/api/public/observations", {"traceId": trace_id, "limit": 100, "page": page})
        items = resp.get("data", [])
        all_obs.extend(items)
        meta = resp.get("meta", {})
        total_pages = meta.get("totalPages", 1)
        if page >= total_pages or not items:
            break
        page += 1
    return all_obs


def fetch_single_trace(trace_id: str) -> dict:
    """Fetch one trace by ID."""
    return _get(f"/api/public/traces/{trace_id}")


def fetch_all_traces_paginated(session_id: str | None = None,
                               limit: int = 50,
                               max_pages: int = 100) -> list:
    """Page through all traces and return combined list."""
    all_traces = []
    for page in range(1, max_pages + 1):
        print(f"  Fetching page {page}...", end=" ", flush=True)
        resp = fetch_traces(session_id=session_id, limit=limit, page=page)
        items = resp.get("data", [])
        print(f"{len(items)} traces")
        all_traces.extend(items)
        meta = resp.get("meta", {})
        total_pages = meta.get("totalPages", 1)
        if page >= total_pages or not items:
            break
        time.sleep(0.2)  # be polite to the API
    return all_traces


# ── GT detection ─────────────────────────────────────────────────────────────

def observation_has_gt(obs: dict) -> bool:
    """Check if an observation contains GT data using any identifier."""
    meta_raw = obs.get("metadata")
    if not meta_raw:
        return False
    try:
        meta = json.loads(meta_raw) if isinstance(meta_raw, str) else meta_raw
    except (json.JSONDecodeError, TypeError):
        return False

    # Top-level flag (new, clearest)
    if meta.get("is_ground_truth_data") is True:
        return True

    # Nested in requester_metadata (older traces)
    rm = meta.get("requester_metadata", {})
    if isinstance(rm, dict):
        if rm.get("is_ground_truth_data") is True:
            return True
        if rm.get("gt_metadata_present") is True:
            return True
        if rm.get("expected_output") is not None:
            return True

    return False


def extract_gt_summary(obs: dict) -> dict:
    """Extract a compact GT summary from an observation."""
    meta_raw = obs.get("metadata")
    try:
        meta = json.loads(meta_raw) if isinstance(meta_raw, str) else meta_raw
    except (json.JSONDecodeError, TypeError):
        meta = {}

    rm = meta.get("requester_metadata", meta)
    return {
        "generation_name": rm.get("generation_name"),
        "scan_id": rm.get("scan_id"),
        "fault_names": rm.get("fault_names", []),
        "expected_output_len": len(rm.get("expected_output") or ""),
        "is_ground_truth_data": rm.get("is_ground_truth_data") or meta.get("is_ground_truth_data"),
        "gt_block_type": rm.get("gt_block_type") or meta.get("gt_block_type"),
        "gt_metadata_source": rm.get("gt_metadata_source"),
    }


# ── Output helpers ────────────────────────────────────────────────────────────

def save_json(path: Path, data) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(data, f, indent=2, ensure_ascii=False)
    print(f"  Saved: {path}  ({path.stat().st_size:,} bytes)")


def timestamp_slug() -> str:
    return datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S")


# ── Main commands ─────────────────────────────────────────────────────────────

def cmd_fetch_all(args) -> None:
    """Fetch all traces and their observations, save to traces/ directory."""
    print(f"Fetching traces from {LANGFUSE_HOST}")
    print(f"  Session filter: {args.session or 'none'}")
    print(f"  Limit per page: {args.limit}")

    traces = fetch_all_traces_paginated(session_id=args.session, limit=args.limit)
    print(f"\nTotal traces fetched: {len(traces)}")

    slug = timestamp_slug()
    trace_list_path = OUTPUT_DIR / f"trace_list_{slug}.json"
    save_json(trace_list_path, traces)

    if args.with_observations:
        print("\nFetching observations for each trace...")
        combined = []
        for i, trace in enumerate(traces):
            tid = trace.get("id")
            print(f"  [{i+1}/{len(traces)}] {tid}")
            obs = fetch_trace_observations(tid)
            combined.append({"trace": trace, "observations": obs})
            time.sleep(0.1)
        save_json(OUTPUT_DIR / f"traces_with_observations_{slug}.json", combined)

    print("\nDone.")


def is_trace_active(trace: dict) -> bool:
    """Return True if the trace is still running (duration not yet set)."""
    duration = trace.get("duration")
    return duration is None or duration == 0


def wait_for_trace_completion(trace_id: str, poll_interval: int = 60) -> dict:
    """Poll trace until duration is set (workflow finished flushing to Langfuse).

    Prints status every poll_interval seconds and returns the final trace dict.
    """
    attempt = 1
    while True:
        trace = fetch_single_trace(trace_id)
        if not is_trace_active(trace):
            print(f"  Trace complete (duration={trace.get('duration'):.1f}s). Proceeding.")
            return trace
        print(f"  [attempt {attempt}] Trace still active (duration=null). "
              f"Sleeping {poll_interval}s before retry...")
        attempt += 1
        time.sleep(poll_interval)


def cmd_fetch_single(args) -> None:
    """Fetch one trace + all its observations."""
    print(f"Fetching trace: {args.trace}")
    if getattr(args, "wait", False):
        print("  --wait enabled: checking if trace is still active...")
        trace = wait_for_trace_completion(args.trace, poll_interval=getattr(args, "wait_interval", 60))
    else:
        trace = fetch_single_trace(args.trace)
        if is_trace_active(trace):
            print("  WARNING: trace duration=null — workflow may still be running.")
            print("  Tip: re-run with --wait to block until the trace is complete.")
    obs   = fetch_trace_observations(args.trace)

    print(f"  Trace name   : {trace.get('name')}")
    print(f"  Observations : {len(obs)}")
    gt_obs = [o for o in obs if observation_has_gt(o)]
    print(f"  GT blocks    : {len(gt_obs)}")
    for g in gt_obs:
        s = extract_gt_summary(g)
        print(f"    - {s.get('generation_name')}  faults={s.get('fault_names')}  expected_output_len={s.get('expected_output_len')}")

    slug = timestamp_slug()
    out = {"trace": trace, "observations": obs}
    save_json(OUTPUT_DIR / f"trace_{args.trace}_{slug}.json", out)


def cmd_gt_only(args) -> None:
    """Fetch all traces, then filter and save only those with GT observations."""
    print(f"Fetching traces from {LANGFUSE_HOST} (GT filter mode)")
    traces = fetch_all_traces_paginated(session_id=args.session, limit=args.limit)
    print(f"Total traces: {len(traces)} — now fetching observations to find GT blocks...")

    gt_results = []
    for i, trace in enumerate(traces):
        tid = trace.get("id")
        print(f"  [{i+1}/{len(traces)}] {tid}", end=" ")
        obs = fetch_trace_observations(tid)
        gt_obs = [o for o in obs if observation_has_gt(o)]
        if gt_obs:
            print(f"  GT blocks: {len(gt_obs)}")
            gt_results.append({
                "trace_id": tid,
                "trace_name": trace.get("name"),
                "gt_observation_count": len(gt_obs),
                "gt_summaries": [extract_gt_summary(o) for o in gt_obs],
                "all_observations": obs,
            })
        else:
            print("  (no GT)")
        time.sleep(0.1)

    print(f"\nTraces with GT: {len(gt_results)} / {len(traces)}")
    slug = timestamp_slug()
    save_json(OUTPUT_DIR / f"gt_traces_{slug}.json", gt_results)
    print("Done.")


# ── CLI ───────────────────────────────────────────────────────────────────────

def main() -> int:
    parser = argparse.ArgumentParser(
        description="Fetch Langfuse traces programmatically via REST API",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    # Global --env-file flag (must come BEFORE subcommand)
    parser.add_argument(
        "--env-file",
        metavar="PATH",
        default=None,
        help="Path to .env file to read LANGFUSE_* config from (e.g. local-custom/config/.env)",
    )
    sub = parser.add_subparsers(dest="command")

    # fetch-all
    p_all = sub.add_parser("fetch-all", help="Fetch all traces (default command)")
    p_all.add_argument("--session",           default=None, help="Filter by session ID")
    p_all.add_argument("--limit",  type=int,  default=50,   help="Page size (default 50)")
    p_all.add_argument("--with-observations", action="store_true",
                       help="Also fetch observations for each trace (slow for large sets)")

    # fetch-trace
    p_trace = sub.add_parser("fetch-trace", help="Fetch single trace + all observations")
    p_trace.add_argument("--trace", required=True, help="Trace ID")
    p_trace.add_argument("--wait", action="store_true",
                        help="Poll until trace is complete (duration non-null) before downloading")
    p_trace.add_argument("--wait-interval", type=int, default=60, metavar="SECONDS",
                        help="Seconds between polls when --wait is used (default 60)")

    # gt-only
    p_gt = sub.add_parser("gt-only", help="Fetch only traces containing GT observations")
    p_gt.add_argument("--session",          default=None, help="Filter by session ID")
    p_gt.add_argument("--limit", type=int,  default=50,   help="Page size (default 50)")

    args = parser.parse_args()

    # Apply .env file BEFORE reading config globals
    if args.env_file:
        _apply_env_file(args.env_file)

    # Initialise config from environment (after potential .env injection)
    _init_config()

    if args.command == "fetch-trace":
        cmd_fetch_single(args)
    elif args.command == "gt-only":
        cmd_gt_only(args)
    else:
        # Default: fetch-all (also triggers with no subcommand)
        if not hasattr(args, "session"):
            args.session = None
            args.limit = 50
            args.with_observations = False
        cmd_fetch_all(args)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
