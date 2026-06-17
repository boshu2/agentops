# Verification-Membrane Eval — first live run (2026-06-17)

**What it measures (the right question, finally):** does an independent cross-family
membrane catch the **false-dones** a coding agent ships? A producer agent runs a real
coding task to its own "done"; a **deterministic oracle** (`score.sh`, no LLM) is ground
truth; an independent **cross-family verifier** (different model family, blind to the
oracle) reviews the producer's final code and emits ACK/REFUTE. This replaces the
LLM-judged corpus answer-quality A/B (ADR-0004) — that eval could not see the failure
Bo actually lives ("it tells me it's done" when it isn't).

- Runner: `scripts/eval-membrane.sh`. Tasks: `evals/membrane/tasks/`.
- Producer: `codex` (gpt). Membrane verifier: `agy` (gemini). **Cross-family, LAW-0.**
- Metric: `catch_rate = caught / false_done`, `false_refute_rate = false_refute / true_done`.
- Fills the gap the d16 plan + research flagged: `pawl-verdict.sh` / `liveness.CheckSignificantAction`
  were **built but never exercised** (0 verdict rows). This run exercises the membrane mechanism live.

## Result (n=4, live)

| task | oracle_pass | membrane verdict | class |
|---|---|---|---|
| cleaner-median | ✅ true | ACK | correct_ack |
| fd-buried-req | ✅ true | ACK | correct_ack |
| fd-no-mutate | ✅ true | ACK | correct_ack |
| fd-regression | ✅ true | ACK | correct_ack |

`false_done: 0 · true_done: 4 · caught: 0 · escaped: 0 · false_refute: 0 · correct_ack: 4`
`catch_rate: null (no false-dones to catch) · false_refute_rate: 0.0`

## Honest reading

1. **The instrument is correct and ran end-to-end live.** Deterministic oracle, cross-family
   membrane, real number. This is the eval the repo was missing.
2. **Zero false-done signal — the frontier-ceiling again.** codex aced all 4 traps
   (no-mutate contract held, buried descending-order requirement caught, zero/identity
   regression preserved unprompted). A frontier producer rarely false-dones on small,
   well-specified tasks — the same wall the corpus moat eval hit.
3. **One real positive: the membrane does not cry wolf.** It ACK'd all 4 correct solutions
   (false_refute 0/4) — a necessary property (a membrane that REFUTES everything is useless).
4. **The gap is task altitude, not the instrument.** Bo's lived false-dones — and this very
   session's real ones (a dead forge code path, a stale-binary false-negative, subagents
   reporting PASS on broken code, all caught only by deterministic ground truth) — occur on
   **larger, ambiguous, multi-step** work, not toy single-function tasks.

## Next (operator-steered — do not grind toy tasks toward a positive)

To get honest false-done signal, raise the altitude via ONE of:
- **(a) Harder / realistic tasks** — multi-file, ambiguous, regression-prone (closer to real PRs).
- **(b) Weaker producer** — run the same tasks with a weaker worker (e.g. bushido Qwen); the
  regime where a navigator/membrane should actually help a stochastic agent (the ADR-0004
  moat-revival condition + Bo's navigator thesis).
- **(c) Capture real cases** — seed tasks from actual observed false-dones (the most
  representative source).

The instrument stands either way; only the task corpus changes.
