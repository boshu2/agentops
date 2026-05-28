# BC1 Corpus Ports Contract

> **Status:** scaffolded 2026-05-12 (cycles 78-81) AND all 4
> production adapters delivered (cycles 83, 112, 113, 114) —
> `productionCitationAdapter`, `productionCorpusReader`,
> `productionCorpusWriter`, and `productionFindingCompiler` live in
> `cli/cmd/ao/`. Wire-up tracked under `soc-pm5t` (BC1.1) now means
> "migrate existing call-sites in the production-path packages to
> route through these adapters" — the adapters themselves are done.

The Corpus bounded context (BC1) is responsible for capturing,
retrieving, decay-ranking, and promoting knowledge artifacts. This
contract names the 4 typed Go interfaces that formalize what BC1
needs from outside. The interfaces themselves live at
`cli/internal/ports/`; the per-port doc-comments are the
authoritative semantics.

See also: [ubiquitous-language.md](ubiquitous-language.md) (BC1 row
canonical naming), [finding-compiler.md](finding-compiler.md) (the
compile-side contract `FindingCompilerPort` mirrors), bd epic
[`soc-2c1p`](https://github.com/boshu2/agentops/issues) (epic
tracking) and `docs/plans/2026-05-12-rescope-evolve-and-architecture.md`
"Wave 2" (rescoping rationale).

## The 4 Ports

| Port | File | Responsibility |
|---|---|---|
| `CorpusReaderPort` | `cli/internal/ports/corpus_reader.go` | Decay-ranked retrieval (Lookup). Read-side of BC1. |
| `CorpusWriterPort` | `cli/internal/ports/corpus_writer.go` | Typed capture (Capture). Write-side of BC1. |
| `FindingCompilerPort` | `cli/internal/ports/finding_compiler.go` | Promote finding artifacts into planning-rules / pre-mortem-checks / constraints (Compile). |
| `CitationPort` | `cli/internal/ports/citation.go` | Verify per-citation freshness against HEAD (Verify). |

Each interface ships with one in-memory adapter (`InMemoryX`) intended
for tests + CLI dry-runs, plus a compile-time `var _ XPort = (*InMemoryX)(nil)`
assertion so a future signature drift fails the build.

## Port Semantics Cheat-Sheet

These are normative summaries; the port doc-comments in
`cli/internal/ports/*.go` are the source of truth.

### CorpusReaderPort.Lookup

- Returns a non-nil (possibly empty) slice on success.
- Respects `LookupOptions.Limit` when > 0.
- Results SHOULD be sorted by `Score` descending.
- Adapters that have no decay signal MAY return all-zero scores.
- Honors `ctx.Err()` best-effort.

### CorpusWriterPort.Capture

- MUST be idempotent: re-Capture with the same `CorpusWriteRequest`
  produces no drift; `Created` flips false on the second call.
- `ResolvedPath` is non-empty on success.
- Empty `Path` is a structural-rejection error.
- Honors `ctx.Err()` best-effort.

### FindingCompilerPort.Compile

- Returns a non-nil slice on success.
- No duplicate `Path` values in the output slice.
- Honors `Frontmatter["compiler_targets"]` (comma-separated list of
  `plan|pre-mortem|constraint`) when present; defaults to all three.
- Unknown target strings are silently skipped — callers can detect
  the gap by comparing requested vs emitted slices.
- Empty `ID` is a structural-rejection error.

### CitationPort.Verify

- Returns a non-nil `CitationVerdict` on success even when `Status`
  is `UNKNOWN`.
- `Reason` is non-empty on every return path.
- Empty `Raw` returns `UNKNOWN` with reason "empty Raw" (malformed
  citation; nothing to resolve).
- Adapter-defined behavior for empty `Cwd`: the in-memory adapter
  accepts it; a future filesystem-backed adapter MAY reject it.

## Adapter Construction Pattern

Each port has the same triplet of files:

```
cli/internal/ports/
  <name>.go               # interface + types + doc-comments
  inmemory_<name>.go      # InMemoryX adapter (test double)
  inmemory_<name>_test.go # 5-7 focused tests covering contract
```

When adding a 2nd adapter (filesystem-backed, durable-store-backed):

1. Create the new file under the owning package
   (NOT under `cli/internal/ports/`).
2. The new file's package imports `cli/internal/ports`.
3. Add a compile-time assertion in the new file:
   `var _ ports.XPort = (*YourAdapter)(nil)`.
4. Reuse the test fixtures from `inmemory_<name>_test.go` shape —
   the contract assertions are kind-agnostic; only the construction
   step differs.

## Wire-Up Order (soc-pm5t and downstream)

The current `cli/cmd/ao/` callers depend on concrete types (e.g.
`Citation`, `VerifyReport`). To route through the ports without
breaking behavior:

1. **CitationPort first** — the smallest existing callers
   (`verifyFunctionCitation` / `verifySymbolCitation` /
   `verifyFileCitation` in `cli/cmd/ao/beads.go`, all 100%-covered
   per cycle 75). Wrap their existing logic in a `*ProductionCitation`
   adapter that satisfies `CitationPort`.
2. **CorpusReaderPort next** — the `ao lookup` / `ao inject` paths
   currently embed the `.agents/learnings/` reader inline. Extract
   a `*LearningsCorpusReader` adapter and route `ao lookup` through
   it. Keep the inline reader as the implementation backing the new
   adapter — the port is a contract, not a rewrite trigger.
3. **CorpusWriterPort** — once readers are routed, do the writers
   (bd-close harvest, forge promotion, dream compounding writes).
4. **FindingCompilerPort** — `ao compile` is the only major caller;
   wrap its existing pipeline behind the port last.

Each step is a single cycle: extract one adapter, route one caller
set, keep the in-memory adapter as the test double, land green.

## What This Contract Does NOT Specify

- **Persistence format.** Adapters decide.
- **Decay-ranking algorithm.** Adapters decide; the port only
  promises Score-descending order.
- **Cross-port composition.** No declared dependency between
  `CorpusReaderPort` and `FindingCompilerPort` — callers compose
  them externally.
- **Concurrent-mutation guarantees.** Adapters document their own
  thread-safety posture (the in-memory writer is mutex-guarded;
  the in-memory reader is read-only at construction).

## Drift-Blocking Surfaces

- Compile-time port assertions in each `inmemory_<name>.go` file.
- 22 Go tests in `cli/internal/ports/*_test.go` (98.8% statement
  coverage as of cycle 81).
- This contract doc is linked in `docs/documentation-index.md`.

## See Also

- [`finding-compiler.md`](finding-compiler.md) — the prevention
  ladder the `FindingCompilerPort` formalizes.
- [`finding-registry.md`](finding-registry.md) — the upstream
  registry layer feeding the compile pipeline.
- [`ubiquitous-language.md`](ubiquitous-language.md) — canonical
  naming per BC.
- `docs/plans/2026-05-12-rescope-evolve-and-architecture.md` —
  Wave 2 rescoping rationale + BC1 epic anchor.
