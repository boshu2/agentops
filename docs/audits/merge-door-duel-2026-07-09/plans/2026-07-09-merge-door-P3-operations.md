# PerspectivePlan P3 — Merge-Door Redesign, LENS = OPERATIONS / MIGRATION

> 2026-07-09. Inputs: `.agents/research/merge-door-constraint-model.md` (R1),
> `merge-door-rebaseline.md` (R2), `merge-door-design-space.md` (R3). Target architecture =
> R3 Composite A (two-phase done + WIP cap + compensators), B/C later. This plan is HOW to get
> there without ever breaking the working system: every rung is one behavior, measured
> before/after, with a literal rollback. Rule: a rung must hold ≥1 week or ≥20 lands (whichever
> is later) with its metric moving the predicted direction before the next rung starts.

**Baseline (R1, measured, 48–72h window ending 07-09):** 1.30 lands/h active · 46-min mean takt
· 14.1 min membrane wait/land (30% of takt) · 1.79 model-review attempts/land · 21% stage-4
deterministic-gate failure rate · stage 4 = 4.9 min median (6.0 mean) per push · reviewer
utilization ~3% (nothing physical saturated — the constraint is the synchronous-door *policy*).

---

## 1. Migration ladder — one behavior per rung, each reversible

**R0 — Fix the METER instrumentation (day 0, before anything else).**
R1 found the cost fields are dead: 549/549 recent yield events carry `cost_usd=0`; only 5
historical events have real `wall_clock_s`/`tokens_out`; the 07-09 `duration_s` regime shift
(~15s→~200s) is unresolved instrumentation semantics. The GOALS.md:242-244 data gate and every
before/after claim below read this meter — **a broken ruler blocks the whole ladder.** Behavior:
usage events emit real tokens/cost/wall-clock from the harness (never self-report, per D17);
resolve the duration_s semantics and document what it measures. No flow change.
*Rollback:* `git revert <meter-fix-sha>` — fields return to zero-fill; harmless, nothing consumes them yet.

