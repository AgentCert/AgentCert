import requests
import json

# API endpoint
url = "https://agentcert.openai.azure.com/openai/deployments/gpt-4.1-mini/chat/completions?api-version=2025-01-01-preview"

# Headers
headers = {
    "Content-Type": "application/json",
    "api-key": "7Sny6BOCWU9JBjOgaEpsPsa4oedY7h5ZqQsgnFSMQhsjRbu8qoM7JQQJ99CAACYeBjFXJ3w3AAABACOGb8ht"
}

# Request payload
data = {
    "messages": [
        {
            "role": "system",
            "content": "You are a helpful assistant."
        },
        {
            "role": "user",
            "content": "Hello!"
        }
    ]
}

# Make the API call
response = requests.post(url, headers=headers, json=data)

# Print the response
print("Status Code:", response.status_code)
print("\nResponse:")
print(json.dumps(response.json(), indent=2))
