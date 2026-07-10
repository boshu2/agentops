# Merge-Door Constraint / Queueing Model — measured, 2026-07-09

> READ-ONLY research artifact for the merge-door redesign. Theory-of-Constraints framing of the
> current merge-to-main system, built from MEASURED data over the last 48–72h (window ends
> 2026-07-09 ~20:00 ET). Data sources: `.agents/yield/yield-ledger.jsonl` (1,262 events; 65 in 48h),
> `docs/provenance/ledger.jsonl` (425 records; 55 verdict edges in 48h, with `duration_s`/`rounds`/
> `reviewer_family`), `git log origin/main` (101 commits/72h), `git reflog origin/main` (push
> completion timestamps — the key stage-4 measurement), `.agents/pawl-evidence/*` mtimes (149 files),
> surviving `$TMPDIR/agentops-prepush-race-*.log` (green logs are deleted → survivors = failures,
> with measured durations), and the gate scripts' own constants.

## 0. Headline numbers

| Metric | Value | Basis |
|---|---|---|
| Lands (CONFIRMED binds), last 48h | **28** (46 in 72h) | git log, `bind pawl CONFIRMED … #trivial` |
| Wall-clock throughput | **0.61 lands/h** (48h) | 28 / 45.8h span |
| Active-window throughput | **1.30 lands/h** (mean active inter-land gap 46 min; median 34) | gaps ≤180 min, n=22 |
| Effective WIP (Little's Law) | **λ·W = 1.30/h × 0.77h ≈ 1.0** | single-piece flow, by construction |
| Membrane on-path cost per land | **~14.1 min mean (8.1 review-to-bind + 6.0 bind-to-push) ≈ 30% of takt** | measured, below |
| Retry amplification | **1.79 model-review attempts per landed bead** (48h; 1.97 at 72h) | yield ledger chains |
| Pre-push deterministic gate failure rate | **~21%** (8 REFUTED gate runs / ~38 gate runs) | yield `deterministic-pre-push` + reflog push count |
| Reviewer (codex) resource utilization | **~3% of wall clock** | 50 calls × 106 s mean |

**TOC verdict: no resource is saturated. The binding constraint is a *policy*: synchronous
single-piece flow (WIP=1) through a serial door.** The orchestrator lane is the bottleneck resource
(≈100% utilized during active windows), and ~30% of its takt is spent idle-waiting on the membrane
(stage 3 + stage 4) rather than producing.

## 1. Stage table (measured latencies, variance, failure rates)

| # | Stage | Service time (median / mean / p90 / max) | Variance | Failure rate | Measurement basis & confidence |
|---|---|---|---|---|---|
| 1 | **Implement** (worktree agent + orchestrator) | ~24 min med / ~32 min mean (residual of takt) | high; inter-land CV 1.30 | rework folded into stage-3 retries | **Inferred as residual** (takt − stages 3+4). No direct start/stop events. Confidence: MEDIUM |
| 2 | **Fast gate** `ao gate check --fast --scope head` | ~40 s (code-comment constant, age-8ais) | unmeasured | not separately logged | scripts/check-pawl-pre-push.sh comment. Confidence: LOW (constant only) |
| 3 | **Cross-family pawl review** (feat-commit → CONFIRMED bind) | **5.0 / 8.1 / 15.9 / 46.3 min** (n=43) | stdev 9.1 min, **CV 1.12**; fat tail (pkrl 46 min, jgf9 29 min = stall/timeout episodes) | **per-attempt REFUTE ≈ 46%** (26 REFUTED / 57 model attempts, 48h — inflated by new membrane-catch logging); **per-bead: 36% of landed beads needed ≥2 attempts** | git log pairing feat→bind commits. Confidence: HIGH |
| 3a | — reviewer call alone (`duration_s` in provenance) | 68 / 106 / 230 / 391 s (n=28, 48h), CV 0.96 | bimodal: warm ~15–20 s cluster vs cold ~200–390 s cluster; duel (claude+gpt) 34–131 s | — | provenance ledger. Confidence: HIGH for the numbers; MEDIUM for semantics (07-09 regime shift ~15 s→~200 s, likely METER instrumentation change) |
| 3b | — evidence-written → bind commit | 0.2 / 2.4 / 12.0 / 35.4 min (n=26) | fat tail = operator batching | — | evidence mtimes vs bind ts. Confidence: HIGH |
| 4 | **Bind + rebase + push** (bind commit → push completed on origin) | **4.9 / 6.0 / 8.5 / 15.7 min** (n=24), stdev 2.6, **CV 0.43** (tight — mechanical) | low-variance | **~21% gate REFUTE per push attempt** (8 deterministic-pre-push REFUTEs / ~38 runs, 48h); ≥4–5 full-race-suite failure episodes in 72h (surviving logs) | reflog `update by push` vs bind-commit ts. Confidence: HIGH |
| 4a | — full race suite (`go test ./... -race -shuffle=on`) | **53–75 s measured** (5 samples; comment says ~77 s) | tight | survivors = failures: 6 logs w/ content in 72h | race-log birth→mtime. Confidence: HIGH |
| 4b | — cockpit per-commit re-target (age-8ais) | ~40 s × N non-trivial commits in push range (detached worktree each) | linear in train length | fail-fast | code comment constant. Confidence: LOW |
| 4c | — host push lock (age-2sog) | 0 observed wait at WIP=1; timeout 300 s; fail-closed | — | 0 observed | pre-push.local. Confidence: HIGH that it's currently free |
| 5 | **Bead close + ledger sync** (`br close` + `_beads` push + evidence/yield appends) | est. <1 min; off next-land critical path | — | — | not instrumented. Confidence: LOW |

Stage-3 queue-vs-service split: the reviewer call itself (`duration_s`, mean 106 s) is only **~22%**
of the 8.1 min feat→bind stage. The other ~78% is wrapper overhead — packet assembly, deterministic
pre-flight battery (age-…ebec.9 runs it BEFORE the model), live smoke, stall retries, and
orchestrator hand-off/bind mechanics. The model is not the slow part of the model review.

Script constants (declared capacity): `PAWL_REVIEW_TIMEOUT` default **300 s**; `scale_review_timeout`
= 300 + 2 s/KB over the 64 KB inline cap, **ceiling 900 s**; stall re-run at **450 s**;
`AGENTOPS_PUSH_LOCK_TIMEOUT` **300 s**. Reviewer route observed 72h: gpt (cold/warm codex) 62/79
verdict edges, claude+gpt duel 10, claude+gemini+gpt 5, gemini (agy) 1, claude 1 — **agy failover is
nearly unused (1.3%)**; codex-family availability is a single point whose stalls produce the stage-3
fat tail.

## 2. Bottleneck verdict

- Utilizations at current load (48h wall): codex reviewer ≈ **3%**; race-suite host CPU ≈ **2%**;
  push lock ≈ **4%** busy (38 runs × ~5 min hold-ish, and zero observed waiting). Nothing physical
  is near saturation.
- The **orchestrator lane is the constraint** (≈100% busy during active windows; everything else
  subordinates to it). Its 46-min mean takt decomposes: **~70% implement, ~17.5% stage 3, ~13%
  stage 4**. The membrane's entire cost is *latency serialization on the constraint*, not capacity.
- Little's Law: WIP ≈ 1.0. Cost of the synchronous door = 14.1 min × 28 lands ≈ **6.6 h of
  constraint-time per 48h spent waiting on verification** — capacity for ~9 more lands/48h at
  current implement times.

## 3. Retry amplification (measured)

- **1.79 model-review attempts per CONFIRMED landed bead** (48h: 50 attempts / 28 landed); 1.97 at
  72h. Distribution (48h): 18 beads ×1, 4 ×2, 2 ×3, 2 ×4, 2 ×5 (72h adds one ×10 chain).
- Attempt-level disposition (48h, model modes only): 26 REFUTED : 31 CONFIRMED (**~46% refute per
  attempt**). Caveat: the membrane-catch logging pipeline (landed 07-08/09, age-9931/age-ulab) made
  previously-invisible catches visible; the *land-level* refuted-bind ratio in git is 3 REFUTED
  binds : 46 CONFIRMED (6.5%) over 72h. Historical verdict-level refute ≈ 3%. Treat 46% as
  attempt-weighted current-truth, not a regression.
- Multi-attempt chain spans (first event → last): median 8.6 min, p90 50 min, max 8.8 h (n=16, 48h).
- REFUTE classes (48h): cli 9, docs 5, scripts 3, skills-codex 3, product-md 1, unclassed 13.
- Deterministic pre-push adds its own retry loop: 8 REFUTED gate runs in 48h ≈ +0.29 gate re-runs
  per land × ~6 min = **~1.7 min/land hidden retry tax** on stage 4.

## 4. What-if throughput (active-window basis, mean takt 46.2 min → 1.30 lands/h)

| Scenario | New takt | Active throughput | Delta |
|---|---|---|---|
| A. Stage 3 **off critical path** (post-merge async review; land on fast gate + pre-push only) | 46.2 − 8.1 = 38.1 min | **1.57 lands/h** | **+21%** |
| B. Stage 3 **2× faster in place** | 46.2 − 4.0 = 42.2 min | **1.42 lands/h** | **+9%** |
| A+ (reference): stages 3 AND 4 both async (post-land audit door) | 46.2 − 14.1 = 32.1 min | 1.87 lands/h | +44% |

Notes: (i) B is weak because the reviewer call is only ~2 min of the 8-min stage — a 2× model
speedup without killing the wrapper overhead yields <5%; (ii) A also moves the 46% attempt-level
refute loop off the constraint, but converts refutes into post-land reverts/fix-forwards — the
unmeasured cost is re-work context-switching, which this dataset cannot price; (iii) wall-clock
throughput scales the same ratios off 0.61/h. Real scaling beyond A requires **WIP > 1** (second
lane), which A enables (reviews no longer hold the lane).

## 5. Secondary constraints once stage 3 moves off-path (ranked)

1. **The pre-push gate (stage 4) — new largest membrane cost on path.** Measured 4.9 min median /
   6.0 mean per push + 21% failure re-runs → effective **~7.3 min/land ≈ 19% of the new 38-min
   takt**. Components to attack, in measured order: the ~3–4 min of non-race overhead (fresh `ao`
   build ×2, verify-pushed-commit-builds temp worktrees, cockpit re-target ~40 s × N commits,
   mutable checks + provenance inside the lock), then the 53–75 s race suite itself.
2. **Deterministic-gate failure rate (21%)** — each failure replays the full ~6 min. At 2× land
   rate this doubles in absolute cost; fixable independently of architecture (the failures are
   regen-drift/pawl-freshness classes, per the REFUTE class keys).
3. **Host push serialization (age-2sog lock + git non-fast-forward rebase-retry).** Free at WIP=1;
   at ≥2 lanes it becomes the physical serializer: with ~6 min gate-per-push the single-host
   ceiling is **~10–12 lands/h**, and queueing (CV 0.43 service, bursty arrivals CV 1.3) adds wait
   before saturation. The race suite is also single-host CPU-bound (cannot overlap two gates
   cleanly on one box).
4. **Bind-commit architecture.** 46 bind commits for 49 non-trivial commits (72h) = **2× commit
   inflation**, each bind itself pushed through the gate (train length N grows → 4b grows), plus
   the amend-into-bind trap class (age-…ebec.11). An async door that records verdicts out-of-band
   deletes this entire class.
5. **Ledger/evidence append contention** (yield-ledger, provenance chain w/ prev_hash, evidence
   files). Hash-chained provenance is order-dependent — concurrent lanes would contend or fork the
   chain. Currently zero observed contention (serialized by WIP=1 + the push lock). Unmeasured;
   becomes real only at multi-lane.

## 6. What could NOT be measured (honesty ledger)

- **Implement stage directly** — inferred as takt residual; no worktree-agent start events in the
  mined ledgers. The 70% share could shift ±10pts with a different active-window cutoff (180 min).
- **Stage-2 fast-gate cost/failure rate** — only the ~40 s code-comment constant; in-session runs
  are not logged.
- **Inside stage 3's 78% wrapper overhead** — cannot split packet assembly vs deterministic battery
  vs stall-wait from these ledgers (`duration_s` covers only the reviewer call; METER token/cost
  fields are ~zero-filled: 549 of 549 recent events have cost_usd=0, only 5 historical events carry
  real wall_clock_s/tokens_out).
- **Push-lock wait time** — zero contention observed at WIP=1; the multi-lane queue model is
  extrapolation, not measurement.
- **Post-merge revert/fix-forward cost under scenario A** — the decisive unknown for the redesign;
  no post-land-defect events exist in the window to price it.
- **07-09 duration_s regime shift** (~15–20 s → ~200 s constant-ish) — likely instrumentation
  semantics change with the METER/pre-flight landings, not a real 10× reviewer slowdown; unresolved.
- The 07-09 04:13Z provenance burst (29 edges in 4 s) is a trunk-bound backfill (age-0tn), excluded
  from all timing stats.

## 7. Model summary (one paragraph)

The merge door is an M/G/1-ish single-server line where the server is the orchestrator lane, not
any verifier: arrivals are self-paced (closed-loop, WIP=1), so there is no queue growth — the
entire membrane cost expresses as **served latency on the constraint** (14.1 of 46 min/land, 30%),
amplified by two retry loops (1.79× model reviews per land; 1.21× pre-push gate runs per land).
Verifier capacity is ~97% idle; therefore *elevating* the constraint means changing the policy
(async the door, then add a lane), not buying a faster verifier — a 2× reviewer yields +9% while
de-serializing yields +21% (stage 3) to +44% (stages 3+4), after which the pre-push gate
(~6 min, single host, 21% retry) is the next constraint at a ~10–12 lands/h ceiling.
