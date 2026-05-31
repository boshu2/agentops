# Triangulated Kernel — Google SRE AI Reliability Method

> The stable, parseable methodology distilled from the primary source and cross-checked against the AgentOps convergence cluster (Anthropic Agent SDK, obra/superpowers, EveryInc) documented in `docs/convergence/`. Consensus axioms only; the one genuine doctrine disagreement is quarantined under DISPUTED. Every axiom cites quote-bank anchors. Operators are defined in `operator_library.md`.

<!-- TRIANGULATED_KERNEL_START v1.0 -->

## Axioms (consensus)

- **A1 — Velocity breaks human-paced practice.** A ~4x rise in code/change volume makes line-by-line human review and manual operations unsustainable; the response is structural, not more reviewers. (§1, §25)
- **A2 — Humans move up the abstraction ladder.** Preserving manual intervention skill is counterproductive; human expertise shifts to guardrails, golden data, and governance of designs/intent/policies. (§3, §25, §28)
- **A3 — Verification must be independent of authorship.** The agent that generates work must be strictly isolated from the agent that defines tests or reviews it, to block cross-bias and catch untested requirements mechanically. (§23)
- **A4 — Intent/spec precedes code.** Specifications are co-authored and approved before generation, validating architecture and safety constraints up front. (§24)
- **A5 — Autonomy is earned, not granted.** Agents start at low autonomy and advance only by demonstrating statistically significant success against human-verified data; rigor scales with the risk of unsupervised action. (§7, §12, §13)
- **A6 — Evaluation data is tiered and calibrated.** Bronze (heuristic) → Silver (calibrated) → Gold (human-verified); stratified sampling calibrates Silver against Gold so the pipeline measures *true* precision, not observed precision. (§14, §15)
- **A7 — Score with a judge AND a deterministic check.** LLM-as-Judge grades reasoning/trajectory; strict deterministic exact-match (e.g. exact binary/version) grades the final action. A vague suggestion is not "correct." (§16)
- **A8 — Golden data is captured in-workflow.** Harvest verified labels as a byproduct of the normal workflow (accept/modify/reject suggestions at close), not as a separate annotation chore. (§17)
- **A9 — Reasoning is decoupled from execution.** A probabilistic reasoning engine never mutates state directly; it routes through a deterministic, human-governed control plane that is safe regardless of caller. (§11, §18)
- **A10 — Mutation requires dry-run.** Any agent-facing mutating interface must support a declarative dry-run that predicts outcome and blast radius before state changes. (§10)
- **A11 — Loops are bounded and interruptible.** Agent-specific rate limits, circuit breakers, stall detection, and an emergency stop ("Red Button") prevent runaway loops; every action is highly interruptible. (§9, §20)
- **A12 — Risk is evaluated in real time, autonomy degrades under risk.** Each action is scored against live context; elevated risk auto-downgrades the authorization level and re-inserts a human. (§6, §19)
- **A13 — Identity is non-ambient and least-privilege.** Agent principals are distinct from humans, strongly authenticated, on-demand, and minimally scoped. (§8, §21)
- **A14 — Every decision is append-only and auditable.** Chain of Thought and actuation traces are persisted immutably so any action can be reconstructed and re-evaluated. (§4, §5, §21)
- **A15 — Context is pulled and grounded, not ambient.** Agents discover and invoke tools through an explicit interface (MCP) and ground answers in current state via retrieval, not training memory alone. (§22)
- **A16 — Validation runs at machine speed.** Continuous production validation / adaptive progressive rollouts replace human-paced soak times as change rate rises. (§27)
- **A17 — Knowledge becomes a constraint.** *(AgentOps extension, consistent with A3/A9.)* A learning is durable only once it compiles into a gate, test, or rule — the deterministic boundary that A9 demands is how learning is enforced, not prose. (cross-ref `docs/3.0.md` ratchet rule #3)

## Operators

Defined with triggers, failure modes, and prompt modules in `operator_library.md`:

`Independent-Harness` (A3) · `Spec-Before-Code` (A4) · `Knowledge-To-Constraint` (A17) · `Progressive-Authorization` (A5, A12) · `Tiered-Eval-Data` (A6) · `Judge-Plus-Deterministic-Scoring` (A7) · `In-Workflow-Golden-Capture` (A8) · `Reasoning-Execution-Decouple` (A9) · `Dry-Run-Before-Mutation` (A10) · `Bounded-Interruptible-Loop` (A11) · `Append-Only-Provenance` (A14) · `Pulled-Grounded-Context` (A15) · `Non-Ambient-Identity` (A13) · `Machine-Speed-Validation` (A16, A2)

<!-- TRIANGULATED_KERNEL_END v1.0 -->

## DISPUTED (disagreement with AgentOps doctrine — not in the consensus kernel)

- **D1 — Fix-forward vs. clean rollback at velocity.** Google asserts that binary rollback becomes *unsafe* at 4x velocity (the Intervening-PR Problem) and prefers AI-assisted fix-forward + feature flags (§26). AgentOps doctrine optimizes for the **coherent-arc PR as the atomic-revert unit** (clean rollback per arc). This is a genuine, unresolved divergence — routed to `/council` at bead `ag-7278`, not silently absorbed.

## UNIQUE (Google-specific, out of AgentOps scope by design)

- **U1 — Production-state actuation** (draining cells, capacity resize, binary rollback on live infra): Google operates the run-time mutation layer; AgentOps deliberately ships no always-on production actuator (`ADR-0009`). Operators A9–A12 are encoded by AgentOps for the *dev loop* (CI gates as the deterministic boundary), not for live-production mutation.
- **U2 — Planetary-scale statistical gating** (A/B-testing SRE practices across population incident volume): AgentOps validates at repo scale via gates + eval suites, not population statistics.
