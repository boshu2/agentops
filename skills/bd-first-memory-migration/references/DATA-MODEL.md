# Memory data model + type taxonomy

> The shared contract every phase references. Defines how a "real" memory is represented **on top of `bd` memory keys** — a convention + thin wrapper, with **no change to bd core**. Authored for epic ag-5u50 (bead .6); consumed by the importer (.10), wrapper (.11), recall (.12), write-path (.13), MEMORY.md generator (.14), scoring (.15), GC (.16), contradiction (.17), stats (.23).

## Design constraint

`bd` already stores memories as `key → text` (via `bd remember --key <k> "<body>"`, searched with `bd memories <kw>`, removed with `bd forget <k>`). This model adds **structure by convention** so we never fork bd:

- **Type** lives in the key prefix.
- **Structured fields** (provenance, scores, timestamps, maturity) live in a small fenced header at the top of the body that round-trips through bd untouched.

A memory body is therefore:

```
<!--mem
type: feedback
source: commit:86d6365
utility: 0.62
access_count: 14
created_at: 2026-06-02T13:00:00Z
last_accessed: 2026-06-02T17:40:00Z
maturity: established
superseded_by: null
-->
bd's per-write Dolt auto-commit makes rapid writes memory-expensive… **Why:** … **How to apply:** …
```

The `<!--mem … -->` block is parsed by the wrapper (.11) and ignored by every other reader (it renders as an HTML comment). If the block is absent, the memory is treated as `type: fact, maturity: provisional, utility: 0` — back-compat with existing plain bd memories.

## Type taxonomy (key prefix)

Key form: `<type>:<kebab-slug>` — e.g. `feedback:dolt-write-burst-oom`, `episodic:2026-06-02-crank1-kickoff`.

| Type | Holds | Decay half-life | Notes |
|---|---|---|---|
| `fact:` | semantic facts about the world/system | medium (~90d) | default for untyped/legacy memories |
| `feedback:` | how the agent should work — corrections + confirmed approaches | long (~365d) | body SHOULD carry `**Why:**` + `**How to apply:**` |
| `project:` | ongoing work/goals/constraints not derivable from code or git | medium (~120d) | convert relative dates to absolute |
| `episodic:` | session events ("on 2026-06-02 we decided X") | **short (~14d)** | ages out fast; cheap to lose |
| `procedural:` | playbooks / how-to / runbooks | **long (~365d), sticky** | rarely evicted |

Mirrors the Claude auto-memory frontmatter taxonomy (`user | feedback | project | reference`): `user`/`reference`→`fact`, `feedback`/`project` map directly. The importer (.10) maps each source store's taxonomy onto these five.

## Per-memory fields

| Field | Type | Set by | Meaning |
|---|---|---|---|
| `key` | string | author | `<type>:<slug>`, unique; updates in place |
| `type` | enum | author | one of the 5 above |
| `body` | markdown | author | the memory itself (below the `<!--mem-->` block) |
| `source` | string | importer/wrapper | provenance: `commit:<sha>` \| `session:<id>` \| `bead:<id>` \| `file:<path>` |
| `utility` | float 0–1 | scoring (.15) | `f(access_count, citations, recency)` |
| `access_count` | int | recall (.12) | bumped each recall |
| `created_at` | ISO-8601 | wrapper | first write |
| `last_accessed` | ISO-8601 | recall | most recent recall |
| `maturity` | enum | scoring (.15) | `provisional → candidate → established` (or `anti-pattern`) |
| `superseded_by` | key\|null | contradiction (.17) | set when a newer memory replaces this one |

## Decay + utility

Recall ranking (bead .12) scores each candidate:

```
rank = type_weight · recency_decay(last_accessed | created_at) · (1 + log1p(access_count)) · (0.5 + utility)
recency_decay(t) = 0.5 ^ (age_days(t) / half_life(type))
```

- `type_weight`: procedural/feedback > fact/project > episodic.
- Token-budgeted: recall fills up to `--max-tokens`, highest rank first.
- Eviction candidate (bead .16): `maturity == provisional && utility < ε && age > 2·half_life(type)`.

## Maturity ladder (wired to eviction — beads .15/.16)

`provisional` (new) → `candidate` (accessed ≥ K times) → `established` (cited / accessed ≥ M times). Unlike the prior `ao` flywheel — which stalled at 99.9% provisional because nothing was ever evicted — this ladder is **coupled to GC**: provisional + low-utility + stale ⇒ archived. The backlog cannot grow without bound.

## Provenance & contradiction

- Every imported/written memory carries `source`, so "why do I believe this" is always answerable and orphans are findable (beads .9 audit, .16 GC).
- On write, the contradiction detector (.17) checks for a live memory with the same `key` or a near-duplicate body asserting something different; resolves via `bd supersede` (sets `superseded_by`) or a human flag. No two **live** memories may assert contradictory facts on the same key.

## Archive, never silent-delete

Evicted memories move to a cold archive (`.agents/archive/memory/` or a bd `archived` state), recoverable within a grace period. Authored content (`.agents/knowledge`, established memories) is **never** hard-deleted by GC. Any bounded coverage (top-N, sampling) must be logged, not silent.
