# pii-redact consolidation + heuristic overhaul + agent wiring

Status: approved
Date: 2026-04-22

## Problem

Two redaction packages exist side by side:

- `pii-redact/` — standalone CLI with JSON/NDJSON/logfmt/line walkers, key/pattern policy, partial-reveal masker, validators. Module path `github.com/join-com/pii-redact` (wrong — this repo is `github.com/enekos/mairu`). Not importable from mairu (everything under `internal/`).
- `mairu/internal/redact/` — 5-layer in-process pipeline (known tokens → arg/flag heuristics → Shannon entropy → command denylist → damage cap). Used by `cmd/ingest`, `cmd/capture`, `cmd/history`, `internal/ingest/server`, `internal/history/{parser,importer}`.

The two packages disagree on what a "secret" is. The mairu agent does not redact tool output before feeding it to the model — a trivially demonstrable leak path.

## Goal

1. Consolidate to a single package: `pii-redact/` as source of truth. Delete `mairu/internal/redact`.
2. Substantially improve heuristics: bigger known-token library, entropy + arg/flag layers, free-text PII (email/phone/IBAN/CC/SSN), format-aware pre-pass (`.env`, connection strings, headers, YAML pairs).
3. Wire pii-redact into the mairu agent's bash tool output, **opt-in**.

Out of scope: web_fetch / file read+edit / memory writes / council messages redaction — each is a follow-up.

## Module structure

Rename pii-redact module to `github.com/enekos/mairu/pii-redact`. Release via Go sub-module tagging (`pii-redact/vX.Y.Z`). Mairu's `go.mod` adds a normal `require` once a tag exists; a local `replace ../pii-redact` keeps inner-loop dev ergonomic.

### Public API (`pii-redact/pkg/redact`)

```go
type Kind int
const (
    KindText Kind = iota
    KindCommand
    KindJSON
    KindLogfmt
    KindNDJSON
)

type Finding struct {
    Layer Layer
    Kind  string
    Start int
    End   int
}

type Result struct {
    Redacted      string
    Findings      []Finding
    EmbeddingSafe bool
    Dropped       bool
    Stats         map[string]int
}

type Options struct {
    Rules         *Ruleset  // optional key-based policy for JSON walker
    RevealMasker  bool
    Strict        bool
    EntropyThresh float64
    DamageCap     float64
    MinEntropyLen int
    Denylist      []string
    ExtraPatterns []Pattern
}

func New(opts Options) *Redactor
func (r *Redactor) Redact(input string, kind Kind) Result
func (r *Redactor) RedactStream(in io.Reader, out io.Writer, kind Kind) (Stats, error)
```

### Internal layout

