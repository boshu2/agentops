# Pawls — the one-way doors (the ratchet's static map)

> **Chaos + Filter + Ratchet = Progress** — see [brownian-ratchet.md](../brownian-ratchet.md).
> This file is the **Filter**: the short, static list of irreversible actions where the ratchet's
> pawl sits. Everything *not* on this list is chaos — cheap, wrong-tolerant, ungated. The router is
> this lookup, **not** a decision engine.

## The rule

1. About to take an action? Check it against the pawl list below.
2. **On the list → it's a pawl.** Fire the cross-family gate ([`/pre-land-refuters`](../../skills/pre-land-refuters/SKILL.md)): an independent, *different-family* reviewer must confirm before the action proceeds. Fail-closed (ambiguity → hold, never silent-proceed).
3. **Not on the list → chaos.** Just run it. No gate, no review, no ceremony. Iterate as wrong as you want between pawls — the pawls **reduce irreversible regression to the known one-way doors**, so between them you only ever touch state you can recover. (This holds only as far as the list below is *complete* and the chaos really is side-effect-free — if you find an irreversible action that isn't here, that's a missing pawl, add it.)

Why so few pawls: a pawl on *every* step is waterfall (validate every tread). It makes every wrong turn expensive and kills the cheap iteration that makes agents productive. The ratchet works **because** the pawls are few and sit only at the irreversible points. Adding a pawl is adding a tread you now validate — do it only when the action is genuinely one-way.

## The pawls (the one-way doors)

| Pawl | What it is | Why irreversible | Already guarded by |
|---|---|---|---|
| **mutate shared trunk** | Push/merge into the **shared** trunk (not a local merge); close a bead as accepted; **or rewrite a shared ref** — `git push --force`, history rewrite on a pushed branch | Shared ground-truth; a bad merge or rewritten ref propagates to every consumer and can't be cleanly un-done | pre-push gate · `/pre-land-refuters` (**enforced**: `scripts/reconcile-pr.sh` requires a CONFIRMED cross-family verdict via `scripts/pawl-verdict.sh check` — green CI alone never authorizes the merge; schema `schemas/pawl-verdict.v1.schema.json`) |
| **delete** | Destroying data/code/state: `rm -rf`, `git reset --hard`, `DROP DATABASE`, `kubectl delete`, `terraform destroy` | The thing is gone; no undo | [`dcg`](../../skills/dcg/SKILL.md) (destructive-command guard) |
| **external-send / shared-state mutation** | Anything that affects state outside your local sandbox: publish, post, deploy, email, a PR/issue to a forge, a side-effectful API call, sending to a person, **or writing to a shared/prod store** (a shared DB, a deployed service's state) | Caching / indexing / people / downstream consumers make it un-retractable even if later "deleted" | *(this list is the trigger)* |
| **schema / contract change** | Changing an interface, schema, or contract other code/agents depend on; regenerating a factory surface; repointing a contract test or canary | Downstream consumers break silently — "looks fine here" ≠ fine for them | [`scope`](../../skills/scope/SKILL.md) (frozen dirs) · contract-canary gates |
| **credential / authority change** | Granting access, rotating or overwriting a credential, changing permissions or who-can-do-what | A granted/leaked credential or authority can't be cleanly un-granted; trust changes propagate | [`dcg`](../../skills/dcg/SKILL.md) · `claude-acct` fail-closed verify |
| **spend** | Actions that cost real money or burn quota at scale: paid API runs, large agent fan-outs, deploys that bill | Money/quota is spent; you can't un-spend it | *(partial: fan-out consent)* |

## What is NOT a pawl (chaos — run free, ungated)

Editing a file · writing a test · running a build · a local experiment · a draft · a throwaway branch · a read-only query · an intermediate RPI slice · a mock→real swap · trying an approach and discarding it. **None of these are irreversible. Do not gate them.** Iterate cheaply; the pawl catches you at the door.

## Escalation — the circuit-breaker model

**The human is NOT needed at a pawl by default.** A pawl fires the cross-family gate
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

**REFUTED → auto-redo (loop, no human). Breaker-trip → HOLD/escalate.** The verdict disposition is set accordingly: a plain REFUTED carries `disposition: REFUTED` and the loop re-works; the disposition is flipped to **`ESCALATE` / `HOLD` only when a breaker trips**, never on plain REFUTED. When a breaker trips the action does **not** proceed: the merge/push is **held**, not landed, not retried-into-landing, and surfaced for a human. Non-convergence **never auto-lands** — fail-closed is the whole point. The enforcing merge path (`scripts/reconcile-pr.sh` → `scripts/pawl-verdict.sh check`) records the `ESCALATE`/`HOLD` disposition and exits **5 (HOLD: no merge, no close)**; only a `CONFIRMED` cross-family verdict opens the door. (A bare `REFUTED` verdict also exits 5 at the merge path — the merge is correctly refused — but the *loop's* response to REFUTED is auto-redo, not human escalation; the merge-path HOLD is just fail-closed enforcement while the redo happens.)

This breaker-governed escalation is the **andon** ("Hey! Listen!") — rare and *earned*, never the default. Even fully unattended, the gate runs model-to-model at every pawl and auto-redoes on REFUTED; pulling a human in is the exception that fires only when a tunable circuit breaker trips — and until the human acts, the pawl **holds**.

> **Scope note (threat model).** The cross-family pawl verdict (`schemas/pawl-verdict.v1.schema.json`) is an **evidence-bound, commit-bound cross-family verdict that requires real reviewer runs** — it defends against a *sloppy agent that skips the real review and self-stamps CONFIRMED*. It is **not** cryptographic provenance: there are no signatures, no peercred, no OS-level writer separation, and cryptographic un-forgeability against a hostile forger is **intentionally out of scope** (single-operator trusted loop — the cut cathedral). What the gate guarantees is that a review *actually ran* (evidence files exist + non-empty), against the *current commit* (head_sha == the PR's live head), by *two real families* (roster-validated, normalized labels).

## Adding a pawl

A new pawl earns its place **only** if the action is genuinely irreversible — data lost, money spent, something left the machine, or a shared contract changed. If it's recoverable, it's chaos, and it stays ungated. Keep this list short: every pawl is a tread you now validate, i.e. a step back toward waterfall.
