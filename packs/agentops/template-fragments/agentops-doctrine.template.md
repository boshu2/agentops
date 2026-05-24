{{ define "agentops-doctrine" }}
## AgentOps Doctrine — the CDLC waist

Every change to the default branch is a **PR**. Every PR cites a **bead**. The
unit of a PR is one **coherent arc** — a closable bead with a single rollback
semantic. Direct pushes are rejected.

### Coherent-arc + session-scope

- **Coherent arc** governs PR *shape*: bundle scenarios that ship-or-revert
  together; split scenarios with independent rollback. The PR is the atomic-revert
  unit.
- **Session scope** governs PR *count*: default 2-4 PRs per autonomous session. At
  >=5 shipped or in-flight, STOP and run a post-mortem before continuing —
  reactive-PR spirals are the dominant back-half failure mode.

### Acceptance is scenario-shaped

Read the bead's acceptance: a `.feature` file (canonical when present) or an
embedded `## Scenarios` block. **Free-text acceptance is INVALID** — promote it to
scenarios before work begins. Each scenario is a shaped behavior; the gates are the
reinforcement; the ratchet locks it in.

### Provenance

Source of truth is the append-only ledger at `docs/provenance/ledger.jsonl`.
`bd update --metadata` is a derived projection — the ledger wins on disagreement.

### Merge gate

CI-green is the merge signal. The mayor (orchestrator) drives every green PR to
merge on the default branch; there is no human merge gate in the autonomous loop.
The human is ON the loop (strategy, promotions, escalations), not IN the merge.

### Verify before committing

Run per-tool sanity checks for the surfaces you touched. Read the diff STAT for
anomalies before committing (a lopsided insertion/deletion count on a small change
= collateral data loss). Never commit unverified code.
{{ end }}
