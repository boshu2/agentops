# @claude Bot Delegation Runbook

> Operational contract for the `@claude` GitHub App in this repo.
> Source of truth for workflow + permissions: `.github/workflows/claude.yml`.

## What @claude is

A GitHub App that watches the repo for `@claude` mentions and, when one fires the workflow, spawns a Claude Code session against the current PR/issue context. The session can read CI results, edit files, commit to the PR branch, mark PRs ready for review, and comment.

It is **not** an autonomous agent that monitors PRs on a schedule. It only acts on mention.

## What it is not

- Not a substitute for branch protection (CI still gates merges)
- Not a `workflows: write` actor — it cannot edit files under `.github/workflows/` by default (see [Gotcha 2](#gotcha-2-workflow-files-fail-silently))
- Not a reviewer in the GitHub "Required reviewers" sense — the required check is the `claude-review` *status check*, not a human-style review

## Install

Run `/install-github-app` from the Claude Code CLI. This:

1. Installs the Anthropic-published GitHub App on the repo
2. Adds `.github/workflows/claude.yml`
3. Stores `CLAUDE_CODE_OAUTH_TOKEN` as a repo secret

## Permissions

The installed `claude.yml` defaults to **read-only** for `contents`, `pull-requests`, and `issues`. With those defaults the bot can see context, post review-style comments, and… that's it. No commits, no PR updates, no issue closes.

**Required for active delegation:**

```yaml
permissions:
  contents: write
  pull-requests: write
  issues: write
  id-token: write
  actions: read
```

After `/install-github-app`, verify the workflow contains these. If not, ship a PR upgrading them (PR #301 in this repo did exactly this). See lesson `claude-app-default-readonly`.

**Permissions that are intentionally not granted** in this repo: `workflows: write`. See [Gotcha 2](#gotcha-2-workflow-files-fail-silently).

## What triggers the bot

The `if:` condition in `claude.yml` fires on the substring `@claude` in:

| Event | Where |
|---|---|
| `issue_comment.created` | Issue/PR comment body |
| `pull_request_review_comment.created` | PR review comment body |
| `pull_request_review.submitted` | PR review body |
| `issues.opened` / `issues.assigned` | Issue title or body |

A `@claude` in a commit message, PR description, code, or markdown file does **not** trigger it.

## Status decoding

`gh run list --workflow=claude.yml` shows:

| `status` | `conclusion` | Meaning |
|---|---|---|
| `in_progress` | (empty) | Bot is actively working |
| `completed` | `success` | Bot finished its turn (may or may not have pushed; check commits) |
| `completed` | `skipped` | The `if:` condition matched the event but anti-loop logic suppressed the run (almost always: the comment came from the bot itself) |
| `completed` | `failure` | Bot errored; check run logs |
| `completed` | `cancelled` | Workflow was cancelled (rare) |

"Skipped" runs are normal: the bot ignores its own comments to avoid loops. They're not a problem — they're the protocol working.

## What @claude can and can't do

| Action | Works? |
|---|---|
| Commit to existing PR branch | yes |
| Open new branches/PRs | yes |
| Resolve merge conflicts | yes (often; not always) |
| Mark draft PR ready for review | yes |
| Close issues | yes |
| Edit `.github/workflows/*.yml` | **no** — see Gotcha 2 |
| Edit repo settings, branch protection | no |
| Push directly to `main` | no — branch protection blocks |
| Merge a PR | only via auto-merge (it can enable auto-merge, then CI must go green) |

## Gotchas

### Gotcha 1: Default install is read-only

`/install-github-app` ships `contents/pull-requests/issues: read`. The bot will respond to mentions, but every push attempt silently fails (you'll see a `success` run with no commits, or a confused comment). Upgrade to `write` before promising delegation works.

### Gotcha 2: Workflow files fail silently

The bot does not have `workflows: write` and cannot edit anything under `.github/workflows/`. In PR #270 (this repo, 2026-05-18) a forward-merge from main pulled in `claude.yml` changes; the bot detected the perm gap and **reverted that subset of its own merge** with the commit message *"revert: undo claude.yml forward-port (requires workflows permission)"*.

Treat workflow-file forward-ports as human-only operations. If you need the bot to update workflow files, ship a separate PR that elevates the perm, or upgrade the App's repo-level workflow permission.

### Gotcha 3: Anti-loop = silent stop on follow-up

Once the bot posts a comment, every subsequent comment on that PR by the bot itself is `skipped`. New `@claude` comments from a human resume work. If a PR looks stuck after the bot's first pass, post a fresh `@claude` comment with sharper guidance — don't expect the bot to "notice" it should try again.

### Gotcha 4: Auto-merge cascades need green required checks

Setting `auto_merge: true` on a PR doesn't merge it — it merges it once **all required status checks** pass on the *current* head. If `main` moves and your PR goes BEHIND, required checks may be stale or marked failed; the PR will sit unmerged until you trigger a branch update (`gh api repos/<owner>/<repo>/pulls/<n>/update-branch -X PUT`). The bot does not auto-trigger this.

In this repo, `claude-review` is a required check. The `claude.yml` workflow fires **automatically on `pull_request: opened` and `synchronize`** (head-SHA changes) — no `@claude` mention is needed to set the check on a fresh PR or after `update-branch`. The check appears as `IN_PROGRESS` while the bot reviews; wait, don't poke. The "mention-only fires the check" pattern applies only to legacy on-mention-only workflow configurations; this repo's `claude.yml` uses the broader trigger set.

Validated 2026-05-18 across PRs #321–#326: every PR received `claude-review: SUCCESS` without any human `@claude` mention.

## When to use @claude

| Situation | Delegate? |
|---|---|
| Routine rebase + push | yes |
| Mark draft ready after CI passes | yes |
| Address known-mechanical conflict (e.g. counts, registry drift) | yes |
| Catch up a long-lived branch to main | yes |
| Semantic refactor across multiple files | maybe — give it a tight scope |
| Architecture decision | no — use `/council --mode=debate` instead |
| Anything requiring `workflows: write` | no — human PR |

## How to delegate well

1. **One comment per intent.** Multiple `@claude` mentions in fast succession dilute the prompt; the bot reads them all but applies the most recent.
2. **Cite file paths and line numbers.** "Fix the test on `cli/internal/foo/foo_test.go:42`" beats "fix the failing test."
3. **State the success criterion.** "Mark this PR ready when CI is green" gives the bot a stop condition.
4. **Don't ask for opinions.** The bot is best at execution. Architecture/design decisions go through `/council`.
5. **Verify with `gh run watch`.** Don't trust "I'll fix it" — confirm the run was `success` and a commit landed.

## Operating contract

- **Default required check:** `claude-review` (this repo's branch protection requires it on `main`).
- **Workflow file:** `.github/workflows/claude.yml`.
- **Bot identity:** commits authored by `github-actions[bot]` with co-author `Claude`.
- **OAuth secret:** `CLAUDE_CODE_OAUTH_TOKEN` (rotate via `/install-github-app` re-run if compromised).

## See also

- Lesson `claude-app-default-readonly` (local-only: `.agents/learnings/2026-05-17-claude-app-default-readonly.md`) — the install-perm gotcha as a one-rule lesson
- Lesson `dogfood-install-pr` (local-only: `.agents/learnings/2026-05-17-dogfood-install-pr.md`) — the install PR must itself follow the discipline it installs
- [Lesson Format](lesson-format.md) — schema for `.agents/learnings/` entries
- [AGENTS.md `## Workflow`](https://github.com/boshu2/agentops/blob/main/AGENTS.md) — PR-only discipline this bot operates inside

> `.agents/learnings/` is repo-local (gitignored). Search local lessons with `bd memories <keyword>` or via the [Lesson Format](lesson-format.md) contract.
