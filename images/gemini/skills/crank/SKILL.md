---
name: crank
description: 'Execute one caller-selected wave by invoking RPI once per lane and return the wave evidence, then stop. Triggers: "crank", "execute the next wave", "run this wave".'
practices:
- small-batch-flow
- design-by-contract
- team-topologies
hexagonal_role: domain
consumes:
- rpi
- caller-selected-wave
produces:
- wave-evidence
context_rel:
- kind: customer-of
  with: rpi
skill_api_version: 1
user-invocable: true
metadata:
  graph_root: true
  tier: meta
  dependencies: [rpi]
  capabilities: [execute_wave]
  effects: [dispatch_rpi_per_lane]
  canonical_status: canonical
  disposition: keep
output_contract: 'wave evidence: per-lane status, the open findings each lane''s validator left, each lane''s subject manifest digest, and the wave acceptance result'
---

# Crank

Crank executes exactly one wave. The caller selects the wave and the repair
bound; crank runs that wave and returns what happened.

```text
caller's wave + repair_rounds -> RPI per lane -> wave acceptance once -> evidence -> stop
```

## Input

The caller supplies all three:

- **The wave** — a list of lanes, each with a write `scope`, a `brief`, and its
  own executable `acceptance`.
- **`repair_rounds`** — the caller's repair bound, forwarded unchanged to every
  lane. Crank neither chooses it nor spends it.
- **The wave acceptance** — the one command set that judges the wave as a whole.

If a lane arrives without a scope, a brief, or an executable acceptance, report
the wave as unrunnable and stop. Crank does not shape the missing part itself.

## Contract

1. Read the caller's wave as given. Do not select, extend, drop, or re-rank
   lanes; the wave arrives already decided.
2. Partition the lanes. Two lanes may run in parallel only when their write
   scopes are disjoint **and** their regen surfaces (generated outputs,
   mirrors, manifests) are disjoint. Any shared surface serializes them;
   unknown overlap counts as overlap.
3. Invoke [`rpi`](../rpi/SKILL.md) once per lane with that lane's intent,
   scope, acceptance, and the caller's `repair_rounds`. Each lane's traversal
   owns its own repair loop. Crank never repairs a lane itself, never
   re-invokes a returned lane, and never re-plans one.
4. Run the wave acceptance once, after every lane has returned. A lane that
   returned `FAIL` or `NOT_PROVEN` does not suppress it; both results are
   reported side by side.
5. Return the wave evidence and stop — per-lane status
   (`PASS | FAIL | NOT_PROVEN | NOT_PLANNED | NOT_BUILT`), the open findings
   each lane's validator left keyed by their `findings[].id`, each lane's
   subject manifest digest, and the wave acceptance result.

Append no next action. The caller reads the evidence and decides whether there
is another wave.

## Boundary

Crank owns no wave selection, retry, budget, queue, claim, lease, Git, closure,
delivery, or next work, and it mints no verdict of its own. Every semantic
result in the wave evidence is the one that lane's fresh validator returned,
cited unchanged; a green wave acceptance is a fact, not a `PASS`.

## Conveyor

In Claude, [`workflows/implement-wave.js`](../../workflows/implement-wave.js)
is one conveyor for step 3's parallel leg: it runs disjoint-ownership
implementers over the lanes the caller supplied. It is transport, never a
substitute for a lane's own fresh validation, and crank runs without it.

## Prompt

```text
Crank this wave, repair_rounds=2.
Lane A - scope cli/internal/gates/**, brief "add the probe-coverage gate row",
acceptance: cd cli && go test ./internal/gates/...
Lane B - scope docs/CI-CD.md, brief "document that gate row",
acceptance: bash scripts/docs-build.sh --check
Wave acceptance: bash tests/run-all.sh. Report per-lane evidence and stop.
```

## It's working if

Observable in the trace, without reading the prose — and the rubric a fresh
independent judge scores this skill against:

- The transcript shows exactly one `rpi` invocation per lane in the caller's
  wave, and no lane appears twice.
- Two lanes that share a regen surface (both reaching `skills-codex/`, say) run
  one after the other, never concurrently.
- The wave acceptance command runs exactly once, after the last lane returns.
- The returned evidence carries a status and a subject manifest digest for
  every caller-supplied lane, and for no lane crank invented.
- The response ends with the evidence: no next wave, no `git` action, and no
  proposed follow-up work.
