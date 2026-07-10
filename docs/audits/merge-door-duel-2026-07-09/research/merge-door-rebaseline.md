# Merge-door re-baseline — what exists, what binds, what must change

> READ-ONLY research pass, 2026-07-09, for the post-merge-review / trunk-based-push redesign.
> Every claim carries file:line against the working tree at 68d10a671 (local main, ahead 1 / behind 3
> of origin/main cdb9a4e5b). Where a surface lives only on an unlanded branch, that is stated.

---

## 1. The pawl contract (docs/contracts/pawls.md) — EXISTS; post-merge review NOT expressible without amendment

**What "mutate shared trunk" binds.** The pawl row is `docs/contracts/pawls.md:21`: *push/merge into the
shared trunk; close a bead as accepted; or rewrite a shared ref* — guarded by
`scripts/hooks/pre-push.local → scripts/check-pawl-pre-push.sh` on push-to-main (pr=0), `/pre-land-refuters`,
and the PR path `scripts/reconcile-pr.sh` requiring CONFIRMED via `scripts/pawl-verdict.sh check`.

**The timing clause is PRE-action, explicitly.** `pawls.md:12`: an independent fresh-context reviewer
*"must confirm before the action proceeds. Fail-closed (ambiguity → hold, never silent-proceed)."*
And `pawls.md:197`: *"When a breaker trips the action does **not** proceed: the merge/push is **held**, not
landed, not retried-into-landing… Non-convergence **never auto-lands**."* So **post-merge verification is
NOT expressible inside the current contract** — the contract's whole mechanism for this pawl is
hold-before-door. Amendment required (see §"one-way-door surface" at the end).

**Escalation / circuit-breaker model** (`pawls.md:178-199`): PASS → proceed; REFUTED → AUTO-REDO loop, no
human (`:187`); human only on a tripped breaker — max-attempts (default 3), time budget, cost/quota budget,
oscillation/no-forward-progress, explicit judgment flag (`:188-193`); same governor as the evolve breakers
(`scripts/evolve/halt-check.sh`, `:195`). Merge path records ESCALATE/HOLD and exits **5 = HOLD: no merge,
no close** (`:197`; exit-code confirmed in `scripts/pawl-verdict.sh:53`). This model is timing-agnostic —
reusable verbatim post-merge; only the *held object* changes (today "the merge", post-merge it would be
"the revert/quarantine decision").

**Directive precedence** (`pawls.md:203-212`): no autonomous drive directive overrides a HOLD or human
STOP. Also timing-agnostic; reusable.

**#trivial waiver.** Single shared implementation `scripts/lib/trivial-waiver.sh`
(`check-pawl-pre-push.sh:51-57`), used by BOTH the pre-push gate and the CI backstop
(`scripts/check-tip-verdict-ci.sh:18-21`) so the two surfaces cannot drift. Waiver = #trivial marker in
subject AND diff provenance-only (`docs/provenance/` only; dual test in `scripts/pawl-land.sh:54-68`).
Memory: waiver is docs/provenance-only diffs ONLY (bypass closed).

**Diversity floor** (`pawls.md:66-96`): `fresh-context` is the DEFAULT (≥1 refuter with
`context_id != author_context_id`, model-agnostic, `:82`); `multi-model` is OPT-IN, strictly stronger
(fresh floor PLUS ≥2 roster-validated families, `:83`); *"Routine mutate-shared-trunk merges ride the
cheap default"* (`:90-91`). **Contract-internal contradiction:** `pawls.md:229` says *"A high-irreversibility
door (push-to-main) demands `multi-model` and refuses a `fresh-context` verdict"* — but the executable
(precedence 1) does NOT refuse: `check-pawl-pre-push.sh:283-285` authorizes a fresh-context verdict with
only a printed NOTE. The executable matches `:90-91`, not `:229`. A redesign must resolve this clause pair
either way.

**Evidence-quality floor** (`pawls.md:98-131`): CONFIRMED must carry a file:line finding or a
reviewed-scope attestation, plus per-adapter genuine-run markers (codex=`tokens used`, agy=`VERDICT:`);
advisory until `FLOOR_ENFORCE_AFTER` (`pawl-verdict.sh:520`, warn paths `:635-666`). Timing-agnostic; reuse.

