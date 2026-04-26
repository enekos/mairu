import json, sys

try:
    data = json.load(open("/tmp/response.json"))
    text = data["choices"][0]["message"]["content"]
    print(text)
except Exception as e:
    print(f"Error parsing response: {e}")
    print(open("/tmp/response.json").read()[:1000])
