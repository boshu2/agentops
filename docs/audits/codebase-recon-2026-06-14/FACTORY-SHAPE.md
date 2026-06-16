# Factory Shape — Adversarial Panel Synthesis

**Date:** 2026-06-14
**Question:** What shape should the AgentOps self-running software factory actually be? Does the "dumb router + restore wrong-tolerance" thesis survive the panel?
**Panel:** Minimalist · Systems Architect · Operator · Shadow/Skeptic · Product Lens
**Verified against the live repo** (facts below are grep-confirmed, not asserted).

---

## Load-bearing facts (confirmed, not claimed)

| Claim | Verified |
|---|---|
| The reversibility gate `liveness.CheckSignificantAction` / `IsSignificantAction` / `AdmitInboundWorkMessage` has **zero production callers** | ✅ grep excl. tests + the `liveness` pkg itself returns empty (exit 1) |
| `reversib\|one-way\|irreversib` appears in **zero** non-test gate/loop code paths | ✅ 23 matches, all in tests/docs/comments — none in a control-flow branch |
| The second trigger (repeated-failure escalation) is **already built**: `rpi.MaxGateRetryDepth = 5`, `rpi.ShouldForceEscalation(attempt)` | ✅ `cli/internal/rpi/phased_gates.go` |
| `ao rpi loop --max-cycles` defaults to **0 = unlimited** (stop only when queue empty) | ✅ `cli/cmd/ao/rpi_loop.go:127` |
| The provenance ledger — declared SDLC source-of-truth — has **1 line** | ✅ `wc -l docs/provenance/ledger.jsonl` = 1 |
| `.agents/` carries **duplicated taxonomies**: `handoff`+`handoffs`, `post-mortem(s)`, `pre-mortem`+`pre-mortems`+`pre-mortem-checks`, `retro`+`retros` | ✅ `ls -d` |
| `cli/cmd/ao/` is **633 `.go` files** | ✅ `ls \| wc -l` |

The single most important fact: **the router Bo says he needs to build is already in the codebase as two orphaned primitives** (`IsSignificantAction` for reversibility, `ShouldForceEscalation` for repeated-failure). Neither has a caller in the loop. The three months did not fail to build the fix — they built the disease on top of the cure and never plugged the cure in.

---

## 1. The verdict

**The thesis survives, with three amendments and one demotion of its own framing.** The factory should be a **short, wrong-tolerant loop (discovery → implement → validate → retry) that is cheap-by-default, with rigor applied only at one-way doors and only after repeated failure** — exactly the cost-law Bo ratified months ago and never wired. The doom-loop diagnosis is correct and confirmed by the repo (1-line ledger, 633-file CLI, 238k LOC of tests vs 150k of product, duplicated `.agents/` dirs, the last 40 commits almost entirely meta-work). **Where the thesis is amended:** (a) the router is not a thing to *build* — it is a ~30-line adapter over two primitives that already exist plus the deletion that makes it the only router (Minimalist + Architect); (b) "reversible → cheap" leaks on **trust-surface writes** — a wrong learning/skill/contract edit is git-reversible but cognition-irreversible because it poisons the next loop's context, so a third axis is required (Architect); (c) wrong-tolerance is only *safe unattended* if the loop has a **global budget and a persistent per-bead failure counter** — without them "cheap retry" is an unbounded `while` loop with Bo's API key, which is literally the 121-agent/5M-token runaway already in memory (Operator). **The demotion:** the thesis still smuggles in "build it well," which is the failure mode. The honest move is subtraction, not construction.

---

## 2. The router, concretely

A dumb three-tier classifier, consulted at exactly one decision point per loop tick. Default is cheap; you escalate, you never start heavy.

### The decision rule