**Strict two-family cold quorum** (`pawls.md:231-270`): `ao verify --strict` exists but is honestly
UNAVAILABLE today — `STRICT_ELIGIBLE_FAMILIES` is codex-only; agy not strict-eligible; no cold claude
adapter (LAW 0) (`:251-265`). Do not design a post-merge tier that assumes an active 2-family cold quorum.

---

## 2. Enforcement points — EXISTS (fully mapped)

**`scripts/hooks/pre-push.local`** (tracked source; installed via `scripts/install-pre-push-gate.sh`,
`:4-6`). Order on push-to-main:
1. Bypass check `AGENTOPS_GATE_DISABLED=1` — audited, logged to `agentops-gate-bypass.log` (`:78-87`).
2. `go build ./...` worktree compile proof (`:110-113`); then per-commit isolated-worktree build of each
   pushed commit when worktree ≠ HEAD (`scripts/verify-pushed-commit-builds.sh`, age-yy24, `:120-122`).
3. Fresh per-run `ao` binary built to a mktemp path (`:123-130`).
4. **FULL race suite** `go test ./... -race -shuffle=on -count=1` (~77s) — push-to-main only, because
   *"Push-to-main is the SOLE routine CI authority (GHA is a tag/PR/manual backstop)"* (`:132-173`);
   plus the cmd/ao integration race shard (`:176-184`). Bypass: `AGENTOPS_PREPUSH_SKIP_FULL_RACE=1`.
5. **age-2sog serial push lock** `acquire_push_lock` — mkdir advisory lock at
   `$TMPDIR/agentops-push.lock`, FAIL-CLOSED after `AGENTOPS_PUSH_LOCK_TIMEOUT` (300s) (`:54-76`);
   build/test deliberately OUTSIDE the critical section (`:55-58`); signal-trap release for INT/TERM
   (age-genn, `:39-47`). Note the lock serializes only *mutable side effects*, acquired AFTER the race
   suite (`:189`).
6. `ao gate check --fast` (`:203-206`).
7. **Pawl gate** `scripts/check-pawl-pre-push.sh` with the fresh binary via `AO_BIN` (age-jmfl, `:208-218`).
8. On BLOCK: deterministic-tier SENSOR feed `scripts/emit-deterministic-catch.sh` records a REFUTED
   gate-verdict so the deterministic tier isn't undercounted (age-srl, `:229-236`; script exists, 3.7K).

**`scripts/check-pawl-pre-push.sh`** (PAWL-HOLD logic):
- Main-ref only, delete-push skipped (`:114-124`, `:225-226`).
- #trivial tip → **age-8ais mixed-range gating**: enumerate `remote_sha..local_sha`, re-target the cockpit
  gate (`ao gate check --fast --scope head`) at EVERY non-trivial commit, each inside a throwaway detached
  worktree whose HEAD *is* that commit, newest-first fail-fast (`:8-36`, `:148-181`, `:183-220`, `:236-265`).
  Pure-#trivial range = byte-identical fast path (`:251-252`). ~40s per hidden commit, by design (`:25-27`).
- Non-trivial tip → bead id extracted from commit message, trailer-precedence rules (`:67-112`), then
  `pawl-verdict.sh check <bead> 0 --head <sha>` (`:271-289`); no bead cited = PAWL-HOLD fail-closed (`:271-274`).
- Tier surfaced, fresh-context = NOTE not refusal (`:278-286`).
- `PAWL_PREPUSH=1` exported so no descendant creates a commit mid-push (re-entrancy, `:303-308`).
- Skips: `AGENTOPS_PREPUSH_SKIP_PAWL=1`, no stdin (`:292-301`).

**Go gate registry** (`cli/internal/gates/`): `Tier` is a bitmask `Fast | Full` (`gates.go:24-32`) —
Fast = cockpit/pre-push, must stay seconds (`gates.go:27-29`); NOT "Blocking tier" — **Blocking is an
orthogonal bool** (`gates.go:66-68`, FAIL on blocking check fails the run; non-blocking advisory).
Checks self-register from `cli/internal/gates/checks/seed.go` (e.g. always-on blocking:
`always.mutation-route`, `always.agents-write-surfaces`, `always.door9-no-claude-p`,
`always.no-tracked-agents` `seed.go:239-242`; path-routed: `skill.schema` `:277`,
`contract.registry-drift` `:279`; Full-only: `go.lint` `:276`, advisory `shell.preamble-ratchet` `:257`).
Fast-mode selection is path-glob routing (`gates.go:62-65`); empty Match = always-run (`gates.go:90`).

