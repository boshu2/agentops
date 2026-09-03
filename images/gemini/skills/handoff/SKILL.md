---
name: handoff
description: 'Write compact caller-authored session evidence without choosing continuation. Triggers: "handoff", "write compact session handoff".'
practices: [adr, wiki-knowledge-surface, code-complete]
hexagonal_role: supporting
consumes: []
produces: [caller-selected handoff path or .agents/ao/handoff/*]
context_rel: []
skill_api_version: 1
context:
  window: inherit
  intent: {mode: none}
metadata:
  capabilities: [handoff]
  effects: [write_handoff_artifact, read_git_state, read_clock]
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

## Prompt

```text
Write a handoff for this session in agentops-wt/train2-c: goal was
migrating regen to Go, I finished cli/internal/gates/regen.go and left
scripts/regen-all.sh untouched, tests are green, and the next context
still needs to update docs/CI-CD.md. Write it to .agents/ao/handoff/.
```

## It's working if

Observable in the trace, without reading the prose:

- Every completed item names an exact evidence path like
  `cli/internal/gates/regen.go`, not a chronological narration.
- The artifact lands at the caller-named path or `.agents/ao/handoff/`.
- `cat .agents/ao/handoff/<id>.json` after writing shows the same
  content it wrote.
- Only caller-supplied `continuation` text appears as next action; no
  owner, tracker state, or verdict is invented.

## Contract

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

Write the artifact to the caller-owned handoff location when the caller names
one; otherwise it is explicit requested proof under `.agents/ao/handoff/`.
There is no permanent generic handoff store — an artifact nobody consumes is
scratch, not evidence.

The ao session handoff and ao session rehydrate commands implement the same
boundary for JSON artifacts under `.agents/ao/handoff/`. The skill may write
Markdown when that better serves a human, but the content semantics remain
identical.

### Earlier default compatibility

JSON artifacts already stored under `.agents/handoff/` remain read-only
evidence. `ao session handoff` writes new JSON to `.agents/ao/handoff/`, while
`ao session rehydrate` searches both directories and selects the newest
lexical handoff id; if an identical filename exists in both, the canonical
`.agents/ao/handoff/` copy wins. No command moves or deletes the legacy files.
Human-authored Markdown consumers receive the exact path, so they do not need
to scan either default. This owning skill contract is the compatibility
authority; no separate migration artifact is required.

Return the artifact path and stop.