```
tier = CHEAP                                  # default — single-model, inline discovery, no gate

# Trigger A — reversibility (one-way door)
if action ∈ { merge-to-main, delete, external-send, schema/contract/migration change,
              spend, public-publish }:
    tier = max(tier, GATED)

# Trigger B — trust-surface write (the Architect's leak fix; reversible BYTES, irreversible CONSEQUENCE)
if scope ∩ { .agents/learnings/**, skills/**, _beads/**, docs/contracts/** } ≠ ∅:
    tier = max(tier, GATED)

# Trigger C — repeated failure (already built: rpi.ShouldForceEscalation)
if per_bead_fail_count(bead) > K:            # K = 5 (rpi.MaxGateRetryDepth)
    tier = max(tier, GATED)

# Escalate GATED → QUORUM only for true cross-family one-way doors
if tier == GATED and action ∈ { merge-to-main, schema/contract change, public-publish }:
    tier = QUORUM                            # liveness.CheckSignificantAction — its FIRST prod caller
```

- **CHEAP** = the old loop: inline discovery → implement → `ao gate check --fast` (the deterministic windshield — keep it) → retry up to K. No bdd-foundry, no provenance, no quorum, no council-floor.
- **GATED** = run the per-tool gate + provenance append. One actor, no cross-model panel.
- **QUORUM** = `liveness.CheckSignificantAction` (cross-model ACK). Reserved for the irreversible 5%.

### Resolving Architect vs Operator on where it leaks and what bounds it

The two lenses disagreed on the leak and the bound; both are right about different halves:

- **The Architect's leak (trust surface) is real and is fixed by Trigger B.** "Reversible-in-git" is the wrong axis for `.agents/learnings/`, `skills/`, `_beads/`, `docs/contracts/` — those feed the next loop's cognition. Byte-reversibility ≠ consequence-reversibility. Trigger B is the only genuinely *new* predicate in the whole router (~5 lines + a path list).
- **The Architect's chicken-and-egg objection (semantic reversibility is itself a fallible agent guess) is handled by keeping QUORUM narrow.** `merge-to-main`/`delete`/`external-send`/`spend` are *syntactically* classifiable — pattern-match the command, no judgment. Only `architecture-change`/`product-north-star` need semantics, and those are exactly the ones routed to QUORUM where a fallible classification gets a second model anyway. The router never asks a stochastic agent to self-certify "this is reversible."
- **The Operator's bound is the safety the Architect's design omits.** A reversibility router over an *unbounded* loop is still a runaway. Three bounds make wrong-tolerance safe to leave running:
  1. **Reversibility boundary = the worktree/push line.** Tolerate wrong *inside the worktree* (retry freely). NEVER tolerate wrong *through `git push HEAD:main`* — that push is the one-way door and must call QUORUM. (`runSyncPushLanding` is the un-gated door today.)
  2. **Global budget.** Change `--max-cycles` default from `0` (unlimited) to a finite cap, and add a session token/agent hard-stop. "Cheap retry" must mean *cheap*, not *free* — free retry is the doom loop in a cheap-retry costume.
  3. **Persistent per-bead failure counter.** Extend `.agents/evolve/next-work-decisions.jsonl` with a cross-session fail count so Trigger C actually fires. Without persisted state, "retry K times then escalate" silently degrades to "retry forever." This is the navi's governor: the deterministic ground-truth that decides when the agent must STOP and surface to a human.

**Net:** the router is cheap-by-default (Minimalist), keyed on reversibility + trust-surface + repeated-failure (Architect), and made unattended-safe by the worktree/push boundary + global budget + persistent fail-counter (Operator). All three escalation primitives already exist in the repo; what is missing is the single decision point that consults them and the budget that bounds the loop.

---

## 3. STOP-DOING list (highest-value section — Bo's failure mode is addition)

Everything here is **demote-to-opt-in or freeze**, not necessarily delete-forever. The reversible move is to flag it off the default path; you can always flip it back.