**Reuse implication:** the per-commit detached-worktree re-target (`gate_nontrivial_commit`,
`check-pawl-pre-push.sh:183-220`) is exactly the mechanism a post-merge async reviewer needs to evaluate a
landed commit "as its own tree" — it already exists and is production-proven.

---

## 3. Directive 17 / verification economics — RULER LANDED; router/diet/cheap-tier DESIGNED + DATA-GATED

- **GOALS.md Directive 17** at `GOALS.md:225-253`: TPVD / VOR / CPCD / escape SLO; *"Spend to the budget,
  not to zero"* (`:234-235`); binding honesty rules — thresholds only from ≥2 weeks of meter data, meter
  reads the harness never self-report (`:237-240`); **"Phase-2 work (risk router, context diet, cheap-tier
  default) stays DEFER until the ruler reads real numbers"** (`:242-244`). Landed: report instrument +
  warn-only gate `b0149df9a` (bead .2; live 322 verdict edges, 2.8% refute) and benched-family stall-tax
  removal `335300b17` (bead .7) (`:246-249`). Report script exists: `scripts/verification-economics-report.sh`.
- **Assessment doc** `docs/audits/verification-economics.md`: status = assessment, not doctrine, promote
  via ADR after 2 weeks of meter data (`:3-4`); the discrimination-price table (`:21-27`); measured refute
  rate ≈3% current regime vs 23.8% in the 6-22 window, 0 escapes ever (`:44-48`); cheap tier caught planted
  subtle bugs 6/6 unaided (`:49`); p50 landed diff 3.2KB ≈ 800 tokens (`:50`); lever map L0-L4 + Router(R,
  bead .4) + Context diet(D, .5) + memoization(M) + shift-left(S) + adaptive stopping(A) + sampling(P)
  (`:124-136`); roadmap table (`:196-206`).
- **Bead status** (br, 2026-07-09):
  - **.4 Router — OPEN, designed, data-gated** (P2, phase-2). Body: deterministic risk rules route each
    close to a tier — docs/provenance-only → L0 deterministic; small in-BC code → single cheap-family
    fresh-context; multi-file/cross-BC/security/contracts/public → full cross-family duel; one-way doors →
    tri-family/council; **uncertainty fails closed to the higher tier**; route decision + outcome logged;
    misroutes are escapes that compile into router rules. Pre-registered finding: **tier-by-inventory
    drift** — `scripts/pawl.sh` defaults to probe-what's-installed so tier=multi whenever ≥2 CLIs exist
    (`pawl.sh:11-12`, `:22-29`, `probe_families` `:89`), while the declared contract prices by stakes;
    first router rule = restore the declared default.
  - **.5 Context diet — OPEN, designed, data-gated**: reviewer packet = bead acceptance contract + diff
    (~800 tokens p50) + declared evidence refs; repo access as escalation not default.
  - **.6 Cheap-tier default + escape harvest — OPEN, designed, data-gated**: cheapest adequate family with
    1-in-N strong-tier sampling audit; every strong-tier catch of a cheap-tier miss = REAL escape data —
    the exact ADR-0011 revival condition.
  - **.10 Delta re-review — OPEN, DEFERRED (deliberate)** with a full 5-point safety design in the bead
    (lineage-gated like the REBOUND path age-rk3r.9; full diff since last-reviewed sha; git-verified
    unchanged-remainder; repo access retained; prior-defect list prepended; fall back to full review).
    Deferred because it changes what the membrane REVIEWS; wants fresh mind + council. Depends on .9.
  - **.9 pre-flight gate — CLOSED, landed 36a891ac8**: pawl-review runs the deterministic battery before
    dispatching the model reviewer; fail-fast exit 3 on red with 0 reviewer tokens.
