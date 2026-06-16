# Loop Inventory — every feedback loop in the system

> Built 2026-06-16 from a 4-scout sweep across agentops, mt-olympus, ntm,
> control-plane, dotfiles, ~/.claude. Purpose: enumerate ALL our loops *before*
> designing the knowledge loop, so the decision is informed. This is the
> "we've done this before" record — what exists, its state, and the lessons.
>
> **Frame:** we are doing **loop engineering** — DevOps' infinite loop, encoded.
> A *system* (stocks + feedback) not a *DAG* (flows, no loops). The unit is the
> loop; loops drive loops. Outer = **GOAL** (not gold — gold is knowledge
> substrate).

## The five loops (+ one ladder)

| # | Loop | Role | Driver / forcing function | State |
|---|------|------|---------------------------|-------|
| 1 | **GOAL (outer)** | decides *what* + persists until met | `GOALS.md` → `/evolve` → `/goal` (Stop-hook); ADR-0008 | ✅ working |
| 2 | **WORKFLOW (inner)** | *executes* the objective | RPI 7 moves; `/crank` for epics | ✅ working |
| 3 | **VERIFICATION (pawl)** | balancing loop — don't regress | Brownian ratchet; pawl gates at merge/accept | ✅ working |
| 4 | **KNOWLEDGE** | the stock + its feedback (mine→compile→retrieve→reward) | *(no forcing function on consume)* | ⚠️ **starved at consume** |
| 5 | **NUDGE / TIME** | keep loops turning, unstick | cron / ScheduleWakeup / NTM ticks | ✅ working |
| L | **PROMOTION ladder** | routes a lesson to its enforcement surface | post-mortem ratchet + finding-compiler | ✅ working |

**The headline: 4 of 5 loops work. The knowledge loop is the lone broken one — and it's broken at *consume*, not anywhere else.**

---

## 1. GOAL loop (outer driver)
- **What:** sets the objective and persists until met; decides what the inner loop runs.
- **Lineage:** `GOALS.md` (steering) → `/evolve` (the original autonomous outer loop, goal-driven) → `/goal` (Stop-hook condition, its current successor — what we used all session).
- **Where:** `GOALS.md`/`PROGRAM.md`/`AUTODEV.md`; `skills/evolve/SKILL.md`; ADR-0007 (deterministic), ADR-0008 (intelligent-agile: re-reads intent each cycle, **cannot redirect outside operator-set goals**).
- **State:** working. `/evolve` perpetuates via ScheduleWakeup (270s productive / 600s scout / 1800s idle) or cron; hard stops never reschedule.
- **Lesson:** the loop is *deterministic* — only the operator's goal moves it; circuit breakers escalate, never auto-redirect.

## 2. WORKFLOW loop (inner — RPI)
- **What:** the 7 moves — BDD intent → bead → vertical slice → TDD per slice → conflict-free wave → close-by-acceptance → capture evidence.
- **Where:** `docs/architecture/operating-loop.md` (spine); `skills/rpi` (orchestrator), `skills/{discovery,plan,crank,validate}`; intent-to-loop-hexagon.
- **State:** working/stable. RPI delegates each move, enforces 5 invariants (no move-skip, test-first, acceptance>activity, ports visible, context density across handoffs). Jump in/out via `--from`.
- **Lesson:** chaos between pawls; validation cadence is pawl-gated, not per-tread.

