{{ define "operating-loop" }}
## The AgentOps Operating Loop

You work TWO nested loops. AgentOps owns both; GC dispatches them whole.

- **Inner loop — `ao rpi`** (one bead, one coherent arc): research → plan →
  implement → validate. Invoked as ONE command. You never decompose it; `ao rpi`
  runs the steps internally and enforces the ratchet rules below.
- **Outer loop — `ao evolve`** (a wave of rpi cycles toward a GOALS.md directive):
  select next-best work → dispatch N inner loops → post-mortem at the session
  threshold. Invoked as ONE command.

### Ratchet rules (loop invariants — AgentOps owns these, not GC)

1. **No self-grade.** The validator is never the implementer. A PASS verdict from
   the agent that wrote the code does not count.
2. **Fresh agent on failure.** On a FAIL, restart with a fresh agent + the durable
   failure reason — do not let the failing agent retry in place.
3. **Knowledge → constraints.** A failure that repeats twice becomes a planning
   rule / pre-mortem check / durable constraint in `.agents/`, not a re-explanation.

### Context compiler

Context compounds through the rig's `.agents/` corpus, not chat history:
- `ao inject --query "<topic>" --apply-decay --max-tokens N` — JIT, decay-ranked,
  token-budgeted slice for the work at hand (the FIRST thing rpi research does).
- `ao compile` — deterministic corpus rebuild (Mine → Grow → Defrag → Lint).
- `ao maturity --scan` — find stale entries; `--evict` archives low-utility knowledge.

Durable decisions/attempts/verdicts also land on the bead (`gc bd update
--set-metadata ...`) — the bead/Dolt metadata is the write-model projection of
the cross-session memory; the append-only `docs/provenance/ledger.jsonl` is the
source of truth and wins on disagreement.
{{ end }}
