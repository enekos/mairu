# pii-redact

A generic PII redaction CLI for log streams. Reads log data on stdin, emits
redacted output on stdout, fails closed on errors.

Designed to sit between `gcloud logging read` (or any other log source)
and anywhere log contents might be persisted or shared — agent context
windows, tickets, screenshots, memory files.

## Install

### One-liner (curl)

```bash
curl -sSL https://raw.githubusercontent.com/enekos/mairu/master/pii-redact/install.sh | sh
```

Customize the install directory:

```bash
curl -sSL https://raw.githubusercontent.com/enekos/mairu/master/pii-redact/install.sh | INSTALL_DIR=~/.local/bin sh
```

Or install a specific release:

```bash
curl -sSL https://raw.githubusercontent.com/enekos/mairu/master/pii-redact/install.sh | VERSION=v0.1.0 sh
```

### Go install

```bash
go install github.com/enekos/mairu/pii-redact/cmd/pii-redact@latest
```

### Build locally

```bash
make build      # produces bin/pii-redact
```

## Quick start

With the bundled GCP profile:

```bash
gcloud logging read '...' --format=json | pii-redact --profile gcp-logging
```

With a custom config directory:

```bash
gcloud logging read '...' --format=json \
  | pii-redact --config-dir "$PII_REDACT_CONFIG_DIR" --mode json
```

## How it works

Four redaction modes; auto-detect picks between json and line:

| Stdin starts with | Mode | Behavior |
|---|---|---|
| `[` or `{` | `json` | Structured tree walk: per-field allowlist/denylist + content regex in string values |
| Anything else | `line` | Regex-only pass over each line (used for `--format="value(...)"`) |
| explicit | `ndjson` | One JSON object per line — preserves NDJSON framing |
| explicit | `logfmt` | Parses `key=value key="quoted val"` pairs; redact_keys honored per key, content regex on all values |

### Masking style

By default matches are **partially revealed** so log entries remain
distinguishable while raw PII never survives: e.g. `john@acme.io` →
`j***n@a***.io`, `10.0.0.5` → `10.0.*.*`, `4111 1111 1111 1111` →
`************1111`, `DE89370400440532013000` → `DE89**************3000`.
Unknown/unmappable values fall back to `X***Y` (first+last char) so you
still get length + locality but not content.

Pass `--opaque` to disable reveal — every match becomes
`[REDACTED:<name>]`, every redact_keys value becomes `[REDACTED:KEY]`.

In JSON mode the precedence for each field is:

1. **Key in `redact_keys`** → replace the value with `"[REDACTED:KEY]"`.
2. **Key in `id_keys`** (exact or `*<suffix>` glob) → primitive passes
   through **without content-regex masking**; container recurses.
3. **Key in `safe_keys`** → primitive passes, container recurses.
4. **Unknown key** in `--strict` (default) → value replaced with
   `"[REDACTED:UNKNOWN_KEY]"`. In `--permissive` mode the value passes
   through.
5. Every string that survives (1)–(4) **and is not under `id_keys`** runs
   through the content regex list (`email`, `ipv4`, `jwt`, …) — matches
   become `[REDACTED:<pattern>]`.
6. Strings longer than `max_safe_string_length` on safe/id keys are
   truncated.

`id_keys` exists so foreign-key columns and primary-key UUIDs piped from a
DB query survive intact. Without it, `*Id`-suffixed columns get redacted
as `UNKNOWN_KEY` (strict mode) and any UUID primitive — even on a safe
key — gets masked by the `uuid` content pattern. Precedence: `redact_keys`
> `id_keys` > `safe_keys`, so a sensitive key in both `redact_keys` and an
`*Id` glob still gets redacted.

Line mode only runs step 5. That is the known weakness of cheap discovery
queries — prefer `--format=json` for anything that might land in a shared
context.

## CLI

```
pii-redact [--mode auto|json|line|ndjson]
           [--config-dir <path>] [--config <file>]...
           [--profile <name>]
           [--service-field <json-path>]
           [--strict|--permissive]
           [--reveal|--opaque]
           [--keep-ids]
           [--stats] [--quiet]
           < stdin > stdout
```

