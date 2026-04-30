---
package: cli/internal/search
status: active
owner: agentopsd
contract_source: cli/internal/search/index.go (Index, IndexEntry JSONL shape)
---

# cli/internal/search

In-memory inverted index over `.md` and `.jsonl` files in `.agents/`, plus the inject/findings/learnings/patterns retrieval surface that powers `ao inject` and bead context loading.

## Ownership

- **Owner:** agentopsd extraction track (epic `agentops-tqc`).
- **Index format:** JSONL on disk — one term per line as `IndexEntry{Term, Paths}`. Loaded into a `map[string]map[string]bool` in memory.
- **Skill surface:** drives `skills/inject/SKILL.md` (decay-ranked, token-budgeted context injection) and `skills/research/SKILL.md`.

## Interfaces

- **Indexer:**
  - `index.go` — `Index`, `NewIndex`, build/load/serialize. `isIndexableFile` gates on `.md` and `.jsonl` extensions only.
  - `inject_run.go`, `inject_options.go`, `inject_citations.go` — the `ao inject` flow.
- **Retrieval:**
  - `findings.go`, `findings_ops.go` — findings surface (Dream output, RPI artifacts).
  - `learnings.go`, `patterns.go` — knowledge-flywheel surfaces.
  - `predecessor.go` — predecessor-aware ranking for bead context.
  - `bead_context.go` — assembles the per-bead retrieval bundle consumed by daemon job runners.
- **Scoring/quality:**
  - `scoring.go` — multi-term scoring (number of matched terms).
  - `constraint.go` — query/result constraints.
  - `quality_gate.go` — gate that filters low-quality matches.
- **Sessions and CASS:**
  - `sessions.go` — session-aware retrieval.
  - `search_cass.go` — CASS (content-addressed) integration.

## Non-obvious rules

- **Indexable extensions are hard-coded.** Only `.md` and `.jsonl` are indexed (see `isIndexableFile`). Adding a new extension requires touching this gate AND understanding that the index will need a full rebuild.
- **In-memory representation is intentionally not the JSON shape.** The on-disk `IndexEntry` is array-of-paths; the in-memory `Index.Terms` is `map[string]map[string]bool` for O(1) presence checks. Don't try to serialize the in-memory form directly — use the converter.
- **`createIndexOutput` is a package-level var on purpose.** It's swapped in tests to capture writes without touching the filesystem. Don't inline `os.Create`.
- **Decay-ranked, token-budgeted.** Inject ranks results with citation/confidence feedback from `.agents/ao/citations.jsonl`; results are capped by a token budget (`--max-tokens`). Don't bypass the budget — context window blowups are the regression mode.
- **Predecessor links are first-class.** When a bead has a `discovered-from` predecessor, `predecessor.go` walks the chain and surfaces parent context. Removing this would silently degrade bead handoffs.
- **Quality gate filters silently.** `quality_gate.go` drops matches below a confidence floor. Surface the drop count in operator output if you debug "missing" results.

## Cross-references

- Parent epic: `agentops-tqc` (Olympus → agentopsd extraction).
- Skills: `skills/inject/SKILL.md`, `skills/research/SKILL.md`.
- Pattern source: olympus per-folder `AGENTS.md` ownership convention.
- Sibling packages: `cli/internal/context` (assembles full context packets using search results), `cli/internal/overnight` (writes findings consumed by `findings.go`).
