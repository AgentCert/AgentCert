import json, sys

HEX_TRACE = "/mnt/c/Users/sanjsingh/Downloads/trace-ac3ccadbea4d4ee1ae39f5bb6cf2f0aa.json"

with open(HEX_TRACE, encoding="utf-8") as f:
    data = json.load(f)

# Flatten OTLP resourceSpans -> scopeSpans -> spans
all_spans = []
resource_spans = data if isinstance(data, list) else data.get("resourceSpans", [])
for rs in resource_spans:
    for ss in rs.get("scopeSpans", []):
        all_spans.extend(ss.get("spans", []))

if not all_spans:
    # Maybe it's already a flat list of spans
    all_spans = data if isinstance(data, list) else []

print(f"Total spans: {len(all_spans)}\n")

def nanos_to_ts(ns):
    from datetime import datetime, timezone
    if not ns:
        return "?"
    return datetime.fromtimestamp(int(ns) / 1e9, tz=timezone.utc).strftime("%H:%M:%S")

for s in all_spans:
    name = s.get("name", "?")
    start = nanos_to_ts(s.get("startTimeUnixNano", ""))
    end   = nanos_to_ts(s.get("endTimeUnixNano", ""))
    attrs = {a["key"]: list(a.get("value", {}).values())[0] if a.get("value") else ""
             for a in s.get("attributes", [])}
    # Show key attrs
    interesting = {k: v for k, v in attrs.items()
                   if any(x in k for x in ["phase", "verdict", "target", "started", "finished",
                                            "resiliency", "fault", "status", "kind", "name"])}
    print(f"  [{start} → {end}]  {name}")
    for k, v in interesting.items():
        print(f"      {k} = {str(v)[:80]}")
