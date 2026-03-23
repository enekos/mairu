# `contextfs` Agent Integration Instructions

Copy these instructions into your project's `CLAUDE.md`, `.cursorrules`, `AGENTS.md`, or equivalent system prompt file so agents can persist and reuse project knowledge across sessions.

---

## Persistent Memory and Context with `contextfs`

This project uses `contextfs` through the `context-cli` command to store and retrieve:
- team conventions
- architectural decisions
- debugging findings
- user preferences

Agents should proactively query `contextfs` at task start, and write back useful knowledge after meaningful progress.

**Required:** Always pass `-P, --project <project_name>` so data stays isolated per repository.

### 1) Deterministic retrieval (default)

Before implementing, debugging, or making architecture changes, search `contextfs` for prior decisions and constraints.

Use direct search commands first so retrieval scope and ranking are explicit.

```bash
# General project conventions
context-cli memory search "test lint file structure conventions" -k 8 -P <project_name>

# Architecture-specific retrieval
context-cli memory search "authentication token validation rules" -k 8 -P <project_name>
context-cli node search "authentication architecture" -k 8 -P <project_name>
context-cli node ls "contextfs://<project_name>/backend/auth" -P <project_name>
```

### 2) Natural-language storage (default)

When you learn something future sessions should know, store it immediately if you consider it's relevant for the future.

Use `vibe-mutation` first because it can create/update memories and context nodes from one instruction.

Use `-y` in non-interactive agent environments, only in case you are certain about the changes, be critical.

```bash
# Save a convention or stable decision
context-cli vibe-mutation "Remember: we use Vitest for tests and place integration tests under tests/integration." -P <project_name> -y

# Update known architecture details
context-cli vibe-mutation "Update auth context: refresh tokens are now rotated on every renewal and revoked on logout." -P <project_name> -y
```

### 3) Natural-language retrieval (optional fallback)

Use `vibe-query` only when direct memory/node retrieval is not enough (for broad or ambiguous requests that benefit from multi-step planning).

```bash
context-cli vibe-query "How does authentication work in this codebase and what are the token validation rules?" -P <project_name>
```

### 4) Precise operations

Use direct commands when you need strict control over retrieval behavior or metadata.

```bash
# Memory search with typo tolerance, phrase boost, and highlights
context-cli memory search "authentication setup" -k 5 -P <project_name> --fuzziness auto --phraseBoost 3 --highlight

# Memory search with strict filtering
context-cli memory search "jwt validation middleware" -k 10 -P <project_name> --fuzziness 0 --minScore 5

# Memory write with explicit metadata
context-cli memory store "API errors use { code, message, details } shape." -c convention -o agent -i 7 -P <project_name>
```

### 5) Context nodes (hierarchical knowledge) & AST Daemon

Use URI-addressed nodes for structured architecture docs.

```bash
# Store a node
context-cli node store "contextfs://<project_name>/backend/auth" "Auth Module" "JWT access tokens, rotating refresh tokens, and revocation list checks." -P <project_name>

# List a subtree
context-cli node ls "contextfs://<project_name>/backend" -P <project_name>
```

#### Context Rollback
If a `vibe-mutation` or manual update hallucinates an operation and corrupts an existing node, you can restore its soft-deleted state:
```bash
context-cli node restore "contextfs://<project_name>/backend/auth"
```

#### AST Codebase Ingestion Daemon
If you are working on a massive codebase and the context tree is outdated, you can boot the background AST daemon. It watches for file changes and auto-ingests class signatures and function definitions into the context nodes tree.
```bash
context-cli daemon . -P <project_name> &
```

## Agent operating rules

1. **Retrieve first:** Start substantial tasks with `memory search` and `node search` (plus `node ls` when you know the URI).
2. **Store outcomes:** After solving complex issues, save conclusions with `vibe-mutation -y`.
3. **Scope always:** Include `-P <project_name>` on every `context-cli` call.
4. **Prefer defaults:** Use direct retrieval commands first; use `vibe-query` only as fallback.
5. **Do not guess conventions:** Check `contextfs` before asking the user to repeat known decisions.
