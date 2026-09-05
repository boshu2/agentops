---
name: plan
description: 'Shape or refine the existing bead or caller intent without a second planning artifact. Triggers: "plan", "discover and plan", "shape this goal", "review write scope", "check scope boundaries", "scope this change".'
practices:
- bdd-gherkin
- design-by-contract
- ddd-bounded-context
hexagonal_role: domain
consumes: []
produces: []
output_contract: 'in-place caller intent update or concise proposed amendment; never an AgentOps planning artifact'
context_rel: []
skill_api_version: 1
user-invocable: true
metadata:
  graph_root: true
  tier: execution
  dependencies: []
  capabilities: [shape_intent, define_acceptance, bound_write_scope]
  effects: [update_intent_source]
  canonical_status: canonical
  disposition: keep
---

# Plan

Turn the caller's intent into one bounded, testable behavior in the place that
already owns the work. Prefer the caller's tracker, if any; otherwise the
caller's conversation or supplied text, which the runtime snapshots so later
contexts read and hash the same bytes.

## Prompt

```text
Plan bead ag-1234: "ao gate check lists the probe-coverage row". Shape it in
the bead itself: one active behavior, acceptance examples, non-goals, write
scope as a class (cli/internal/gates/** plus regen outputs), first check
`cd cli && go test ./internal/gates/...`. Update the bead in place.
```

## It's working if

- The bead or issue text itself gains acceptance, non-goals, and write scope;
  no plan file appears under `.agents/` in the diff.
- Write scope names a regen class (`skills/**` plus every output of
  `scripts/regen-all.sh`), not a hand-enumerated path list.
- The plan names one first check as a runnable command, such as
  `bash scripts/check-x.sh`, and a fresh context given only the source can
  start Implement.
- On a risky write scope the Plan output carries `binding_judge` and names the
  evidence the change will orphan, rather than leaving either for verify time.

## Workflow

1. Resolve the intent source and choose one active behavior. When the source
   is not durable, have the runtime pass its exact bytes to the validate
   skill's `scripts/validate.py snapshot-intent --source -` (under
   `skills/validate/` in a checkout, `.agents/skills/validate/` when
   installed) and carry the returned `intent_ref` into later phases.
2. Route the work by type (Integrate, Extend, or Greenfield) and name its
   ground truth, control experiment, and deviation ledger first from
   [references/ground-truth-routing.md](references/ground-truth-routing.md).
   Then inspect only enough real context to make paths, interfaces, and
   evidence concrete, carrying citations forward; research and specialist
   skills are advisory inputs.
3. Ensure the source contains acceptance examples, important non-goals, and the
   allowed write scope. Name `write_scope`, whether it hits a risky surface
   (the list [`validate`](../validate/SKILL.md) owns), the caller's
   `repair_rounds`, and the evidence this change will orphan: bound scorecards
   or contracts whose evaluator files sit in the write scope. Recapturing that
   evidence is work this plan carries, not a discovery for verify time. Use
   lightweight prose or
   Given/When/Then only where it removes ambiguity. Write-scope checks (folded
   from the retired `scope` skill):
   - patterns are normalized repository-relative paths;
   - includes cover the behavior without granting unrelated directories;
   - excludes do not contradict required changes;
   - generated companions that must move with the sources are explicit;
   - no ownership, scheduling, Git, hook, retry, release, or delivery state.
4. Name the first useful acceptance check.
5. If authorized and the source is writable, update that bead or issue in
   place. Otherwise return a concise proposed amendment to the caller.

## The binding judge

On a risky write scope the Plan output carries `binding_judge`, `primary` or
`cross`, and it is bound into the plan identity like acceptance and scope. A
caller argument that disagrees with the bound value refuses the traversal
rather than rebinding it silently.

`binding_judge` declares a disposition: which leg the caller would act on if
the two judges still split after repair. It is not a verdict override. The
convergence law still requires both legs to return PASS on a risky surface, a
split still never certifies PASS, and no finding leaves the open set because
this field named a leg. Validate reads it as caller intent, never as authority
over its own result.

Planning produces no AgentOps packet: the runtime carries the source's
reference and digest to detect acceptance drift. Bound the work around the
caller-visible outcome, not files, gates, or reviewer comments; decompose only
when it reduces reasoning cost.

## Scope admission

At scope, read `boundaries.md` in the rpi skill's `references` directory for
what Plan does not own. In a repository with generated projections, write
scope names generator-owned outputs as a class (the hand-edited sources plus
all outputs of the owning regen commands), because a hand-enumerated list is
falsified the first time a regen command rewrites an unlisted companion.
Before freezing acceptance, enumerate the generated companions, parity twins
such as `skills-codex/`, and tests asserting on the changed paths;
anything unadmitted here surfaces later as an out-of-scope diff or a broken
gate.

A plan is done only when it passes the fresh-context test: a cold context,
given the intent source alone, could execute it. Move any fact that lives only
in the planning conversation into the source before freezing.
