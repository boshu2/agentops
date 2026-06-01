---
package: cli/internal/types
status: active
owner: agentopsd
contract_source: schemas/{bead,briefing,learning,phase,quest,verdict}.v1.schema.json
---

# cli/internal/types

Shared Go domain types for the agentopsd lane.

## Status

**Active package, expanding.** Already populated with transcript-pipeline and
Memory RL policy types (see "Existing surface" below). Bead `agentops-e46`
added this AGENTS.md and pre-published the contract source for the wave of
types ported by `agentops-l12`.

> **Resolution (l12):** olympus domain types landed in a subpackage at
> `cli/internal/types/quest/` rather than flat alongside the existing types.
> Rationale: the two domains (Claude transcript pipeline / MemRL policy vs.
> olympus quest/bead lifecycle) are semantically unrelated. Co-locating would
> have caused namespace clutter (two `Sections`, two `Source`/`Now` candidates)
> and misled readers about ownership. The `quest` subpackage matches the
> contract source of truth flagged in the e46 finding.

## Ownership

- **Owner:** agentopsd extraction track (epic `agentops-tqc`).
- **Contract source of truth for olympus-derived types** (the wave coming via
  `agentops-l12`): the 6 v1 JSON schemas at the repo root:
  - [`schemas/bead.v1.schema.json`](../../../schemas/bead.v1.schema.json)
  - [`schemas/briefing.v1.schema.json`](../../../schemas/briefing.v1.schema.json)
  - [`schemas/learning.v1.schema.json`](../../../schemas/learning.v1.schema.json)
  - [`schemas/phase.v1.schema.json`](../../../schemas/phase.v1.schema.json)
  - [`schemas/quest.v1.schema.json`](../../../schemas/quest.v1.schema.json)
  - [`schemas/verdict.v1.schema.json`](../../../schemas/verdict.v1.schema.json)
- Go structs added by `agentops-l12` MUST round-trip cleanly against these
  schemas. The schema is the contract; Go types are the in-process projection.
- The schemas are JSON Schema draft 2020-12 with `additionalProperties: false`
  and ULID-pattern IDs (`^[0-9A-Za-z]{26}$`).

## Existing surface (pre-l12)

| File | Domain |
|---|---|
| `types.go` | Claude Code transcript pipeline types (TranscriptMessage, etc.) |
| `memrl_policy.go` | Memory RL policy contract: modes, actions, failure classes, attempt buckets, policy evaluation |
| `errors.go` | Shared sentinel errors |
| `*_test.go` | L1/L2 coverage for the above |

These pre-date the agentopsd extraction and are not in scope for `agentops-e46`
or `agentops-l12`. Do not refactor them while porting olympus types.

## Planned surface (owned by agentops-l12)

When `agentops-l12` lands, this package (or a sub-package) should expose:

- `Bead`, `Quest`, `Briefing`, `Learning`, `Verdict`, `Phase` Go structs that
  marshal/unmarshal cleanly against the schemas above.
- ULID generation/parsing helpers (the `^[0-9A-Za-z]{26}$` pattern is shared
  across `bead_id`, `quest_id`, `briefing_id`, `verdict_id`).
- Atomic-write helpers used by callers that persist these types to disk
  (apollo-style state transitions in the agentopsd lane).

## Non-goals

- This package does NOT validate against the JSON schemas at runtime — that's
  the caller's responsibility (likely a separate `cli/internal/schemas`-style
  package or an external `jsonschema` lib).
- This package does NOT implement state-machine transition rules. Those live
  in the consumer (apollo-equivalent).

## Cross-references

- Parent epic: `agentops-tqc` (Olympus → agentopsd extraction)
- This bead: `agentops-e46` (port 6 schemas + record contract source here)
- Successor: `agentops-l12` (fill this package with concrete olympus types)
- Olympus source: `~/dev/personal/olympus/contracts/schemas/` and
  `~/dev/personal/olympus/types/` (see olympus repo for original Go impls)
