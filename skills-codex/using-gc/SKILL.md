---
name: using-gc
description: Operate a caller-selected Gas City 1.4
---
# Using GC

Use Gas City only when the caller explicitly selects it. Treat it as a
replaceable execution adapter, not a correctness or completion boundary.

## Gas City 1.4 operating model

Gas City 1.4 is run-centered. The supervisor serves the dashboard and typed,
paginated session/run APIs. Every graph-owning city or rig scope needs its own
`core.control-dispatcher`; that deterministic worker advances formula control
beads. Agent workers claim routed work. The optional AgentOps Mayor is an
on-demand human/agent door for status and one manual dispatch, not a polling
control plane.

The normal AgentOps path is:

1. `invoke.sh --city C create "<title>" -d "<acceptance>"` writes one
   caller-owned source bead.
2. `invoke.sh --city C feed <bead-id>` homes it in the rig and starts the native
   `agentops-experiment` formula. `gc sling` returns a run id and dashboard deep
   link.
3. `invoke.sh --city C dashboard` prints the embedded supervisor dashboard URL.
4. Read run, session, bead, and verdict state. Completion is never inferred from
   chat or pane prose.

The Mayor remains available when explicitly useful:

```sh
invoke.sh --city C mayor start
invoke.sh --city C mayor status
invoke.sh --city C mayor tell "dispatch <bead-id>"
```

Hand it bead ids only. It never authors or claims work.

## Packs and registries

The built-in `main` registry catalogs official packs. The community registry is
optional configuration:

```sh
gc pack registry list
gc pack registry refresh
gc pack registry search --all
gc pack registry show main:gascity
gc pack registry add community https://registry.gascity.com/registry.toml
gc pack registry search --registry community --all
```

`search` reads the local registry cache; `show` reports release provenance and
exact import commands. `gc import add` declares a source/version, and `gc import
install` resolves it into `packs.lock`. Prefer an exact accepted release for
reproducible cities.

AgentOps composes the official `gascity` 0.1.6 workflow at commit
`3b3b89f2011e06d84459aa7bea1552382f13930a`, plus its rig roles. This is the
workflow family visible in the public Maintainer City factory:

- dashboard: `https://factory.gascity.com`;
- official roles, bound at the stock default-rig namespace:
  `gc.run-operator` and `gc.implementation-worker`;
- scope-local formula control: `core.control-dispatcher`;
- production formulas including `do-work`, build, review, issue, and PR flows.

The workflow pack is composed into `agentops-factory`; its sibling role pack is
instead imported as `defaults.rig.imports.gc`. Keep that split: nesting the roles
inside `agentops-factory` renames them to `agentops.*`, while the stock formulas
still target `gc.*`. AgentOps keeps `agentops-experiment` as its bounded
default. The imported pack is compositional access to upstream workflows, not
permission to replace the AgentOps verdict boundary.

## Upgrade an existing city to 1.4

Before starting its orchestrator, run once per city:

```sh
gc doctor --fix
gc import install
gc supervisor stop --wait   # macOS when an older direct supervisor remains
gc start
```

Then confirm:

- `gc version` reports `1.4.0` from the intended path;
- `gc doctor` has no blocking failures;
- each graph-owning rig has an unsuspended `core.control-dispatcher`;
- imports and `packs.lock` resolve;
- old standalone-dashboard bookmarks or reverse proxies are removed.

A stale registered city may block every start. Repair that city with `gc doctor
--fix`, or explicitly unregister it if it is intentionally retired.

Retire an old HQ/canary by exact registered name or path, without stopping the
machine-wide supervisor needed by its replacement:

```sh
gc cities --json
gc stop /path/to/old-city --timeout 45s
gc unregister /path/to/old-city
gc cities --json
```

`unregister` fails rather than silently accepting an unknown target. Preserve
the city directory until its Beads state is backed up or confirmed disposable.
Then use `deploy/gc/bootstrap.sh ... --start` for the new city and verify it with
`gc cities --json`, `gc --city <new-city> status`, and
`gc --city <new-city> doctor --json`.

## Stall protocol

First classify the bead.

- Still `ready`: dispatch it once to its `gc.run_target`, then stop and inspect.
- Already routed/in progress: re-slinging is a **NO-OP**. Wake its owning worker
  once:

  ```sh
  gc session wake <run_target>
  ```

Then capture the exact tmux pane named by session state and run `gc doctor`.
Never repair a city from inside that city.

## Visibility: four layers

1. **Supervisor/run state** — `gc dashboard`, run detail, `gc status`, and
   `gc session list --json`. Run detail unifies the stage ladder, structured
   transcripts, token rate, and estimated burn rate. A roster may still report
   active while a provider is wedged.
2. **Bead graph** — `gc bd --rig <rig> ready --json` and `show <id> --json`.
   This is workflow-state truth, but a claimed bead cannot reveal a wedged pane.
3. **Pane truth** — `tmux -L <socket> capture-pane -pt <session>`. This exposes
   trust prompts, update nags, API/DNS failures, and interactive wedges.
4. **Health machinery** — `gc doctor`, `gc order history`, storage health, and
   events. This proves metabolism, not semantic acceptance.

When layers disagree, trust the more direct observation: pane over roster for a
session wedge, bead/run state over prose for workflow completion.

## Boundaries

- GC quests, runs, attempts, stalls, cancellations, and internal close state
  stay in GC. They never become AgentOps Plan, Candidate, RPI, or verdict state.
- A GC close or completed run is not AgentOps completion. Only a fresh Validate
  context issues the semantic result or, when requested, persists `verdict.v2`.
- This skill performs no automatic selection, retry, semantic validation, Git,
  integration, closure, release, or delivery.
