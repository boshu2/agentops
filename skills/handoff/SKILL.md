---
name: handoff
description: 'Write compact caller-authored session evidence without choosing continuation. Triggers: "handoff", "write compact session handoff".'
practices: [adr, wiki-knowledge-surface, code-complete]
hexagonal_role: supporting
consumes: []
produces: [.agents/handoff/*.md]
context_rel: []
skill_api_version: 1
context:
  window: inherit
  intent: {mode: none}
  intel_scope: none
metadata:
  capabilities: [handoff]
  effects: []
  canonical_status: canonical
  disposition: keep_specialist
  graph_root: true
  tier: session
  dependencies: []
output_contract: caller-authored handoff artifact
---

# Handoff

A handoff works because the next context can act on exact paths and facts
without trusting the author's memory; any line the reader cannot verify from
the artifact itself is decoration, not handoff.

Write a factual session artifact that another context can read. Include:

- caller-supplied goal and summary;
- completed artifacts and exact evidence paths;
- unresolved facts or risks;
- optional caller-supplied continuation text;
- best-effort read-only repository identity when useful.

Do not infer a next action, select work, assign ownership, consume the artifact,
change tracker or Git state, classify a verdict, govern retries, or restart a
runtime. Reading a handoff must not mutate it.

Named failure mode — **optimistic closure**: writing "done" for work whose
evidence path does not exist, so the next context builds on a phantom.

Anti-pattern: narrating the session chronologically ("first I tried…, then…").
Corrective: record end-state facts — artifacts, paths, unresolved risks — and
drop the journey.

The ao session handoff and ao session rehydrate commands implement the same
boundary for JSON artifacts. The skill may write Markdown when that better
serves a human, but the content semantics remain identical.

Return the artifact path and stop.
