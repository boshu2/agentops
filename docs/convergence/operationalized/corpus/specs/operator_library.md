# Operator Library — Google SRE AI Reliability Method, mapped to AgentOps

> Each operator is one executable behavior from the [kernel](triangulated_kernel.md). Cards carry the standard fields **plus** an `AgentOps-Enforcement` line that records — from the 2026-05-29 code+skills audit — whether AgentOps actually enforces it, where, and which bead closes any gap. This is the bridge that makes "are we doing what Google says" a *checkable* artifact, not a claim.
>
> **Enforcement legend:** `ENFORCED` (a mechanism requires/checks it) · `PARTIAL` (mechanism exists but incomplete or convention-only) · `GAP` (documented/desired, not mechanically enforced).
>
> **Scorecard:** 7 ENFORCED · 6 PARTIAL · 1 GAP (of 14). Open beads: `ag-w3fg` `ag-r6l3` `ag-xert` `ag-o5xp` `ag-fjbu` `ag-wrom` `ag-7278`.

---

### Independent-Harness

**Definition**: The agent that produces work is strictly isolated from the agent that tests or reviews it.
**When-to-Use Triggers**:
- Any validation/review phase after an implement phase.
- Any council/judge run scoring produced work.
**Failure Modes**:
- Implementer self-validates in the same context → cross-bias, rubber-stamp.
- "Fresh eyes" asserted in prose but not enforced at runtime.
**Prompt Module**:
~~~text
[OPERATOR: Independent-Harness]
1. Confirm the validator's agent/session id != the implementer's.
2. If equal, abort: spawn a fresh-context validator.
3. Validator sees the artifact + acceptance, NOT the implementer's reasoning trace.
~~~
**Canonical tag**: independent-harness
**Quote-bank anchors**: §23, §4
**AgentOps-Enforcement**: ENFORCED at skills layer (`skills/council/SKILL.md`, AP#7 gate `scripts/verify-gate-claim.sh`, ratchet rule "no self-grade"); **PARTIAL at CLI** — `cli/cmd/ao/rpi_phased.go:66-76` separates phases by invocation but has no per-agent runtime guard → **`ag-w3fg`**.

---

### Spec-Before-Code

**Definition**: Behavior/spec (BDD/Gherkin) is authored and approved before implementation; free-text acceptance is invalid.
**When-to-Use Triggers**:
- Start of any bead/feature before code.
- Acceptance arrives as prose.
**Failure Modes**:
- Coding begins with no failing test / no scenario.
- Free-text acceptance smuggled past planning.
**Prompt Module**:
~~~text
[OPERATOR: Spec-Before-Code]
1. Require Given/When/Then (happy + >=1 edge) before implement.
2. Reject free-text acceptance; loop back to plan.
3. Link each scenario to a test name (the slice's contract).
~~~
**Canonical tag**: spec-before-code
**Quote-bank anchors**: §24, §25
**AgentOps-Enforcement**: ENFORCED — `skills/discovery/references/discovery.feature` rejects free-text; `skills/scenario/SKILL.md`; scenarios↔test linkage CI gate (`validate.yml`, scenario-hash-stability).

---

### Knowledge-To-Constraint

**Definition**: A learning is durable only once it compiles into a gate, test, or rule.
**When-to-Use Triggers**:
- A failure repeats (≥2×).
- A post-mortem produces an "observation."
**Failure Modes**:
- Learning captured as prose advice that decays.
- Compound-notes accumulate without enforcement (the EveryInc gap).
**Prompt Module**:
~~~text
[OPERATOR: Knowledge-To-Constraint]
1. For each durable learning, name the gate/test/rule it becomes.
2. If it cannot become one, it is not yet durable — hold it.
3. Wire the gate; only then mark the learning promoted.
~~~
**Canonical tag**: knowledge-to-constraint
**Quote-bank anchors**: §3 (golden-data curation as governance)
**AgentOps-Enforcement**: ENFORCED — `skills/ratchet/SKILL.md` (Chaos→Filter→Ratchet), `skills/post-mortem/SKILL.md` (observation→repeat→gate→doctrine), CI skills-integrity gate. **This is the invariant the convergence cluster most consistently misses — AgentOps' sharpest edge.**

---

### Progressive-Authorization

**Definition**: Autonomy is earned incrementally against verified success; loops carry explicit stop policies.
**When-to-Use Triggers**:
- Granting an agent a new unsupervised capability.
- Any unbounded loop (crank/evolve, including a PROGRAM.md/AUTODEV.md-contract-bounded evolve run).
**Failure Modes**:
- Full autonomy on day one.
- Loops with no cycle/wave cap.
**Prompt Module**:
~~~text
[OPERATOR: Progressive-Authorization]
1. Start at human-approved; require a pass record before unsupervised.
2. Bound every loop (max cycles/waves; stop on operator marker).
3. State the promotion gate explicitly (what proof earns the next level).
~~~
**Canonical tag**: progressive-authorization
**Quote-bank anchors**: §7, §12, §13
**AgentOps-Enforcement**: ENFORCED bounds (`skills/crank` MAX_EPIC_WAVES=50, `skills/evolve --max-cycles`, `skills/rpi` complexity-scaled gates, session-scope 2-4 PR rule); **but no explicit L0–L4 ladder with named promotion gates** → **`ag-wrom`**.

---

### Tiered-Eval-Data

**Definition**: Eval data is stratified Bronze/Silver/Gold and Silver is calibrated against Gold so the pipeline measures true (not observed) precision.
**When-to-Use Triggers**:
- Building or scoring an eval suite.
- Reporting agent precision.
**Failure Modes**:
- Scoring against unlabeled/heuristic data and reporting it as precision.
- Holdout leakage into the scored set.
**Prompt Module**:
~~~text
[OPERATOR: Tiered-Eval-Data]
1. Tag every eval item Bronze/Silver/Gold.
2. Calibrate Silver against a Gold sample (stratified).
3. Report true-vs-observed precision; refuse to leak Silver/Gold targets.
~~~
**Canonical tag**: tiered-eval-data
**Quote-bank anchors**: §14, §15
**AgentOps-Enforcement**: PARTIAL — holdout-leak deny-by-default exists (`skills/eval-outcomes`, `cli/cmd/ao/eval_outcomes.go`), but tier strata + true-vs-observed naming are undocumented → **`ag-fjbu`**.

---

### Judge-Plus-Deterministic-Scoring

**Definition**: Grade reasoning with an LLM judge AND the final action with strict deterministic exact-match.
**When-to-Use Triggers**:
- Scoring an agent's mitigation/output.
- Gating a release on eval results.
**Failure Modes**:
- Accepting a vague suggestion as "correct."
- Judge-only scoring (non-reproducible) with no deterministic check.
**Prompt Module**:
~~~text
[OPERATOR: Judge-Plus-Deterministic-Scoring]
1. LLM-judge grades trajectory/reasoning against golden.
2. Deterministic check requires exact actionable params (binary/version).
3. Lock judge roster + rubric hash for reproducibility.
~~~
**Canonical tag**: judge-plus-deterministic-scoring
**Quote-bank anchors**: §16
**AgentOps-Enforcement**: PARTIAL — multi-judge consensus exists (`skills/council`, `judge_content_hash`), but the golden-data loader and deterministic-judge endpoint are stubbed in `cli/cmd/ao/eval.go` → **`ag-xert`**.

---

### In-Workflow-Golden-Capture

**Definition**: Harvest human-verified labels as a byproduct of the normal close workflow, not a separate chore.
**When-to-Use Triggers**:
- A bead/incident closes with a verified outcome.
- A human accepts/modifies/rejects an agent suggestion.
**Failure Modes**:
- Golden data requires a dedicated annotation pass (annotator fatigue → no data).
**Prompt Module**:
~~~text
[OPERATOR: In-Workflow-Golden-Capture]
1. At close, emit a structured suggestion of what was actually done.
2. Capture the human's accept/modify/reject as a golden label.
3. Feed it to the eval corpus with provenance.
~~~
**Canonical tag**: in-workflow-golden-capture
**Quote-bank anchors**: §17
**AgentOps-Enforcement**: GAP — no close-time golden-label capture wired (post-mortem/handoff capture learnings, not labeled eval pairs) → tracked under **`ag-xert`** (golden-data layer).

---

### Reasoning-Execution-Decouple

**Definition**: The probabilistic reasoning engine never mutates state directly; it routes through a deterministic, caller-agnostic boundary.
**When-to-Use Triggers**:
- Any agent action that changes durable state.
- Designing a new agent-facing capability.
**Failure Modes**:
- Agent runs raw scripts against live state.
- Domain core imports the runtime/shell.
**Prompt Module**:
~~~text
[OPERATOR: Reasoning-Execution-Decouple]
1. Route mutations through a port; never raw side effects from the core.
2. The boundary enforces safety regardless of caller (agent or human).
3. CI is the authoritative gate on what merges.
~~~
**Canonical tag**: reasoning-execution-decouple
**Quote-bank anchors**: §11, §18
**AgentOps-Enforcement**: ENFORCED — hexagonal "domain never imports shell" (`docs/architecture/ports-and-adapters.md`), CI-as-authoritative-gate, append-only provenancegraph as the deterministic record. Scope note: encoded for the *dev loop* (CI boundary), not live-production actuation (U1).

---

### Dry-Run-Before-Mutation

**Definition**: Every agent-facing mutating interface supports a declarative dry-run that previews outcome and blast radius.
**When-to-Use Triggers**:
- Any `ao` command that writes durable state.
- Any agent action with a non-trivial blast radius.
**Failure Modes**:
- Mutators with no preview → unpredictable blast radius.
- Dry-run on some commands but not all (inconsistent contract).
**Prompt Module**:
~~~text
[OPERATOR: Dry-Run-Before-Mutation]
1. Expose --dry-run on every mutating command.
2. Dry-run prints the exact diff/outcome without writing.
3. Gate: a mutating command without --dry-run fails CI.
~~~
**Canonical tag**: dry-run-before-mutation
**Quote-bank anchors**: §10
**AgentOps-Enforcement**: PARTIAL — present on ~6 commands (batch_forge/promote, harvest, compile/repair); absent on `ao provenance add`, `ao evolve blocked`, `ao ratchet *`; no CI gate enforcing universal coverage → **`ag-r6l3`**.

---

### Bounded-Interruptible-Loop

**Definition**: Agent loops carry rate limits, circuit breakers, stall detection, and an emergency stop; every action is interruptible.
**When-to-Use Triggers**:
- Any long-running or autonomous loop.
- A session shipping many consecutive PRs.
**Failure Modes**:
- Runaway loop / reactive-PR spiral with no stop.
- Stall with no watchdog/fallback.
**Prompt Module**:
~~~text
[OPERATOR: Bounded-Interruptible-Loop]
1. Wire a stall watchdog with timeout + fallback.
2. Cap cycles/waves; stop on operator marker.
3. Provide an emergency stop that halts in-flight actions.
~~~
**Canonical tag**: bounded-interruptible-loop
**Quote-bank anchors**: §9, §20
**AgentOps-Enforcement**: PARTIAL — live stall watchdog (`cli/cmd/ao/rpi_phased_stream.go:456-466`, 30s) + loop caps exist, but the **session-PR-scope guardrail is unenforced post-hookless** (`hooks/session-pr-counter.sh` absent, CLAUDE.md stale) → **`ag-o5xp`**; legacy supervisor coexistence is the known `soc-1gbpz` migration.

---

### Append-Only-Provenance

**Definition**: Agent reasoning (CoT) and actions persist immutably so any decision can be reconstructed.
**When-to-Use Triggers**:
- Any agent decision or actuation.
- A verdict/gate result.
**Failure Modes**:
- Decisions logged mutably / overwritten.
- Provenance as a derived projection treated as source of truth.
**Prompt Module**:
~~~text
[OPERATOR: Append-Only-Provenance]
1. Write events append-only (O_APPEND / hash-chained).
2. Ledger is source of truth; metadata is a projection.
3. Evidence must be reconstructable from the ledger.
~~~
**Canonical tag**: append-only-provenance
**Quote-bank anchors**: §4, §5, §21
**AgentOps-Enforcement**: ENFORCED — `cli/internal/provenancegraph/store.go:102-160` (O_APPEND, hash-chained, idempotent), `docs/provenance/ledger.jsonl` (schema `agentops-sdlc-provenance.v1`, ledger-wins), `scripts/lint-evidence-lines.sh`.

---

### Pulled-Grounded-Context

**Definition**: Context is pulled through explicit, decay-ranked, token-budgeted channels and grounded in current state — never sprayed ambiently.
**When-to-Use Triggers**:
- Session start / phase start.
- Any agent needing prior context.
**Failure Modes**:
- Ambient hooks stacking noise into the prompt (the 2.x delta=0 failure).
- Stale/training-only grounding.
**Prompt Module**:
~~~text
[OPERATOR: Pulled-Grounded-Context]
1. Pull via an explicit query (ao inject), decay-ranked + token-budgeted.
2. Ground in current corpus state, not just model memory.
3. No ambient push; dense and just-in-time.
~~~
**Canonical tag**: pulled-grounded-context
**Quote-bank anchors**: §22
**AgentOps-Enforcement**: ENFORCED — `cli/cmd/ao/inject.go` + `cli/internal/search/*` (1500-token budget, exp decay, utility weighting); hookless-first architecture (`docs/3.0.md`).

---

### Non-Ambient-Identity

**Definition**: Agent principals are distinct from humans, strongly authenticated, on-demand, least-privilege, and attributable.
**When-to-Use Triggers**:
- An agent performs an attributable action (commit, ledger write).
- Granting an agent any standing credential.
**Failure Modes**:
- Agents run with the developer's standing credentials.
- Actions not attributable to a unique agent principal.
**Prompt Module**:
~~~text
[OPERATOR: Non-Ambient-Identity]
1. Use a distinct agent principal, not human standing creds.
2. Stamp every action with the agent id (forensics).
3. Grant access on-demand, minimally scoped.
~~~
**Canonical tag**: non-ambient-identity
**Quote-bank anchors**: §8, §21
**AgentOps-Enforcement**: PARTIAL — agent-identity trailers via bd `prepare-commit-msg` hook provide attribution; standing-credential least-privilege is largely an out-of-session/substrate concern (U1) rather than an in-session AgentOps gate. Tracked as scoped-partial; no bead until a concrete in-session vector appears.

---

### Machine-Speed-Validation

**Definition**: Validation runs at machine speed (continuous/adaptive) as change rate rises, replacing human-paced soak.
**When-to-Use Triggers**:
- Every change to main.
- Rising change volume / progressive rollout.
**Failure Modes**:
- Human-paced review as the only gate at 4x volume → rubber-stamping.
- No automated done-condition per slice.
**Prompt Module**:
~~~text
[OPERATOR: Machine-Speed-Validation]
1. Make CI the authoritative, machine-speed gate (T0/T1/T2).
2. Every slice has an executable done-condition (failing test first).
3. Humans review design/intent/policy, not lines.
~~~
**Canonical tag**: machine-speed-validation
**Quote-bank anchors**: §27, §25, §2
**AgentOps-Enforcement**: ENFORCED — tiered CI gates (`.github/workflows/validate.yml`, T0≤30s / T1≤5m / T2≤15m, all required), coverage floor, complexity budget. Maps A2 (humans up the ladder) to the coherent-arc review model.