- **Is ebec.4 the risk model a post-merge design needs? YES, substantially.** Its tier ladder
  (L0 → cheap fresh-context → cross-family duel → tri-family/council), fail-closed-upward rule,
  route+outcome logging, and misroute-as-escape feedback are precisely the "which lanes may land first and
  get reviewed after vs which must still be reviewed pre-merge" classifier. It is DESIGNED but UNBUILT and
  explicitly deferred until Directive 17's meter produces data (`GOALS.md:242-244`) — a post-merge design
  that needs it must either land the meter data first or accept promoting .4 out of its data gate (an
  explicit operator decision, since the gate is a binding honesty rule).

---

## 4. Observe/measure substrate ("membrane as telemetry") — LARGELY EXISTS

- **Yield ledger** `cli/internal/yieldledger/` writing `.agents/yield/yield-ledger.jsonl`
  (`yieldledger.go:40`). Three event types: `accept` (bead reached gated merge; carries `merge_sha` +
  `gate_verdict_ref`, `:44-45`, `:96-101`), `gate-verdict` (`:46-47`, body `:103-152` — disposition
  CONFIRMED/REFUTED/ESCALATE/HOLD `:53-58`; modes fresh-context/multi-model/**deterministic** (`:60-69`,
  age-srl — the deterministic gate refusing a claimed-done is itself a recorded verdict); Domain/Reason/
  Class/ClassKey/AffectedPaths + Detector fields that make an escape mechanically re-introducible
  `:120-151`), and `usage` (tokens/cost/wall-clock/model/phase `:154-163`, phases plan/implement/review/
  rework/coordination `:71-78`).
- **Escape detection is ALREADY post-hoc by definition**: `escape.go:5-19` — an escape is a CONFIRMED for
  a bead that a later, strictly-higher-attempt gate-verdict for the SAME bead REFUTED; detected as a pure
  READ over the append-only ledger, no row mutation. **This is the exact record shape a
  REFUTED-after-land produces — the telemetry layer needs no schema change for post-merge review.**
- **`ao yield` surface**: `emit <accept|gate-verdict|usage>` (`cli/cmd/ao/yield.go:44-45`), `gauge` (A, Q,
  A/R, E, L gauges, `:64-65`), `tokens --transcript` (`:97-98`).
- **`ao yield report` — IN-FLIGHT, NOT LANDED.** Commit `9926109c2` ("the on-the-loop governance surface
  (yield + andon queue)", age-mv67, dated 2026-07-09) adds `cli/cmd/ao/yield_report.go` (618 lines: yield
  counts since cutoff + andon queue of blocked beads / ESCALATE/HOLD verdicts / REFUTED-with-open-bead,
  `--since`, `--json`) — but it exists only on branch `worktree-agent-adc473da4b1957eb8`; bead `age-mv67`
  is OPEN and the file is absent from origin/main. The flywheel status ledger still marks it "specced"
  (`docs/architecture/the-flywheel.md:153`). **The andon-queue surface a post-merge design would page the
  human through is being built right now — coordinate, don't duplicate.**
- **`ao membrane digest [--deltas --since]`** — LANDED (`d91e01043` on this checkout): global top-N
  recurring-defect checklist mined from catches, written to `.agents/pre-mortem-checks/catch-digest.md`
  (the same dir /pre-mortem Step 1.4b loads; `membrane_digest.go:36-43`); `--deltas` = per-class
  recurrence before/after for the producer-defect register (`:52-53`); honest-scope comment: no
  compounding claim (`:28-31`). Also `ao membrane catch/recall/triage/calibrate/derive-checks`
  (`membrane.go:76-138`); catch auto-invoked from the pawl on every REFUTE (ADR-0014:10).
- **Catch corpus + ADR-0014** (`docs/adr/ADR-0014...md`): 512 gate-verdicts, 201 REFUTED catches, 22
  classes / 13 recurring / 122 unclassified, **Axis2Compilable = 0.00** — every recurring catch is
  judgment-class (`:14-16`); decision = Loop C catch→producer route with the committed producer-defect
  register measuring per-class recurrence drop (`:33-39`); proof = recurrence DROP per class, not corpus
  size (`:39`).
- Discovery now injects recurring catch classes as `known_risks` into execution packets (`14875496e`,
  age-8rrz).

**Reuse implication:** post-merge review needs almost no new telemetry — emit a `gate-verdict` with the
landed sha; an overturn is auto-detected as an Escape; digest/report/gauges consume it unchanged.

---

## 5. Revert/rollback machinery — MOSTLY MISSING at the trunk layer (deliberately)

- **No auto-revert of landed trunk commits exists.** The refinery explicitly refuses:
  *"it NEVER blind-reverts (the repo's 18-30% flake rate would make auto-revert fight developers)"*
  (`cli/internal/refinery/refinery.go:4-5`; same note `cli/cmd/ao/refinery.go:20`).
- **Evolve loop auto-revert is LOCAL-only**: regressed cycles auto-revert *local* commits before push
  (`docs/evolve-setup.md:21`; `docs/scale-without-swarms.md:103`; recipe
  `skills/evolve/references/fitness-scoring.md:66-68`).
- **Manual incident path**: `docs/INCIDENT-RUNBOOK.md:130-133` — `git revert --no-commit ${GOOD_SHA}..HEAD`
  + commit; quick-card `:395`.
- **`scripts/pawl-land.sh` restamp semantics** (the closest thing to landed-state reconciliation):
  fetch + rebase onto origin/main (`:82-87`); resolve the reviewed FEAT (tip's parent when tip is the
  #trivial auto-bind — dual signature test `:54-68`); **restamp = `pawl-verdict.sh rebind` onto the
  post-rebase feat under `PAWL_AUTOBIND=0`** so no second #trivial is committed (the age-fkps
  double-#trivial bug, `:14-23`, `:99-114`); ledger-dirt restore so the next queued land isn't
  dead-lettered (`:115-127`); single-shot `push origin HEAD:main` (`:130-136`). The land shape is
  `[feat, #trivial-bind]` with the verdict binding the FEAT, not the bind commit (`:7-13`).
- **REFUTED-after-land today = record + fix-forward, never revert.** The machinery: the ledger records the
  overturning REFUTED (escape shape, §4); the EM spine compiles it into a derived check; a REFUTED verdict
  can even be bound into provenance on main (live example: origin/main `4707457f8` "bind pawl REFUTED
  verdict for age-cysr" followed by the fixed `cdb9a4e5b` CONFIRMED bind). There is no quarantine, no
  auto-revert, no compensation transaction.
- **A revert IS itself a pawl.** A forward `git revert` push is mutate-shared-trunk (`pawls.md:21`);
  a force-push rollback is the shared-ref-rewrite case the contract says to opt UP to multi-model
  (`pawls.md:88-90`). Any auto-revert design must route its own action back through a door.

**Reuse implication:** the redesign must BUILD the revert/quarantine arm; nothing exists to reuse except
the incident-runbook recipe, the pawl-land rebind pattern (for re-stamping a reverted state), and the
explicit prior art that blind auto-revert was REJECTED for flake-rate reasons — a post-merge REFUTED
handler must first classify flake vs defect (`scripts/land-lane-flaky-retry.sh` exists for the land lane's
flake-retry classification).

---

## 6. Queue/lock machinery — EXISTS (a full land queue + single-writer lane is already built)

- **age-2sog serial push lock**: `pre-push.local:54-76` (mkdir `$TMPDIR/agentops-push.lock`, fail-closed
  300s timeout, releases via EXIT/INT/TERM traps `:32-47`). Same lock dir shared by
  **`scripts/push-serial.sh`** (`:20`, ag-qidx P1.4): within-host mkdir lock + cross-host convergence via
  git's atomic non-fast-forward rejection + rebase-retry loop (max 5) (`:24-60`).
- **Land queue — BUILT (epic agentops-2pl)**:
  - `scripts/land-submit.sh` — pushes the bead branch to `refs/heads/land-queue/<bead>` with `--no-verify`
    (queue ref is never main; the lane owns the gated trunk push, `:2-5`, `:126`) and appends a request row
    to `.agents/land-queue/requests.jsonl` under a mkdir lock (`:128-158`). AM backend explicitly
    unavailable → file backend (`:102-110`).
  - `scripts/land-queue-next.sh` — pops the oldest unclaimed request (`:46-62`).
  - `scripts/land-lane-run.sh` — **"the LAND LANE: a single serialized writer that owns `main`"**
    (header `:2-3`): singleton mkdir lane lock so a 2nd lane refuses to start; pops oldest, rebases onto
    origin/main, runs the gate ONCE per landing, lands via pawl-land, loops; `--drain | --once | --watch
    [--poll N]` modes; flaky-retry knob `LAND_LANE_FLAKY_RETRY_MAX`; a guard script blocks GitHub
    Actions/PR/merge paths from the lane; **"ADR-0009 … this is a thin foreground loop, not a service"**;
    AM is `degraded_read_only` so the lane deliberately does NOT depend on `am`; host = the always-on Mac,
    never bushido (all in the header block, `land-lane-run.sh:1-60`).
  - Plus `scripts/land.sh` (single-bead: ship → pawl-review → pawl-land → strict post-land provenance →
    `ao done <bead> --sha <verdict-bound-feat>` verdict-stamped close, `:8-9`, `:147-197`) and
    `scripts/land-lane-flaky-retry.sh`, `scripts/land-queue-test.sh`.
- **Merge-queue precedent**: GitHub merge queue is tracked as `ag-arpk`, disposition **kept PLANNED,
  sequenced after the land.sh epic**, named as the only listed serializer for the cross-host residual
  (host-local lock doesn't span hosts) —
  `docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/spec.md:302-308`, `behaviors.md:361-369`.
- **Agent-Mail reservations** (`skills/agent-mail/SKILL.md`): ≥2 writers → reserve is non-negotiable, but
  partition-before-lock (`:24`); one-writer-per-hot-dir, reserve-before-edit, coordinate on conflict
  (`:33-37`, `:105-109`). Could serve as a land-queue backend in principle, but the land lane already
  rejected it for reliability (`land-lane-run.sh` header) and `land-submit.sh:104-106` hard-dies on
  `am|auto`.

**Reuse implication:** a post-merge design's "land fast, review after" trunk needs a serializer — it is
already built twice (in-hook lock + the land lane). The land lane is architecturally the stronger seam: a
single writer that could trivially be split into "land now" and "review after" phases, and its
`LAND_LANE_GATE_ONLY_CMD` / `LAND_LANE_LAND_CMD` injection seams (`land-lane-run.sh` knobs block) already
separate gate from land.

---

## 7. ADR-0009 runner constraint + doctrinal basis — EXISTS

- **ADR-0009** (`docs/adr/ADR-0009-daemon-deletion-in-session-only.md`): AgentOps ships **no standalone
  daemon, scheduler, or overnight runner** (`:26`); always-on opts into a substrate (`:28`); the seam —
  the substrate dispatches the whole loop as one invocable unit, never re-expressed as substrate steps
  (`:29`); substrate renarrated 2026-06-03 to **NTM + MCP (`ao mcp serve`) + managed-agents (`ao agent`)**
  (`:9`). **Gas City is additionally a blessed coexisting substrate** — operator choice, never auto-routed,
  driven via `skills/using-gc/SKILL.md` (`CLAUDE.md` "Gas City (`gc`) is a blessed coexisting substrate…").
- **The sanctioned scheduling shapes are already enumerated** in the pawl contract itself, for `ao pawl
  reap` (`pawls.md:222-227`): **cron** (`*/30 * * * * … ao pawl reap`), **launchd** (`StartInterval=1800`),
  or an **NTM tending tick** — "the reap *schedule* lives in your substrate, not the repo." This is the
  direct precedent for scheduling a post-merge async reviewer.
- **A post-push CI hook already exists**: `.github/workflows/verdict-backstop.yml` runs **on: push to
  main** (`:15-17`) calling `scripts/check-tip-verdict-ci.sh` — report-only, records-only (*"it never
  produces verdicts (no reviewer calls, no secrets, no subscription auth; posture per docs: no hosted
  control plane)"*, `check-tip-verdict-ci.sh:5-8`; "verdict PRODUCTION stays local", workflow `:12-13`).
  So CI can OBSERVE post-merge but is doctrinally barred from PRODUCING the post-merge verdict.
- **Doctrinal basis for post-merge-as-sensor**: `docs/architecture/the-flywheel.md` — *"Validate is a
  sensor, not a controller"* (`:41-48`); the loop belongs to the driver at three timescales (`:50-54`);
  the **andon is three-tier** (Auto / Council / Human) and *"the missing piece is the router — a per-goal
  policy"* (`:56-73`); the human moves from IN the loop to ON it, reviewing yield + andon queue
  asynchronously (`:113-121`); status ledger rows (`:142-155`). GOALS.md D16 milestone 3 states the
  current doctrine being changed: *"the in-repo ratchet pawl-gate writes the binding verdict at the merge
  pawl"* (`GOALS.md:215`).

**Sanctioned runner options for async post-merge review, given ADR-0009** (in-repo daemon is forbidden;
verdict production must stay local/subscription-side, not CI):
1. **NTM tending tick / warm pawl-service pane** — the pawl already runs as a standing warm service
   (`ao pawl up`, `pawls.md:220-229`); a tick that drains a review queue is the reap-schedule pattern.
2. **cron / launchd on the always-on Mac** — explicitly sanctioned shapes (`pawls.md:224-225`); precedent:
   the land lane is host-pinned to the Mac (`land-lane-run.sh` header).
3. **A thin foreground loop à la `land-lane-run.sh --watch`** — self-described as ADR-0009-compliant
   ("a thin foreground loop, not a service").
4. **Gas City durable supervised agent** — blessed substrate for exactly "durable supervised agents over
   hours" (`the-flywheel.md:123-128`; CLAUDE.md), with the agentops-membrane pack as close door.
5. **CI (GHA)** — observation/annotation ONLY (verdict-backstop precedent); cannot produce verdicts.

---

## The one-way-door surface: contract clauses that MUST change for post-merge review

1. **`pawls.md:12`** — "must confirm **before the action proceeds**". The core clause. Needs a new
   sanctioned disposition path: e.g. a *deferred-verification lane* where the trunk mutate proceeds on a
   deterministic-tier + risk-router pass, with the model verdict bound post-land.
2. **`pawls.md:13`** — "the pawls reduce irreversible regression to the known one-way doors… between them
   you only ever touch state you can recover." Post-merge review re-classifies routine trunk pushes as
   *recoverable* — that claim must be made TRUE by shipping the revert/quarantine arm (§5: currently
   MISSING) before the clause can honestly change. This is the real one-way door of the redesign.
3. **`pawls.md:197` + `:199`** — "the merge/push is **held**, not landed… Non-convergence never
   auto-lands." Needs a post-land analogue: what a breaker-trip HOLD means when the commit is already on
   main (quarantine main? block subsequent lands? auto-revert through its own pawl?).
4. **`pawls.md:21`** — the mutate-shared-trunk row's "Already guarded by" column (pre-push gate +
   reconcile-pr) must name the new enforcement point(s).
5. **`pawls.md:229` vs `:90-91`** — resolve the multi-model-demand contradiction (executable currently
   sides with `:90-91`, `check-pawl-pre-push.sh:283-285`).
6. **`GOALS.md:215`** (D16 milestone 3) — "writes the binding verdict at the merge pawl" must be re-worded
   to the new timing.
7. **`schemas/pawl-verdict.v1.schema.json` head_sha binding semantics** — today `head_sha == the PR's live
   head` pre-merge (`pawls.md:76`, `:201`); post-merge binding targets the landed sha (and the gate
   rewrites commits on rebase — the merge_sha-from-origin rule), so `check`'s commit-binding clause and the
   rebind/REBOUND lineage path need a post-land binding mode.
8. **Enforcement scripts keyed to push-time** — `check-pawl-pre-push.sh` (verdict requirement `:271-289`)
   and the #trivial/age-8ais range logic (`:236-265`) must be re-homed or forked; the CI backstop's
   "every commit carries a bound verdict" check (`check-tip-verdict-ci.sh:15-21`) needs a grace-window
   semantic or it will annotate every post-merge-lane commit as unverified.

**What does NOT need to change (reuse as-is):** the escalation/circuit-breaker model (`pawls.md:178-199`),
directive precedence (`:203-212`), diversity floor + evidence-quality floor (`:66-131`), the "what good
means" REFUTE bar (`:137-176`), the escape/overturn telemetry (`escape.go:5-19` — already post-hoc), the
detached-worktree per-commit gate re-target (`check-pawl-pre-push.sh:183-220`), the land queue + single
writer lane (§6), the deterministic-catch sensor feed (`pre-push.local:229-236`), and the ebec.4 risk-tier
ladder as the routing brain (data-gate caveat, §3).
