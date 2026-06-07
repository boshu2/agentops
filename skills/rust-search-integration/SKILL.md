---
name: rust-search-integration
description: |
  Add a fast, hybrid (lexical + semantic) code-search capability to a Rust project
  so agents and developers can query the codebase in milliseconds — choose an
  indexing strategy, design an ergonomic query API, keep the index fresh with
  incremental re-indexing, and wire a CLI/library surface that returns ranked,
  source-anchored hits.
  Triggers: "add code search to my Rust project", "hybrid lexical + semantic
  search in Rust", "build a code index in Rust", "incremental re-indexing",
  "fast codebase search for agents", "tantivy full-text search", "embedding-based
  code search Rust", "wire a search API into my crate", "fuzzy + vector search
  for a codebase", "make my repo agent-searchable".
practices:
- hexagonal-architecture
- design-for-the-machine
- pragmatic-programmer
hexagonal_role: supporting
consumes: []
produces:
- SEARCH-INTEGRATION-PLAN.md
context_rel:
- kind: complements
  with: legacy-codebase-recon
- kind: complements
  with: cli-agent-ux-audit
skill_api_version: 1
context:
  window: inherit
  intent:
    mode: task
metadata:
  tier: library
  stability: experimental
  dependencies: []
output_contract: 'file: SEARCH-INTEGRATION-PLAN.md (index strategy + query API + incremental-reindex design + wiring), plus an in-tree search module'
user-invocable: false
allowed-tools:
- Read
- Edit
- Bash
- Grep
- Glob
---
# rust-search-integration — Hybrid code search for a Rust codebase

> **Purpose:** Give a Rust project a fast, hybrid (lexical + semantic) search surface
> that returns ranked, source-anchored hits in milliseconds — usable both as a library
> call inside the crate and as a robot-friendly CLI for agents.

**YOU MUST PRODUCE A REAL INTEGRATION, NOT A DESCRIPTION.** Write `SEARCH-INTEGRATION-PLAN.md`,
then land an in-tree search module and wire it to a query surface.

## ⚠️ Critical Constraints

- **Pick ONE index home and own its lifecycle.** Persist the index to a known path
  (e.g. `target/.search-index/`) and gitignore it. **Why:** an index that lives only
  in memory is rebuilt on every process start — that is a re-indexing cost masquerading
  as a query cost, and it makes search unusable for short-lived agent invocations.
  - WRONG: `let index = Index::create_in_ram(schema);` for a CLI that exits per query.
  - CORRECT: `let index = Index::open_or_create(MmapDirectory::open(idx_path)?, schema)?;`
- **Never block the query path on embedding generation.** Compute and store semantic
  vectors at index time; at query time only embed the *query string*. **Why:** embedding
  the whole corpus per query turns a 5 ms lookup into seconds and defeats the point.
- **Re-index incrementally by content hash, not wall-clock.** **Why:** mtime lies (clones,
  checkouts, `touch`); a stale or over-eager full rebuild is the most common cause of
  "search is slow" and "search returns deleted code."
  - WRONG: `if file.mtime() > last_index_time { reindex_all() }`
  - CORRECT: `if blake3(contents) != stored_hash { reindex_one(path) }`
- **Every hit MUST carry `path:line` and a snippet.** **Why:** a score without a location
  is useless to an agent — it cannot open, read, or edit what it cannot locate.

## Why This Exists

LLM agents and developers both navigate a codebase by *asking it questions* — "where is X
defined", "what handles auth", "show me code like this". Naive `grep` is exact-only and
misses synonyms/intent; pure vector search is fuzzy and misses exact identifiers. The
durable answer is a **hybrid** index: a lexical engine for precise token/identifier matches
plus a semantic engine for intent, fused into one ranked result set. Doing this *well* in
Rust means treating search as a real bounded subsystem — schema, persistence, incremental
freshness, and a clean port — not a one-off `walkdir + contains()` loop that rots the first
time the repo changes. This skill is the repeatable way to add that subsystem.

## Quick Start

1. Read this whole file, then scan the target crate's layout (`Glob "**/*.rs"`, read
   `Cargo.toml`) so the plan matches the real workspace.
2. Run `bash {baseDir}/scripts/validate.sh` to confirm the skill's own structure is sound.
3. Write `SEARCH-INTEGRATION-PLAN.md` using the sections in **Output Specification**.
4. Implement the module per **Methodology**; verify each phase before moving on.

## Methodology

### Phase 1 — Decide the index strategy
Choose, and record the choice + rationale in the plan:

- **Lexical engine:** a full-text index (e.g. an inverted-index crate like `tantivy`) over
  fields `{path, lang, symbol, body}`. Tokenize code-aware (split on camelCase/snake_case,
  keep identifiers searchable whole and in parts).
- **Semantic engine:** chunk each file into symbol- or window-sized units, embed each chunk
  once at index time, store vectors in an ANN structure (HNSW, or a flat cosine scan if the
  repo is small). Keep the model + dimension fixed and recorded — changing either invalidates
  the whole vector store.
- **Fusion:** run both, then merge with Reciprocal Rank Fusion (RRF) or a weighted score.
  Default RRF (`score = Σ 1/(k + rank_i)`, `k≈60`) — it needs no score calibration between
  the two engines.