| Flag | Behavior |
|---|---|
| `--mode auto` (default) | `[`/`{` → `json`; otherwise `line`. Auto never picks `ndjson` |
| `--mode ndjson` | One JSON object per line, explicit only |
| `--config-dir` | Directory with `global.json` + `services/*.json`. Repeatable, later wins |
| `--config` | Single override file. Repeatable, later wins |
| `--profile` | Embedded preset (e.g. `gcp-logging`) |
| `--service-field` | JSON path used to pick per-service overrides per entry |
| `--strict` (default) | Unknown keys redacted |
| `--permissive` | Unknown keys pass through; content regex still runs |
| `--stats` | Emit redaction histogram to stderr before exit |
| `--quiet` | Suppress non-fatal warnings |
| `--reveal` (default) | Partial-reveal masking (keeps tails/prefixes per pattern) |
| `--opaque` | Disable partial-reveal — render every match as `[REDACTED:<name>]` |
| `--keep-ids` | Skip ID redaction: drop the `uuid` content pattern and broaden `id_keys` to `id`, `*Id`, `*_id`, `*Uuid`, `*_uuid`, `*UUID`, `*_UUID` plus tracing IDs. Use when piping DB query output where foreign-key columns must survive intact |

### Exit codes

- `0` success
- `1` config load error
- `2` JSON parse error in `--mode json`
- `3` I/O error

Non-zero always means no partial output was written.

## Config shape

`global.json`:

```json
{
  "safe_keys": ["timestamp", "status", "message"],
  "id_keys": ["id", "*Id", "*_id", "*Uuid", "*_uuid"],
  "redact_keys": ["email", "firstName", "iban"],
  "content_patterns": {
    "email": "[\\w.+-]+@[\\w-]+\\.[\\w.-]+"
  },
  "max_safe_string_length": 2000,
  "service_field": "resource.labels.container_name"
}
```

`services/ats.json`:

```json
{
  "safe_keys": ["applicationStatus"],
  "id_keys": ["scorecardRef"],
  "redact_keys": ["candidateNotes"]
}
```

Service overrides are **additive**: they can extend the global allow/deny/id
lists but cannot remove entries from them.

### `id_keys` syntax

Each entry is either:
- an **exact key name** (`"userId"`, `"insertId"`), or
- a **suffix glob** of the form `"*<suffix>"` (`"*Id"` matches
  `candidateId`/`postingId`; `"*_id"` matches `candidate_id`).

Only leading `*` is supported. `*` in the middle or end is treated as a
literal character.

## Bundled profiles

| Name | What it sets |
|---|---|
| `gcp-logging` | `service_field = resource.labels.container_name`, sensible default content patterns, `max_safe_string_length = 2000` |

Without `--profile` and without `--config-dir`/`--config`, the tool loads
only the embedded default content patterns and has an empty allowlist.
In strict mode that redacts everything, which is safe but unhelpful —
supply a profile or a config.

## Limitations

- Names and free-text PII that does not match a content pattern will leak
  through free-text strings. This is defense-in-depth, not a guarantee.
- Content regex has known false positives (e.g. a version string
  `1.2.3.4` is still redacted as `ipv4`). Validators reject obvious
  cases (`01.02.03.04`, `0.0.0.0`, non-Luhn card numbers, non-base64
  JWT headers, invalid IBAN checksums) but some ambiguity is inherent
  to regex. Use `--permissive` for local debugging, never in automated
  pipelines that feed PII-sensitive contexts.
- Service overrides only apply when `--service-field` resolves to a
  non-empty string in a given entry.
- Reveal mode intentionally leaks ~2 bits per redacted value (first and
  last character for generic fallbacks, pattern-specific tails for known
  types). If even that is too much, pass `--opaque`.

## Testing

```bash
make test                        # full suite
go test -run RedTeam ./...       # CI hard-gate: no raw PII survives on
                                 # the red-team fixture
```

The red-team fixture (`testdata/fixtures/red_team.json`) contains every
PII shape we know about at multiple nesting levels and is append-only.

### Approved-output tests

Exact output is pinned to golden files in `testdata/approved/`. Any
intentional change to redaction behavior shows up as a reviewable diff on
these files. Regenerate after a deliberate change:

```bash
UPDATE_APPROVED=1 go test ./internal/walkers/ -run Approved
```

`TestApproved_FixtureCoverage` asserts every input fixture has a matching
approved file — new fixtures fail CI until their approved companion is
generated and reviewed.
