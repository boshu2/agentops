---
title: Phase 2 Summary - Agents Control Plane Hardening
date: 2026-04-25
skill: agentops:crank
epic: agentops-gpu
status: DONE
---

# Phase 2 Summary: Crank Implementation

- **Epic:** `agentops-gpu`
- **Complexity:** standard
- **Waves completed:** 3
- **Child beads closed:** 6 of 6
- **Status:** DONE
- **Timestamp:** 2026-04-25T17:30:26-04:00

## Delivered

- `ao agents inspect` and `ao agents lint` now resolve default contract,
  script, and skill paths from the repo root, including when run from `cli/`.
- `ao agents doctor` provides read-only text and JSON diagnostics for contract
  path, allowlist count, skill count, lint status, unknown top-level on-disk
  dirs, and the next lint command.
- `scripts/check-agents-write-surfaces.sh` now emits `source_locations` in JSON
  and source-file evidence in human failures.
- The write-surface lint now detects split Go joins such as
  `filepath.Join(..., ".agents", "wiki")`.
- The contract now catalogues previously hidden production surfaces:
  `context`, `packets`, `skill-drafts`, `topics`, and `wiki`.
- The pre-push agents-hub hash gate now has bounded local timeout behavior via
  `HASH_GATE_TIMEOUT_SECONDS`; local timeouts warn, CI failures remain strict.
- The operator guide documents `doctor`, source-location lint evidence, split
  path triage, and hash-gate local controls.

## Validation During Implementation

- `cd cli && go test ./cmd/ao -run 'TestAgents|TestAgentsLint|TestAgentsDoctor' -count=1`
- `bats tests/scripts/check-agents-write-surfaces.bats`
- `bats tests/scripts/pre-push-gate.bats --filter 'agents hash|retrieval ratchet'`
- `shellcheck --severity=error scripts/check-agents-write-surfaces.sh scripts/check-agents-hash-snapshot.sh scripts/pre-push-gate.sh`
- `scripts/generate-cli-reference.sh --check`
- `npx -y markdownlint-cli docs/agents-operator-guide.md docs/INDEX.md README.md`
- `bash tests/docs/validate-doc-release.sh`

<promise>DONE</promise>
