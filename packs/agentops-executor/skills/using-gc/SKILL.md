---
name: using-gc
description: 'Operate a caller-selected Gas City 1.4 with upstream registry packs and native run-centered surfaces while keeping GC runtime state out of AgentOps verdicts. Triggers: "using gc", "gas city", "drive the mayor", "dispatch through gc".'
practices: [team-topologies, design-by-contract]
hexagonal_role: driving-adapter
consumes: [explicit-packets]
produces: [gas-city-runtime-evidence]
context_rel:
- kind: partnership
  with: agent-native
skill_api_version: 1
user-invocable: true
metadata:
  tier: execution
  dependencies: []
  capabilities: [dispatch_explicit_packet, observe_gc_runtime, inspect_pack_registries, drive_mayor_door]
  effects: [operate_gas_city, configure_codex_trust]
  canonical_status: canonical
  disposition: keep_optional_adapter
output_contract: runtime evidence per supplied packet
---

# Using GC

Use Gas City only when the caller explicitly selects it. Treat it as a
replaceable execution adapter, not a correctness or completion boundary.

## Choose the factory first

AgentOps supports both Gas City and the
[Agentic Coding Flywheel](https://agent-flywheel.com) as external
software-factory runtimes. Use this skill only for Gas City. If the caller
selects the Flywheel, use its native workflow instead of wrapping it in Gas
City.

AgentOps supplies skills and evidence contracts to either factory. It does not
need its own Gas City formula or role pack. Install or link AgentOps skills into
the provider runtime before starting workers; the upstream Mayor, coordinator,
and workers can then discover and select `plan`, `implement`, `test`,
`validate`, and other AgentOps skills normally.

## Gas City 1.4 operating model

Gas City 1.4 is run-centered. The supervisor serves the dashboard and typed,
paginated session/run APIs. Every graph-owning city or rig scope needs its own
`core.control-dispatcher`; that deterministic worker advances formula control
beads. Agent workers claim routed work. The upstream `gc.mayor` skill is the
guided coordinator; `gc.run-operator` launches and supervises formulas.

The normal AgentOps path is:

1. Install and pin the upstream `gascity` workflow and rig-role imports.
2. Add the project as a rig, prepare its stock maintainer runtime, and make
   AgentOps skills visible to its provider sessions.
3. Create a caller-owned bead and launch the upstream `build-basic`,
   continuation, review, or implementation formula that matches the available
   artifacts.
4. Read run, session, bead, artifact, and verdict state. Completion is never
   inferred from chat or pane prose.

Prepare and qualify a rig before its first build with the shipped AgentOps
CLI (no repo checkout required):

```sh
ao gc prepare --city /path/to/city --rig /path/to/rig
ao gc check --city /path/to/city --rig /path/to/rig
```

The command verifies the exact official workflow and role pins, snapshots the
upstream validation scripts and schemas unchanged inside the rig's `.gc`
runtime, installs only small AgentOps-owned wrappers at the formula check
paths, selects an existing Python that can import PyYAML, and links the
AgentOps skills into the city and rig Codex sinks. Skills come from the
enclosing AgentOps checkout when one is present, otherwise from the installed
skills root; pass `--skills-source` to pin a different directory. It never
modifies the GC binary, cache, formulas, roles, or upstream pack. `check`
issues only native inspection commands, writes no adapter files, and fails
before model spend when that runtime contract is missing or drifted.

## Preferred pack and registries

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

AgentOps prefers the official `gascity` build pack, the workflow family visible
in the public Maintainer City factory. The current accepted reference is
`gascity` 0.1.6 at commit
`3b3b89f2011e06d84459aa7bea1552382f13930a`:

- dashboard: `https://factory.gascity.com`;
- workflows: `build-basic`, `build-from-*`, `implement`, review, issue, and PR
  flows;
- stock rig roles: `gc.run-operator`, `gc.implementation-worker`, planners,
  reviewers, and publisher;
- scope-local formula control: `core.control-dispatcher`;
- guided coordination: the upstream `gc.mayor` skill.

Install the workflow pack at city scope and its sibling roles pack on every rig
that runs work, following the exact commands returned by
`gc pack registry show main:gascity`. Keep the stock `gc.*` namespace; do not
nest or rename the roles behind an AgentOps pack.

For a starter build:

```sh
gc bd create "Add a --json flag to the export command"
gc sling gc.run-operator <bead-id> --on build-basic \
  --var artifact_root=plans/json-flag/build
```

For guided requirements, planning, and launch, tell the active agent:

```text
Use skill gc.mayor
```

AgentOps skills are tools available to those factory agents, not a replacement
workflow. Explicitly name a skill in the bead or prompt when its behavior is
required. The current upstream decomposition does not automatically propagate a
free-form `Required Skills` section from the caller-owned source bead into every
generated work item. Inspect the decomposition before implementation; put a
required skill name on the actual work item or worker prompt when its use is an
acceptance condition. Skill presence and skill invocation are different facts.

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
- `ao gc check` accepts the contained maintainer runtime and
  AgentOps skill links;
- on macOS, the supervisor LaunchAgent resolves to the same executable as the
  selected `gc` binary;
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
Create the replacement from the upstream Gas City template, install its pinned
imports, and verify it with `gc cities --json`, `gc --city <new-city> status`,
and `gc --city <new-city> doctor --json`.

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

The upstream pack may leave a future affinity-bound step assigned to a session
that has already drain-acked. Diagnose this only from outside the city:

```sh
ao gc recover-affinity --city /path/to/city --rig /path/to/rig
```

The default is a dry run. If every listed assignment is correct, repeat with
`--apply`. The bounded repair only clears the assignee on a currently ready
formula bead whose `gc.session_affinity=require` session is no longer live. It
does not sling, retry, close, restart, or select work.

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

`gc status` may return a partial `no_agents_running` snapshot while
`gc session list --json` shows a live Mayor or worker. Treat that as an
observability disagreement, not permission to restart. Use session and pane
truth for liveness, bead/run state for workflow progress, and Doctor for
metabolism. A supervisor with abnormal CPU, a timed-out native stop, or a
recurring hook rewrite remains an upstream operational defect; this helper
reports it but never kills or patches GC processes.

The caller-owned input bead and the generated workflow root have separate
lifecycles. A successful `build-basic` run may close its workflow root while
leaving the input bead open. Likewise, `push=false` and `open_pr=false` produce
a successful no-op publish while the approved commit remains in its source
anchor worktree. Neither state is semantic completion by itself.

## Boundaries

- GC quests, runs, attempts, stalls, cancellations, and internal close state
  stay in GC. They never become AgentOps Plan, Candidate, RPI, or verdict state.
- A GC close or completed run is not AgentOps completion. Only a fresh Validate
  context issues the semantic result or, when requested, persists `verdict.v2`.
- This skill performs no automatic selection, retry, semantic validation, Git,
  integration, closure, release, or delivery.
