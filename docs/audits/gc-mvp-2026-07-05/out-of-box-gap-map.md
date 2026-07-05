# Out-of-box gap map — stock Gas City vs the AgentOps loop

> Bead `age-gc-mvp-w2-nuiw.9`. Run 2026-07-05 against the native bd/dolt city
> `~/dev/gc-city` (gascity edge fork `8b17c64`, bd 1.1.0, dolt 2.1.10). This
> exercises gc's **OWN shipped core-pack formulas** (`mol-scoped-work`,
> `mol-review-quorum`) and default surfaces — **no custom `quest.toml`, no
> `membrane/quest-gate.sh`**. Contrast with `MVP-VERDICT-ADDENDUM.md`, which ran
> the A/B drill through OUR custom gate. This decides whether the parked
> custom-pack beads (`.5`–`.8`) are still needed.
>
> Method note: findings are from **actually running** the commands/formulas in
> the live city, not from reading docs. Where a stock formula stalled, the exact
> stall + the nudge that unblocked it is recorded as a finding (not hidden).

---

## Task A — exercising gc's stock core-pack formulas

### A1. `mol-scoped-work` — native worktree lifecycle (STOCK)

Slung a real NEW sandbox task (not the hello fixture): `quests/mathutil/` — a
2-function shell util (`calc.sh add|max`) with a 4-assertion `test.sh` and a
`CONTRACT.md`, its own git repo with a bare self-`origin` so `origin/main` exists
for the worktree step. Base stub deliberately unimplemented (real work to do).

```
gc sling builder "Implement quests/mathutil/calc.sh …" \
  --on mol-scoped-work --var base_branch=main \
  --var test_command="cd quests/mathutil && ./test.sh"
→ {"bead_id":"gc-tjj","formula":"mol-scoped-work","routed":true,"workflow_id":"gc-tjj"}
```

**Observation — orchestration lane: PROVIDED, out of the box.** The formula
compiled natively into the full worktree-lifecycle DAG as real `bd`/dolt rows —
verified via `gc bd show gc-tjj` + `gc events`. Every step from the formula is a
first-class bead with dependencies and control beads:

- Execution steps: `load-context` → `workspace-setup` (`git worktree add
  --detach origin/main`) → `preflight-tests` → `implement` → `self-review` →
  `submit` → `cleanup-worktree`, plus a `Worktree body scope` latch and
  `Finalize workflow`.
- Per step gc also emits a **Step spec** bead (`gc.kind=spec`) and a **Finalize
  scope / scope-check** control bead (`gc.kind=scope-check`, `gc.on_fail=abort_scope`)
  and a **retry** control bead (`gc.kind=retry`, `gc.max_attempts=3`,
  `gc.on_exhausted=hard_fail`).
- Routing is split natively: execution beads carry `gc.routed_to=builder`;
  control/retry/scope beads carry `gc.routed_to=core.control-dispatcher`. The
  input convoy `gc-73t` (`issue_type=convoy`, `gc.synthetic=true`) `tracks` the
  work bead `gc-o6o`. Correlation id `run_id=gc-tjj` + `step_ref`
  (e.g. `mol-scoped-work.implement.attempt.1`) + monotonic `seq` on every event.

So stock gc gives, natively: worktree isolation, an explicit step DAG,
per-step retry with bounded attempts, scope-fail-fast, and a full correlated
event trail — **without any custom formula**.

**Observation — execution lane: PROVIDED but nudge-gated.** The builder is a
`claude/opus` interactive pane. Getting it to actually run the steps reproduced
the **idle-interactive-pane drain gap** from the addendum — harder than the
drill:

- The reconciler correctly repointed the pool builder session's workdir to the
  new workflow's first step (`gc-pqb`), but the session sat permanently in
  `draining` state (an idle claude TUI at the composer with an unsent
  `gc hook builder-1`).