**R1 — Land the observation surface (coordinate, don't duplicate).**
`ao yield report` (pending-window depth + andon queue) is **NOT on origin/main** — verified:
`git show origin/main:cli/cmd/ao/yield_report.go` → does not exist; it lives on
`worktree-agent-adc473da4b1957eb8`, bead `age-mv67` OPEN. This rung = help age-mv67 land, then
add the verified-frontier / pending-window fields the cap (R4) will read. No flow change.
*Rollback:* revert the landing commit; the ledger (source of truth) is untouched.

**R2 — Pilot trailing-bind on the docs/#trivial (L0) lane.**
The lane is already effectively async via the #trivial waiver (`scripts/lib/trivial-waiver.sh`);
this rung only exercises the *bookkeeping*: push first, bind the verdict to the landed
`merge_sha` after, verify-landed, close. Proves trailing-bind + `check-tip-verdict-ci.sh`
grace-window semantics end-to-end with zero semantic risk. One prose/skill edit in the land flow
for that lane only.
*Rollback:* revert the skill/prose commit — lane returns to bind-before-push.

**R3 — Flip the default order: push-then-pawl, global WIP cap N=3.**
The one-way-door rung — but operationally it is an *ordering* flip, not architecture: pre-push
keeps the full deterministic battery (unchanged floor: main stays deterministically green per
commit); the pawl reviews the *pushed* sha; bind trails; **close stays gated on verdict**
(no-verdict = not-done moves to closure, where it always really lived). New: one deterministic
pre-push check — count commits on origin/main past the ledger-derived verified frontier; block at
`AGENTOPS_PENDING_CAP` (default 3; **unset = check disabled**, that's the kill switch). Cap is
global-per-repo (ledger-derived), never session-local. Cap-hit message must *route* ("drain:
run pawl review on the oldest pending sha"), not merely refuse. N=3 is consistent with measured
data already in hand (λ=1.30/h × W=0.235h → L≈0.3; 3 ≈ 10× steady-state, 2–3× burst headroom) —
this sizing uses latency/throughput data the ledgers already carry, NOT the phase-2 cost data
behind the D17 gate. Requires the R2-mapped contract amendments (pawls.md:12/:197/:21,
GOALS.md:215, schema post-land binding mode) in the same slice — prose follows executable.
*Rollback:* unset `AGENTOPS_PENDING_CAP` + revert the order-flip commit in
`pawl-review`/`check-pawl-pre-push.sh`/AGENTS-WORKFLOW. Nothing structural destroyed; pending
window drains under the old rules (review + bind each already-landed sha, then resume).

**R4 — Compensators, in three independently-removable sub-slices.**
(a) **Auto-file P0 fix bead on REFUTED-after-land**, carrying the defect list, bound to the
refuting verdict (default path — the tree is still deterministically green). (b) **Mechanical
revert L0 lane** for escalation classes (security/contract/schema/public-surface): clean
`git revert <sha>` verified deterministically as the inverse patch, bound to the *existing*
REFUTED verdict id — no second reviewer round-trip. Honors the refinery's prior art: **no repro →
HOLD, never auto-revert** (18–30% flake rate is why blind auto-revert was rejected;
`land-lane-flaky-retry.sh` classifies flake vs defect first). (c) **BC freeze valve**: unresolved
REFUTED in a BC → that BC's cap→0.
*Rollback:* each is one commit; revert independently (see table §6).

**R5 — Pre-push gate cost-down (the NEXT constraint, per R1 §5 — sequence AFTER R3 when the
meter shows stage 4 ≈19% of the new takt).** In measured-payoff order:
(a) **Single `ao` build per push** — the hook builds a fresh binary and the pawl gate builds
again; build once, thread via `AO_BIN`. (b) **age-wy2t range-collapse adoption** — `ao gate
check --scope range <base>..<head>` is LANDED (bead CLOSED 07-02) but `check-pawl-pre-push.sh`'s
age-8ais loop still re-targets the cockpit per non-trivial commit in a throwaway worktree
(~40s × N); adopt the range scope where per-commit isolation isn't semantically required.
(c) **Bind-commit dedup** — R1 measured 2× commit inflation (46 binds / 49 non-trivial commits,
72h), and each bind lengthens the next push's train; after R3 the verdict already trails
out-of-band, so batch binds (one provenance commit per drain cycle) or record verdicts
ledger-only with a periodic bind. Also attack the 21% gate-failure replay tax here — the REFUTE
classes are regen-drift/pawl-freshness, fixable independent of architecture.
*Rollback:* each is one commit; revert restores dual-build / per-commit loop / per-land binds.

**Deliberately NOT on the ladder:** ebec.4 router, ebec.5 context diet, ebec.6 cheap-tier
(D17-gated, §3); ebec.10 delta re-review (epic-deferred to fresh-mind + council); merge-queue
infrastructure (R3 survey: LOW fit); any in-repo daemon (ADR-0009 — async review runs as a
sanctioned shape: NTM tick / launchd on the Mac / `land-lane-run.sh --watch` thin loop).

## 2. Measurement plan per rung

Discipline: capture the before-number the day the rung lands; publish before/after at the rung's
gate (≥1wk / ≥20 lands). Primary surfaces: `.agents/yield/yield-ledger.jsonl` (accept /
gate-verdict / usage events), `docs/provenance/ledger.jsonl` (verdict edges, duration_s,
reviewer_family), `git reflog origin/main` (push timestamps), `ao yield report` once landed,
`scripts/verification-economics-report.sh` weekly.

| Rung | Expected delta | Proving fields / method |
|---|---|---|
| R0 | 0 flow change; `cost_usd`>0 and sane `wall_clock_s` on ≥95% of new usage events; duration_s semantics documented | yield `usage` events; provenance `duration_s`; spot-audit 10 events vs harness logs |
| R1 | 0 flow change; report prints pending-window depth + andon queue | `ao yield report --json` non-empty on live ledger |
| R2 | 0 takt change (lane already async); 100% of pilot binds reference post-push `merge_sha`; CI backstop 0 false annotations | git log bind commits vs reflog push order; verdict-backstop.yml output |
| R3 | Takt 46→~38 min; active throughput 1.30→~1.57 lands/h (+21%, R1 scenario A); membrane on-path wait 14.1→~6 min (stage 4 only); pending window ≤3, p90 time-to-verdict measured fresh | inter-land gaps from git log; feat→bind latency distribution (now off-path — track separately); cap-hit count in pre-push output; REFUTED-after-land count (expect ≈ 2.8–6.5% of lands — this is the decisive unmeasured number R1 §6 flagged; it prices the whole design) |
| R4 | Time-from-REFUTED-to-fix-bead < 5 min (auto); reverts only in escalation classes; 0 revert-of-revert | yield gate-verdict REFUTED events → br bead creation timestamps; git log `revert` commits bound to verdict ids |
| R5 | Stage 4: 6.0→≤3.5 min mean; gate-failure replay tax 1.7→<1 min/land; commit inflation 2×→~1.1× | reflog bind→push deltas; deterministic-pre-push REFUTE rate; binds/feat-commits ratio |

**Escape SLO guard (all rungs):** overturns are auto-detected by the existing escape reader
(`cli/internal/yieldledger/escape.go` — CONFIRMED then later REFUTED, pure ledger read; needs no
schema change). Any post-land escape on a *closed* bead = andon, regardless of rung metrics.

## 3. The ebec.4 / ebec.10 data gate — wait it out, escalate explicitly, never silently bypass

GOALS.md:242-244 is a **binding honesty rule**: "Phase-2 work (risk router, context diet,
cheap-tier default) stays DEFER until the ruler reads real numbers" — thresholds only from ≥2
weeks of meter data. This plan's position:

- **The ladder does not need the gate opened.** Rungs R0–R5 are Composite A + gate cost-down;
  none routes by risk tier or cheapens a review tier. The WIP cap N is sized from
  latency/throughput data already measured (R1), not from the phase-2 cost ruler. So the plan
  **waits the gate out by default**: R0 fixes the meter on day 0; the 2-week clock runs during
  R1–R3; by the time A is steady-state (~day 14–21) the ruler has real numbers and ebec.4 can be
  promoted through its own done-criteria (20+ routed closes, log-only first — R3 survey's
  Composite B migration).
- **If the operator wants the router sooner** (e.g. to shrink the pending window's L2 share
  before the flip): that is a named **andon item — "promote ebec.4 out of its D17 data gate ahead
  of meter maturity"** — escalated to Bo as an explicit one-way-door decision with the gate text
  quoted. Not a plan default, not a silent bypass, not "the data is probably fine."
- **ebec.10 (delta re-review)** stays behind its own epic-prescribed door (fresh mind + council)
  regardless of meter state; it is C-stage, last, and never a prerequisite (R3 survey §6).
- One consequence stated honestly: R3's pending window is **the only named source of real escape
  data** (ADR-0011 revival fuel). Waiting the gate out costs nothing here — the window produces
  that data whether or not the router exists.

## 4. Concurrent-session operations (day-1 rules for lane #2)

The multi-writer discipline is the **already-built land queue (epic agentops-2pl)** — do not
invent a second one: `scripts/land-submit.sh` (pushes bead branch to
`refs/heads/land-queue/<bead>` + appends `.agents/land-queue/requests.jsonl` under mkdir lock) →
`scripts/land-queue-next.sh` → `scripts/land-lane-run.sh` (the single serialized writer that owns
main; singleton lane lock; host-pinned to the always-on Mac; `--drain|--once|--watch`; file
backend, deliberately not AM-dependent). The age-2sog push lock stays as the inner belt.

**What the second session does differently, day 1:** (1) never pushes main directly — it
`land-submit.sh`s and walks away; only the lane pawl-lands; (2) works in a worktree-per-bead off
fresh origin/main — never edits the canonical checkout under swarm load (this session's collision
class: two sessions grinding the same shared checkout; check `git show origin/main:<file>` before
re-grinding anything); (3) at ≥2 writers, Agent Mail reservation before any hot-path edit —
partition-before-lock, one-writer-per-hot-dir; (4) reads the pending window (`ao yield report`)
before submitting — at cap, it becomes a **drainer** (runs the oldest pending review), not a
queue-jumper; (5) ledger appends (provenance prev_hash chain, yield ledger) are lane-owned —
sessions never append provenance concurrently; the hash chain is order-dependent and forks under
concurrent writers (R1 §5.5). WIP>1 is the *payoff* of R3, but only through the lane.

## 5. Observability runbook (on-the-loop surface)

- **Daily / per-drain:** `ao yield report --since 24h` — **NOTE: not yet on origin/main
  (age-mv67 open, branch `worktree-agent-adc473da4b1957eb8`); landing it is ladder rung R1 —
  coordinate with that lane, don't duplicate.** Reads: pending-window depth vs cap, andon queue
  (blocked beads, ESCALATE/HOLD, REFUTED-with-open-bead), verified-frontier sha (the only sha
  releases/tags may reference).
- **REFUTED-after-land alerting:** the escape reader over the yield ledger is the detector
  (existing shape, zero schema change). Alert = andon-queue row + auto-filed P0 bead (R4a).
  Repro-required rule: REFUTED without a runnable repro → HOLD, never compensation (degraded
  warm-pawl false-REFUTEs; cold fallback `PAWL_NO_SERVICE=1`).
- **Revert-storm andon (stop-the-line, human):** ≥2 reverts in one 24h window, OR any
  revert-of-revert, OR cap pinned at N for >4h with a live drainer (reviewer capacity is the
  constraint, not discipline), OR ≥3 same-class refutes (existing scope-grind rule).
- **Weekly review (operator, ~20 min):** `scripts/verification-economics-report.sh` (TPVD/VOR/
  CPCD once the meter is live) + `ao yield gauge` + `ao membrane digest --deltas` (per-class
  recurrence) + the rung gate decision: metric moved as predicted → next rung; didn't → hold or
  roll back, never proceed on vibes. D17 gate check: 2 weeks of real meter data yet? → ebec.4
  promotion decision (per §3).

## 6. Rollback drill — literal reversions per rung

| Rung | Rollback (literal) | Residual after rollback |
|---|---|---|
| R0 meter | `git revert <meter-sha>` | cost fields zero-fill again; no consumer breaks |
| R1 report | `git revert <yield-report-sha>` | ledger intact; lose the surface only |
| R2 pilot | `git revert <lane-prose-sha>` | L0 lane binds pre-push again; waiver unchanged |
| R3 flip | **unset `AGENTOPS_PENDING_CAP`** (kill switch, immediate) then `git revert <order-flip-sha>`; drain the pending window: pawl-review + bind each unverified landed sha oldest-first before resuming old flow | contract prose reverted in same commit; no landed commit is ever rewritten (forward-only, dcg-compliant) |
| R4a auto-file | `git revert <autofile-sha>` | REFUTED-after-land handled manually via andon queue |
| R4b revert lane | `git revert <revert-lane-sha>` | escalation classes fall back to incident runbook (`git revert --no-commit ${GOOD_SHA}..HEAD`) |
| R4c freeze valve | `git revert <freeze-sha>` (or per-BC cap override env) | window can stack on a known-bad BC — watch andon |
| R5a single build | `git revert <sha>` | dual `ao` build returns (~+40–60s/push) |
| R5b range-collapse | `git revert <sha>` | per-commit ~40s×N loop returns |
| R5c bind dedup | `git revert <sha>` | 2× bind inflation returns |

Every rollback is a forward `git revert` or an env unset — no reset/force-push (dcg + pawl
contract: a shared-ref rewrite is itself a multi-model door we never open on this ladder).
