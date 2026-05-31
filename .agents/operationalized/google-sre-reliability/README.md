# Operationalized: Google SRE AI Reliability Method

> Google's 2026 SRE whitepaper *"AI Engineering Reliable Operations"* turned from a doc into **executable, auditable artifacts** — a kernel, an operator library, and validators — via the `operationalizing-expertise` skill (Track A: static distillation). This is the bridge that makes **"are we actually doing what Google says?"** a checkable artifact instead of a claim. Produced under bead `ag-4hf7`; companion to [`docs/convergence/`](../../../docs/convergence/).

## What's here

```
corpus/
  primary_sources/google-sre-ai-reliable-operations.md   # the paper (verbatim archive)
  quote_bank/quote_bank.md                                # §1..§28 anchored quotes, fixed tag taxonomy
  distillations/opus/distillation.md                      # single-model raw extraction
  specs/triangulated_kernel.md                            # marker-bounded: 17 axioms + 14 operators (+ DISPUTED / UNIQUE)
  specs/operator_library.md                               # 14 operator cards, each with an AgentOps-Enforcement verdict
scripts/
  validate-corpus.py      # anchors sequential, entries well-formed, tags in taxonomy
  validate-operators.py   # every operator has required fields + valid verdict + real anchors
  extract-kernel.py       # markers present, versioned, min axiom/operator counts
```

## The scorecard (from the 2026-05-29 audit)

Each operator card records whether AgentOps actually enforces the behavior, where, and the gap bead. As of this writing: **7 ENFORCED · 6 PARTIAL · 1 GAP** of 14 operators.

| Verdict | Operators |
|---|---|
| **ENFORCED** | Spec-Before-Code · Knowledge-To-Constraint · Reasoning-Execution-Decouple · Append-Only-Provenance · Pulled-Grounded-Context · Machine-Speed-Validation |
| **SPLIT** | Independent-Harness — ENFORCED at the skills layer (council/AP#7), PARTIAL at the CLI (no per-agent runtime guard → `ag-w3fg`) |
| **PARTIAL** | Progressive-Authorization (L0–L4 ladder → `ag-wrom`) · Tiered-Eval-Data (`ag-fjbu`) · Judge-Plus-Deterministic-Scoring (`ag-xert`) · Dry-Run-Before-Mutation (`ag-r6l3`) · Bounded-Interruptible-Loop (session-scope → `ag-o5xp`) · Non-Ambient-Identity (scoped) |
| **GAP** | In-Workflow-Golden-Capture (`ag-xert`) |
| **DISPUTED** | Fix-Forward vs. clean rollback (`ag-7278`) |

**Headline:** the one invariant AgentOps enforces that the convergence cluster consistently misses is **Knowledge-To-Constraint** — learnings compiled into gates, not prose. That's the sharpest edge.

## Re-run the validators

```bash
cd .agents/operationalized/google-sre-reliability
python3 scripts/validate-corpus.py      # OK corpus validation
python3 scripts/validate-operators.py   # OK operator validation (14 operators)
python3 scripts/extract-kernel.py >/dev/null   # OK kernel v1.0: 17 axioms, 14 operators
```

## How to keep it honest

- When a gap bead closes, flip that operator's `AgentOps-Enforcement` line to ENFORCED and update the scorecard here + in `operator_library.md`.
- When a new convergence signal lands in `docs/convergence/ledger.md`, check whether it adds or sharpens an operator.
- The kernel is versioned (`v1.0`); bump it if axioms/operators change, and keep START/END markers intact (the extractor enforces this).
