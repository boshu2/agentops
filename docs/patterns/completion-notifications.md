# Pattern: completion notifications without a webhook server

> **Status:** Active
> **Captured from:** epic soc-yjzp (Anthropic Managed Agents parity, May 2026)
> **Applies to:** off-API users who want "outcome complete → notify external system" behavior without standing up a webhook server.

Anthropic's 2026-05-06 Managed Agents launch shipped webhooks as a first-class
primitive. Off-API users running AgentOps locally don't need a webhook server.
Three patterns cover the common cases using infrastructure you already have.

## Pattern A — GitHub Actions issue creation

Use when: the work is being run inside a GitHub Actions workflow (nightly
dream cycle, scheduled `ao daemon` run, release proof harness).

The repo's `.github/workflows/nightly.yml` already does this for the dream-cycle
proof job — when the run completes, a step opens or updates a GitHub issue
with the result summary and links to the run logs. Same shape works for any
outcome: a job step that calls `gh issue create` or `gh issue comment` with
the structured result.

Skeleton:

```yaml
- name: Notify on completion
  if: always()  # fires on success and failure
  run: |
    gh issue create \
      --title "Outcome: ${{ github.workflow }} #${{ github.run_number }}" \
      --body  "Status: ${{ job.status }}
Result: $(cat .agents/rpi/last-result.json | jq -c)
Run: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}" \
      --label  outcome
  env:
    GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Trade-off: GitHub-only, asynchronous (you watch the issue), but you already
have the auth and infrastructure.

## Pattern B — git post-commit hook → curl

Use when: the work writes a tracked artifact (a learnings file, a
`.agents/rpi/execution-packet.json`, a release note) and you want to fan out
to Slack / Discord / Mattermost / a custom endpoint when that artifact lands
on a specific branch.

Skeleton (`.git/hooks/post-commit`, marked executable):

```bash
#!/usr/bin/env bash
set -euo pipefail

branch=$(git rev-parse --abbrev-ref HEAD)
[[ "$branch" == "main" || "$branch" == "crank/"* ]] || exit 0

# Only fire when the watched artifact actually changed in this commit.
git diff-tree --no-commit-id --name-only -r HEAD | grep -q '^\.agents/rpi/' || exit 0

payload=$(jq -nc \
  --arg branch "$branch" \
  --arg sha    "$(git rev-parse --short HEAD)" \
  --arg subj   "$(git log -1 --format=%s)" \
  '{text: "✅ \($branch) @ \($sha): \($subj)"}')

curl -sS -X POST -H 'Content-Type: application/json' \
  -d "$payload" "$SLACK_WEBHOOK_URL" >/dev/null || true
```

Trade-off: scoped to the local clone (only fires on the developer / runner
that ran the commit), but reaches any HTTP endpoint and runs synchronously
on commit so debugging is easy.

## Pattern C — `ao daemon` log tailing

Use when: the work runs as a long-lived job inside `ao daemon` (scheduled
overnight Dream, recurring `ao goals measure`, scheduled wiki forge) and you
want a downstream consumer to react to specific job events.

The daemon writes one JSON line per job event to its log. A consumer process
(systemd unit, tmux pane, container sidecar) tails the log and forwards the
events it cares about.

Skeleton:

```bash
# Consumer running alongside the daemon
tail -F ~/.local/state/agentops/daemon.log \
  | jq -c --unbuffered 'select(.event == "job.completed" and .job.kind == "dream.run")' \
  | while read -r line; do
      url=$(echo "$line" | jq -r '.job.report_url // empty')
      curl -sS -X POST -H 'Content-Type: application/json' \
        -d "{\"text\": \"Dream cycle complete: $url\"}" \
        "$SLACK_WEBHOOK_URL" >/dev/null || true
    done
```

Trade-off: needs a long-running consumer process and the daemon to be running,
but is the closest analog to a real webhook stream — every job event is
visible, including failures and partial runs, with no GitHub or commit
dependency.

## Decision matrix

| Situation | Pattern |
|---|---|
| Work runs in GitHub Actions; you watch issues | A — GitHub Actions issue creation |
| Work writes a tracked artifact; you have a chat webhook URL | B — git post-commit → curl |
| Work runs continuously under `ao daemon`; you want real-time events | C — daemon log tailing |
| You think you need a webhook server | You probably don't — start with A or B; reach for C only if you genuinely need event streaming |

## What this is NOT

- Not a managed cloud service. Nothing leaves your infrastructure.
- Not a replacement for the Anthropic Managed Agents webhook primitive — if
  you're on the Claude Platform, use that. This pattern is for the off-API
  case where standing up a webhook server is more infrastructure than the
  problem warrants.
- Not strongly-ordered. None of the three patterns guarantees delivery; if
  you need that, you need a queue, not a webhook.
