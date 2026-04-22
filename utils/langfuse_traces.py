#!/usr/bin/env python3
"""Fetch all Langfuse traces after a given IST time and show their full structure.

Usage:
    python3 fetch_otel_traces2.py
    python3 fetch_otel_traces2.py --from-ist "2026-03-26 12:47"
    python3 fetch_otel_traces2.py --no-observations --output traces.json
"""

from __future__ import annotations

import argparse
import json
from datetime import datetime, timezone, timedelta
from typing import Any, Dict, List

from langfuse import Langfuse

LANGFUSE_PUBLIC_KEY = "pk-lf-78b3d210-9695-41b3-8d29-d0b1ddd1ff4d"
LANGFUSE_SECRET_KEY = "sk-lf-ad2112b2-6421-4ac1-abea-51d3dda54902"
LANGFUSE_BASE_URL = "http://100.78.130.20:3001"

IST = timezone(timedelta(hours=5, minutes=30))


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Fetch Langfuse traces after a given IST time")
    parser.add_argument("--base-url", default=LANGFUSE_BASE_URL)
    parser.add_argument("--public-key", default=LANGFUSE_PUBLIC_KEY)
    parser.add_argument("--secret-key", default=LANGFUSE_SECRET_KEY)
    parser.add_argument(
        "--from-ist", default="",
        help="Start time in IST, format: 'YYYY-MM-DD HH:MM'  e.g. '2026-03-26 12:47'"
    )
    parser.add_argument("--page-size", type=int, default=100)
    parser.add_argument("--max-pages", type=int, default=20)
    parser.add_argument("--output", default="traces.json")
    parser.add_argument("--no-observations", action="store_true",
                        help="Skip fetching per-trace spans (faster)")
    return parser.parse_args()


def to_ist(dt: datetime) -> datetime:
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(IST)


def fmt_ts(dt: Any) -> str | None:
    """Format a datetime (or ISO string) to UTC Z-suffix ISO string."""
    if dt is None:
        return None
    if isinstance(dt, str):
        try:
            dt = datetime.fromisoformat(dt.replace("Z", "+00:00"))
        except ValueError:
            return dt
    dt = dt.astimezone(timezone.utc)
    return dt.strftime("%Y-%m-%dT%H:%M:%S.") + f"{dt.microsecond // 1000:03d}Z"


def to_json_str(val: Any) -> str | None:
    """Ensure a value is a JSON string (stringify dicts/lists, pass strings through)."""
    if val is None:
        return None
    if isinstance(val, str):
        return val
    return json.dumps(val, ensure_ascii=False)


def compute_depths(observations: List[Dict[str, Any]]) -> Dict[str, int]:
    """Compute depth for each observation based on parent_observation_id."""
    parent_map = {o["id"]: o.get("parent_observation_id") for o in observations}
    depth_cache: Dict[str, int] = {}

    def get_depth(obs_id: str) -> int:
        if obs_id in depth_cache:
            return depth_cache[obs_id]
        parent_id = parent_map.get(obs_id)
        depth_cache[obs_id] = 0 if not parent_id or parent_id not in parent_map else get_depth(parent_id) + 1
        return depth_cache[obs_id]

    return {obs_id: get_depth(obs_id) for obs_id in parent_map}


