# AOP-CLAIM-README-FACTORY-CONTEXT — evidence (v2.39.0)

**Claim location:** README.md sections that describe AgentOps as the
context-compiling factory layer.

**Claim summary:** AgentOps compiles context from session-end
artifacts (decisions, attempts, citations, verdicts, learnings) into
durable corpus state that the next session consumes.

## Repo surfaces that demonstrate it

- `cli/cmd/ao/factory.go`, `cli/cmd/ao/factory_*.go` — factory entry
  points (admission, claim ledger, yield ledger).
- `docs/contracts/factory-admission.md`, `docs/contracts/factory-
  claim-ledger.md`, `docs/contracts/factory-yield-ledger.md` — the
  three contracts that define what the factory accepts, claims, and
  yields.
- `scripts/check-factory-claim-ledger.sh` — blocking CI gate that
  fails if the claim ledger structure regresses.
- `.agents/factory/` (gitignored) — the actual ledger files on
  operator machines.

## Verification surface

`scripts/check-factory-claim-ledger.sh` runs in CI as
`validate-factory-claim-ledger` (see AGENTS.md CI Jobs table). If the
factory contract drifts, this gate fails.

## Why this is enough

The claim is a structural one: the factory layer exists and is
enforced. Live throughput numbers (sessions/day, yield rate) belong
in a separate operator-runtime report and are not promised by this
public marker.
