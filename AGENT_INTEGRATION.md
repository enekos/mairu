# `mairu` Agent Integration Instructions

Copy these instructions into your project's `CLAUDE.md`, `.cursorrules`, `AGENTS.md`, or equivalent system prompt file so agents can persist and reuse project knowledge across sessions.

---

## Persistent Memory and Context with `mairu`

This project uses the Mairu context engine through `mairu` to store and retrieve:
- team conventions
- architectural decisions
- debugging findings
- user preferences

Agents should proactively query the context engine at task start, and write back useful knowledge after meaningful progress.

**Required:** Always pass `-P, --project <project_name>` so data stays isolated per repository.

### 1) Deterministic retrieval (default)

Before implementing, debugging, or making architecture changes, search stored context for prior decisions and constraints.

Use direct search commands first so retrieval scope and ranking are explicit.

```bash
# General project conventions
mairu memory search "test lint file structure conventions" -k 8 -P <project_name>

# Architecture-specific retrieval
mairu memory search "authentication token validation rules" -k 8 -P <project_name>
mairu node search "authentication architecture" -k 8 -P <project_name>
mairu node ls "contextfs://<project_name>/backend/auth" -P <project_name>
```

### 2) Natural-language storage (default)

When you learn something future sessions should know, store it immediately if you consider it's relevant for the future.

Use `vibe mutation` first because it can create/update memories and context nodes from one instruction.

Use `-y` in non-interactive agent environments, only in case you are certain about the changes, be critical.

```bash
# Save a convention or stable decision
mairu vibe mutation "Remember: we use Vitest for tests and place integration tests under tests/integration." -P <project_name> -y

# Update known architecture details
mairu vibe mutation "Update auth context: refresh tokens are now rotated on every renewal and revoked on logout." -P <project_name> -y
```

### 3) Natural-language retrieval (optional fallback)

Use `vibe query` only when direct memory/node retrieval is not enough (for broad or ambiguous requests that benefit from multi-step planning).

```bash
mairu vibe query "How does authentication work in this codebase and what are the token validation rules?" -P <project_name>
```

### 4) Precise operations

Use direct commands when you need strict control over retrieval behavior or metadata.

```bash
# Memory search with typo tolerance, phrase boost, and highlights
mairu memory search "authentication setup" -k 5 -P <project_name> --fuzziness auto --phraseBoost 3 --highlight

# Memory search with strict filtering
mairu memory search "jwt validation middleware" -k 10 -P <project_name> --fuzziness 0 --minScore 5

# Memory write with explicit metadata
mairu memory store "API errors use { code, message, details } shape." -c convention -o agent -i 7 -P <project_name>
```

### 5) Context nodes (hierarchical knowledge) & AST Daemon

Use URI-addressed nodes for structured architecture docs.

```bash
# Store a node
mairu node store "contextfs://<project_name>/backend/auth" "Auth Module" "JWT access tokens, rotating refresh tokens, and revocation list checks." -P <project_name>

# List a subtree
mairu node ls "contextfs://<project_name>/backend" -P <project_name>
```

#### Context Rollback
If a `vibe mutation` or manual update hallucinates an operation and corrupts an existing node, you can restore its soft-deleted state:
```bash
mairu node restore "contextfs://<project_name>/backend/auth"
```

#### AST Codebase Ingestion Daemon
If you are working on a massive codebase and the context tree is outdated, you can boot the background AST daemon. It watches for file changes and auto-ingests class signatures and function definitions into the context nodes tree.
```bash
mairu daemon . -P <project_name> &
```

## Agent operating rules

1. **Retrieve first:** Start substantial tasks with `memory search` and `node search` (plus `node ls` when you know the URI).
2. **Store outcomes:** After solving complex issues, save conclusions with `vibe-mutation -y`.
3. **Scope always:** Include `-P <project_name>` on every `mairu` call.
4. **Prefer defaults:** Use direct retrieval commands first; use `vibe-query` only as fallback.
5. **Do not guess conventions:** Check `contextfs` before asking the user to repeat known decisions.

### 6) Bash History Auto-Logging
When using the mairu agent (TUI, Web, or headless), all bash commands executed through the agent are **automatically stored** in searchable history. This includes command, exit code, duration, and output. Query this history with `mairu history search` - no manual submission needed.

### 7) Code Analysis (`mairu analyze`)
- `mairu analyze diff` -> Analyzes blast radius of current git changes.
- `mairu analyze graph` -> Analyzes the codebase graph to build project understanding.

### 8) Web Scraping (`mairu scrape`)
Agents can extract documentation or read web sources using LLM-powered scrapers:
- `mairu scrape web <url>` -> Fetch, summarize, and store as context node.
- `mairu scrape smart <url> --prompt "..."` -> Extract structured data via LLM.
- `mairu scrape search <query>` -> Search the web and extract structured data.
- `mairu scrape multi <url1> <url2>` -> Scrape multiple URLs concurrently.
- `mairu scrape depth <url> -d 2` -> Crawl up to depth 2 and extract.
- `mairu scrape omni <urls...>` -> Scrape and merge results into a single summary.
- `mairu scrape script <url>` -> Auto-generates a Go `goquery` scraper script for a given URL.

### 9) Bash Command History (`mairu history`)
Agents can query the developer's bash history to understand previous commands, outputs, and workflows:
- `mairu history search "test fail"` -> Semantically search past bash commands and their outputs.
- `mairu history stats` -> Show the most frequently run bash commands.
- `mairu history feedback <id> -r 10` -> Apply reinforcement learning feedback to a command execution.

**Note:** When using the mairu agent (via `mairu tui`, `mairu web`, or headless mode), all bash commands executed by the agent are **automatically stored** in mairu history. This happens transparently through the `HistoryLogger` interface - no manual submission is needed. The stored data includes the command, exit code, duration, and output, making it fully searchable for future sessions.

#### Manual Bash Command Logging (when not using mairu agent)
If you are running bash commands outside the mairu agent context (e.g., as a standalone Kimi agent without mairu integration), **you must manually store important commands** to preserve project knowledge:

```bash
# After running a complex or important command, store it as a memory
mairu memory store "Command: go test ./... -race | Result: All tests passed | Purpose: Verified race condition fix" -c command -o agent -i 6 -P <project_name>

# Store a debugging command that solved an issue
mairu memory store "Debug: Used 'lsof -i :8080' to find process blocking port. Kill with 'kill -9 <pid>'" -c debugging -o agent -i 8 -P <project_name>

# Store build/test commands that work
mairu memory store "Build command: make build-docker TAG=latest | Use this for local testing" -c build -o agent -i 5 -P <project_name>
```

**When to manually log:**
- Complex multi-step commands that took time to figure out
- Debugging commands that resolved issues
- Build/test commands with specific flags that work
- Any command you or future agents might want to reuse

Use a descriptive format: `Command: <cmd> | Result: <output/summary> | Purpose: <why>`

### AI-Optimized GNU Tools
Agents are encouraged to use the `mairu` binary for token-dense, strictly parsable exploration:
- `mairu map [dir] -d 2` -> Fast, `.gitignore` aware, token-counted directory tree
- `mairu outline <file>` -> Emits imports and logic symbols (classes, functions) via AST
- `mairu peek <file> -s <symbol>` -> Smart, bracket-aware symbol extraction
- `mairu scan <regex> [dir] -C 1 -e .go -H -n 5` -> Token-budgeted regex search
- `mairu info [dir]` -> Repository analytics
