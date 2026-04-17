# llmeval

A fast, lightweight, and concurrent CLI tool designed to evaluate Large Language Model (LLM) performance, specifically tailored for Kimi (Moonshot AI) APIs but adaptable. It evaluates models across an array of structured and unstructured scenarios.

## Features

- **Kimi Native:** Directly uses the Kimi (Moonshot AI) OpenAI-compatible REST API.
- **Concurrent Evaluations:** Tests are run in parallel, maximizing throughput.
- **Multiple Evaluator Types:** Check for exact match, regex match, JSON schema validity, JSON path assertions, or delegate to an LLM Judge (`kimi-k2-0711-preview` by default).
- **Prompt Templating:** Use parameterized templates to cleanly decouple data from instruction prompts.
- **Cost & Telemetry:** Approximates costs based on usage metadata directly extracted from API calls.

## Setup

First, ensure you have Go 1.20+ installed.

```bash
# Clone the repository
cd llmeval

# Install dependencies and build the binary
go mod tidy
go build -o llmeval ./cmd/llmeval
```

Set your Kimi API Key:

```bash
export KIMI_API_KEY="your-api-key-here"
```

## Running Evaluations

You can run an evaluation against a JSON dataset:

```bash
./llmeval --dataset sample_dataset.json --model kimi-k2-0711-preview --concurrency 4
```

### CLI Flags

- `--dataset`: **(Required)** Path to the JSON dataset file containing the test cases.
- `--model`: Model to evaluate (default: `kimi-k2-0711-preview`).
- `--judge-model`: Model to use for `llm_judge` evaluations (default: `kimi-k2-0711-preview`).
- `--concurrency`: Number of parallel evaluations (default: `4`).
- `--base-url`: Base URL for the Kimi API (optional).
- `--api-key`: Your Kimi API Key (defaults to `KIMI_API_KEY` env var).
- `--format`: Output format, accepts `text`, `json`, or `csv` (default: `text`).
- `--output-file`: File to write results to (defaults to stdout if empty).

## Dataset Schema

The dataset should be an array of JSON objects following this structure:

```json
[
  {
    "id": "test_id",
    "system_prompt": "Optional system instruction.",
    "prompt": "The prompt to send to the LLM. You can use {{vars}}.",
    "template_vars": {
      "vars": "This string will replace {{vars}}"
    },
    "expected": "The expected output, regex, or JSON schema depending on eval_type",
    "eval_type": "exact|includes|regex|json_path|valid_json|json_schema|llm_judge"
  }
]
```

## Evaluator Types

- `exact`: Checks if the actual output exactly matches the expected output (case-insensitive, whitespace trimmed).
- `includes`: Checks if the expected output is a substring of the actual output (case-insensitive).
- `regex`: Evaluates whether the actual output matches the provided Regular Expression in `expected`.
- `valid_json`: Ensures the output is valid JSON (attempts to strip Markdown fences).
- `json_schema`: Validates the output JSON against a strict JSON Schema provided in `expected`.
- `json_path`: Checks a specific key in JSON output. `expected` must be formatted as `path=value` (e.g., `user.name=Alice`).
- `llm_judge`: Asks the `judge-model` to determine if the ACTUAL output satisfies the EXPECTED output's requirements for the given prompt.

## Extending `llmeval`

To add a new evaluator, modify `pkg/evaluator/evaluator.go`. Add a new constant to `EvalType`, a case statement in `Evaluate()`, and the evaluation logic function returning an `EvalResult`.
