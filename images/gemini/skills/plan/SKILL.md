---
name: plan
description: 'Shape or refine the existing bead or caller intent without creating a second planning artifact. Triggers: "plan", "discover and plan", "shape this goal".'
practices:
- bdd-gherkin
- design-by-contract
- ddd-bounded-context
hexagonal_role: domain
consumes: []
produces: []
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
already owns the work. Prefer the caller's tracker, if any. When no tracker is
available, use the caller's conversation or supplied issue text; the runtime
snapshots the resolved intent bytes automatically so later contexts can read
and hash the same source. Do not make the model restate those facts in a packet.

## Workflow

1. Resolve the intent source and choose one active behavior. When that source
   is not already durable, have the runtime pass its exact bytes to
   `python3 skills/validate/scripts/validate.py snapshot-intent --source -` and
   use the returned `intent_ref` for later phases.
2. Inspect only enough real context to make paths, interfaces, and evidence
   concrete. Existing research and specialist skills are advisory inputs.
3. Ensure the source contains acceptance examples, important non-goals, and the
   allowed write scope. Use lightweight prose or Given/When/Then only where it
   removes ambiguity; do not require both normal and edge ceremony for every
   change.
4. Name the first useful acceptance check.
5. If authorized and the source is writable, update that bead or issue in
   place. Otherwise return a concise proposed amendment to the caller.

Planning produces no AgentOps packet. The runtime stores and hashes the resolved
source bytes to detect later acceptance drift. That content-addressed snapshot
is derived automatically and is not another model-authored planning artifact.

Bound the work around the caller-visible outcome, not individual files, gates,
or reviewer comments. Decomposition is useful only when it reduces reasoning
cost; it must not multiply invocations or proof artifacts.
