---
name: status
description: Report observable AgentOps evidence without
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
