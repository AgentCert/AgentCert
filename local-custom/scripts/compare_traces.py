import json, sys

ref = json.load(open('/mnt/c/Users/sanjsingh/Downloads/traces_sorted.json'))
print(f"=== REFERENCE traces_sorted.json: {len(ref)} traces ===")
for t in ref:
    obs = t.get('_observations', [])
    types = {}
    for o in obs:
        k = o.get('type', '?')
        types[k] = types.get(k, 0) + 1
    print(f"  id={t.get('id','')[:36]}  name={t.get('name','')}  userId={t.get('userId','')}  sessionId={t.get('sessionId','')}  obs={len(obs)} {types}")

print()

# Current run (latest hex trace)
cur_hex = json.load(open('/mnt/c/Users/sanjsingh/Downloads/trace-c5fe568d47ce4388893f847154442acd.json'))
cur_uuid = json.load(open('/mnt/c/Users/sanjsingh/Downloads/trace-c5fe568d-47ce-4388-893f-847154442acd.json'))
print(f"=== CURRENT hex trace (OTEL spans): {len(cur_hex)} obs ===")
for o in cur_hex:
    print(f"  depth={o.get('depth')}  type={o.get('type')}  name={o.get('name')}")
print()
print(f"=== CURRENT uuid trace (LLM gens): {len(cur_uuid)} obs ===")
types2 = {}
for o in cur_uuid:
    k = o.get('type','?')
    types2[k] = types2.get(k,0)+1
print(f"  types: {types2}")
names = set(o.get('name','').split(' (')[0] for o in cur_uuid)
print(f"  unique gen names: {sorted(names)}")