**Checkpoint:** the plan names the lexical crate, the embedding source, the chunking unit,
the fusion method, and the on-disk index path. Do not proceed without all five.

### Phase 2 — Build the indexer
- Walk the repo respecting `.gitignore`; filter to source extensions; skip vendored/generated
  trees. For each file: compute `blake3(contents)`, extract chunks, write lexical docs and
  semantic vectors, and persist `{path → content_hash, chunk_ids}` to a manifest.
- Commit/flush the index transactionally so a crashed run leaves a consistent index.

**Checkpoint:** a cold full index of the repo completes and persists to disk; re-running with
no changes does near-zero work (manifest hashes all match).

### Phase 3 — Design the query API (the port)
Define one trait that both the library and the CLI call — this is the hexagonal port:

```rust
pub struct Hit { pub path: String, pub line: u32, pub snippet: String, pub score: f32 }
pub trait CodeSearch {
    fn search(&self, query: &str, opts: &SearchOpts) -> anyhow::Result<Vec<Hit>>;
}
pub struct SearchOpts { pub limit: usize, pub mode: Mode } // Mode: Lexical|Semantic|Hybrid
```

Embed only the query string; run both engines (or one, per `mode`); fuse; truncate to
`limit`; attach `path:line` + snippet to every hit.

**Checkpoint:** `search("...")` returns ranked `Hit`s, each with a resolvable location.

### Phase 4 — Incremental re-indexing
- On change (file watcher, pre-query staleness check, or an explicit `reindex` command):
  diff current `blake3` hashes against the manifest. For each changed/new path, delete its
  old lexical docs + vectors and re-add; for each deleted path, remove its docs + vectors.
- Never rebuild the whole index for a one-file change.

**Checkpoint:** editing one file updates exactly that file's entries; a deleted file stops
appearing in results.

### Phase 5 — Wire the surfaces
- **Library:** export `CodeSearch` from the crate so other modules call it directly.
- **CLI/agent:** add a subcommand (e.g. `mytool search "<query>" --json --limit N`) that
  prints NDJSON `Hit`s and uses stable exit codes — so agents can parse results without
  scraping human text. See **Robot Mode** and **Exit Codes**.

**Checkpoint:** both surfaces return identical ranked results for the same query/opts.

## Output Specification

Write **`SEARCH-INTEGRATION-PLAN.md`** at the repo root with these sections:
`Index Strategy` (engines, chunking, fusion, on-disk path) · `Query API` (the port + Hit
shape) · `Incremental Re-indexing` (hash policy + change handling) · `Wiring` (library
export + CLI subcommand) · `Risks & Limits` (index size, model pinning, cold-start cost).
Then land the in-tree `search` module and the CLI subcommand it describes.

## Robot Mode

Agents consume search via NDJSON — one JSON object per line, no prose:

```
$ mytool search "where is auth verified" --json --limit 3
{"path":"src/auth/verify.rs","line":42,"snippet":"pub fn verify_token(...)","score":0.91}
{"path":"src/middleware/auth.rs","line":17,"snippet":"let claims = verify_token(...)","score":0.77}
```

Flags: `--json` (NDJSON out), `--limit N`, `--mode lexical|semantic|hybrid` (default hybrid).

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Query ran; zero or more hits returned |
| 2 | Bad usage (missing query, invalid `--mode`/`--limit`) |
| 3 | Index missing or corrupt — run `reindex` |
| 4 | Embedding/model unavailable for semantic/hybrid mode |

## Quality Rubric

- **Sub-100 ms warm queries:** a hybrid query over an already-built, persisted index returns
  in <100 ms on a mid-size repo (measure and record it; an in-RAM rebuild per query fails this).
- **Incremental, not full:** editing one file re-indexes one file (prove via manifest-hash diff,
  not by timing a full rebuild).
- **Every hit is actionable:** 100% of returned hits include a resolvable `path:line` and a
  non-empty snippet.

## Examples

- *Agent triage:* `mytool search "rate limit middleware" --json --limit 5` → agent opens the
  top `path:line` and edits in place — no `grep` sweep, no manual ranking.
- *Refactor scoping:* hybrid search for a concept (e.g. "retry with backoff") surfaces both the
  exact `retry_with_backoff` fn and semantically-similar ad-hoc retry loops grep would miss.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Every query is slow | Index built in RAM per process | Persist to disk; `open_or_create` (Constraint 1) |
| Semantic results are garbage | Query/corpus embedded with different models or dims | Pin model + dim; re-index; record both in the manifest |
| Deleted code still appears | Re-index keyed on mtime, or deletes not handled | Hash-based diff; remove docs+vectors on delete (Phase 4) |
| Exact identifiers don't match | Tokenizer splits identifiers and drops the whole form | Code-aware tokenizer: index whole + sub-tokens |
| Hits have no location | Snippet/line not stored at index time | Store `path`+`line`+`snippet` per chunk (Constraint 4) |

## See Also

| I need to… | Use |
|------------|-----|
| Understand the legacy code before indexing it | `codebase-archaeology` |
| Make the search CLI agent-friendly (flags, exit codes, JSON) | `agent-ergonomics-and-intuitiveness-maximization-for-cli-tools` |
