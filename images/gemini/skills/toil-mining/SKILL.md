---
name: toil-mining
description: 'Mine caller-supplied usage history for repeated toil and emit ranked evidence. Triggers: "mine toil", "find repeated operational work".'
practices:
- sre
- lean-startup
hexagonal_role: supporting
consumes: []
produces:
- result.json
context_rel:
- kind: supplier-to
  with: automation-shape-routing
skill_api_version: 1
user-invocable: false
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
metadata:
  capabilities: [toil_mining]
  effects: []
  canonical_status: canonical
  disposition: keep_specialist
  tier: meta
  dependencies: []
  stability: experimental
output_contract: ranked evidence report under .agents/toil-mining/
---
# Toil Mining — rank repeated friction

Mine explicitly supplied session, shell, RTK, or CASS history without modifying
the sources. The result is evidence for a caller; this skill does not file work,
schedule automation, or mutate a tracker.

## Procedure

1. Record the input sources, time window, filters, and query.
2. Normalize repeated human actions while excluding documented machine echoes
   and generated repetitions. For caller-supplied Codex JSONL, use the
   deterministic helper below rather than an ad hoc transcript scan.
3. Cluster equivalent actions and preserve representative evidence references.
4. Score each cluster from measured frequency and observed pain such as elapsed
   time, failure count, interruption, or token cost.
5. Emit a ranked report and stop.

Each candidate must contain a measured count, source references, confidence in
the clustering, pain evidence, and the smallest plausible automation shape.
Separate observations from recommendations.

### Deterministic recent-human extraction (Codex JSONL)

The helper accepts only explicit session paths and requires an explicit,
timezone-qualified window:

```bash
python3 skills/toil-mining/scripts/recent_human.py --since 2026-07-12T00:00:00Z \
  --until 2026-07-16T00:00:00Z /path/to/session-a.jsonl /path/to/session-b.jsonl \
  > /tmp/recent-human.json
```

It extracts `event_msg` / `user_message` records with `source_path`, one-based
`line`, normalized UTC `timestamp`, and request `text`. Codex attachment and IDE
wrappers are normalized by keeping the text after `# My request for Codex:`.
Restored or forked copies are deduplicated by `client_id`, with the earliest
occurrence retained.

The extractor treats a nonempty `client_id` as the high-confidence UI-origin
boundary. Records without it are reported as `missing_client_id`, not guessed to
be human. It also excludes these narrow generated envelope families and reports
their counts: internal context tags (`codex_internal_context`, environment,
permissions, skill/app/plugin instructions), fresh-context cross-family refuter
prompts, and agent `Message Type:` envelopes. This is deliberately conservative:
a directly typed message from a client that omits `client_id` remains unchecked.

The JSON result includes input/parsed/candidate/emitted counts, exclusions by
reason, checked facts, and not-checked facts. Malformed records are counted, not
silently discarded. The helper reads only the supplied JSONL and writes only to
stdout; it does not read attachment contents, discover more sessions, cluster
meaning, score toil, file work, or schedule automation. It is
retrieval/report-only.

## Output

Write `.agents/toil-mining/YYYY-MM-DD-candidates.md` only when the caller asks for
a local artifact; otherwise return the report inline. Include checked and
not-checked sources. Do not include owners, priorities, claims, queues, or a next
action.

## References

- [Automation shape routing](../automation-shape-routing/SKILL.md)
- [CASS](../cass/SKILL.md)
