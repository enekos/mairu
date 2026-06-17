# 🏛️ Review Council

A council-based PR review system that uses multiple AI personas to review pull requests from different perspectives, then aggregates all findings into a single PR comment.

## How It Works

When a PR is opened or updated, the `Council PR Review` workflow runs 4 reviewers in parallel:

| Reviewer | Focus |
|---|---|
| 🏛️ **The Architect** | Design patterns, modularity, coupling, API design |
| 🔒 **The Security Sentinel** | Vulnerabilities, secrets, injection, auth |
| ⚡ **The Performance Hawk** | Complexity, resource usage, concurrency, I/O |
| 🛠️ **The Maintainer** | Readability, tests, docs, error handling, style |

Each reviewer receives:
1. The PR diff (truncated if very large)
2. Their persona prompt from `reviewers/<name>.md`
3. All guideline files from `guidelines/*.md`

The workflow aggregates all outputs into a single, updatable PR comment.

## Customization

### Adding Guidelines
Drop any `.md` file into `guidelines/`. It will automatically be injected into every reviewer's prompt. This is the easiest way to add project-specific rules, conventions, or context.

Examples:
- `guidelines/api-design.md` — REST/gRPC conventions
- `guidelines/deployment.md` — Release and ops rules
- `guidelines/frontend.md` — UI/component patterns

### Adding Reviewers
1. Create a new file in `reviewers/<persona-name>.md`
2. Add it to the workflow matrix in `.github/workflows/council-pr-review.yml`:
   ```yaml
   strategy:
     matrix:
       reviewer:
         - architect
         - security
         - performance
         - maintainer
         - your-new-reviewer  # <-- add here
   ```

### Modifying a Persona
Edit any file in `reviewers/` to change the focus areas, review style, or output format.

### Skipping the Review
Add `skip-council-review` label to a PR to bypass the council.

## Required Secrets

- `KIMI_API_KEY` — Used to call the Kimi API for review generation.

## Workflow Triggers

- `pull_request`: opened, synchronize, reopened
- Manual dispatch via `workflow_dispatch`

## Troubleshooting

**Review comment not appearing?**
- Check the Actions tab for the `Council PR Review` workflow run.
- Ensure `KIMI_API_KEY` is set in repository secrets.

**Output is truncated?**
- Very large diffs are truncated to stay within token limits. Consider splitting large PRs.

**Want to re-run?**
- Re-run the workflow from the Actions tab, or push a new commit.
