# AgentOps — One-Session Mode (maximally-Claude)

> Run the AgentOps factory loop in a **single Claude session, no external infrastructure** — the assured work-loop with nothing but Claude + a beads tracker (`br`) + git. This is the distilled core of AgentOps, and the **native Claude golden image**: the same laws scale out unchanged to the orchestrated factory (NTM swarms of Claude/Codex/AGY). It realizes the `ag-eli0` distillation *from the other direction* — instead of carving orchestration out, rebuild the core up from one session and prove it runs assured.
>
> **Canonical reference implementation + proof:** the control-plane bootstrap at `~/dev/control-plane` — n=1 green, independently validated, zero Dolt, zero `claude -p`.

## What it is
The control plane **is** the main Claude agent + `br` (SQLite + JSONL + git) + subagents. No daemon, no message bus, no Dolt. One operator, one session. Three roles, mapped to Claude-native primitives:

| Role | Primitive |
|---|---|
| State store | `br` (SQLite + JSONL + git) |
| Orchestrator | the main agent + a `tick.sh` helper |
| Worker | an Agent subagent (fresh context), loads the AgentOps image (skills/corpus) |
| Validator | a *separate-context* Agent subagent (author ≠ judge) |

## The loop (one tick, idempotent)
`next → claim → worker subagent → separate-context validator subagent → orchestrator closes on PASS (carrying an evidence ref) → br sync + git commit → repeat`.
- Empty ready set = **NO_READY** (stop scheduling). No open/in-progress beads = **CONVERGED**.

## The disciplines (what this encodes as standards)
- **The seam = different context, not different vendor.** Author ≠ judge holds in an all-Claude system because worker and validator are separate Agent invocations.
- **Single writer.** Only the orchestrator closes a bead; workers and validators propose.
- **Evidence-gated, verified against reality.** Validators execute/inspect the real artifact, not assert from a glance. Close only on `VERDICT: PASS`.
- **Contested/unverified FAIL → fresh tie-break validator.** Record both verdicts; the orchestrator never self-overrules. (Earned: a real false-FAIL was caught this way.)
- **git is the durable ledger.** Committed `issues.jsonl` is the provenance; `br close --reason` carries the evidence ref. **No Dolt.**
- **No `claude -p`.** In-harness subagents only (Max sub). **Idempotent ticks** — read state first.

## The fractal (why this scales)
Native Claude image (this) → Codex image → AGY image → orchestrated NTM swarms managing swarms. **Same contract at every scale**; harden a discipline in the native image once and it propagates to the big factory. Native ⊂ orchestrated, same laws.

## Quickstart
1. `br init --prefix <p>` in a git repo — the state store.
2. Map work to `br` beads with dependencies (the DAG).
3. Run the tick loop: orchestrator = main agent + tick helper; dispatch worker then a separate-context validator per bead; close on PASS with an evidence ref.
4. Every close cites evidence; the commit is the ledger.

See the reference implementation for the exact contracts: `control-plane/{SOURCE-OF-TRUTH,ARCHITECTURE,ORCHESTRATOR,WORKER,VALIDATOR,LEARNINGS,N1-PROOF}.md` + `tick.sh` + `status.sh`.

## Relation to `ag-eli0`
`ag-eli0` distills AgentOps to its core (the mind) by relocating orchestration to MTO. **One-session mode is the same destination, validated** — it rebuilds the core up from a single session and proves the assured loop runs with only Claude + `br` + git. The disciplines above are the standards that core enforces.