| Machinery | Action | Why |
|---|---|---|
| **bdd-foundry on the default path** | **Demote to opt-in.** Keep the skill; remove it from "no bead without a runnable acceptance test." Auto-on ONLY past the one-way-door check. | It replaces cheap "discovery" with a waterfall in TDD costume (intent→Gherkin→executed-red→spec→DAG→cross-family validate→*then* a bead). Correct for the irreversible 5%; pure tax on reversible feature work. The single most expensive deviation from the loop that shipped. |
| **Provenance ledger (append on every event)** | **Demote to `--provenance`, default OFF.** | Tamper-evident audit trail for a record no one is auditing (it has **1 line**). A team/MTO-scale problem pre-paid at solo scale. |
| **Multi-family context-quorum / council context-floor gate** | **Demote: fire only at QUORUM tier.** | Load-bearing at a shared-main merge; pure tax on a solo reversible commit. The cost-law says exactly this; the code ignores it. |
| **`ao rpi loop --max-cycles 0` (unlimited default)** | **Change default to a finite cap.** | The single most dangerous line for unattended runs. Directly enables the runaway. |
| **`failure-policy=continue` as the unattended default** | **Gate on the per-bead fail counter.** | "Continue forever on failure" with no counter = the doom loop. |
| **Duplicated `.agents/` taxonomies** | **One subtraction pass, time-boxed to a day.** Collapse `handoff(s)`, `post-mortem(s)`, `pre-mortem(s)/pre-mortem-checks`, `retro(s)`. | Proof the OUTER loop accreted noise, not judgment. evolve has only ever added. |
| **The 633-file `cli/cmd/ao/` surface** | **30-day-zero-invocation deprecation flag.** "What did evolve add that nothing uses?" becomes a recurring evolve move. | The CLI is the cathedral. Subtraction must be a first-class loop move or evolve is just a slower cathedral-builder. |
| **MTO-handoff wiring (`.agents/mto-handoff`, two-factory docs)** | **FREEZE.** No further MTO work until AgentOps-alone ships substance. | Bo said forget MTO; the most recent merges are mid-sentence wiring it. |
| **The k8s-control-plane factory build (7-epic: HA store, API object model, reconcile controller, scheduler, self-healing)** | **FREEZE the unbuilt epics.** | Building a control plane for a factory that has shipped one ledger line. |

**Do NOT delete** (these are navis — deterministic ground truth, the windshield, not ceremony): `ao gate check --fast`, the real test runners, CI on main, the worktree isolation, `commitIfDirty`'s scoped staging. A failing real test stays. A quorum-of-models agreeing on reversible work goes. **The line is: delete rigor that PREVENTS wrongness; keep rigor that DETECTS it deterministically.**

---

## 4. The PM/EM/business question — emergent vs designed-up-front

**Resolved answer: the CONTRACT SHAPE is designed up front; the CONTRACT CONTENTS accrete via evolve. There is no standing committee of role-agents.**

- **Always-on PM/EM/business role-agents are rejected.** That is a one-person bureaucracy that ships nothing — the simulation-of-a-company the Shadow flagged. Judgment lives in **accumulated planning-rules + gates + the PROGRAM.md contract**, not a standing panel (the thesis's point 5, confirmed by the repo's own promotion ratchet: noticed-once dies at handoff → repeats-twice becomes a learning → must-never-regress becomes a gate).
- **What is genuinely load-bearing UP FRONT (the Product Lens caught this; the thesis half-missed it):** the **workload-object schema** — what a unit of work *is* and what its acceptance contract *is*. You cannot accrete your way to an API/object model. The factory doc already correctly flags this as the one open port that must be designed before anything codes against it. The PROGRAM.md *fields* (`Decision Policy`, `Escalation Rules`, and the new `Routing` section) are the up-front shape; their *values* accrete.
- **The critical amendment to the ratchet:** today it only ratchets **up** (every promotion adds rigor — a learning becomes a gate). For the factory to escape the cathedral, **the ratchet must be allowed to ratchet DOWN** — to promote a reversibility *exemption* (mark a work-class `cheap`), not only a constraint. That single change (the outer loop can REMOVE rigor) is the structural difference between a wrong-tolerant factory and a slower cathedral-builder. This is the "letting go" the thesis correctly named as harder to build than a gate.

So: design the bead + acceptance + PROGRAM.md shape once (it mostly exists). Let evolve fill the door-list, the planning-rules, and the cheap-exemptions over time. No role-agents.

---

## 5. The Shadow verdict — is building the factory the right work, or avoidance?

**Right now, it is avoidance — with a precise, falsifiable tripwire to tell when it flips back to real infrastructure.**