def format_observations(observations: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    """Convert SDK observation dicts to the dummy.txt format."""
    depth_map = compute_depths(observations)
    result = []
    for o in observations:
        result.append({
            "id": o.get("id"),
            "type": o.get("type"),
            "name": o.get("name"),
            "startTime": fmt_ts(o.get("start_time")),
            "endTime": fmt_ts(o.get("end_time")),
            "depth": depth_map.get(o.get("id", ""), 0),
            "input": to_json_str(o.get("input")),
            "output": to_json_str(o.get("output")),
            "metadata": to_json_str(o.get("metadata")),
        })
    result.sort(key=lambda x: (x["depth"], x["startTime"] or ""))
    return result


def fetch_all_traces(
    client: Langfuse,
    from_utc: datetime,
    page_size: int,
    max_pages: int,
    include_observations: bool,
) -> List[Dict[str, Any]]:
    results = []

    for page in range(1, max_pages + 1):
        response = client.api.trace.list(
            from_timestamp=from_utc,
            page=page,
            limit=page_size,
        )
        traces = response.data
        if not traces:
            break

        for trace in traces:
            trace_dict = trace.model_dump()
            if include_observations:
                obs_response = client.api.legacy.observations_v1.get_many(trace_id=trace.id, limit=100)
                trace_dict["_observations"] = [o.model_dump() for o in obs_response.data]
            results.append(trace_dict)

        if page >= response.meta.total_pages:
            break

    return results


def print_summary(traces: List[Dict[str, Any]]) -> None:
    print(f"\nFound {len(traces)} trace(s)\n")
    for t in traces:
        ts_raw = t.get("timestamp") or t.get("createdAt") or t.get("startTime") or t.get("updatedAt")
        if isinstance(ts_raw, datetime):
            ts_ist = to_ist(ts_raw)
        elif isinstance(ts_raw, str):
            try:
                ts_ist = to_ist(datetime.fromisoformat(ts_raw.replace("Z", "+00:00")))
            except ValueError:
                ts_ist = None
        else:
            ts_ist = None
        ts_str = ts_ist.strftime("%Y-%m-%d %H:%M:%S IST") if ts_ist else "n/a"
        name = t.get("name", "(unnamed)")
        tid = t.get("id", "")
        metadata = t.get("metadata") or {}
        attrs = metadata.get("attributes") or {} if isinstance(metadata, dict) else {}
        model = (
            t.get("model")
            or attrs.get("gen_ai.request.model")
            or attrs.get("llm.model_name")
            or "n/a"
        )
        source = "otel" if attrs else "sdk"
        obs = t.get("_observations", [])
        print(f"  [{ts_str}] {name} | model={model} | source={source} | id={tid[:8]}... | spans={len(obs)}")
        print(f"    trace keys : {[k for k in t if not k.startswith('_')]}")
        if obs:
            print(f"    span keys  : {[k for k in obs[0] if not k.startswith('_')]}")
        if attrs:
            print(f"    otel attrs : {list(attrs.keys())[:10]}")
        print()


def main() -> None:
    args = parse_args()

    if args.from_ist:
        try:
            naive = datetime.strptime(args.from_ist, "%Y-%m-%d %H:%M")
        except ValueError:
            raise SystemExit("--from-ist format must be 'YYYY-MM-DD HH:MM'  e.g. '2026-03-26 12:47'")
        from_ist_dt = naive.replace(tzinfo=IST)
    else:
        now_ist = datetime.now(IST)
        from_ist_dt = now_ist.replace(hour=12, minute=47, second=0, microsecond=0)

    from_utc = from_ist_dt.astimezone(timezone.utc)
    print(f"Fetching traces from: {from_ist_dt.strftime('%Y-%m-%d %H:%M IST')}  ({from_utc.strftime('%H:%M UTC')})")

    client = Langfuse(
        public_key=args.public_key,
        secret_key=args.secret_key,
        host=args.base_url,
    )

    traces = fetch_all_traces(
        client=client,
        from_utc=from_utc,
        page_size=args.page_size,
        max_pages=args.max_pages,
        include_observations=not args.no_observations,
    )

    print_summary(traces)

    all_observations: List[Dict[str, Any]] = []
    for t in traces:
        all_observations.extend(format_observations(t.get("_observations", [])))

    with open(args.output, "w", encoding="utf-8") as f:
        json.dump(all_observations, f, indent=2)
    print(f"Saved {len(all_observations)} observations across {len(traces)} traces to: {args.output}")


if __name__ == "__main__":
    main()