| Path | Role |
|---|---|
| `internal/patterns/` | Content pattern set (expanded library) |
| `internal/mask/` | Partial-reveal masker + validators (Luhn/IBAN/JWT/IP) |
| `internal/pipeline/` | NEW — Layer 0-6 orchestrator, folded from `mairu/internal/redact` |
| `internal/walkers/` | JSON / NDJSON / logfmt / line (today's `redact/*.go` moved here) |
| `internal/config/` | Existing ruleset loader |
| `cmd/pii-redact/` | CLI — behavior-identical, calls public API |

## Pipeline stages

Fixed order. Each layer consumes the previous layer's output. Only Layer 1 flips `EmbeddingSafe=false`.

| # | Layer | Scope | Role |
|---|---|---|---|
| 0 | Format-aware pre-pass | text/cmd | Detect `.env`/YAML/connection-strings/HTTP headers, surgical-replace value half |
| 1 | Known-token regex | text/cmd | Provider-specific tokens; hit ⇒ `EmbeddingSafe=false` |
| 2 | Arg/flag heuristics | text/cmd | `-H "Authorization: …"`, `--token=…`, `FOO_SECRET=bar cmd`, `-u user:pass`, `-p/--password`, `-i keyfile`, Docker `-e`, `--service-account-key-file` |
| 3 | Free-text PII | text/cmd | Email, phone (E.164 + national), IBAN (mod-97), CC (Luhn), SSN, public IPv4 |
| 4 | Shannon entropy | text/cmd | Long high-entropy tail. UUID/git-SHA allowlisted |
| 5 | Command denylist | cmd only | `vault kv read`, `op read`, `gcloud auth` → collapse args |
| 6 | Damage cap | text/cmd | If redacted/total > ratio → collapse. Commands keep program name. JSON walker computes per-document |

`KindJSON` / `KindNDJSON` / `KindLogfmt` run the existing walker. Every surviving string leaf runs the full `KindText` pipeline (today's walker only does Layer-1). Damage-cap is computed per document.

## Heuristic library additions

### Known tokens (Layer 1)

Cloud (Azure storage/AD, GCP SA JSON + API key, Cloudflare, AWS session, DigitalOcean); SaaS (Datadog, PagerDuty, New Relic, Sentry DSN, Linear, Asana, Notion, Airtable); package registries (npm, PyPI, crates); messaging (Slack webhooks, Discord webhooks, Twilio, SendGrid, Mailgun); AI (OpenAI `sk-proj-`/`sk-`, Anthropic `sk-ant-`, HuggingFace `hf_`, Replicate); dev (Shopify, Figma PAT, Vercel, Netlify, Heroku, DockerHub PAT); generic PEM/PKCS#8 bodies. Each pattern gets a validator where structurally possible.

### Free-text PII (Layer 3)

- Email — domain must contain a dot + TLD
- Phone — E.164 (`+\d{7,15}`) + national common forms, length-gated to avoid `1.2.3.4.5` version strings
- IBAN — mod-97 checksum (lift `mask/validators.go`)
- Credit card — Luhn + BIN range sanity (lift existing)
- SSN (US) — `NNN-NN-NNNN` with area/group/serial sanity; reject `000-NN-*`, `*-00-*`, `*-*-0000`, `9XX-*-*` (ITIN-style still redacted)
- IPv4 — only public ranges; RFC1918 / loopback / link-local / CGNAT allowlisted

### Format-aware (Layer 0)

- `.env` / shell `export`: `(?m)^(?:export\s+)?([A-Z][A-Z0-9_]*)=(.+)$` — redact value when key matches `(?i)(token|secret|key|password|passwd|auth|credential|private)`
- YAML pair: `^\s*([a-z_][\w-]*(?:token|secret|key|password))\s*:\s*(.+)$`
- Connection strings: generic `([a-z][a-z0-9+.\-]*)://[^\s:@/]+:([^\s@/]+)@` (extends today's `uri_credentials`)
- HTTP headers: `Authorization:`, `Cookie:`, `Set-Cookie:`, `X-*-Token:`, `Proxy-Authorization:`

## Agent wiring

Surface: `mairu/internal/agent/bash.go`. Opt-in only.

Config:
- `mairu.toml`: `[agent] redact_bash_output = true` (default `false`)
- CLI: `--redact` on `mairu tui` / `mairu web`
- Env: `MAIRU_REDACT_BASH=1`

Behavior (when enabled):
1. After bash stdout+stderr are collected, run `Redactor.Redact(body, KindText)` with `RevealMasker=true`.
2. If `Dropped`, replace body with `[REDACTED:damage_cap]` + short footer noting finding count.
3. Exit code + duration + timing preserved; only the body is transformed.
4. Debug log reports finding count — never the raw pre-redact body.

Not wired: web_fetch, file read/edit, memory writes, council. Follow-ups.

## Consumer migration

One-line import swap per file in:
- `mairu/internal/cmd/{ingest,capture,history}.go`
- `mairu/internal/ingest/server.go`
- `mairu/internal/history/{parser,importer}.go`

All currently use `redact.New(...)` + `Redact(input, kind)` — same signature in the new public API. No logic changes.

`mairu/internal/redact/` is deleted after migration. Tests move to `pii-redact/internal/pipeline/fixtures_test.go` with approved goldens.

## Testing

- `testdata/fixtures/red_team.json` extended with new patterns; `TestRedTeam` hard-gates raw PII survival.
- Approved-goldens regenerate via `UPDATE_APPROVED=1 go test ./...`.
- `mairu/internal/agent/bash_test.go` gets one case: command emits a known token; asserts tool-result body sent to model is redacted.
- `BenchmarkPipelineKindText` on 4 KiB blob targets <200 µs.

## Non-goals

- No cross-process sidecar or daemon for redaction. In-process library only.
- No ML / NER model. Regex + validators + entropy only.
- No redaction of content the user has explicitly asked the agent to handle (e.g. a bash command that greps for a known string — we don't redact the user's literal query in the command itself beyond the Layer-2 flag heuristics that would already fire).
