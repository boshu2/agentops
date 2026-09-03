---
name: status
description: 'Report observable AgentOps evidence without selecting work. Triggers: "status", "show AgentOps status".'
practices: [dora-metrics, sre]
hexagonal_role: driving-adapter
consumes: []
produces: [stdout]
context_rel: []
skill_api_version: 1
allowed-tools: Read, Grep, Glob, Bash
model: haiku
context:
  window: inherit
  intent: {mode: none}
metadata:
  capabilities: [status]
  effects: [read_filesystem, read_clock]
  canonical_status: canonical
  disposition: keep_specialist
  graph_root: true
  tier: session
  dependencies: []
output_contract: read-only status snapshot
---

# Status

A status snapshot is trustworthy exactly when every line traces to an artifact
that exists on disk right now; the first inferred line turns the report into a
guess wearing a report's clothes.

## Prompt

```text
Show AgentOps status for fleet-router (ao is installed). Report only observable .agents/ao evidence: intent and verdict counts, evidence recency, and any corrupt or unavailable sources. Disclose checked and not_checked explicitly.
```

## It's working if

- The report cites `ao status` output directly, never a paraphrase of what it might show.
- Intent and verdict counts trace to `.agents/ao/intents/sha256` and `.agents/ao/verdicts/sha256`, never an inferred count.
- Runtime phase and tool-call activity get labeled `not_checked` unless the caller supplied that source separately.
- Tracker, Git, and factory state stay in their own reported lines rather than one blended `health` claim.

## Contract

Report only observable local facts: available intent and verdict artifacts and
their counts; deterministic check results; evidence recency; and unavailable or
corrupt sources. The canonical durable stores are `.agents/ao/intents/sha256`
and `.agents/ao/verdicts/sha256`. Subject manifests are caller-supplied: report
them only when the caller names their location, otherwise disclose them as
`not_checked`. When `.agents/ao` evidence exists, report which stored artifact
kind is newest and label that conclusion as evidence recency, not runtime phase
or process activity. The `ao status` snapshot emits the two counts plus that
newest-artifact recency conclusion, not a per-artifact digest or timestamp
listing; report a specific artifact's digest or timestamp only when the caller
asks about a named artifact.

Always disclose `checked` and `not_checked`. Runtime phase, execution elapsed
time, tool-call activity, and remaining work are `not_checked` unless a caller
provides a separate authoritative source for them.

`ao status` is the evidence-store view. It validates content-addressed artifact
names and content before counting them, reports corrupt and unavailable entries,
and shows only intent/verdict counts plus evidence recency. Retired legacy
surfaces (session indexes, provenance summaries, knowledge-health signals) are
not aggregated into this command.

Distinct state classes stay distinct in every status report: tracker state
belongs to the tracker, Git state to Git, factory or runtime state to the
selected factory's own doors, deterministic-check results to the executable
that produced them, and semantic validation to a fresh verdict. Report each
from its own authority, never merged into one blended "health" claim —
factory-complete, checks-green, and AgentOps-PASS are three different facts.

Status does not inspect work queues, assign priority, claim work, infer a next
action, repair records, govern retries, or change any state. Optional Git or
tracker metadata may be displayed only when the caller supplies it; absence
cannot change the report interpretation.

Named failure mode — **recency-as-activity**: reading "newest artifact is a
verdict" as "validation is running", which invents a runtime phase from a
timestamp.

Anti-pattern: filling `not_checked` gaps with plausible narrative so the
snapshot feels complete. Corrective: report the gap as a gap; an honest hole
outranks a smooth story.

Return the snapshot and stop.
