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
  intel_scope: none
metadata:
  capabilities: [status]
  effects: []
  canonical_status: canonical
  disposition: keep_specialist
  graph_root: true
  tier: session
  dependencies: [sbh]
output_contract: read-only status snapshot
---

# Status

Report only observable local facts: available intent, candidate, and verdict
artifacts; their digests and timestamps; deterministic check results; and
unavailable or corrupt sources. Label staleness and uncertainty explicitly.

Status does not inspect work queues, assign priority, claim work, infer a next
action, repair records, govern retries, or change any state. Optional Git or
tracker metadata may be displayed only when the caller supplies it; absence
cannot change the report interpretation.

Return the snapshot and stop.
