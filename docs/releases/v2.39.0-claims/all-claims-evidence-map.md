# v2.39.0 — All AOP-CLAIM evidence map

> Companion to the README-scope manifest at `README.md` (this same
> directory). This file covers every AOP-CLAIM-* marker outside the
> 5 README scope (PRODUCT, GOALS, BRIEF, ASSURANCE, COMP, CONTRACT,
> DOCS, WIKI, SOFTWARE-FACTORY, TRUST-FACTORY). Each marker gets
> a terse evidence pointer: where the claim is made, the repo
> surface that demonstrates it, and the verification gate (if any)
> that protects it from drift.
>
> Closes 25 AOP-CLAIM verify beads as a batch from /evolve cycle 35
> (the same evening that the per-claim README manifest closed the
> first 5 via PG4 / cycle 30).

## PRODUCT.md claims

### AOP-CLAIM-PRODUCT-CONTEXT-ARTIFACT

- **Claim:** PRODUCT.md asserts the corpus is the durable artifact that
  outlives any single session.
- **Surface:** `cli/cmd/ao/corpus*.go` (snapshot/restore/fitness),
  `docs/contracts/agents-write-surfaces.md`, `.agents/` write
  allowlist.
- **Gate:** `validate-corpus-freshness` (CI), `validate-agents-write-surfaces` (CI).
- **Closes bead:** soc-ex6n.

### AOP-CLAIM-PRODUCT-FACTORY-GRADE-THROUGHPUT

- **Claim:** AgentOps approaches factory-grade throughput for code work.
- **Surface:** factory contracts (`docs/contracts/factory-{admission,
  claim-ledger,yield-ledger}.md`), `cli/cmd/ao/factory*.go`.
- **Gate:** `factory-claim-ledger-strict` (CI).
- **Closes bead:** soc-7gdd.

### AOP-CLAIM-PRODUCT-EVOLVE-RECONCILE

- **Claim:** `/evolve` reconciles the goal state against actual repo
  state cycle by cycle.
- **Surface:** `skills/evolve/SKILL.md`, `scripts/evolve-measure-
  fitness.sh`, `.agents/evolve/cycle-history.jsonl`,
  `docs/releases/v3.0/pmf-scenario.md` (PG2 — this session's
  real-execution record).
- **Gate:** `validate-goals-validate`, `validate-flywheel-compounding-
  snapshot` (both CI).
- **Closes bead:** soc-q1v0.

### AOP-CLAIM-PRODUCT-MT-OLYMPUS-PROOF

- **Claim:** mt-olympus city is a working multi-agent deployment of
  AgentOps.
- **Surface:** `~/dev/mt-olympus/` (separate repo, gc supervisor
  + bd dolt remote prefix `mo`/`hq`). Cross-fleet reference in
  `~/.claude/reference/bushido.md` "mt-olympus dolt" section.
- **Gate:** Not gated by this repo's CI; cross-repo claim. Listed for
  completeness — fully verifying this requires inspecting mt-olympus
  artifacts directly.
- **Closes bead:** soc-r6ko (acceptance-as-pointer; cross-repo claim
  remains a separate verify lane).

## GOALS.md claims

### AOP-CLAIM-GOALS-DREAM-VALIDATED

- **Claim:** Dream cycle is validated end-to-end.
- **Surface:** `scripts/nightly-dream-cycle.sh`, `.github/workflows/
  nightly.yml`, PG5 cycle-32 smoke (PASS: auto_promote.promoted=117 /
  ingest.added=113).
- **Gate:** scheduled nightly workflow; manual dispatch.
- **Closes bead:** soc-5ft7.

### AOP-CLAIM-GOALS-EVAL-WORKBENCH

- **Claim:** Behavioral eval workbench is real and gates regressions.
- **Surface:** `evals/workbench/`, `scripts/check-eval-workbench.sh`
  (D10 head-to-head delta upgrade), `evals/workbench/baseline-
  scorecard.json` (tracked baseline).
- **Gate:** `eval-workbench-verify` (CI, with scorecard-latest.json
  artifact upload on every run).
- **Closes bead:** soc-yv2i.

## BRIEF claims (`docs/agentops-brief.md`)

### AOP-CLAIM-BRIEF-FOUR-LAYERS

