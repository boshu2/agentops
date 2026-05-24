# Output Schema Caching Guidance

> When to cache vs inline council output schemas in worker prompts.

## Decision Matrix

| Scenario | Approach | Rationale |
|----------|----------|-----------|
| Single council invocation | Inline schema in prompt | Simple, no file overhead |
| Swarm with 3+ workers using same schema | Cache to `.agents/council/output-schema.json` | Avoids duplicating ~50 lines per worker |
| Cross-wave validation | Reference `skills/council/schemas/verdict.json` | Canonical source, no drift |
| Codex workers (sandbox) | Inline schema in prompt | Workers may lack file access |

## Cache Location

When caching, write to `.agents/council/output-schema.json` (gitignored, session-scoped):

```bash
cp skills/council/schemas/verdict.json .agents/council/output-schema.json
```

Workers reference via: `"See output schema at .agents/council/output-schema.json"`

## Size Guard

Council output schemas are small (~50 lines). Inlining adds ~500 tokens per worker prompt. For waves with 5+ workers, this is 2500+ tokens of repeated schema — consider caching instead.

**Rule of thumb:** Inline for ≤4 workers, cache for ≥5 workers per wave.

## Schema Version Pinning

Always reference `schema_version` in worker prompts so output can be validated:

```json
{
  "schema_version": 3,
  "verdict": "PASS | WARN | FAIL",
  "...": "..."
}
```

Workers that omit `schema_version` produce output that cannot be mechanically validated by downstream skills.
