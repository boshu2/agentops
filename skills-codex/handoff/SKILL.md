---
name: handoff
description: Write compact caller-authored session
---
# Handoff

Write a factual session artifact that another context can read. Include:

- caller-supplied goal and summary;
- completed artifacts and exact evidence paths;
- unresolved facts or risks;
- optional caller-supplied continuation text;
- best-effort read-only repository identity when useful.

Do not infer a next action, select work, assign ownership, consume the artifact,
change tracker or Git state, classify a verdict, govern retries, or restart a
runtime. Reading a handoff must not mutate it.

The ao session handoff and ao session rehydrate commands implement the same
boundary for JSON artifacts. The skill may write Markdown when that better
serves a human, but the content semantics remain identical.

Return the artifact path and stop.