The repo states the case against itself: the provenance ledger has **1 line**, `git log | grep -iE 'mnemosyne|shield|client|deliver'` is **empty**, and 451 `.agents/*.md` files contain retire/supersede/reverse/teardown language. The factory has produced almost exclusively more factory, instrumented its own construction as if it were production, and reversed its own architecture nine ADRs deep. The cheapest, most reversible, most retryable, most judgment-free object in Bo's world is his own tooling — which is *precisely why the engine fled there for three months* ("am I enough?" made institutional: handed the whole answer, reach for the deeper architecture instead of standing on the ground).

**The tripwire (run it every morning):** *Name the external artifact this commit unblocks. If the honest answer is "the factory" — stop and go ship the artifact.*

- Factory-building is real infrastructure **iff** each factory change is *pulled by a concrete external deliverable that is actively shipping and is blocked on that exact change*. "I'm adding the router so the factory can ship" = procrastination when nothing is shipping. "Mnemosyne's release is blocked because the loop can't X, so I'm adding X" = real.
- **Drop factory work the moment** you cannot name a currently-shipping external artifact that a factory change unblocks. That has been the case for ~451 teardown docs and 130 ag-beads. The honest read today: **stop, ship one thing, then let the next factory gap be defined by what that shipping reveals.**

The router is correct *and* it can wait until something real is blocked on it. No router teaches wrong-tolerance. Only shipping-and-surviving-being-wrong does.

---

## 6. Smallest next real step

**Pick ONE external deliverable (Mnemosyne is the named, correctly-sized candidate — a hand-coded Go KV store) and drive it to a shipped, demonstrable, externally-judgeable state using the loop that EXISTS TODAY (`ao rpi` phased, or evolve→discovery→implement→validate by hand), with a hard rule: ZERO commits to `~/dev/agentops` for two weeks.**

This is the only step that makes contact with reality instead of producing another tool. The constraint *is* the medicine — the factory's gravity will pull every problem back to "I should fix the router first," and that pull must be mechanically forbidden. Restoring wrong-tolerance is not built; it is practiced by shipping a crude thing on the short loop and surviving it being wrong.

**The router work, when it returns, is then ~1–2 days, not a rebuild** — because the proof above shows it's an adapter, not a subsystem: one new file `cli/cmd/ao/route.go` calling the already-built `liveness.IsSignificantAction` + `rpi.ShouldForceEscalation`, one trust-surface path-guard, one `route:` field on the PROGRAM.md/autodev contract, and the finite `--max-cycles` default. File that bead; don't start it until a shipping deliverable is blocked on it.

---

### Load-bearing files (all absolute)

- `/Users/bo/dev/agentops/cli/internal/liveness/quorum.go` — the reversibility gate, built, **zero prod callers** (Trigger A/QUORUM source)
- `/Users/bo/dev/agentops/cli/internal/rpi/phased_gates.go` — `MaxGateRetryDepth=5`, `ShouldForceEscalation` (Trigger C source, also built, not wired to a router)
- `/Users/bo/dev/agentops/cli/cmd/ao/rpi_loop.go:127` — `--max-cycles` default `0` = unlimited (the unattended-safety hole)
- `/Users/bo/dev/agentops/cli/cmd/ao/rpi_loop_supervisor.go` — `runSyncPushLanding` (the un-gated push-to-main one-way door), scoped `commitIfDirty` (keep)
- `/Users/bo/dev/agentops/docs/provenance/ledger.jsonl` — 1 line (the factory has shipped one block)
- `/Users/bo/dev/agentops/skills/autodev/SKILL.md` + `/Users/bo/dev/agentops/cli/cmd/ao/autodev.go` — add the `## Routing` section / `route:` field
- `/Users/bo/dev/agentops/docs/architecture/the-agent-factory.md` — cost-law sidebar (unimplemented) + one-way-door table (the door-list seed) + the FROZEN unbuilt epics
- `/Users/bo/dev/agentops/docs/architecture/operating-loop.md` — the 7-move loop that collapses to 3 on the CHEAP path; the promotion ratchet that must be allowed to ratchet DOWN
- `/Users/bo/.claude/projects/-Users-bo-dev-agentops/memory/cost-law-quorum-at-gates-cheap-at-generation.md` — the router law, ratified, never wired
