---
title: Native ao beads audit and cluster command verification
date: 2026-04-13
bead_id: na-4bhk
source: rpi-validation
tags:
  - beads
  - cli
  - validation
---

# Native ao beads audit and cluster command verification

## Scope

The harvested follow-up asked to port `scripts/bd-audit.sh` and
`scripts/bd-cluster.sh` into native Go command support. The original item named
`gastown/internal/cmd`, but this repository's matching executable surface is the
AgentOps CLI under `cli/cmd/ao`.

Implemented scope:

- Added native `ao beads audit` and `ao beads cluster` commands in
  `cli/cmd/ao/beads_audit_cluster.go`.
- Preserved the shell scripts as compatibility entrypoints for existing skill
  guidance and hooks.
- Added focused tests for missing-`bd` degradation, audit classification, and
  cluster scoring/representative selection in `cli/cmd/ao/beads_test.go`.
- Regenerated `cli/docs/COMMANDS.md`.

## Validation

Commands run with `AGENTOPS_RPI_RUNTIME` removed to avoid the local `bushido`
environment leak:

```bash
env -u AGENTOPS_RPI_RUNTIME go test ./cmd/ao -run 'Test(Audit|Cluster)'
env -u AGENTOPS_RPI_RUNTIME go test ./...
env -u AGENTOPS_RPI_RUNTIME bash scripts/generate-cli-reference.sh --check
env -u AGENTOPS_RPI_RUNTIME bash scripts/validate-go-fast.sh
```

Live non-mutating command smokes used a built binary from the repository root:

```bash
env -u AGENTOPS_RPI_RUNTIME go build -o /tmp/ao-beads-native ./cmd/ao
env -u AGENTOPS_RPI_RUNTIME /tmp/ao-beads-native beads audit --json
env -u AGENTOPS_RPI_RUNTIME /tmp/ao-beads-native beads cluster --json
```

Observed smoke results:

- `ao beads audit --json` returned valid JSON with empty arrays for empty
  buckets and one likely-fixed open epic (`na-gtm`) from historical commit
  evidence.
- `ao beads cluster --json` returned valid JSON clusters for the current open
  backlog without applying reparenting.

## Notes

The commands intentionally do not rewire `skills/crank` or `skills/swarm` away
from the shell scripts in this cycle. That keeps this item scoped to native CLI
support while preserving existing compatibility paths.
