# Pawls — the one-way doors (the ratchet's static map)

> **Chaos + Filter + Ratchet = Progress** — see [brownian-ratchet.md](../brownian-ratchet.md).
> This file is the **Filter**: the irreversible actions where the ratchet's pawl sits — a short list
> of the common instances, governed by the [blast-radius rule](#the-blast-radius-rule-the-list-is-examples-not-the-boundary).
> Anything that trips no clause of the rule is chaos — cheap, wrong-tolerant, ungated. The router is
> a cheap classification (list + rule), **not** a heavyweight decision engine.

## The rule

1. About to take an action? Check it against the pawl list **and the [blast-radius rule](#the-blast-radius-rule-the-list-is-examples-not-the-boundary)** below.
2. **A pawl (on the list, OR it trips any clause of the blast-radius rule) → fire the gate**: [`/pawl-review`](../../skills/pawl-review/SKILL.md) obtains independent fresh-context lane evidence, then `ao pawl` applies diversity and binds the decision. Fail-closed (ambiguity → hold, never silent-proceed).
3. **Not a pawl (not on the list AND it trips no clause of the blast-radius rule) → chaos.** Just run it. No gate, no review, no ceremony. Iterate as wrong as you want between pawls — the pawls **reduce irreversible regression to the known one-way doors**, so between them you only ever touch state you can recover. (The list is the common instances; the **rule** is the authority — if an action trips a clause but isn't listed, it is still a pawl, and it's a missing list entry to add.)

Why so few pawls: a pawl on *every* step is waterfall (validate every tread). It makes every wrong turn expensive and kills the cheap iteration that makes agents productive. The ratchet works **because** the pawls are few and sit only at the irreversible points. Adding a pawl is adding a tread you now validate — do it only when the action is genuinely one-way.

## The pawls (the one-way doors)

| Pawl | What it is | Why irreversible | Already guarded by |
|---|---|---|---|
| **mutate shared trunk** | Push/merge into the **shared** trunk (not a local merge); close a bead as accepted; **or rewrite a shared ref** | Shared ground-truth propagates to every consumer | pre-push gate · `/pawl-review` lane evidence · `ao pawl` CONFIRMED verdict bound to current head |
| **delete** | Destroying data/code/state: `rm -rf`, `git reset --hard`, `DROP DATABASE`, `kubectl delete`, `terraform destroy` | The thing is gone; no undo | [`dcg`](../../skills/dcg/SKILL.md) (destructive-command guard) |
| **external-send / shared-state mutation** | Anything that affects state outside your local sandbox: publish, post, deploy, email, a PR/issue to a forge, a side-effectful API call, sending to a person, **or writing to a shared/prod store** (a shared DB, a deployed service's state) | Caching / indexing / people / downstream consumers make it un-retractable even if later "deleted" | *(this list is the trigger)* |
| **schema / contract change** | Changing an interface, schema, or contract other code/agents depend on; regenerating a factory surface; repointing a contract test or canary | Downstream consumers break silently — "looks fine here" ≠ fine for them | [`scope`](../../skills/scope/SKILL.md) (frozen dirs) · contract-canary gates |
| **credential / authority change** | Granting access, rotating or overwriting a credential, changing permissions or who-can-do-what | A granted/leaked credential or authority can't be cleanly un-granted; trust changes propagate | [`dcg`](../../skills/dcg/SKILL.md) · `claude-acct` fail-closed verify |
| **spend** | Actions that cost real money or burn quota at scale: paid API runs, large agent fan-outs, deploys that bill | Money/quota is spent; you can't un-spend it | *(partial: fan-out consent)* |
| **plan-pawl** *(`multi-model`)* | Committing a **fanout-class / irreversible discovery plan** — an architecture fork, a one-way-door decision, a contract/coordination change — *before* the fan-out builds on it | Once N agents (or one expensive wave) build on the wrong plan SHAPE, the direction is costly-to-impossible to reverse; the plan is the architecture, and the architecture is the one-way door | the SAME enforcement as every pawl — the [`multi-model` diversity mode](#diversity-mode-per-pawl-fresh-context-by-default) (fresh-context floor **plus** ≥2 families) + [REFUTED→auto-redo loop + circuit-breaker](#escalation-the-circuit-breaker-model) **verbatim**, written through `schemas/pawl-verdict.v1.schema.json`. Invoked by the [`discovery`](../../skills/discovery/SKILL.md) skill's plan-pawl **duel** over the plan artifact (the consumer-side wiring is delivered by epic `age-plan-pawl-9yib`) |

> **Scope note (plan-pawl).** The plan-pawl gates the plan's **SHAPE**, never behavior — it is the
> [`multi-model`](#diversity-mode-per-pawl-fresh-context-by-default) mode applied to the discovery
> *plan artifact* instead of a code diff (recombination + naming, not a new engine). It **never**
> replaces the acceptance-test layer: a clean cross-family plan review still misses defects only a
> running test catches (the 2026-06-12 auth-bypass learning). It is **risk-class-gated** — ON for
> fanout/irreversible discovery, OFF for cheap reversible MVP vertical slices (gating those is the
> waterfall this file forbids). By design it **subsumes** the discovery skill's two redundant
> cross-family-review gates (the single-judge fanout approval + the pre-mortem council) into one gate
> — that consumer-side fusion is wired into the discovery skill by epic `age-plan-pawl-9yib` (this
> contract row defines the pawl + its governance; it does not itself edit the discovery skill). The escalation governor (auto-redo on REFUTED;
> a tripped breaker takes one bounded helper pass; human only past the helper or on the
> classes that skip it) is the [circuit-breaker model](#escalation-the-circuit-breaker-model) above, unchanged.

## The blast-radius rule (the list is examples, not the boundary)

The table above is the *common* one-way doors, not an exhaustive taxonomy — a
static list breaks by omission (it silently misses CI/gate edits, authz/default
changes, data migrations, alert disabling, generated-registry regen,
AGENTS/skill contract edits, and "temporary" bypasses). The boundary is a
**rule**; the list is just its frequent instances:

> **An action is a pawl if ANY clause holds:** it **mutates shared state**, it
> **changes enforcement / gate logic**, it has an **external effect** (something
> leaves the machine), or it is **hard to roll back**. Otherwise it is chaos —
> run it free.

When you hit an action that isn't in the table, apply the rule, not the list. If
it trips any clause, it is a missing pawl: treat it as a pawl now, and add it.

`scripts/intake.sh` applies this rule at the front door as **advisory triage** —
CHAOS routes solo, PAWL routes to the cross-family Navi for a receipt before the
door. It is a cheap early-warning (a keyword classifier, kept deliberately
simple and fail-toward-PAWL), **not** the enforcement: the un-leakable gates are
at the doors themselves — `scripts/reconcile-pr.sh` for the merge and
[`dcg`](../../skills/dcg/SKILL.md) for destructive commands. A pawl the triage
misses is still caught at the door; it just means the Navi is summoned later than
ideal, never that a one-way door ships ungated.

## Diversity mode — per-pawl (fresh-context by default)

The gate's **diversity requirement is per-pawl and operator-tunable** — the same way the
circuit-breaker thresholds (max-attempts, time/cost budgets) are tunable. Each pawl has a
`mode`; the verdict records it (`schemas/pawl-verdict.v1.schema.json` → `mode`) and the merge
path enforces it (`scripts/pawl-verdict.sh check`). The **fresh-context floor is the foundation
and is enforced in EVERY mode** — `multi-model` is strictly STRONGER than `fresh-context`, not a
swap: it **adds** family-diversity **on top of** the fresh-context floor, it does not waive it.
(A 2-family verdict whose refuters both ran in the author's own context is family-diverse but
has zero context-independence — a self-approval bypass — so it is refused, age-les.) Every other
guarantee (CONFIRMED, `head_sha` == the PR's live head, real non-empty reviewer evidence,
schema-validity, terminal green CI, roster-validated families) is **mode-independent and always
enforced**.

| Mode | Requirement | Catches | Cost | Use for |
|---|---|---|---|---|
| **`fresh-context`** *(DEFAULT)* | ≥1 refuter whose `context_id` != the verdict's `author_context_id` — a genuine fresh red-team: a **separate invocation** that did not share the author's accumulated context. **Model-agnostic** — same model in a fresh context counts. | the author's **tunnel-vision / accumulated-context errors** — the dominant failure (a worker rubber-stamps its own work *because it has the author's context*). | one extra invocation | **every pawl, by default.** Cheap; keeps the ratchet's iteration cheap. |
| **`multi-model`** *(OPT-IN)* | the fresh-context floor (above) **AND** ≥2 **distinct, roster-validated model families**, all CONFIRMED — strictly stronger, not a swap. | the above **plus** a single model's **systematic blind spots** (a whole family sharing a failure mode). | ≥2 family invocations (cross-vendor) | opt up only the **highest-irreversibility doors**. |

**Default to `fresh-context`.** A fresh-CONTEXT reviewer is the high-value, low-cost catch:
most landing misses are the author not seeing what it was too close to, and a separate
invocation — even of the same model — surfaces them. Reserve **`multi-model`** for the doors
where a model's *systematic* blind spot would be catastrophic and irreversible: e.g. **a shared
ref rewrite** (`git push --force` / history rewrite on a pushed branch) or **a schema / contract
change** that silently breaks every downstream consumer. Routine **mutate-shared-trunk** merges
ride the cheap default. (Opting a pawl up is a one-line operator choice, like raising a breaker
threshold — set the verdict's `mode` to `multi-model` for that pawl.)

This keeps the ratchet cheap by default while letting the operator pay for the stronger door
exactly where irreversibility earns it — the same economy as the pawl list itself (gate only
where it's worth it).

## Evidence-quality floor — a CONFIRMED must carry substance (age-rk3r.11)

Evidence-binding proves a review file *exists and is non-empty*. It does **not** prove the
review carried **substance**: a 155-byte "no blocking defects" stub once passed `check`. The
**evidence-quality floor** (`scripts/pawl-verdict.sh check`) raises the bar — a CONFIRMED's
evidence must carry **one of**:

- **a file:line-shaped finding/observation** — the review cites concrete code (`path.ext:NNN`,
  or a `line N` reference). A genuine review of a diff does this naturally; **real reviews pass
  untouched**. This is the primary signal.
- **an explicit reviewed-scope attestation** — a *files-reviewed count* plus the file names
  (e.g. `Files reviewed: 2 (scripts/pawl-verdict.sh, schemas/pawl-verdict.v1.schema.json)`).
  This is the **escape for a legitimately clean review** that found nothing concrete to cite:
  a review that blocks nothing must still *attest what it looked at*.

**Plus a per-adapter genuine-run marker.** Each refuter attributable to a cold reviewer adapter
must carry that adapter's genuine-run marker in its **own** evidence — `codex` = `tokens used`,
`agy` = `VERDICT:` (mirroring the reviewer-adapter contract in `scripts/lib/codex-exec.sh`
`reviewer_adapter_marker`). A family with no cold-adapter marker defined yet (`claude`, the warm
reviewer) is skipped — advisory, never a hard fail. This distinguishes a real cross-family review
from a lazy stub or an echo.

**It measures SUBSTANCE, not correctness.** The floor proves a real, specific review *ran over
named code* — it does **not** and **cannot** certify the verdict is *right*; a substantive review
can still be wrong. `check` prints that caveat whenever the floor runs.

**Rollout is ADVISORY-FIRST.** Until `FLOOR_ENFORCE_AFTER` (`scripts/pawl-verdict.sh`; env override
`PAWL_FLOOR_ENFORCE_AFTER`) the floor only **measures + warns** — it never changes the
authorize/refuse decision — so the false-positive rate on real reviews is observed for one cycle
before it fail-closes. On/after the flip date a violation **HOLDs** (a CONFIRMED without substance
names the floor; a refuter without its marker names the adapter). `PAWL_FLOOR_ENFORCE=1|0` forces
enforce|advisory regardless of the date (operator kill-switch). *At flip time*, bump/remove the
date **and** update any stub-evidence behavior-lock suites (their thin fixtures deliberately carry
no substance and begin to HOLD).

## What is NOT a pawl (chaos — run free, ungated)

Editing a file · writing a test · running a build · a local experiment · a draft · a throwaway branch · a read-only query · an intermediate RPI slice · a mock→real swap · trying an approach and discarding it. **None of these are irreversible. Do not gate them.** Iterate cheaply; the pawl catches you at the door.

## What "good" means — the bar a change must clear to PASS the gate

The pawl fires at a one-way door (above) and an independent reviewer must CONFIRM. But CONFIRM
against *what bar*? The wrong answer — a maximal-adversarial *"actively REFUTE anything
plausible-but-wrong"* reviewer — has a documented **infinite false-alarm tail**: on any non-trivial
change it always finds *one more* cosmetic / theoretical / pre-existing / hallucinated nit, and since
the gate is all-must-confirm fail-closed, one nit blocks forever. That is the gate eating its own
productivity (observed 2026-06: a single landing refuted ~9× with ~2/3 of findings non-blocking).
**The gate's job is to stop a *bad merge*, not to demand a *perfect* one.**

**"Good" = would a thoughtful senior engineer BLOCK this merge?** — never *"can I find any
imperfection?"* A finding REFUTEs the change only when it clears **all three** filters:

1. **Introduced** — the defect is created or newly *made-reachable* by THIS diff (against its
   parent). A problem true of the codebase *before* this commit is a backlog note, not a merge
   blocker; adjacent hardening the change merely "could have also done" is scope-creep, not a
   defect in the change.
2. **Real / verifiable** — concrete, and it survives deterministic ground truth. A finding
   contradicted by a green check the reviewer can actually run (`go vet`, `go test ./pkg`,
   `bash -n`, the parity/regen/audit scripts) is *wrong* — drop it. No claims about a file the
   reviewer did not read. (This is the hallucination kill-switch.)
3. **Blocking** — it breaks correctness or safety (**fail-open**: writes/certifies what it should
   not · **data-loss / corruption** · **wrong-object**), makes a **claimed contract** false (a
   documented behavior, a commit-message promise, a public interface, or a test's stated guarantee
   — *including a test that would pass even if the code were wrong*), or ships **non-working**
   (does not build/parse, the relevant test fails, or it provably does not do what the change says).

A finding that fails *any* filter is the **accepted tail** — record it as an audited NOTE, do not
block: cosmetic/style, a coverage gap on otherwise-correct code, a theoretical edge-case with no
reachable path in this diff, adjacent/out-of-scope hardening, a pre-existing condition, or a
design-disagreement with a deliberate documented choice whose worst case stays fail-closed.
**CONFIRM when the only remaining findings are tail.**

**Fail-closed is never relaxed** (the bar is calibrated, not fail-open): ambiguity holds; a reviewer
that timed out, crashed, or couldn't read the change is *no verdict*, never a CONFIRM. And there is
**no author-selectable knob** — the calibrated bar is the mandatory default posture
([`scripts/pawl-review.sh`](../../scripts/pawl-review.sh)), so an author cannot route around it to
leniency (the anti-Goodhart property council C built). When even a calibrated reviewer can't
converge — a *different* non-blocking nit every round — that is the circuit-breaker's
**no-forward-progress** signal (below): trip the breaker — one bounded helper pass, then a
human if it survives — never loop forever.

## Escalation — the circuit-breaker model

**The human is NOT needed at a pawl by default.** A pawl fires the pawl gate
([`/pawl-review`](../../skills/pawl-review/SKILL.md)) **autonomously — model reviews model.**
The loop self-corrects; the human is the exception a *circuit breaker* trips into, not the checkpoint.

The model has three layers: a **default auto-redo loop**, a set of **tunable circuit breakers** that bound it, and a **bounded helper pass** that sits between a breaker trip and the human — a stuck context consults a fresh one before it consults the operator.

- **PASS (CONFIRMED)** → proceed through the door. No human.
- **FAIL (REFUTED) → AUTO-REJECT → AUTO-REDO (the default path, no human).** A REFUTED verdict means the gate *rejected*; the loop **automatically** sends the work back to be re-done with the findings and **re-gates** it. This is continuous self-correction: the loop redoes on REFUTED **on its own**, with no human in the loop. A plain REFUTED is *never* an escalation — it is the ordinary, expected path, and most FAILs converge here.
- **A CIRCUIT-BREAKER trip stops the auto-redo loop — but its first stop is the HELPER, not the human.** The breakers are **plural and operator-tunable** — they bound the auto-redo loop so it can't burn forever:
  1. **max-attempts** — N re-work/re-gate cycles still REFUTED (default 3, tunable).
  2. **time budget** — wall-clock with no productive forward progress (the evolve loop's existing 60-min "no productive work" breaker; tunable).
  3. **cost / quota budget** — paid API spend or usage-quota ceiling for the loop (tunable).
  4. **oscillation / no-forward-progress** — the *same* failure repeating (the evolve oscillation quarantine: a target with 3+ improved→fail transitions; tunable threshold). Also covers reviewer deadlock — refuters contradicting and staying contradicted is a no-forward-progress signal.
  5. **explicit judgment flag** — a reviewer explicitly raises a value / irreversibility judgment that models should not make alone (an immediate, hard breaker).

  These are the **same governor** the autonomous loop already runs: the evolve circuit breakers (time-based + oscillation quarantine, Step 1 / [`scripts/evolve/halt-check.sh`](../../scripts/evolve/halt-check.sh)). The pawl gate *references and extends* that mechanism as its escalation governor; thresholds are configurable (e.g. `EVOLVE_KILL_TTL_DAYS`, `--max-cycles`, max-attempts) rather than hard-coded.

- **THE HELPER PASS — one bounded consult before the operator.** Breakers 1 and 4 are *stuck states*, and breaker 2's default form — wall-clock with **no productive forward progress** — is a *stall signal*, not a spent ceiling. Stuckness is usually model-adjudicable: the context that ground to a halt is in a rut a fresh one is not in. On those trips the loop takes **exactly one helper pass** — hand the blocker statement, the evidence, and what was tried to a **fresh context, a cross-family model (`codex exec`), or a [`/council`](../../skills/council/SKILL.md) panel** — which returns **UNSTUCK** (a concrete next action; the loop resumes with it, breaker counters reset for the *new* approach) or **ESCALATE** (it confirms the blocker needs a human). The helper is an *advisor, never a second driver*: it reasons about the blocker and returns a recommendation; it does not take over the work or own the loop. Bounds, so the helper cannot become its own grinder: **one pass per distinct blocker class** — a blocker class that survives its helper pass goes to the human, never to a second pass. A budget trip takes the pass only while its governing ceiling still has room (breaker 3's spend/quota ceiling, or a breaker-2 trip that is a hard operator deadline rather than the stall detector); a *spent* ceiling skips the helper entirely (next bullet).
- **ESCALATE to a human — only past the helper, or on the classes that skip it.** Breaker 5 (explicit judgment flag), the refusal lane (money, legal, irreversible-external), and a **spent hard ceiling** — breaker 3's cost/quota ceiling, or a hard time deadline, with no room left — go **straight to the human; the helper is skipped**: no model consult can own those, and a spent ceiling buys no consults. Everything else reaches the human only when its helper pass failed to unstick it or the helper itself returned ESCALATE.

**REFUTED → auto-redo (loop, no human). Breaker-trip → HOLD + helper pass; human only past the helper.** The verdict disposition is set accordingly: a plain REFUTED carries `disposition: REFUTED` and the loop re-works; the disposition is flipped to **`ESCALATE` / `HOLD` only when a breaker trips**, never on plain REFUTED. When a breaker trips the action does **not** proceed: the merge/push is **held**, not landed, not retried-into-landing, while the helper pass runs; it is surfaced for a human when the pass fails to unstick it, the helper returns ESCALATE, or the class skips the helper. A helper UNSTUCK never opens the door — it only resumes the *work*, and the hold lifts solely by re-earning a `CONFIRMED` verdict through the gate. Non-convergence **never auto-lands** — fail-closed is the whole point. The enforcing merge path (`scripts/reconcile-pr.sh` → `scripts/pawl-verdict.sh check`) records the `ESCALATE`/`HOLD` disposition and exits **5 (HOLD: no merge, no close)**; only a `CONFIRMED` pawl verdict opens the door. (A bare `REFUTED` verdict also exits 5 at the merge path — the merge is correctly refused — but the *loop's* response to REFUTED is auto-redo, not human escalation; the merge-path HOLD is just fail-closed enforcement while the redo happens.)

This breaker-governed escalation is the **andon** ("Hey! Listen!") — rare and *earned*, never the default. Even fully unattended, the gate runs model-to-model at every pawl and auto-redoes on REFUTED; pulling a human in is the exception that fires only when a tunable circuit breaker trips **and the helper pass could not resolve it** (or the class skips the helper) — and until the human acts, the pawl **holds**.

> **Scope note (threat model).** The pawl verdict (`schemas/pawl-verdict.v1.schema.json`) is an **evidence-bound, commit-bound verdict that requires real reviewer runs** — it defends against a *sloppy agent that skips the real review and self-stamps CONFIRMED*. It is **not** cryptographic provenance: there are no signatures, no peercred, no OS-level writer separation, and cryptographic un-forgeability against a hostile forger is **intentionally out of scope** (single-operator trusted loop — the cut cathedral). What the gate guarantees is that a review *actually ran* (evidence files exist + non-empty), against the *current commit* (head_sha == the PR's live head), by reviewer(s) meeting the pawl's **diversity mode** — fresh-context (≥1 fresh red-team, model-agnostic) by default, or multi-model (the fresh-context floor **plus** ≥2 roster-validated families — strictly stronger) where a pawl is opted up (see [Diversity mode](#diversity-mode-per-pawl-fresh-context-by-default)).

## Directive precedence — autonomy never overrides a human gate

A pawl **HOLD** (the merge held on a breaker trip, above) and a **human STOP/KILL marker** both live in the
**deterministic boundary**. An **autonomous-drive directive** — `/goal`'s "do not pause to ask" Stop-hook, `/loop`'s
"keep driving", on-the-loop / NTM unattended runs — lives in the **stochastic reasoning core**. The core cannot relax
the boundary: **no drive directive overrides a HOLD or a human STOP**, and "be autonomous" is *never* authorization to
self-approve a door a human deliberately gated.

- The two HOLD senses are distinct: a **breaker-HOLD** holds the *merge* until a breaker clears (the [Escalation](#escalation-the-circuit-breaker-model) model above); a **human STOP marker** halts the *loop* (the evolve Red Button / [`halt-check.sh`](../../scripts/evolve/halt-check.sh) — pre-cycle, mechanical, "not prose the agent can rationalize past"). Neither is overridable by a drive directive.
- This **generalizes the evolve Red Button to the in-session path**: an explicit human-authorization gate (a typed ACK on an irreversible action) is a marker in the boundary — an active drive directive does not consume or satisfy it. A drive loop may re-enter and re-present the gate; it does not cross it. Crossing requires the marker cleared by the human, not by the directive that said "keep going".

## Adding a pawl

A new pawl earns its place **only** if the action is genuinely irreversible — data lost, money spent, something left the machine, or a shared contract changed. If it's recoverable, it's chaos, and it stays ungated. Keep this list short: every pawl is a tread you now validate, i.e. a step back toward waterfall.

## Operating the warm pawl-service: idle reaping

> **Operator-only (requires NTM).** Everything in this section is OPTIONAL operator
> machinery: the warm verbs (`up`/`down`/`reap`/`health`/`doctor`/`smoke`/`route`/`metrics`)
> drive tmux panes through the NTM swarm substrate and expect the repo under a
> `projects_base`. **A user never needs any of it** — the front door is plain
> `ao pawl review`, which runs cold (codex/agy one-shot) from any git repo with zero
> NTM, zero `projects_base`, zero config. `ao pawl --help` groups the surface the same way.

The cross-family pawl can run as a **standing warm service** (`ao pawl up` — capability-adaptive over the installed families; see [`scripts/pawl.sh`](../../scripts/pawl.sh)) so reviews route to warm panes instead of spinning a cold `codex exec` each time. Warm panes hold a model-account slot, so the service has an idle reaper:

- **`ao pawl reap`** tears the session down **iff** it has been idle longer than `PAWL_IDLE_TTL` (default 1800s); otherwise it is a no-op. The next review's lazy-auto-up brings the service back.
- AgentOps ships **no in-repo daemon or scheduler** ([ADR-0009](../adr/ADR-0009-daemon-deletion-in-session-only.md)): out-of-session orchestration is a swappable substrate, never in-repo. So the reap *schedule* lives in your substrate, not the repo:
  - **cron:** `*/30 * * * * cd /path/to/agentops && ao pawl reap >> /tmp/pawl-reap.log 2>&1`
  - **launchd:** a `StartInterval=1800` agent that runs `ao pawl reap` in the repo dir.
  - **NTM:** call `ao pawl reap` on a tending tick.
- Without a schedule, the panes stay warm until an explicit `ao pawl down`. That is safe — the membrane gates on the recorded verdict, never on a live pane — it just holds account slots.

### Service-command contract (age-l3xj hardening)

Every `ao pawl` service verb (`up`/`down`/`reap`/`health`/`doctor`/`smoke`/`route`/`metrics`) runs under the **same trust split** as `ao pawl review`:

- **Genuine checkout** (the running `ao` binary physically lives inside the resolved AgentOps repo — forge-proof, unlike marker files): the LIVE `scripts/pawl.sh` runs, so a script edit is exercised immediately.
- **Installed binary, any git repo**: the **embedded** `pawl.sh` bundle runs against that repo, with a sanitized environment (trusted PATH, `BASH_ENV`/`ENV`/`GIT_EXTERNAL_DIFF` neutralized, `PAWL_UNTRUSTED_REPO=1`). A repo-planted `scripts/pawl.sh` is **never executed**. The session's family/pane **layout** is a property of the (global) tmux session, so it lives in a **session-scoped shared** file (`${TMPDIR:-/tmp}/pawl-session-<session>.json`) — a second repo routing to one existing `PAWL_SESSION` reads the same layout `up` wrote, not a wrong default. The per-repo `metrics.jsonl` stays under **that repo's** `.agents/pawl/`; a symlink anywhere in the state path (ancestor or leaf) is refused/neutralized so writes never escape the repo.
- **Outside any git repo**: fail closed before mutation, naming the requirement.

**`ao pawl up` (spawn).** The scripts resolve the swarm binary through one ntm-first seam (`PAWL_SWARM_BIN` override → the public `ntm` → the operator's `atm` alias; `doctor` reports which resolved as `swarm-bin`). `ntm spawn <project>` roots its panes at `projects_base/<project>` — there is no cwd flag — so `up` can only spawn correctly when the repo is a **direct child of the NTM `projects_base`**. The project defaults to the `basename` of the git toplevel (`PAWL_PROJECT` overrides). Before spawning, `up` **verifies** that `projects_base/<project>` resolves back to this repo; if it does not (a nested worktree, or a repo outside `projects_base`), it **fails closed before any mutation** with an actionable message — never spawning into the wrong directory. When the target session already exists, `up` is idempotent (no spawn, no verification). The read-only verbs and `route` are fully cross-repo regardless.

**Dry-run.** Global `--dry-run` on a mutating verb (`up`/`down`/`reap`/`route`/`review`) — in either `--dry-run` or `--dry-run=true` form — executes **nothing** (no tmux/NTM spawn/kill/send, no state/verdict/metric/lock write) and reports the exact planned action; with `--json` it emits exactly one JSON object (`action`, `dry_run`, `mutated`, `session`, `families`, `tier`, `planned_steps`). The planned `session` is derived exactly as a real run would (`${PROJECT}--${LABEL}`). Read-only verbs (`health`/`doctor`/`smoke`/`metrics`) may inspect real state under `--dry-run` but run with `PAWL_DRY_RUN=1`, which suppresses even prompt-clearing key sends.

**Route lease.** Exactly one route owns the service at a time. The lease is an atomic lock **directory** (`mkdir` is the sole ownership barrier) with owner metadata at `<lease>/owner`, keyed by **session** — the protected resource is the global tmux session, so it defaults to `${TMPDIR:-/tmp}/pawl-lease-<session>.lock`, **not** under any repo. `PAWL_ROUTE_LOCK` overrides; a path that is not a lease directory (it holds nothing but `owner`), or a symlink, is refused — never recursively deleted. Concurrency safety:

- A second concurrent `route` fails closed before sending to any pane or writing evidence; `down`/`reap` acquire the same lease before teardown (no check-then-kill race).
- A **stale** lease (crashed route) is broken under a **generation-scoped break-token** so exactly one breaker acts per generation and a peer's fresh lease is never touched. The freshness window is `2*ROUTE_TIMEOUT`.
- A **background heartbeat**, tied to the route process lifetime, refreshes the lease across every phase (send/respawn AND poll), so a live route is never mistaken for stale; it dies with the route, so a crashed lease still ages out. Release is **ownership-checked** (only the owning pid removes it).

**Route ids + evidence.** Bead ids are containment-validated (`[A-Za-z0-9._-]`, 1–64 chars, leading alphanumeric) **before** any file write, so evidence/state paths cannot escape their roots. Per-route packet/evidence files are scoped by **session and repo** (`${TMPDIR:-/tmp}/pawl-evidence-<session>-<repo>/`, `PAWL_EVID_DIR` overrides), and the lease/evidence slug uses a reversible (injective) encoding, so two repos — even under one shared `PAWL_SESSION` — never collide on evidence. On the embedded path the route's verdict is written into the **caller's** repo (`--dir`), never into the extracted script bundle the CLI discards on exit.

**Tier transparency.** A routed verdict is stamped `multi-model` (≥2 families — the real cross-family gate) or `fresh-context` (1 family — a single fresh-context refuter, weaker). A high-irreversibility door (push-to-main) demands `multi-model` and refuses a `fresh-context` verdict; `ao pawl review` and the pre-push gate surface the achieved tier so a single-family land is a conscious choice, not silent.

## Strict two-family cold quorum — the portable `multi-model` door (age-rk3r.13)

The [`multi-model`](#diversity-mode-per-pawl-fresh-context-by-default) mode above is the strongest
diversity requirement, and until now the only place it ran *portably* was the warm tri-family duel
(operator machinery). **`ao verify --strict`** (or the `strict: true` key in `.aoverify.yaml` →
`PAWL_STRICT`) is the **cold, portable** `multi-model` door: it runs **two DISTINCT strict-eligible
cold reviewer families**, requires **both** CONFIRMED, and writes ONE `multi-model` verdict recording
**both** families + **both** evidence paths (the ledger edge's `reviewer_family` is the joined label,
e.g. `gpt+gemini`). It **DOUBLES review cost**, so it is **opt-in only, never the default** — reserve
it for the highest-irreversibility doors (a shared-ref rewrite, a schema/contract change).

**Strict REFUSES to degrade — that refusal is the whole point.** This is the load-bearing contrast
with the failover chain ([`scripts/pawl-review.sh`](../../scripts/pawl-review.sh) — the ordinary
cold path degrades on an outage: it falls over to the next family and labels the verdict
`degraded=true`, to *keep going*). **Strict does the opposite:** a single-family answer on a strict
door is exactly what strict exists to forbid, so any family OUTAGE **HOLDs** (exit 5, non-authorizing,
no verdict) rather than falling back to one family. A REFUTED from either family is FINAL (a REFUTED is
a result, never overturned by asking the other family — the same invariant the whole gate holds).

**Eligibility is a ONE-list seam that DRIVES the voter set — and today it is honestly `codex`-only.**
`STRICT_ELIGIBLE_FAMILIES` (override `PAWL_STRICT_ELIGIBLE_FAMILIES`) is the single list of families
measured trustworthy enough to serve a strict door. In strict mode the **voters are derived from this
list** (each entry → its adapter, excluding the author family, keeping only *reachable* distinct
families) — **not** from `PAWL_REVIEWER_CHAIN`, which is the *failover*-mode ordering, a different
selection. That is what makes the seam real: **flipping ONLY `STRICT_ELIGIBLE_FAMILIES` activates the
quorum with no other change.** The set is deliberately *narrower* than the failover chain (a family
fine as a fallback is not automatically fit to be one of two independent quorum voters on a one-way
door). Right now that set is **codex only**: per the A7 bench ruling **agy is not yet strict-eligible**
(routine + degraded-fallback only until it is graduated), and there is **no cold claude-family adapter**
(LAW 0: never `claude -p`; Fable/opus are warm-only). So there is **no second strict-eligible cold
family today**, and `--strict` **DETECTS this and prints an honest UNAVAILABLE** (naming *why* + the
non-strict alternative — `ao verify` fresh-context, or `--converge`) and exits 5. It **never fakes a
strict pass and never degrades to one family**. The full two-family machinery is built and locked; the
moment a second cold family is graduated (a real measured strict certification), **flipping that one
list** turns real strict on with no other change.

**Honesty boundary (do not overstate).** What is shipped and claimable is the **mechanism + the honest
self-report**. **Active** strict cross-family protection — a real two-family default, or *any* claim of
active strict protection in docs/marketing — gates on a second measured strict-eligible family
graduating; **do not claim active strict protection anywhere** until then.
