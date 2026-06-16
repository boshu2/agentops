# Effective Feedback Compute (EFC) — the harness's scaling coordinate

> **Doctrine.** AgentOps is an agent harness. This doc operationalizes the EFC scaling law (Zhang et al. 2026, *arXiv:2605.29682*) into the rules the harness runs by. Source mining: [`.agents/research/2026-06-16-efc-agent-harness-scaling-laws.md`](https://github.com/boshu2/agentops/blob/main/.agents/research/2026-06-16-efc-agent-harness-scaling-laws.md) (local corpus). Evidence tier: [`the-science.md`](../the-science.md) Part 10.
>
> **The claim in one line:** harness success scales with *useful feedback*, not *spend*. Same tokens, better feedback steering → ~3× success (0.27 → 0.90 at matched budget). Optimize the feedback, not the budget.
>
> ⚠️ **Validation status (2026-06-16): IMPORTED, NOT LOCALLY PROVEN.** The paper's numbers are mostly synthetic/oracle. Transfer to real AgentOps traces is **untested** — the experiment is pre-registered ([`.agents/research/2026-06-16-efc-transfer-preregistration.md`](https://github.com/boshu2/agentops/blob/main/.agents/research/2026-06-16-efc-transfer-preregistration.md) (local corpus), bead `age-k2w`) but **not yet runnable** (yield ledger N≈3; success label is entangled with the gate disposition that feeds EFC). Treat the rules below as a useful imported lens, **not** a measured law for this harness, until the N≥50 pilot returns a CI-separated result. A null is a plausible, accepted outcome.

## The coordinate

A harness turns raw compute `C_raw` (tokens, tool calls, wall-clock) into feedback events. **Only feedback that is Informative · Valid · Non-redundant · Retained counts.** Per event:

```
EFCₜ = κ · Iₜ · Vₜ · Rₜ · Mₜ
```

It's a **product** — any factor at zero kills the event. Run-level EFC sums over the trace. The two ratios that matter:

```
η = EFC / C_raw      harness EFFICIENCY  — what we control by design
X = EFC / D_task     feedback SUFFICIENCY — measured against task demand
```

**η — not `C_raw` — predicts success (R²=0.97 vs 0.01).** Spending more is not progress. This is the quantitative form of "tokens ≠ progress" and "don't admire the cathedral."

## The four factors → what the harness already does (and must do harder)

| Factor | "Counts only when…" | AgentOps mechanism | Failure mode it names |
|---|---|---|---|
| **I — Informative** | reveals task-relevant info | `ao inject` / corpus retrieval; discovery→plan packet; routing the right context to the right move | context-stuffing that surfaces nothing relevant |
| **V — Valid** | grounded in reliable evidence | gates (`ao gate`), evidence-before-close, provenance ledger, cross-family review, `eval-outcomes`, real tests | confident-but-ungrounded output; "looks good" self-grade |
| **R — Non-redundant** | addresses an *active* subgoal, no repeats | corpus dedup, dry-runs, the promotion ratchet (kill artifacts that don't change behavior) | re-observing the same failure; flailing; landfill `.agents/` |
| **M — Retained** | actually affects later decisions | the Knowledge Flywheel; `σ×ρ` in `ao metrics health` (ρ ≈ M); durable `.agents/` corpus; handoffs | learnings written and never re-read |

> **V is the load-bearing one for AgentOps.** "Evidence over comfort," gates, cross-family verdicts, TDD acceptance — they all exist to keep `Vₜ → 1`. An agent that grades its own work "looks good" is producing feedback with `V≈0`: zero EFC no matter how many tokens it burned.

## Five operating rules (derived, enforceable)

1. **Optimize η, not C_raw.** Adding budget without adding routing/verification/memory quality does not move success. Before spending more (bigger model, more turns, more swarm), ask: *does this raise EFC, or just `C_raw`?* If only `C_raw`, don't.

2. **A real oracle lowers task demand — buy it first.** `D_task ∝ (1 − V_oracle)`. A failing acceptance test, a deterministic gate, a real reproduction is the single cheapest way to *reduce required EFC*. This is the math behind BDD-first / `bdd-foundry` / "no runnable acceptance test, no bead." The windshield (deterministic ground truth) beats more driving.

3. **Discount repetition, hard.** Re-running the same failing command, re-reading the same file, re-deriving a known fact is `R≈0` — pure `C_raw`, zero EFC, and on real traces it actively *subtracts* (NRS divides by `1 + 0.35·attempts`). When a loop repeats an observation, that's the stop/escalate signal, not a reason to retry.

4. **Feedback you don't retain is feedback you didn't get.** A gate verdict, a review finding, a learning that doesn't change a later decision has `M=0`. Capture must meet *re-use* — the promotion ratchet (R3: no learning without a constraint) is the M-enforcer. Citation tracking (`ao metrics cite`) measures whether `M` actually fired.

5. **Match feedback rigor to task demand, never flat.** High `D_task` (deep reasoning, high tool entropy, noisy observations, no oracle) needs more EFC → more verification, quorum, fresh-context subagents. Low `D_task` (trivial, well-verified) needs almost none → single-model, cheap. This is "quorum at gates, cheap at generation" with a demand formula attached. There is **no universal best harness** — efficiency is task-relative (H5/H6 win HumanEval, H0/H3 win SWE-bench).

## The feedback-quality lens (apply at every feedback event)

When a harness move produces feedback (a gate result, a tool observation, a review, a retrieved artifact), score it before trusting it:

- **I?** Does this tell us something task-relevant we didn't have? (else it's noise)
- **V?** Is it grounded in a real check/test/evidence, or self-asserted? (else `EFC=0`)
- **R?** Is this new, or a repeat of an observation already in the trace? (repeat → discount/stop)
- **M?** Will this change a later decision, and is it captured where the later decision will see it? (else it evaporates)

A feedback event that fails any of these is `C_raw` masquerading as progress. This lens is the operational test behind `/validate`, `/review`, the gates, and the ratchet.

## Trace-observable scoring recipe (for the future `ao efc`)

The paper's **Estimated-EFC** uses only features a harness can see — AgentOps already emits most via the provenance ledger and session trace:

| Feature | Source in AgentOps |
|---|---|
| `cₜ` checker fired | gate / `ao gate` verdict events |
| `zₜ` tool result referenced | tool-call → later-message reference |
| `pₜ` plan update | bead/plan state change |
| `mₜ` memory retention | citation events (`reference`/`applied`) |
| `qₜ` observation consistency | repeated vs. novel observation |
| `Δₜ` subgoal progress | bead status improvement |
| `aₜ`, `ρₜ` attempt count / repetition | retry detection in the trace |
| `Q_t` status quality | pass=1.0 / assert=0.42 / runtime=0.12 / timeout=0.06 / API-err=0.0 |

`ao efc` (filed as a bead, **not built yet** — CLI-for-deterministic: it's measurable, so it's a subcommand) would compute run-level Estimated-EFC, `η = EFC/C_raw`, and `X = EFC/D_task` over a session, giving the harness a number for "was this run efficient feedback, or just expensive?"

## Relationship to the flywheel equation

`the-science.md` Part 2 gives the knowledge-**stock** law `dK/dt = I(t) − δ·K + σ·ρ·K`. EFC is its per-**trace** twin:
- `I(t)` (real input) is only the feedback where **I·V·R·M** all fire.
- `σ·ρ` (retrieval × decision-influence) is the run-level **M** factor measured at the corpus level.

Same claim, two altitudes: **compounding requires useful feedback, not volume — at the trace level (EFC) and the corpus level (σ×ρ).**

## See also

- [`the-science.md`](../the-science.md) Part 10 — EFC as Tier-A external evidence.
- [`architecture/operating-loop.md`](../architecture/operating-loop.md) — the seven moves; EFC explains *why* move 6 (prove acceptance) and move 7 (ratchet) are load-bearing.
- [`knowledge-flywheel.md`](../knowledge-flywheel.md) — the M factor at corpus scale.
