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
| **mutate shared trunk** | Push/merge into the **shared** trunk (not a local merge); close a bead as accepted; **or rewrite a shared ref** — `git push --force`, history rewrite on a pushed branch | Shared ground-truth; a bad merge or rewritten ref propagates to every consumer and can't be cleanly un-done | pre-push gate · `/pre-land-refuters` |
| **delete** | Destroying data/code/state: `rm -rf`, `git reset --hard`, `DROP DATABASE`, `kubectl delete`, `terraform destroy` | The thing is gone; no undo | [`dcg`](../../skills/dcg/SKILL.md) (destructive-command guard) |
| **external-send / shared-state mutation** | Anything that affects state outside your local sandbox: publish, post, deploy, email, a PR/issue to a forge, a side-effectful API call, sending to a person, **or writing to a shared/prod store** (a shared DB, a deployed service's state) | Caching / indexing / people / downstream consumers make it un-retractable even if later "deleted" | *(this list is the trigger)* |
| **schema / contract change** | Changing an interface, schema, or contract other code/agents depend on; regenerating a factory surface; repointing a contract test or canary | Downstream consumers break silently — "looks fine here" ≠ fine for them | [`scope`](../../skills/scope/SKILL.md) (frozen dirs) · contract-canary gates |
| **credential / authority change** | Granting access, rotating or overwriting a credential, changing permissions or who-can-do-what | A granted/leaked credential or authority can't be cleanly un-granted; trust changes propagate | [`dcg`](../../skills/dcg/SKILL.md) · `claude-acct` fail-closed verify |
| **spend** | Actions that cost real money or burn quota at scale: paid API runs, large agent fan-outs, deploys that bill | Money/quota is spent; you can't un-spend it | *(partial: fan-out consent)* |

## What is NOT a pawl (chaos — run free, ungated)

Editing a file · writing a test · running a build · a local experiment · a draft · a throwaway branch · a read-only query · an intermediate RPI slice · a mock→real swap · trying an approach and discarding it. **None of these are irreversible. Do not gate them.** Iterate cheaply; the pawl catches you at the door.

## Adding a pawl

A new pawl earns its place **only** if the action is genuinely irreversible — data lost, money spent, something left the machine, or a shared contract changed. If it's recoverable, it's chaos, and it stays ungated. Keep this list short: every pawl is a tread you now validate, i.e. a step back toward waterfall.
