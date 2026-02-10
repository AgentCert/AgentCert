import json
import urllib.request

query = {"query": "{ __type(name: \"DeployAgentWithHelmRequest\") { inputFields { name } } }"}

req = urllib.request.Request(
    "http://localhost:8080/query",
    data=json.dumps(query).encode("utf-8"),
    headers={"Content-Type": "application/json"}
)

try:
    with urllib.request.urlopen(req) as response:
        result = json.loads(response.read().decode("utf-8"))
        if result.get("data") and result["data"].get("__type"):
            fields = result["data"]["__type"]["inputFields"]
            field_names = [f["name"] for f in fields]
            print("Input fields:", ", ".join(field_names))
            print()
            if "chartData" in field_names:
                print("✅ SUCCESS: chartData field FOUND in schema!")
            else:
                print("❌ FAILED: chartData field NOT found in schema")
        else:
            print("Error:", result)
except Exception as e:
    print(f"Error: {e}")
