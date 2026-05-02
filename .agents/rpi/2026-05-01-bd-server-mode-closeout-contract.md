---
id: rpi-2026-05-01-bd-server-mode-closeout-contract
type: rpi-report
date: 2026-05-01
issue: soc-y8b.8
verdict: PASS
---

# RPI Report: bd Server-Mode Closeout Contract

## Objective

Repair the closeout contract so agents distinguish mandatory Git push from
conditional bd Dolt remote push for server-mode trackers.

## Discovery

Existing beads skill docs already said to run `bd dolt push` only when a Dolt
remote is configured. Drift remained in `cli/AGENTS.md`, where the quick
reference and session completion block told agents to run a bare
`bd dolt push`. The root `AGENTS.md` had moved away from unconditional bd push
but did not document the no-remote server-mode case.

Current live tracker evidence:

- `bd vc status` reports the canonical Dolt server under
  `/home/boful/dev/personal/agentops/.beads/dolt`.
- `bd dolt remote list` reports no remotes configured.
- `soc-y8b.8` already tracks the contract drift.

## Implementation

Changed files:

- `AGENTS.md`
- `cli/AGENTS.md`
- `docs/runbooks/bd-server-mode-closeout.md`
- `docs/documentation-index.md`
- `scripts/validate-bd-closeout-contract.sh`
- `scripts/pre-push-gate.sh`

Behavioral contract:

- `git push` remains mandatory.
- `bd vc status` is the tracker visibility check.
- `bd dolt commit` is run when tracker changes are pending.
- `bd dolt remote list` determines whether `bd dolt push` is valid.
- No-remote `bd dolt push` is recorded as unavailable, not treated as a
  session failure.
- Agents must not add a self-remote to silence no-remote output.

## Validation

Passed:

```bash
scripts/validate-bd-closeout-contract.sh
npx --yes markdownlint-cli AGENTS.md cli/AGENTS.md docs/runbooks/bd-server-mode-closeout.md docs/documentation-index.md
git diff --check
bash -n scripts/validate-bd-closeout-contract.sh scripts/pre-push-gate.sh
shellcheck --severity=error scripts/validate-bd-closeout-contract.sh scripts/pre-push-gate.sh
```

Pending before close:

```bash
scripts/pre-push-gate.sh --fast
bash scripts/check-worktree-disposition.sh
```

## Result

`soc-y8b.8` acceptance is satisfied for the repo-local contract: normal
session closeout no longer instructs agents to run a misleading unconditional
`bd dolt push`, and the new validator gates the root/CLI AGENTS plus runbook
contract.
