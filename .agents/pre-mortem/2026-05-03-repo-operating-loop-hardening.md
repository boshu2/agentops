---
id: pre-mortem-2026-05-03-repo-operating-loop-hardening
type: pre-mortem
date: 2026-05-03
objective: repo-operating-loop-hardening
epic: soc-0qz1
verdict: WARN
mode: inline
---

# Pre-Mortem: Repo Operating Loop Hardening

## Verdict

PRE-MORTEM VERDICT: WARN

The plan is worth doing, but it is broad enough to create new policy drift if
each issue invents local rules. The first implementation wave should define the
`.agents` lifecycle vocabulary and validation lane metadata before broad script
changes land.

## Failure Modes

| Risk | Trigger | Mitigation |
| --- | --- | --- |
| No-mutation mode hides needed release artifacts | `ci-local-release.sh` is made read-only without an explicit artifact-producing release lane | Add lane metadata: read-only fast gate versus artifact-producing release gate. |
| `.agents` policy becomes another stale doc | Contract changes are not enforced by `check-agents-write-surfaces.sh` and fixtures | Make classification enforcement part of the first child issue. |
| Hook install fix only works in source tree | Tests keep executing `hooks/*.sh` from the repo, not installed plugin cache | Add installed-cache fixture that runs `dangerous-git-guard.sh` after `install-codex-plugin.sh`. |
| Dangerous git parser under-blocks real force pushes | Regex is replaced with token parsing but misses combined flags or aliases | Cover `-f`, `--force`, `--force-with-lease`, branch names containing `-f`, and refspec variants. |
| Worktree gate suppresses policy edits | Generated `.agents` ignore rules grow until `.gitignore` or tracked policy churn is hidden | Classify `.gitignore` and tracked policy edits as explicit operator-action paths. |
| Release audit validator becomes too lax | Historical artifact scoping skips the current release's required evidence | Add explicit modes: changed/pre-push, target release, and strict official release. |
| bd timeout wrapper loses successful writes | `gtimeout` kills a command after JSON output and automation treats it as total failure | Preserve stdout, re-read by ID, and document "timed out after emitted JSON" as indeterminate-success. |

## Required Gates

- `bats tests/scripts/check-agents-write-surfaces.bats`
- `bash tests/scripts/test-codex-plugin-install.sh`
- `bats tests/hooks/hook-stdin-contracts.bats`
- `bash tests/scripts/test-worktree-disposition.sh`
- `bats tests/scripts/release-artifacts.bats`
- `bash scripts/check-contract-compatibility.sh`
- `scripts/pre-push-gate.sh --fast --scope worktree`

## Sequencing Guard

Do not start by editing every validation script. Start with the vocabulary and
fixtures that make later edits unambiguous:

1. `.agents` lifecycle classifications and enforcement.
2. Execution-profile validation lane metadata.
3. Independent concrete bug fixes: Codex hook install layout and release audit
   scoping.
4. Broader no-mutation validation changes.
5. Canonical root and bd/Dolt closeout ergonomics.
