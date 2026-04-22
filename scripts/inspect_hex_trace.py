import json, sys

HEX_TRACE = "/mnt/c/Users/sanjsingh/Downloads/trace-76d98242445d44b68035fcd99fc45765.json"

with open(HEX_TRACE) as f:
    data = json.load(f)

print("Top-level keys:", list(data.keys()) if isinstance(data, dict) else f"list[{len(data)}]")
print()

# Collect all spans from any OTEL format
spans = []

def collect_spans(obj):
    if isinstance(obj, dict):
        # OTEL resourceSpans format
        if "scopeSpans" in obj:
            for ss in obj.get("scopeSpans", []):
                for s in ss.get("spans", []):
                    spans.append(s)
        elif "spans" in obj and isinstance(obj["spans"], list):
            for s in obj["spans"]:
                if isinstance(s, dict) and "name" in s:
                    spans.append(s)
        # Langfuse trace format
        elif "name" in obj and ("startTime" in obj or "start_time" in obj):
            spans.append(obj)
        for v in obj.values():
            if isinstance(v, (dict, list)):
                collect_spans(v)
    elif isinstance(obj, list):
        for item in obj:
            collect_spans(item)

collect_spans(data)

print(f"Total spans found: {len(spans)}")
print()
print("All span names:")
for s in spans:
    name = s.get("name", "?")
    attrs = {}
    for a in s.get("attributes", []):
        attrs[a["key"]] = a.get("value", {})
    start = s.get("startTimeUnixNano") or s.get("startTime") or s.get("start_time", "")
    print(f"  {name}")
    for k, v in attrs.items():
        print(f"      {k} = {v}")
