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

## Constraints

- Consequential access is the package-owned `scripts/safe-gc.sh` surface. It
  exposes read-only `check`, exact-token `prepare`, and exact-token `dispatch`
  because this keeps lifecycle-changing upstream commands outside the adapter.
- The city is a canonical non-root directory with `city.toml`; the rig is a
  canonical named directory below that city's `rigs/`. Symlinked paths and the
  operator home fail closed because ambiguous city identity can target a
  different factory.
- Runtime use requires structured Gas City `1.4.x`, AgentOps `3.5.0`, the
  required command/help surfaces, and bounded `ao gc check` qualification.
  Missing or development versions such as `edge` stop before mutation or mail
  because their behavior is outside this package's attested contract.
- Dispatch accepts one open source bead whose native record contains a title
  and acceptance. One caller-approved, digest-bound message goes to `mayor`
  with notify; direct sling and session lifecycle are absent from the surface.
- After dispatch, observation is capped at 12 polls spaced 30 seconds apart
  (six minutes). Terminal run state, Mayor input, or the deadline stops the
  loop. Stop after at most 12 polls; one explained-stall message is the full
  nudge budget.
- Factory state remains runtime evidence. A separate fresh validation judges
  semantic acceptance.

The adapter cannot select AgentOps semantics, issue a binding verdict, or turn
factory completion into delivery or validation proof.

## Enforced operation surface

Generate a token without contacting either runtime, show that exact token to
the caller, and pass the caller-returned value to the corresponding operation:

```sh
skills/using-gc/scripts/safe-gc.sh token prepare \
  --city /path/to/city --rig /path/to/city/rigs/project
skills/using-gc/scripts/safe-gc.sh prepare \
  --city /path/to/city --rig /path/to/city/rigs/project \
  --approve 'gc:prepare:...'

skills/using-gc/scripts/safe-gc.sh token dispatch \
  --city /path/to/city --rig /path/to/city/rigs/project \
  --bead ago-123 --message-file /absolute/request.txt
skills/using-gc/scripts/safe-gc.sh dispatch \
  --city /path/to/city --rig /path/to/city/rigs/project \
  --bead ago-123 --message-file /absolute/request.txt \
  --receipt /absolute/gc-dispatch.json --approve 'gc:dispatch:...'
```

`prepare` performs one bounded native prepare followed by a read-only check.
`dispatch` qualifies the exact rig again, reads the native source bead, and
sends exactly one structured, notified mail to the Mayor. Receipt
`not_checked` fields keep the external runtime, Mayor processing, and eventual
model dispatch explicit.

## Choose the factory first

AgentOps supports Gas City and other external software factories. Use this
skill only for Gas City. AgentOps supplies skills and evidence contracts; it
does not absorb the factory's roles, formulas, queues, or lifecycle.

Install or link AgentOps skills into provider runtimes before workers start.
The upstream Mayor, coordinator, and workers can then discover and select
`plan`, `implement`, `test`, `validate`, and other AgentOps skills normally.

## Gas City 1.4 operating model

Gas City 1.4 is run-centered. The supervisor serves typed session/run state.
Each graph-owning city or rig needs its upstream `core.control-dispatcher`;
workers claim routed work, `gc.mayor` coordinates, and `gc.run-operator`
launches and supervises formulas.

The normal path is:

1. Pin the official workflow and rig-role imports.
2. Add the project as a rig and make AgentOps skills visible to its sessions.
3. Create one caller-owned source intent bead with acceptance.
4. Use the wrapper to hand that bead to the Mayor; the Mayor authors workflow
   beads and dispatches the selected upstream formula.
5. Read native run, session, bead, artifact, and verdict state.

`ao gc prepare` snapshots upstream validation assets unchanged, installs small
AgentOps-owned check wrappers, links skills, selects a Python with PyYAML, and
pre-seeds Codex workspace/hook trust for session directories that already
exist. It does not modify the GC binary or upstream packs. `ao gc check` reads
local state and writes no adapter files.

Two limits remain visible:

1. `check` sees a recorded hook hash but does not ask Codex whether it is now
   stale. The bounded `prepare` is the operation that can detect and refuse a
   changed hook.
2. A session home created after `prepare` was not present to seed. Run the
   approved prepare again only under a newly obtained token before dispatch.

## Preferred pack and registries

The built-in `main` registry catalogs official packs; community registries are
optional operator configuration. Inspect with `gc pack registry list`,
`search`, and `show`. Exact accepted releases and `packs.lock` make a city's
imports reproducible.

The accepted reference is upstream `gascity` 0.1.6 at commit
`3b3b89f2011e06d84459aa7bea1552382f13930a`. It supplies `build-basic`,
implementation/review flows, stock `gc.*` roles, `core.control-dispatcher`, and
the `gc.mayor` coordinator. Installation and registry changes remain explicit
operator work outside this package wrapper.

Work enters through the Mayor. The caller owns one source intent bead; the
Mayor owns decomposition, workflow beads, dispatch, retries, and tending.
Required AgentOps skills belong on the actual generated work item or worker
prompt because presence and invocation are separate facts.

## Failure reasoning

Named failure mode — **handmade-session squatting**: manually creating a
pack-owned singleton takes its canonical name while remaining mis-scoped, so
the reconciler cannot create the real session.

Anti-pattern: re-slinging or patching a stalled run from the operator lane.
Corrective: observe in the bounded order, mail the Mayor once with exact
bead/run evidence, and stop at the envelope. This preserves one lifecycle
owner and prevents a racing second author.

The detailed read-only visibility order, upgrade boundary, stall envelope, and
completion handoff live in [bounded operations](references/OPERATIONS.md).

## Quality, boundaries, and done

- GC quests, attempts, stalls, cancellations, and close state stay in GC.
- This surface performs no automatic selection, semantic validation, Git,
  integration, closure, release, delivery, direct sling, session creation,
  affinity apply, doctor fix, supervisor change, or import mutation.
- Applied success for `prepare` requires the post-check. Dispatch success
  requires one structured Mayor mail receipt with `notified: true`.
- A timeout, failed qualification, missing capability/version, changed message,
  unsafe path, missing acceptance, or failed notify is terminal for the call.
  The wrapper makes no retry and launches no model itself.
- Local wrapper tests prove the package boundary only. They do not prove Gas
  City, AgentOps, Beads, mail transport, Mayor behavior, or model execution.
