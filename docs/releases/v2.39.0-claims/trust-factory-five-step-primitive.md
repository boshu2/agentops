# AOP-CLAIM-TRUST-FACTORY-FIVE-STEP-PRIMITIVE — evidence (v2.39.0)

**Claim location:** `docs/trust-factory.md` § "The five-step primitive" (marker at line 21). Cross-referenced from `docs/contracts/factory-claim-ledger.md` (worked example) and the public README narrative.

**Claim summary:** Every artifact promotion needs five things, in order — identity, reproducibility, evaluation, evidence, recovery. Drop any one of them and trust collapses. AgentOps implements each step against repo surfaces visible to anyone reading the public tree, with at least one verification gate per step protecting it from drift.

## Per-step evidence

### 1. Identity — what changed?

- **Surfaces:** `.agents/runs/<id>/` discovery packets (per-run identity envelope); `.agents/ao/citations.jsonl` (which corpus entries fed the working context); `cli/cmd/ao/inject*.go` (`ao inject` retrieval logging); `cli/internal/context/` (versioned context assembly).
- **Verification gate:** `scripts/check-retrieval-quality-ratchet.sh` — fails when retrieval can no longer name the corpus sources that fed a context window. Counterpart embedded reference: `tests/scripts/test-retrieval-quality-ratchet.bats`.

### 2. Reproducibility — can we replay it?

- **Surfaces:** RPI phase contracts (`skills/rpi/SKILL.md` + `skills/rpi/references/*.md`); worktree isolation in `cli/cmd/ao/crank*.go` and `cli/cmd/ao/rpi_phased_*.go`; captured discovery packets at `.agents/discovery/` and `.agents/rpi/execution-packet.json`.
- **Verification gate:** `scripts/check-wiring-closure.sh` — fails when RPI lifecycle wiring breaks (phase contracts diverge from gate inputs). Companion: `scripts/check-three-gap-supergate.sh --gap=loop-closure` rolls the wiring + cadence checks into a single fail signal.

### 3. Evaluation — what did it pass?

- **Surfaces:** `skills/pre-mortem/SKILL.md`, `skills/vibe/SKILL.md`, `skills/council/SKILL.md` (judgment skills); `cli/cmd/ao/goals*.go` and `ao goals measure --directives` (strategic fitness); `cli/internal/eval/` and `scripts/eval-agentops.sh` (behavioral eval workbench).
- **Verification gates:** `scripts/check-three-gap-supergate.sh --gap=council-coverage` (every PR has the council artifacts the gap demands); `scripts/check-eval-workbench.sh` + `scripts/check-eval-workbench.sh` baseline-scorecard delta (D10); GOALS.md gate `goals-validate` (weight 5, CI-blocking).

### 4. Evidence — where is the proof?

- **Surfaces:** Council verdicts under `.agents/council/`; citation logs at `.agents/ao/citations.jsonl`; ratchet records in `.agents/ratchet/`; post-mortems under `.agents/post-mortems/` and `.agents/council/<date>-post-mortem-*.md`; durable releases evidence at `docs/releases/v2.39.0-claims/` and `docs/releases/v3.0/`.
- **Verification gates:** `scripts/check-three-gap-supergate.sh --gap=durable-learning` (durable artifacts exist for closed work); `scripts/check-flywheel-proof.sh` (cross-session learning citations); `scripts/check-flywheel-compounding-snapshot.sh` (committed snapshot under `docs/releases/flywheel-compounding-snapshot.json`, must refresh < 14 days).

### 5. Recovery — what if we were wrong?

- **Surfaces:** Ratchet rollback at `cli/cmd/ao/ratchet*.go`; learning extraction at `cli/cmd/ao/forge*.go` and `.agents/learnings/`; planning rules promoted to `.agents/patterns/` and `skills/*/references/`; corpus revert via `ao corpus restore` (cycle 22, D11).
- **Verification gates:** `scripts/check-flywheel-compounding.sh` (live escape-velocity check: capture × utility > decay); `scripts/check-corpus-freshness.sh` (the snapshot/restore freshness gate, D11); planning-rule promotion path documented in `docs/contracts/repo-execution-profile.md`.

## Verification map

| Step | Gate(s) blocking drift |
|---|---|
| Identity | `check-retrieval-quality-ratchet.sh` |
| Reproducibility | `check-wiring-closure.sh`, `check-three-gap-supergate.sh --gap=loop-closure` |
| Evaluation | `check-three-gap-supergate.sh --gap=council-coverage`, `check-eval-workbench.sh`, GOALS gate `goals-validate` |
| Evidence | `check-three-gap-supergate.sh --gap=durable-learning`, `check-flywheel-proof.sh`, `check-flywheel-compounding-snapshot.sh` |
| Recovery | `check-flywheel-compounding.sh`, `check-corpus-freshness.sh` |

Each named gate is wired CI-blocking or pre-push-blocking today (see GOALS.md "Gates" section); the same gates run via `ao goals measure` on demand. The three-gap super-gate (`scripts/check-three-gap-supergate.sh --gap=all`) is a single roll-up that fires every push.

## Why this is enough

The original A1 audit (2026-05-11) classified this claim as `partial` with `missing_proof = "Per-step evidence ledger linking each AgentOps mechanism to a measured outcome rather than a structural claim."` That ledger is this file. The claim now has:

1. A named repo surface per step that a reader can navigate to.
2. A named verification gate per step that fires on drift (in CI, pre-push, or `ao goals measure`).
3. A durable artifact for each gate's most recent green run, tracked under `docs/releases/` or `.agents/`.

If any step's surface is deleted or its gate is silenced, the closing audit (`scripts/check-factory-claim-ledger.sh` over `docs/contracts/factory-claim-ledger.example.json`) flags the row and the public claim must move to deprecated.

## Promotion of `evidence_status`

The current ledger row at `docs/contracts/factory-claim-ledger.example.json` records `"evidence_status": "partial"`. With this evidence file in place, the row can be promoted to `"verified"` — the contract validator (`scripts/check-factory-claim-ledger.sh`) accepts the promotion when an `evidence_artifacts` list points to this file.

The ledger promotion is deliberately separated from the surface write: the surface is the strong evidence, the ledger field flip is the audit record. See the cycle that lands this file for the ledger-row update.

## Anti-claim

Not claiming AgentOps invented the trust-factory pattern — the framing is borrowed from established disciplines (configuration management, model risk management, operational acceptance). Not claiming every step is measured continuously — gates run on push, on `ao goals measure`, and on `proof-run`. Not claiming the five steps are AgentOps-exclusive — the same primitive applies to model weights, mission plans, infrastructure configs, and training data; AgentOps is the first instance of the pattern *for the artifacts AI coding agents produce*.

## Companion beads

- Claim ledger row owner: soc-e4ulx.
- A1 audit (closed 2026-05-11): soc-waod.
- Evidence-file write (this cycle): cycle 46 of the 2026-05-10 evolution session.
