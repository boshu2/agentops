# Claude Code Silent Edit Revert Escalation

Date: 2026-05-03
Related bead: `soc-ff2p.7`

## Summary

AgentOps research reproduced a tracked-file edit that appeared to succeed, then
returned to `HEAD` with `git status` clean. The surveyed AgentOps mechanisms do
not explain that behavior. This document is the escalation packet to file
upstream or keep as the local internal report when no upstream URL is available.

## Reproduction Recipe

1. Start a Claude Code session in a clean AgentOps worktree.
2. Perform three or more sequential Edit-tool calls against tracked files under
   `lib/`, `scripts/`, or `skills/`.
3. After each Edit success response, immediately run:
   ```bash
   grep -n "<unique marker>" <edited-file>
   git diff HEAD -- <edited-file>
   git status --short
   ```
4. Capture a failure when the marker is missing, `git diff` is empty, and
   `git status` is clean.
5. Attach:
   ```bash
   git reflog --date=iso | head -20
   lsof +D . | grep '<edited-file>' || true
   shasum -a 256 <edited-file>
   ```

## AgentOps Non-Causes Already Surveyed

| Candidate | Result |
| --- | --- |
| `hooks/dangerous-git-guard.sh` | Blocks destructive git commands; does not restore files. |
| `hooks/go-vet-post-edit.sh` | Runs vet checks only; no git mutation. |
| `hooks/edit-knowledge-surface.sh` | Injects context; no file restore path. |
| `hooks/session-end-maintenance.sh` | Confined to `.agents/` session cleanup. |
| Overnight checkpoint rollback | Scoped to `.agents/overnight/` paths. |
| RPI supervisor fetch/rebase | Runs during landing, not between Edit calls; would leave normal git evidence. |
| bd/Dolt autosync | Tracker state only; not tracked source files under `lib/`, `scripts/`, or `skills/`. |

## Requested Upstream Fix

Claude Code should expose enough post-Edit evidence to distinguish a real write
from a harness-level restore or file-sync race:

- post-write file hash in the Edit response or audit stream;
- a setting to disable background restore/sync for tracked repo files;
- an event log entry when the harness rewrites a file after an Edit operation;
- clear diagnostics when a tool reports success but the final on-disk content
  does not contain the requested change.

## Local Defensive Practice

Until the upstream behavior is resolved:

- verify a unique marker and `git diff` after each edit batch;
- stage completed files immediately after verification;
- commit per wave when running multi-file implementation work;
- attach the commands above if the symptom recurs.

## Upstream URL

Not filed from this environment. This artifact is the local escalation packet;
replace this line with the upstream issue URL when filed.