## 3. VERIFICATION loop (pawl / Brownian ratchet — the balancing loop)
- **What:** chaos + filter + ratchet = progress. Pawls are the one-way doors where the gate fires; the ratchet locks gains so they can't slip back.
- **Where:** `docs/contracts/pawls.md`, `docs/brownian-ratchet.md`; `/pre-land-refuters`; `scripts/reconcile-pr.sh`; `schemas/pawl-verdict.v1.schema.json`.
- **Pawls:** mutate-shared-trunk, delete, external-send, schema/contract change, credential change, spend. **Blast-radius rule** is the authority (mutates shared state / changes gate logic / external effect / hard to roll back).
- **Diversity:** fresh-context default (≥1 fresh red-team, model-agnostic); multi-model opt-in (≥2 families) for highest-irreversibility doors.
- **Escalation:** circuit-breaker model — REFUTED auto-redoes; escalate to human only on max-attempts / time / cost / oscillation / explicit-judgment.
- **State:** working. **Frontier: the meta-pawl** (validate the validator; terminate the regress) — `mt-olympus/docs/plans/meta-pawl-brief.md`.
- **Why it matters for pay-it-forward:** this is the loop that stops the reinforcing loops from compounding *slop*. A pay-it-forward system without this pawl drifts; with it, the stock is monotone (gold ratchets in, slop can't).

## 4. KNOWLEDGE loop ⚠️ — the starved one
- **What:** produce → refine → compile → retrieve → reward. The stock that should make every next pass start richer.
- **Parts (ALL built):**
  - **Produce:** `forge` (transcript→`.agents/knowledge/pending`), `curate`.
  - **Refine/promote:** `flywheel` close-loop scoring (Gold>0.85/Silver/Bronze), age+citation gates → `.agents/learnings`.
  - **Compile→gold:** `ao wiki gold` (`.agents`→`.ao/wiki`, sanitize+durability-gate+OKF), auto-run by `ao compile`; `ao lookup --gold` (shipped THIS session).
  - **Retrieve:** decay-ranked scoring (`scoring.go`: freshness `exp(-0.17·weeks)`, maturity weights, multi-feature relevance). Scores **0.93 synthetic.**
  - **Reward:** citation → EMA utility (`feedback.go`) → `reward_count`/maturity (`ratchet/maturity.go`: provisional→candidate@≥3→established@≥5).
  - **Eval:** `eval-outcomes` + holdout — but it never measures "does retrieval help."
- **State: BUILT BUT STARVED.** The Apr-2026 `flywheel-audit.md` measured **0% cross-session citation** — 2,275 learning files across 14 workspaces, *zero* cited by any later plan/pre-mortem. Live retrieval scores **0.13** (vs 0.93 synthetic).
- **Root cause — the over-correction:** the v2.x **bookend push** failed (ADR-0002: a trivial RPI cycle hit **10.35M tokens @ 97.6% cache-read / $7.48**, and the **A/B eval showed injected context delta = 0** — "the problem was never hooks; it was noise stacking in the prompt"). 3.0 moved to **JIT pull**… but then **nothing pulls.** Injection was disabled (post-`ag-8km`) with no replacement, and **no decision-point skill calls `ao lookup`** (pre-mortem is the lone exception). The pendulum swung **spray → silence.**
- **The diagnosis:** this is the **only loop with no forcing function on its consume step.** Goal is forced by the Stop-hook; workflow by RPI's mandatory phases; verification by the merge gate; nudge by the timer. Knowledge-consume relies on an agent *choosing* to retrieve — and it doesn't. **Enforce the knowledge loop = give consume a forcing function**, the way every other loop has one.

## 5. NUDGE / TIME loops (keep loops turning)
- **What:** the time/event drivers that fire the other loops.
- **Surfaces:** `/loop` (in-session, fixed-interval or self-paced); **ScheduleWakeup** (in-session perpetuation, cache-aware delays); **CronCreate / shell cron** (off-session, survives session death); **NTM/ATM tending** (8-step tick: baseline→attend→classify→score→act→verify→stop→log; the Liveness Truth Stack); **continuity-loop** (renewal ticks, 2-tick stall rule, `.agents/continuity/state.json`).
- **State:** working (continuity-loop experimental). Temporal tiers: scheduling (hours/days) → autonomous (5–30 min, evolve) → operational (15–30 min, NTM tending) → workflow (3–5 min/phase, RPI).
- **Lesson:** one continuous machine with multiple control surfaces, not a cascade of systems. All honor the same kill markers.

## L. PROMOTION ladder (routes a lesson to its surface)
- once→handoff / twice→`.agents/learnings` / changes-behavior→`SKILL.md` / must-never-regress→gate / doctrine→`PRODUCT.md`. Plus **R3: no durable learning without a constraint** (`check-ratchet-r3-constraint.sh`). Enforcement-strength routing (AGENTS.md / skill / hook — weakest that changes behavior) added to the post-mortem skill this session.
- **Where:** `docs/architecture/operating-loop.md#the-promotion-ratchet`, `docs/contracts/finding-compiler.md`, `.agents/findings/`.

---

## The decision this inventory points to

**Don't reopen the bookend.** The history is unambiguous: bookend *push* → delta=0 + 10.35M tokens. The fix is not "force mining at close-out."

**The knowledge loop's three cadences are different and must be decoupled:**
1. **PRODUCE (harvest the session-end gold):** async + gated, decoupled from the session boundary — a separate loop on its own timer (nudge tier), not a synchronous SessionEnd. The end-of-session agent is the highest-knowledge *and* most-biased context → the harvest must pass the **pawl** (independent verification), or it pays forward confident slop.
2. **CONSUME (feed the next agent):** **JIT pull at the point of need, wired as a mandatory step in the decision-point skills** (discovery/plan/pre-mortem call `ao lookup --gold`) — a *forcing function*, the thing the knowledge loop uniquely lacks. Never bookend-push.
3. **MEASURE (does it help):** the eval/holdout closes the loop empirically — did the next agent avoid the mistake? Without this, you can't tell gold from slop accumulating.

**One line:** every other loop is enforced by a forcing function; the knowledge loop isn't. Give *consume* a forcing function (mandatory decision-point retrieval), keep *produce* async+gated, and let the eval prove it — and the dormant reward/maturity machinery lights up the moment retrieval happens.

## Open frontiers (named, not yet built)
- **Knowledge-consume forcing function** — the core gap above.
- **Meta-pawl** — validate the validator (recursion termination).
- **Eval measures retrieval value** — the missing empirical link.
- **Auto-ingestion of session lessons** — so feed-forward isn't manual/lossy (this session proved manual is lossy).
