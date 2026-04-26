import json, sys

with open("/tmp/prompt.txt", "r") as f:
    prompt = f.read()

payload = {
    "model": "kimi-latest",
    "messages": [
        {"role": "user", "content": prompt}
    ],
    "temperature": 0.2,
    "max_tokens": 8192,
    "top_p": 0.95
}

with open("/tmp/payload.json", "w") as f:
    json.dump(payload, f)
