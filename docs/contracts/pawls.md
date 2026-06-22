# Pawls — the one-way doors (the ratchet's static map)

> **Chaos + Filter + Ratchet = Progress** — see [brownian-ratchet.md](../brownian-ratchet.md).
> This file is the **Filter**: the irreversible actions where the ratchet's pawl sits — a short list
> of the common instances, governed by the [blast-radius rule](#the-blast-radius-rule-the-list-is-examples-not-the-boundary).
> Anything that trips no clause of the rule is chaos — cheap, wrong-tolerant, ungated. The router is
> a cheap classification (list + rule), **not** a heavyweight decision engine.

## The rule

1. About to take an action? Check it against the pawl list **and the [blast-radius rule](#the-blast-radius-rule-the-list-is-examples-not-the-boundary)** below.
2. **A pawl (on the list, OR it trips any clause of the blast-radius rule) → fire the gate** ([`/pre-land-refuters`](../../skills/pre-land-refuters/SKILL.md)): an independent **fresh-context** reviewer (a separate invocation, no shared accumulated context with the author) must confirm before the action proceeds. Fail-closed (ambiguity → hold, never silent-proceed). Fresh-context is the **default** diversity mode; a pawl can be **opted up to multi-model** (≥2 model families) per the [Diversity mode](#diversity-mode--per-pawl-fresh-context-by-default) section below.
3. **Not a pawl (not on the list AND it trips no clause of the blast-radius rule) → chaos.** Just run it. No gate, no review, no ceremony. Iterate as wrong as you want between pawls — the pawls **reduce irreversible regression to the known one-way doors**, so between them you only ever touch state you can recover. (The list is the common instances; the **rule** is the authority — if an action trips a clause but isn't listed, it is still a pawl, and it's a missing list entry to add.)

Why so few pawls: a pawl on *every* step is waterfall (validate every tread). It makes every wrong turn expensive and kills the cheap iteration that makes agents productive. The ratchet works **because** the pawls are few and sit only at the irreversible points. Adding a pawl is adding a tread you now validate — do it only when the action is genuinely one-way.

## The pawls (the one-way doors)

| Pawl | What it is | Why irreversible | Already guarded by |
|---|---|---|---|
| **mutate shared trunk** | Push/merge into the **shared** trunk (not a local merge); close a bead as accepted; **or rewrite a shared ref** — `git push --force`, history rewrite on a pushed branch | Shared ground-truth; a bad merge or rewritten ref propagates to every consumer and can't be cleanly un-done | pre-push gate (`scripts/hooks/pre-push.local` → `scripts/check-pawl-pre-push.sh` on push-to-main, pr=0) · `/pre-land-refuters` · PR merge (`scripts/reconcile-pr.sh` requires CONFIRMED pawl verdict via `scripts/pawl-verdict.sh check`; green CI alone never authorizes; schema `schemas/pawl-verdict.v1.schema.json`) |
| **delete** | Destroying data/code/state: `rm -rf`, `git reset --hard`, `DROP DATABASE`, `kubectl delete`, `terraform destroy` | The thing is gone; no undo | [`dcg`](../../skills/dcg/SKILL.md) (destructive-command guard) |
| **external-send / shared-state mutation** | Anything that affects state outside your local sandbox: publish, post, deploy, email, a PR/issue to a forge, a side-effectful API call, sending to a person, **or writing to a shared/prod store** (a shared DB, a deployed service's state) | Caching / indexing / people / downstream consumers make it un-retractable even if later "deleted" | *(this list is the trigger)* |
| **schema / contract change** | Changing an interface, schema, or contract other code/agents depend on; regenerating a factory surface; repointing a contract test or canary | Downstream consumers break silently — "looks fine here" ≠ fine for them | [`scope`](../../skills/scope/SKILL.md) (frozen dirs) · contract-canary gates |
| **credential / authority change** | Granting access, rotating or overwriting a credential, changing permissions or who-can-do-what | A granted/leaked credential or authority can't be cleanly un-granted; trust changes propagate | [`dcg`](../../skills/dcg/SKILL.md) · `claude-acct` fail-closed verify |
| **spend** | Actions that cost real money or burn quota at scale: paid API runs, large agent fan-outs, deploys that bill | Money/quota is spent; you can't un-spend it | *(partial: fan-out consent)* |
| **plan-pawl** *(`multi-model`)* | Committing a **fanout-class / irreversible discovery plan** — an architecture fork, a one-way-door decision, a contract/coordination change — *before* the fan-out builds on it | Once N agents (or one expensive wave) build on the wrong plan SHAPE, the direction is costly-to-impossible to reverse; the plan is the architecture, and the architecture is the one-way door | the SAME enforcement as every pawl — the [`multi-model` diversity mode](#diversity-mode--per-pawl-fresh-context-by-default) (fresh-context floor **plus** ≥2 families) + [REFUTED→auto-redo loop + circuit-breaker](#escalation--the-circuit-breaker-model) **verbatim**, written through `schemas/pawl-verdict.v1.schema.json`. Invoked by the [`discovery`](../../skills/discovery/SKILL.md) skill's plan-pawl **duel** over the plan artifact (the consumer-side wiring is delivered by epic `age-plan-pawl-9yib`) |

> **Scope note (plan-pawl).** The plan-pawl gates the plan's **SHAPE**, never behavior — it is the
> [`multi-model`](#diversity-mode--per-pawl-fresh-context-by-default) mode applied to the discovery
> *plan artifact* instead of a code diff (recombination + naming, not a new engine). It **never**
> replaces the acceptance-test layer: a clean cross-family plan review still misses defects only a
> running test catches (the 2026-06-12 auth-bypass learning). It is **risk-class-gated** — ON for
> fanout/irreversible discovery, OFF for cheap reversible MVP vertical slices (gating those is the
> waterfall this file forbids). By design it **subsumes** the discovery skill's two redundant
> cross-family-review gates (the single-judge fanout approval + the pre-mortem council) into one gate
> — that consumer-side fusion is wired into the discovery skill by epic `age-plan-pawl-9yib` (this
> contract row defines the pawl + its governance; it does not itself edit the discovery skill). The escalation governor (auto-redo on REFUTED;
> human only on a tripped circuit breaker) is the [circuit-breaker model](#escalation--the-circuit-breaker-model) above, unchanged.

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

## What is NOT a pawl (chaos — run free, ungated)

Editing a file · writing a test · running a build · a local experiment · a draft · a throwaway branch · a read-only query · an intermediate RPI slice · a mock→real swap · trying an approach and discarding it. **None of these are irreversible. Do not gate them.** Iterate cheaply; the pawl catches you at the door.

## Escalation — the circuit-breaker model

**The human is NOT needed at a pawl by default.** A pawl fires the pawl gate
([`/pre-land-refuters`](../../skills/pre-land-refuters/SKILL.md)) **autonomously — model reviews model.**
The loop self-corrects; the human is the exception a *circuit breaker* trips into, not the checkpoint.

The model has two layers: a **default auto-redo loop**, and a set of **tunable circuit breakers** that govern when that loop stops and hands off to a human.

- **PASS (CONFIRMED)** → proceed through the door. No human.
- **FAIL (REFUTED) → AUTO-REJECT → AUTO-REDO (the default path, no human).** A REFUTED verdict means the gate *rejected*; the loop **automatically** sends the work back to be re-done with the findings and **re-gates** it. This is continuous self-correction: the loop redoes on REFUTED **on its own**, with no human in the loop. A plain REFUTED is *never* an escalation — it is the ordinary, expected path, and most FAILs converge here.
- **ESCALATE to a human — ONLY when a CIRCUIT BREAKER trips.** The breakers are **plural and operator-tunable** — they bound the auto-redo loop so it can't burn forever:
  1. **max-attempts** — N re-work/re-gate cycles still REFUTED (default 3, tunable).
  2. **time budget** — wall-clock with no productive forward progress (the evolve loop's existing 60-min "no productive work" breaker; tunable).
  3. **cost / quota budget** — paid API spend or usage-quota ceiling for the loop (tunable).
  4. **oscillation / no-forward-progress** — the *same* failure repeating (the evolve oscillation quarantine: a target with 3+ improved→fail transitions; tunable threshold). Also covers reviewer deadlock — refuters contradicting and staying contradicted is a no-forward-progress signal.
  5. **explicit judgment flag** — a reviewer explicitly raises a value / irreversibility judgment that models should not make alone (an immediate, hard breaker).

  These are the **same governor** the autonomous loop already runs: the evolve circuit breakers (time-based + oscillation quarantine, Step 1 / [`scripts/evolve/halt-check.sh`](../../scripts/evolve/halt-check.sh)). The pawl gate *references and extends* that mechanism as its escalation governor; thresholds are configurable (e.g. `EVOLVE_KILL_TTL_DAYS`, `--max-cycles`, max-attempts) rather than hard-coded.

**REFUTED → auto-redo (loop, no human). Breaker-trip → HOLD/escalate.** The verdict disposition is set accordingly: a plain REFUTED carries `disposition: REFUTED` and the loop re-works; the disposition is flipped to **`ESCALATE` / `HOLD` only when a breaker trips**, never on plain REFUTED. When a breaker trips the action does **not** proceed: the merge/push is **held**, not landed, not retried-into-landing, and surfaced for a human. Non-convergence **never auto-lands** — fail-closed is the whole point. The enforcing merge path (`scripts/reconcile-pr.sh` → `scripts/pawl-verdict.sh check`) records the `ESCALATE`/`HOLD` disposition and exits **5 (HOLD: no merge, no close)**; only a `CONFIRMED` pawl verdict opens the door. (A bare `REFUTED` verdict also exits 5 at the merge path — the merge is correctly refused — but the *loop's* response to REFUTED is auto-redo, not human escalation; the merge-path HOLD is just fail-closed enforcement while the redo happens.)

This breaker-governed escalation is the **andon** ("Hey! Listen!") — rare and *earned*, never the default. Even fully unattended, the gate runs model-to-model at every pawl and auto-redoes on REFUTED; pulling a human in is the exception that fires only when a tunable circuit breaker trips — and until the human acts, the pawl **holds**.

> **Scope note (threat model).** The pawl verdict (`schemas/pawl-verdict.v1.schema.json`) is an **evidence-bound, commit-bound verdict that requires real reviewer runs** — it defends against a *sloppy agent that skips the real review and self-stamps CONFIRMED*. It is **not** cryptographic provenance: there are no signatures, no peercred, no OS-level writer separation, and cryptographic un-forgeability against a hostile forger is **intentionally out of scope** (single-operator trusted loop — the cut cathedral). What the gate guarantees is that a review *actually ran* (evidence files exist + non-empty), against the *current commit* (head_sha == the PR's live head), by reviewer(s) meeting the pawl's **diversity mode** — fresh-context (≥1 fresh red-team, model-agnostic) by default, or multi-model (the fresh-context floor **plus** ≥2 roster-validated families — strictly stronger) where a pawl is opted up (see [Diversity mode](#diversity-mode--per-pawl-fresh-context-by-default)).

## Directive precedence — autonomy never overrides a human gate

A pawl **HOLD** (the merge held on a breaker trip, above) and a **human STOP/KILL marker** both live in the
**deterministic boundary**. An **autonomous-drive directive** — `/goal`'s "do not pause to ask" Stop-hook, `/loop`'s
"keep driving", on-the-loop / ATM unattended runs — lives in the **stochastic reasoning core**. The core cannot relax
the boundary: **no drive directive overrides a HOLD or a human STOP**, and "be autonomous" is *never* authorization to
self-approve a door a human deliberately gated.

- The two HOLD senses are distinct: a **breaker-HOLD** holds the *merge* until a breaker clears (the [Escalation](#escalation--the-circuit-breaker-model) model above); a **human STOP marker** halts the *loop* (the evolve Red Button / [`halt-check.sh`](../../scripts/evolve/halt-check.sh) — pre-cycle, mechanical, "not prose the agent can rationalize past"). Neither is overridable by a drive directive.
- This **generalizes the evolve Red Button to the in-session path**: an explicit human-authorization gate (a typed ACK on an irreversible action) is a marker in the boundary — an active drive directive does not consume or satisfy it. A drive loop may re-enter and re-present the gate; it does not cross it. Crossing requires the marker cleared by the human, not by the directive that said "keep going".

## Adding a pawl

A new pawl earns its place **only** if the action is genuinely irreversible — data lost, money spent, something left the machine, or a shared contract changed. If it's recoverable, it's chaos, and it stays ungated. Keep this list short: every pawl is a tread you now validate, i.e. a step back toward waterfall.