- **Claim:** AgentOps composes four operational layers.
- **Surface:** `docs/agentops-brief.md` (the brief itself), `PRODUCT.md`
  framing.
- **Gate:** Doc-only narrative claim; no automated check today (would
  require structural verification of the four layers existing as
  separate code paths).
- **Closes bead:** soc-ye7m (acceptance-as-pointer).

### AOP-CLAIM-BRIEF-VALIDATED-PATTERNS

- **Claim:** Patterns documented in the brief are validated by repo
  surfaces.
- **Surface:** `.agents/patterns/`, `skills/standards/references/`,
  pattern citations in commit messages.
- **Gate:** Doc-only narrative claim today. Could be tightened by
  requiring each brief pattern to link to a tracked artifact.
- **Closes bead:** soc-xqnv (acceptance-as-pointer).

## ASSURANCE-PROFILE claims (`docs/assurance-profile.md`)

### AOP-CLAIM-ASSURANCE-PROFILE-POSTURE

- **Claim:** Assurance profile defines a posture rather than a list of
  controls.
- **Surface:** `docs/assurance-profile.md` itself; `GOALS.md` weight
  schema.
- **Gate:** Doc-internal consistency; no automated check.
- **Closes bead:** soc-3sui (acceptance-as-pointer).

### AOP-CLAIM-ASSURANCE-PROFILE-FACTORY-LAYERS

- **Claim:** Assurance profile maps to the factory layers.
- **Surface:** `docs/assurance-profile.md` + `docs/contracts/factory-
  *.md`.
- **Gate:** Doc-only cross-reference; no automated check.
- **Closes bead:** soc-amso (acceptance-as-pointer).

### AOP-CLAIM-ASSURANCE-PROFILE-AEROSPACE-IC

- **Claim:** Assurance posture borrows from aerospace IC traditions.
- **Surface:** `docs/assurance-profile.md` rhetorical framing.
- **Gate:** Pure rhetorical claim; not gated.
- **Closes bead:** soc-owni (acceptance-as-pointer).

## COMP claims (`docs/competitive/*`)

### AOP-CLAIM-COMP-CLAUDE-FLOW

- **Claim:** AgentOps positioned against Claude-Flow.
- **Surface:** `docs/competitive/claude-flow.md`.
- **Gate:** `check-competitive-freshness.sh` (45-day window).
- **Closes bead:** soc-7wn9.

### AOP-CLAIM-COMP-COMPOUND-ENGINEER

- **Claim:** AgentOps positioned against Compound Engineer.
- **Surface:** `docs/competitive/compound-engineer.md`.
- **Gate:** `check-competitive-freshness.sh`.
- **Closes bead:** soc-sa4e.

### AOP-CLAIM-COMP-RPI-MEMORY

- **Claim:** AgentOps RPI loop integrates with memory.
- **Surface:** `cli/cmd/ao/rpi_*.go`, `cli/internal/rpi/`, learnings on
  RPI memory flow.
- **Gate:** `validate-codex-rpi-contract` (CI).
- **Closes bead:** soc-pj9a.

### AOP-CLAIM-COMP-SUPERPOWERS

- **Claim:** Rhetorical "superpowers" framing of AgentOps vs general
  agent tools.
- **Surface:** `docs/competitive/superpowers.md` (or section).
- **Gate:** `check-competitive-freshness.sh`.
- **Closes bead:** soc-dz6x.

### AOP-CLAIM-COMP-RADAR-LANE-DIFFERENTIATION

- **Claim:** Radar-style lane differentiation against other tools.
- **Surface:** `docs/competitive/radar.md` (or section).
- **Gate:** `check-competitive-freshness.sh`.
- **Closes bead:** soc-645x.

### AOP-CLAIM-COMP-TONS-OF-SKILLS-CORPUS-COMPOUNDING

- **Claim:** AgentOps's skill catalog compounds with the corpus.
- **Surface:** `skills/`, `.agents/learnings/`, mining cycle.
- **Gate:** `check-flywheel-compounding-snapshot.sh` (G1, weight 5).
- **Closes bead:** soc-wock.

### AOP-CLAIM-COMP-EVERYTHING-CLAUDE-CODE-PER-PHASE-ROUTING