- `gc session kill` did **not** recycle it; `gc session reset` ("Controller will
  restart it fresh") did **not** recycle it either; raw `tmux send-keys Enter`
  did **not** submit. The pane's claude process was alive (pid confirmed) but
  unreachable by those paths.
- **Only `gc session submit <sess> "gc hook builder-1"`** (the native
  semantic-delivery verb) landed the hook. This is the reliable native delivery
  contract; raw pane manipulation and `kill`/`reset` are not.
- After that one nudge, the builder ran the lifecycle **natively**: it primed,
  identified work bead `gc-o6o`, read the spec, and **closed `load-context`
  (`gc-pqb` → CLOSED)** on its own, then advanced to `workspace-setup`. Opus with
  max-effort thinking is slow (~4–5 min/step) but correct.

Net: stock `mol-scoped-work` **produces** the worktree lifecycle and **executes**
it, but each round-transition on an interactive pane needs one operator
`gc session submit` — the exact residual the addendum flagged. It is a
**delivery/session-boundary** gap, not a workflow-DAG gap.

**Gap in the stock formula itself:** `mol-scoped-work`'s `submit` step is
*intentionally generic* — its description literally says "push the branch and
hand off to a reviewer / close the original work bead … intentionally generic so
cities can opt into the graph contract without inheriting Gastown's exact
workflow." **Stock `mol-scoped-work` does NO cross-family review and NO
verification-gated close.** It is a build-and-self-review lifecycle only.

### A2. `mol-review-quorum` — native cross-family review (STOCK)

Compiled with two different families as the two lanes (codex + antigravity/agy):

```
gc sling verifier "Review mathutil diff" --on mol-review-quorum \
  --var lane_one_id=codex-lane --var lane_one_provider=codex --var lane_one_model=gpt-5.3-codex --var lane_one_target=verifier \
  --var lane_two_id=agy-lane  --var lane_two_provider=antigravity --var lane_two_model=gemini --var lane_two_target=agy-verifier \
  --var synthesis_target=verifier
```

**Observation — fan-out: PROVIDED.** The formula compiles a 3-step DAG:
`review-lane-one` + `review-lane-two` (parallel, read-only reviewers, each
required to emit a durable `review-quorum.lane.v1` JSON with
`verdict/findings/evidence/read_only_enforcement/mutations_delta/failure_class`)
→ `synthesize-review-quorum` (`needs` both lanes; emits
`review-quorum.summary.v1`). Two-family cross-review is a **real shipped
capability**, and lanes are fully parameterized (provider/model/target per lane),
so builder=claude with judge=codex **or** agy is a config choice.

**Observation — the verdict is NOT fail-closed out of the box.** The formula's
own description warns "this formula's synthesis step is currently **agent-executed
rather than directly wired to** the Go finalizer … `internal/reviewquorum.Finalize`
is not invoked by this step yet." Verified in the fork
(`~/dev/gascity`, read-only):

```
grep -rn "reviewquorum.Finalize" --include=*.go
→ only internal/reviewquorum/finalize_test.go  (NO production caller)
```

So `reviewquorum.Finalize` (the deterministic combine-lane-verdicts function)
exists and is unit-tested but is **never called by the running formula**. The
quorum verdict is whatever the synthesis *agent* writes. There is no deterministic
close door, no "unknown lane verdict → hard fail" enforcement, no
CONFIRMED-required gate — an agent could synthesize "pass" over two failing
lanes and nothing stops the close. **Stock cross-family review exists; stock
fail-closed cross-family *gating* does not.**

---

## Task B — what stock gc provides out of the box

Every row is from a command actually run in the live city.

| Capability | Status | Exact command | What it gave us (observed) |
|---|---|---|---|
| Work tracking | **PROVIDED** | `gc bd ready` / `gc bd list --json` / `gc bd show <id>` / `gc bd update <id> …` | Native in-process dolt store (`NativeDoltStore`), 261 live rows; full CRUD against the bead graph. |
| Dependency waves | **PROVIDED** | `gc bd ready` (frontier); deps in every bead (`blocks`/`tracks`/`depends_on`) | Ready-frontier computed from the DAG; `mol-scoped-work` emitted `blocks`/`tracks` edges so only the unblocked step (`gc-pqb`) surfaced as routed-ready. |
| Claim / close | **PROVIDED** | `gc hook <agent>` (claim next routed); `bd update <id> --set-metadata gc.outcome=pass --status=closed` | Builder claimed `gc-pqb` via the hook and closed it natively with typed work-record metadata (`gc.work_outcome`, `gc.work_commit`, `gc.work_branch`). |
| Roles / pools | **PROVIDED** | `gc agent list`; `city.toml [agents.*]`; `gc status` | 10 agents: `builder` (pool, min0/max∞), `verifier`/`agy-verifier`/`codex`/`claude`/`antigravity`, `mayor`, `planner`, `core.control-dispatcher`, `bd.dog`. Author≠judge RBAC is config. |
| Review quorum | **PARTIAL** | `gc sling <t> <b> --on mol-review-quorum --var …` | 2-lane cross-family fan-out + agent synthesis (see A2). **Verdict is agent-produced, not the Go finalizer — not fail-closed.** |
| Provenance / events | **PROVIDED** | `gc events` (JSON-lines); HTTP `/v0/city/gc-city/events` | Every `bead.created/closed`, `order.fired/completed`, session event with `seq`/`actor`/`subject`/`type`/`ts`, `run_id` + `step_ref` correlation. Per-work trail for `gc-tjj` is complete. `events.jsonl` 353 KB and growing. |
| Cost metering | **ABSENT** | `gc costs` | `"No usage facts recorded yet (…/.gc/usage.jsonl)"` — the file does not even exist. Surface + path exist; nothing populates them (providers emit no usage facts). Confirms the A/B "usage.jsonl empty" finding — actually *absent*, not just empty. |
| Doctor | **PROVIDED** | `gc doctor` | **73 passed, 5 warnings.** dolt/bd/events-log health, backup freshness, worktree hygiene, custom-types, binaries — plus it hosts the city's LAW-0 check. Rich native health framework. |
| Orders / housekeeping | **PROVIDED** | `gc order list` | **20 native orders**: `dolt-health`, `beads-health`, `gate-sweep`, `reaper`, `orphan-sweep`, `prune-branches`, `wisp-compact`, `jsonl-export`, `nudge-mail-sweep`, `nudge-on-route`, `cascade-nudge-on-blocker-close`, `spawn-storm-detect`, `cross-rig-deps`, `order-tracking-sweep`, `mol-dog-{backup,compactor,doctor,phantom-db,stale-db}`, `dolt-remotes-patrol`. Triggers: cooldown / cron / event. Firing live in `gc events`. |
| Dashboard | **PROVIDED** | `gc dashboard --no-open` → embedded SPA at `http://127.0.0.1:8373` | `curl /` → HTTP 200 (embedded SPA served same-origin by the supervisor; no separate static server / port). Data API `/v0/city/gc-city/events` returns JSON. |
| Analyze | **PARTIAL** | `gc analyze reliability` | Correlates session lifecycle × model/version/rig (Crashed/Quarantined/IdleKilled/Drained/Crash%). Runs, but shows 0 sessions and warns `session.quarantined is not emitted by current production paths` — the report exists; the instrumentation feeding it is incomplete. |

---

## Task C — gap map vs the AgentOps loop + membrane

Grounded in `docs/architecture/operating-loop.md` (the seven moves) and
`docs/contracts/pawls.md` (the membrane). For each AgentOps capability: whether
stock gc PROVIDES it, has a weaker/different PARTIAL, or is MISSING.

### The seven-move operating loop

| AgentOps move | Stock gc | Evidence |
|---|---|---|
| 1. Shape intent as BDD (Given/When/Then, acceptance) | **MISSING** | No formula shapes acceptance criteria; `mol-scoped-work` takes free-text + optional command vars. Intent/acceptance is the operator's to author (our quest `CONTRACT.md`). |
| 2. Track as a bead | **PROVIDED-BY-GC** | Native bd/dolt store; sling auto-creates the work bead + convoy; typed work-record metadata. |
| 3. Slice vertically | **PARTIAL** | The formula DAG is a *lifecycle* decomposition (setup→impl→review), not a behavior-slice decomposition; slicing-by-behavior is still human/planner work. |
| 4. TDD per slice | **PARTIAL** | `mol-scoped-work` runs a `preflight-tests` step and a `self-review` test step **if you pass `test_command`**; it does not enforce test-first (red-before-green). It runs whatever command you give, whenever the step reaches it. |
| 5. Group into a wave (no scope collision) | **PROVIDED-BY-GC** | Worktree-per-work isolation + pool scaling + `blocks`/`tracks` deps + `spawn-storm-detect`/`reaper` orders. This is genuinely strong native infra. |
| 6. Close by proving acceptance (the windshield) | **MISSING** | Stock `mol-scoped-work` `submit` is intentionally generic (self-review only). `mol-review-quorum` fans out but its verdict is agent-written, not a deterministic finalizer. **No deterministic acceptance gate on close.** |
| 7. Capture evidence + ratchet | **PARTIAL** | Rich event/provenance capture (PROVIDED) + typed work records; but no escape-corpus, no promote-on-learning, no compile-the-catch ratchet. |

### The membrane (pawls.md)

| Membrane property | Stock gc | Evidence |
|---|---|---|
| Fresh-context adversarial refute | **PARTIAL** | `mol-review-quorum` lanes are separate agent invocations (structural fresh context) and read-only, so the *shape* exists. But they review for findings; there is no adversarial "establish ground truth and try to REFUTE" contract, and no fail-closed consumer of their verdict. |
| Evidence + commit-bound verdict | **PARTIAL** | Lanes emit durable `review-quorum.lane.v1` JSON with `evidence`; work beads carry `gc.work_commit`/`gc.work_branch`. But the verdict is not bound to a commit as a gate precondition — nothing checks it before close. |
| Fail-closed close door (CONFIRMED required, ambiguity→hold) | **MISSING** | The decisive gap. `mol-review-quorum` never invokes `reviewquorum.Finalize`; `mol-scoped-work` closes on self-review. Green/agent-"pass" authorizes close. No "no verdict = not done". |
| REFUTED → auto-redo loop + circuit-breaker | **PARTIAL** | The formula has native per-step **retry** (`max_attempts=3`, `on_exhausted=hard_fail`) and scope-abort — a bounded-redo *primitive*. But it retries on step *failure*, not on a *reviewer REFUTED verdict*; there is no verdict→redo wiring out of the box. |
| Escape-corpus / ratchet | **MISSING** | No escape ledger, no compile-the-catch, no promotion. |
| LAW-0 (no `claude -p`) | **MISSING (stock)** / PROVIDED via our add-on | Stock providers ship `--print`/print-args; the city's `law0-print-args` doctor check + `print_args=[]` overrides are OUR add-on, not stock. |
| Cross-family by default | **PARTIAL** | Cross-family is *possible* and first-class (per-lane provider/model), but not the *default* posture and not *enforced* — a city can run same-family lanes, and nothing requires ≥2 families. |
| Few-pawls / blast-radius model | **MISSING** | gc has no concept of pawls / one-way-door gating; orders and retries are operational, not a risk-classified gate router. |

---

## BOTTOM LINE

### (1) What stock gc gives us — USE it, stop rebuilding

Stock Gas City is a **strong orchestration substrate**. We should consume, not
re-implement:

- **Native work store + graph** (bd/dolt `NativeDoltStore`, ready-frontier,
  `blocks`/`tracks` deps) — replaces any home-rolled tracker plumbing.
- **The v2 workflow compiler + worktree lifecycle** (`mol-scoped-work`): worktree
  isolation, an explicit step DAG, per-step bounded retry + scope-fail-fast,
  split execution/control routing. This is exactly the "slice into a wave with no
  scope collision" (move 5) infra — do not rebuild it.
- **Pool/roles/RBAC** (`gc agent list`, author≠judge as config), **the
  reconciler + control-dispatcher serve loop**, and **20 native housekeeping
  orders** (reaper, orphan-sweep, gate-sweep, dolt/beads health, prune-branches).
- **Provenance**: `gc events` + `run_id`/`step_ref`/`seq` correlation + the
  `/v0/.../events` API + embedded dashboard + `gc doctor` (73 checks). Our
  membrane's evidence layer should *emit into* this, not parallel it.
- **The 2-lane review-quorum fan-out scaffold** (`mol-review-quorum`) — reuse the
  fan-out + durable `review-quorum.lane.v1` schema; only the verdict/close needs
  replacing.

### (2) What our AgentOps layer must ADD on top (the minimal set)

The one thing stock gc structurally lacks is **the verification membrane** — a
deterministic, fail-closed, verdict-bound close door. Concretely, the minimal
add-on is:

1. **A fail-closed close gate** — the piece that turns `mol-review-quorum`'s
   agent synthesis into a deterministic verdict. Either wire the existing
   `internal/reviewquorum.Finalize` into the synthesis step, or supply our own
   `quest-gate.sh`-style gate that requires a `pawl-verdict.v1` CONFIRMED before
   any close/merge (ambiguity→hold, green≠authorized). **This is the product.**
2. **An adversarial-refute reviewer contract** (establish ground truth → try to
   REFUTE → evidence+commit-bound verdict), layered onto the quorum lanes —
   stronger than stock "find findings" review.
3. **Verdict→redo wiring** — connect a REFUTED verdict to the DAG's existing
   retry/iteration primitive (stock retry fires on step failure, not on a
   reviewer verdict).
4. **LAW-0 as structure** (`print_args=[]` + doctor check) — already our add-on;
   keep it.
5. **Cross-family-by-default enforcement** — make ≥2 families the required posture
   for the gate, not an optional var.

Escape-corpus/ratchet and BDD-intake shaping are *nice-to-have* on top but are
NOT the minimal set (and the corpus is ADR-0004/0011 unproven anyway).

### (3) Honest residual reliability gaps to zero-nudge

Three stalls stand between here and unattended operation (all
**session/provider-boundary**, none bd-absence):

- **`gc.source_bead_id` engine gap** — `gc sling --on` still does not stamp
  `gc.source_bead_id` on the workflow root (unchanged from the addendum). Our
  gate's compensation (recover from input-convoy title, stamp root) is still
  required.
- **Idle-interactive-pane drain gap** — reproduced *harder* here: an idle
  `claude`/opus pane in `draining` was not recyclable by `gc session kill`,
  `gc session reset`, or raw `tmux send-keys`; **only `gc session submit`**
  (semantic delivery) advanced it. Every round-transition needs one native
  submit. Candidate fix: dispatcher-driven `session submit` on iteration spawn,
  or run the builder as a headless/exec target rather than an interactive pane.
- **Codex trust modal** — verifier codex sessions can wedge at startup on the
  user-global `~/.codex/hooks.json` trust modal (environmental). Pre-trust the
  hooks or point city sessions at a clean `CODEX_HOME`.

### Verdict on the parked custom-pack beads (.5–.8)

**Still needed — NOT superseded by stock gc, but their scope shrinks.** Stock gc
provides the entire orchestration substrate (store, DAG, worktrees, pools,
orders, provenance, dashboard, doctor, *and* a cross-family fan-out scaffold), so
the parked beads should **stop re-deriving orchestration** and narrow to the one
thing stock gc structurally does not have: **the fail-closed verification
membrane as a composable pack**:

- `.5` (membrane add-on pack: LAW-0 + trinity + gate) — **keep**, this is the
  product; but scope it to items (1)–(5) above, riding *on* the stock
  `mol-review-quorum` fan-out and the existing `reviewquorum.Finalize` rather
  than a from-scratch gate.
- `.7` (planner intake + quest template) — **keep** (move 1 / BDD shaping is
  MISSING in stock gc), lower priority.
- `.6` (city self-verification canary) — **keep but make gc-idiomatic**: it is a
  thin wrapper over stock `gc doctor` + a canary quest, not new infra.
- `.8` (bridge-lite to agentops br) — **keep**, orthogonal to gc's capabilities
  (cross-tracker reporting).

In one line: **stock Gas City is the substrate; AgentOps is the membrane.** Use
gc's orchestration wholesale, and spend the custom-pack budget only on the
fail-closed verdict-bound close door that stock gc pointedly leaves generic.