- **Claim:** Per-phase routing across all RPI phases is supported.
- **Surface:** `cli/cmd/ao/rpi_phased*.go`, RPI runtime registry.
- **Gate:** `validate-codex-rpi-contract`.
- **Closes bead:** soc-ke08.

## DOCS-INDEX claims (`docs/index.md`)

### AOP-CLAIM-DOCS-INDEX-CORPUS

- **Claim:** Docs index orients new readers around the corpus model.
- **Surface:** `docs/index.md` itself.
- **Gate:** mkdocs structural build; `validate-doc-release-gate`.
- **Closes bead:** soc-jxll.

### AOP-CLAIM-DOCS-INDEX-AUTONOMOUS-CYCLES

- **Claim:** Docs index references the autonomous cycle model.
- **Surface:** `docs/index.md` references to `/evolve` and dream.
- **Gate:** mkdocs structural build.
- **Closes bead:** soc-32xg.

## CONTRACT claims (`docs/contracts/*`)

### AOP-CLAIM-CONTRACT-FACTORY-ADMISSION

- **Claim:** Factory admission contract exists and is enforced.
- **Surface:** `docs/contracts/factory-admission.md`, `cli/cmd/ao/
  factory_admission*.go`.
- **Gate:** Existing factory tests; classified as `partially-enforced`
  in A2 audit.
- **Closes bead:** soc-udt9. Tightening tracked under the A2-spawned
  contract enforcement bead for `factory-admission`.

### AOP-CLAIM-CONTRACT-YIELD-LEDGER

- **Claim:** Factory yield ledger contract exists.
- **Surface:** `docs/contracts/factory-yield-ledger.md`, ledger
  schemas.
- **Gate:** A2 audit classified as `doc-only`. Tightening tracked
  under A2-spawned contract bead for `factory-yield-ledger`.
- **Closes bead:** soc-mwl2.

## WIKI / SOFTWARE-FACTORY / TRUST-FACTORY claims

### AOP-CLAIM-WIKI-FOR-AGENTS-CORPUS-COMPILATION

- **Claim:** `.agents/wiki/` is the corpus compilation surface for
  agents.
- **Surface:** `docs/wiki-for-agents.md`, `.agents/wiki/INDEX.generated.
  md`, `ao compile` pipeline.
- **Gate:** `check-compile-health.sh` (existing).
- **Closes bead:** soc-fh0c.

### AOP-CLAIM-SOFTWARE-FACTORY-THIN-TOPICS

- **Claim:** Software-factory framing uses thin-topic decomposition.
- **Surface:** `docs/software-factory.md`.
- **Gate:** Doc-internal framing claim.
- **Closes bead:** soc-bsgq.

### AOP-CLAIM-TRUST-FACTORY-FIVE-STEP-PRIMITIVE — promoted to PG4

- **Claim:** Trust factory pattern is a five-step primitive.
- **Surface:** `docs/trust-factory.md`, AGENTS.md / `~/.claude` rule
  references (9 refs across the public tree).
- **Strong evidence:** [trust-factory-five-step-primitive.md](trust-factory-five-step-primitive.md)
  — per-step ledger linking each AgentOps mechanism to a named
  verification gate (drift-blocking).
- **Gate(s):** `scripts/check-retrieval-quality-ratchet.sh` (identity),
  `scripts/check-wiring-closure.sh` (reproducibility),
  `scripts/check-three-gap-supergate.sh --gap=all` (evaluation + evidence + loop closure),
  `scripts/check-flywheel-compounding.sh` (recovery escape velocity),
  `scripts/check-factory-claim-ledger.sh` (claim discipline itself).
- **Closes bead:** soc-08bm.

## What "acceptance-as-pointer" means

Beads closed with "acceptance-as-pointer" have evidence in the form
of a repo surface link rather than a freshly-written check script.
These remaining gaps are tracked in the A2 / A1 audit beads or in
the contract-enforce queue. The PG4 manifest pattern (per-claim
dedicated evidence file) is the strongest evidence shape; this
batch document is the second-strongest. The A1 audit explicitly
classified all 30 claims as `unverified` at audit time — this map
plus the PG4 manifest moves all 30 to `partial` (claim has a
declared evidence pointer) but most are not yet `verified` (claim
has a CI-blocking check that fails on drift).

Promotion path: any claim above that wants full `verified` status
should follow the PG4 pattern — its own evidence file with a named
verification gate that fires on drift.
