# Go CLI Dead-Weight Keep/Delete/Extract Manifest (2026-07-19)

Bead: `age-gocli-audit-remediation-6fybr.4` (D0: manifest only — NO code changes in this bead).
Base: `origin/main` @ `a2f00256b`. Worktree `gocli-remed-D`.

## Method (reachability receipt contract)

Every disposition below carries an evidence receipt of the form:

- **deps** = membership in `go list -deps ./cmd/...` (captured once from `cli/`), the set of
  packages linked into the shipped binaries. Being IN this set means "some reachable file imports
  it," NOT "it is live" — an orphan file in a reachable package (e.g. `package main` in `cmd/ao`)
  drags its imports into the deps set even when nothing ever calls them.
- **chain** = a traced call-chain (or its documented absence) proving whether the entry point is
  actually invoked, so we distinguish "linked" from "live."

Grep alone is never sufficient: it both **over-matches** (`internal/llm` substring-matches
`internal/llmwiki`) and **under-matches** (an aliased import — `ratchet "…/internal/evidence"` —
hides a live consumer from any `evidence.` search). Both traps were hit and are documented below.

Build-tag sweep: `go list -deps -tags "flywheel legacy" ./cmd/...` yields the **same** package set as
the untagged run (diff = ∅). **No target package or file is behind a `//go:build` / `+build` guard.**
So no item is "dead only because a tag is off."

---

## DELETE set

Fully dead: no live consumer. Each is either absent from the deps set, or present only via a dead
orphan intermediary (traced below).

### cmd/ao orphan file + its two packages

| Target | Evidence | Disposition |
|---|---|---|
| `cli/cmd/ao/reviewer_health_composition.go` | Defines package-level `var reviewerHealthService = reviewerhealth.NewService(...)` and `const reviewerProbeTimeout`. `chain`: **grep across all of `cmd/ao/*.go` finds ZERO references to `reviewerHealthService` or `reviewerProbeTimeout` outside this file.** The var is init-evaluated but never read; no cobra command wires it. Orphan. | **DELETE** |
| `cli/internal/reviewerhealth` | `deps`: IN set (line 227) — but ONLY because the orphan `reviewer_health_composition.go` imports it. Sole non-test importer of the package is that orphan file. `chain`: dead once the orphan is removed. | **DELETE** |
| `cli/internal/adapters/reviewerhealth` | `deps`: IN set (line 228) via the same orphan (`revieweradapter.SystemProbe()`). Non-test importers: only `reviewer_health_composition.go` (+ its own `probe_test.go`). `chain`: dead once the orphan is removed. | **DELETE** |

> This trio is the textbook "reachable via dead intermediary": all three sit in `go list -deps`
> yet none is live. Deps membership alone would have wrongly saved them.

### TestMain husks (packages with zero production code and zero tests)

| Target | Evidence | Disposition |
|---|---|---|
| `cli/internal/canon` | `deps`: NOT in set. Only file is `canon_testmain_test.go` — a `TestMain` that calls `testsupport.ScrubGitDiscoveryEnv()` then `m.Run()` over an **empty** package (no `.go` production files, no other `_test.go`). No non-test importers. A guard around nothing. | **DELETE** |
| `cli/internal/vibecheck` | `deps`: NOT in set. Only file is `vibecheck_testmain_test.go` — identical empty-package `TestMain` husk. No importers. | **DELETE** |

### notebook (chained deletion — depends on the search reduction)

| Target | Evidence | Disposition |
|---|---|---|
| `cli/internal/notebook` | `deps`: IN set (line 235). `chain`: **sole importer is `cli/internal/search/inject_run.go`** (exact `internal/notebook"` match; no other importer anywhere). `inject_run.go` is in the DELETE-portion of `internal/search` (it is NOT part of the constraint-index extract slice; `constraint.go` references nothing in `notebook`/`inject_run`). Therefore notebook is orphaned the moment `internal/search` is reduced to `constraint.go`. | **DELETE — sequence AFTER the search extract** |

### cmd/ao dead helper files (fully self-contained, zero external refs)

| Target | Evidence | Disposition |
|---|---|---|
| `cli/cmd/ao/advertised_commands.go` | Entry point `extractAdvertisedAoInvocations` appears in **exactly one file** (itself); the other funcs (`validateAdvertisedAoInvocation`, `findAdvertisedSubcommand`, `isCommandGroup`) only call each other. No test file exists. Dead. | **DELETE** |
| `cli/cmd/ao/shell_quote.go` | `shellQuote` appears in **exactly one file** (itself). No caller, no test. Dead. | **DELETE** |
| `cli/cmd/ao/beads_json_compat.go` | `emitJSON` appears in **exactly one file** (itself). No caller, no test. Dead. | **DELETE** |

### Test-only files (referenced only by their own `_test.go`, no production caller)

| Target | Evidence | Disposition |
|---|---|---|
| `cli/cmd/ao/template_detect.go` | `detectTemplateFromProjectRoot` referenced **only** by `cmd/ao/template_detect_test.go`; zero non-test refs. | **DELETE (test-only)** — delete with its test |
| `cli/cmd/ao/federation.go` | `resolveFederated` / `federatedProbe` / `ErrArtifactNotFound` referenced **only** by `cmd/ao/federation_test.go`; zero non-test refs. | **DELETE (test-only)** — delete with its test |

---

## EXTRACT-THEN-DELETE set

Package has ONE live externally-consumed slice; extract that slice to its own package, then delete
the remainder.

### `cli/internal/search` — keep ONLY the constraint-index slice

- `deps`: IN set (line 236). **Sole external non-test importer: `cli/internal/gates/checks/constraints.go`.**
- Symbols `constraints.go` actually references (the LIVE surface): `search.ConstraintIndex`,
  `search.ConstraintEntry`, `search.ConstraintIndexPath`, `search.LoadConstraintIndex`,
  `search.PublishedConstraintIndexRelPath`.
- **The constraint-index slice = the file `internal/search/constraint.go`** (+ `constraint_test.go`).
  All of its symbols are self-contained in that file: `ConstraintIndex`, `ConstraintEntry`,
  `ConstraintIndexPath`, `ConstraintLockPath`, `PublishedConstraintIndexRelPath`, `PublishedLeaks`,
  `SanitizeForPublish`, `WithConstraintLock`, `LoadConstraintIndex`, `SaveConstraintIndex`,
  `SaveConstraintIndexUnlocked`, `BuildConstraintEntry`, `ValidateConstraintActivation`,
  `UpsertConstraintAt`, `loadConstraintIndexAtPath`, `saveConstraintIndexAtPath`, `FindConstraint`,
  `FilterStaleConstraints`.
- **Only cross-package import of `constraint.go` is `internal/ports`** (for `ports.DetectorReplayResult`
  in `BuildConstraintEntry`). Clean extraction; pulls in no other search file.
- **EXTRACT** `constraint.go` (+ test) to its own package (e.g. `internal/constraintindex`); repoint
  `constraints.go`'s import. **Then DELETE** the remaining ~20 search files: `bead_context.go`,
  `exploration.go`, `findings.go`, `findings_ops.go`, `index.go`, `inject_citations.go`,
  `inject_options.go`, `inject_run.go`, `learnings.go`, `patterns.go`, `predecessor.go`,
  `quality_gate.go`, `scoring.go`, `search_cass.go`, `sessions.go`, `types.go`, `util.go` (+ their
  tests). `inject_run.go`'s deletion is what orphans `internal/notebook` (above).

### `cli/internal/llm` — keep ONLY `RedactBytes` (via `redactor.go`)

- `deps`: IN set (line 238). **Sole external importer: `cli/cmd/ao/redact.go`** (exact `internal/llm"`
  match — the earlier `internal/storage/atomicfile.go` "hit" was a **false positive**: that file
  contains the comment substring `internal/llmwiki`, not an import).
- `redact.go` uses **only `llm.RedactBytes`** (the apparent `llm.Redact` hit is a substring of
  `llm.RedactBytes`). Bead claim "consumer: cmd/ao/redact.go, minus RedactBytes" is **correct**.
- **The kept slice = `internal/llm/redactor.go`** (+ `redactor_test.go`): `Redact`, `RedactBytes`
  (thin `[]byte` wrapper over `Redact`), `redactionDenylistLiterals`, `secretPatterns`,
  `homePathPattern`, `init`. Fully self-contained — imports only `os`, `regexp`, `strings`
  (the `ChunkTurns` mention in `redactor.go` is a **comment**, not a call).
- **EXTRACT** `redactor.go` to its own package; repoint `redact.go`. **Then DELETE** the rest of
  `internal/llm`: `chunker.go`, `ollama_client.go`, `review.go`, `session_writer.go`,
  `summarizer.go`, `types.go`, `wiki_log.go` (+ their tests).

---

## `cli/internal/ports` — exact keep/delete list

`deps`: IN set (line 221). Whole tree grep of `ports.X` from every file importing `internal/ports`
(no aliased imports exist) yields exactly these externally-referenced symbols:
`GateVerdict, GateStatus(+Pass/Warn/Fail/Skip/Unknown), GateName, GateRunRequest, GateRunnerPort,
NewInMemoryGateRunner, FrontmatterCodecPort, FrontmatterDocument, FrontmatterConfidence,
DetectorReplayResult`.

### KEEP

| File / symbols | Evidence |
|---|---|
| `gate_runner.go` — `GateName`, `GateVerdict`, `GateStatus` + 5 consts, `GateRunRequest`, `GateRunnerPort` | Production-live: consumed by `internal/gates/{gates,orchestrator,scriptrunner,report}.go`, `internal/gates/checks/{go_build,native_inline,constraints}.go`, `internal/gate/check_service.go`. Self-contained (imports only `context`). |
| `inmemory_gate_runner.go` — `InMemoryGateRunner`, `NewInMemoryGateRunner` | Consumed by `internal/gates/orchestrator_test.go` (**test-only**). Keep while that test stays; the only inmemory adapter with any external consumer. |
| `frontmatter_codec.go` — `FrontmatterDocument`, `FrontmatterConfidence`, `FrontmatterCodecPort` | Consumed by `internal/wiki/frontmatter.go`. Self-contained. |
| `finding_compiler.go` — **PARTIAL**: `DetectorReplayResult` **and `DetectorPrecisionEvidence`** | `DetectorReplayResult` consumed by `internal/search/constraint.go` (the surviving extract slice). **`DetectorPrecisionEvidence` is a transitive keep** — it is the type of `DetectorReplayResult.Precision`; deleting it breaks the build (see flag ⚠️ below). |
| `doc.go` | Package doc. Keep. |

### DELETE — the dead ~12 (enumerated with evidence)

Every symbol below has **zero external `ports.X` references** (whole-tree grep) and every
`NewInMemory*` constructor except `NewInMemoryGateRunner` has **zero external consumers**.

| # | File(s) | Dead symbols | Evidence |
|---|---|---|---|
| 1 | `agent_mail.go` | `AgentMailPort`, `AgentMailIdentityRequest`, `AgentMailIdentity`, `AgentMailReservationRequest`, `AgentMailReservation`, `AgentMailMessageRequest`, `AgentMailMessage` | no external `ports.AgentMail*` ref |
| 2 | `ci_status.go` + `inmemory_ci_status.go` | `CIStatusPort`, `CIRun`, `CIRunStatus`, `CIRunConclusion`, `InMemoryCIStatus`, `NewInMemoryCIStatus` | `NewInMemoryCIStatus` DEAD; no external ref |
| 3 | `citation.go` + `inmemory_citation.go` | `CitationPort`, `CitationKind`, `CitationStatusResult`, `CitationRequest`, `CitationVerdict`, `InMemoryCitation`, `NewInMemoryCitation` | `NewInMemoryCitation` DEAD; no external ref |
| 4 | `context_compiler.go` + `inmemory_context_compiler.go` | `ContextCompilerPort`, `ContextSection`, `ContextAssemblyRequest`, `ContextPacket`, `InMemoryContextCompiler`, `NewInMemoryContextCompiler` | `NewInMemoryContextCompiler` DEAD; no external ref |
| 5 | `corpus_reader.go` + `inmemory_corpus_reader.go` | `CorpusReaderPort`, `CorpusItem`, `LookupOptions`, `InMemoryCorpusReader`, `NewInMemoryCorpusReader` | `NewInMemoryCorpusReader` DEAD; no external ref |
| 6 | `corpus_writer.go` + `inmemory_corpus_writer.go` | `CorpusWriterPort`, `CorpusWriteRequest`, `CorpusWriteResult`, `InMemoryCorpusWriter`, `NewInMemoryCorpusWriter` | `NewInMemoryCorpusWriter` DEAD; no external ref |
| 7 | `finding_compiler.go` (**dead portion**) + `inmemory_finding_compiler.go` | `FindingCompilerPort`, `FindingArtifact`, `DetectorFixture`, `DetectorEvidence`, `CompiledOutputKind` (+3 consts), `CompiledOutput`, `ReplayDetectorEvidence`, `InMemoryFindingCompiler`, `NewInMemoryFindingCompiler` | `ReplayDetectorEvidence` has **zero external callers**; `NewInMemoryFindingCompiler` DEAD. NOT a whole-file delete — `DetectorReplayResult`/`DetectorPrecisionEvidence` stay (see KEEP). |
| 8 | `finding_recurrence.go` | `FindingRecurrenceReducerPort`, `FindingObservation`, `ProducerEvidenceRef`, `ProducerRuleCandidate`, `InMemoryFindingRecurrenceReducer`, `NewInMemoryFindingRecurrenceReducer`, `ReduceFindingRecurrence` | `ReduceFindingRecurrence` + constructor both DEAD (no external ref) |
| 9 | `freshness_policy.go` | `FreshnessPolicyPort`, `FreshnessSignalKind`, `FreshnessChangeSignal`, `FreshnessVerdict` | no external ref |
| 10 | `harness.go` + `inmemory_harness.go` | `HarnessPort`, `HarnessName`, `HarnessSkillSync`, `InMemoryHarness`, `NewInMemoryHarness` | `NewInMemoryHarness` DEAD; no external ref |
| 11 | `llm.go` | `LLMClient`, `CompletionOptions` | no external ref (distinct from `internal/llm`) |
| 12 | `wiki_index.go` | `WikiIndexPort`, `WikiIndexRecord`, `WikiIndexResult` | no external ref |

Plus the matching `*_test.go` for each group.

---

## EXPLICITLY EXCLUDED — LIVE, do NOT delete

| Target | Evidence (the alias trap) |
|---|---|
| `cli/internal/evidence` | `deps`: IN set (line 234). **Imported in `cmd/ao/flywheel_metrics.go:9` under the alias `ratchet "…/internal/evidence"`.** `chain`: live call `ratchet.LoadCitations(baseDir)` at `flywheel_metrics.go:142`, backing `ao flywheel compare/status`. A naive grep for `evidence.LoadCitations` / `evidence.` returns **nothing** — the alias hides the live consumer. This is the canonical reason deps+chain beats grep. **KEEP.** |
| `cmd/ao/flywheel_metrics.go` + `metrics_flywheel.go` (flywheel compute block) | No build tag; wired into the live `ao flywheel` command; consume `internal/evidence` as above. **KEEP.** |

---

## Discrepancies flagged (bead disposition vs. evidence)

1. ⚠️ **`DetectorPrecisionEvidence` is a mandatory transitive keep the bead omitted.** The bead's
   ports keep list names only `DetectorReplayResult`. But `DetectorReplayResult.Precision` is typed
   `*DetectorPrecisionEvidence`. Deleting `DetectorPrecisionEvidence` (it sits in the otherwise-dead
   `finding_compiler.go`) while keeping `DetectorReplayResult` **will not compile.** The deleter must
   keep both, and `finding_compiler.go` is therefore a **partial-keep file, not a whole-file delete.**

2. ⚠️ **The bead's gate keep-shorthand ("GateVerdict/GateStatus*") undersells the live gate cluster.**
   `GateName`, `GateRunRequest`, and `GateRunnerPort` are all production-live (consumed across
   `internal/gates/*` and `internal/gate/check_service.go`); `NewInMemoryGateRunner` is test-live.
   The entire `gate_runner.go` file and `inmemory_gate_runner.go` are KEEP — not just two symbols.
   Not a wrong delete (nothing claimed-dead is actually live), but the keep-list must be widened or a
   deleter working from the shorthand could over-trim `gate_runner.go`.

3. **`internal/notebook` delete is CONDITIONAL/ordered**, not free-standing: it is live today (via
   `search/inject_run.go`) and only becomes dead once the `internal/search` extract lands. Sequence:
   extract `search/constraint.go` → delete rest of `search` → then delete `notebook`.

4. **False-positive corrected:** bead reasoning could have implied a second `internal/llm` consumer;
   `internal/storage/atomicfile.go` does **not** import `internal/llm` (substring match on the
   `internal/llmwiki` comment). Sole llm consumer is `cmd/ao/redact.go`.

No claimed-DELETE item turned out to be genuinely live/load-bearing. No build-tag-guarded packages
found among any target (untagged vs. `flywheel legacy` deps diff = ∅).

---

## Compact summary

| Set | Items | One-line evidence |
|---|---|---|
| DELETE | `reviewer_health_composition.go` + `internal/reviewerhealth` + `internal/adapters/reviewerhealth` | in deps only via the orphan file; `reviewerHealthService` var never read |
| DELETE | `internal/canon`, `internal/vibecheck` | TestMain husks over empty packages; not in deps |
| DELETE | `internal/notebook` | sole consumer `search/inject_run.go` is itself deleted (ordered) |
| DELETE | `cmd/ao/advertised_commands.go`, `shell_quote.go`, `beads_json_compat.go` | each self-contained; entry symbol appears in one file only |
| DELETE (test-only) | `cmd/ao/template_detect.go`, `federation.go` | referenced only by their own `_test.go` |
| EXTRACT→DELETE | `internal/search` → keep `constraint.go` (5 externally-used symbols, consumer `gates/checks/constraints.go`) | rest (~20 files incl `inject_run.go`) dead |
| EXTRACT→DELETE | `internal/llm` → keep `redactor.go` (`RedactBytes`, consumer `cmd/ao/redact.go`) | rest (chunker/ollama/review/summarizer/…) dead |
| KEEP (ports) | `gate_runner.go` + `inmemory_gate_runner.go`, `frontmatter_codec.go`, `finding_compiler.go`::{`DetectorReplayResult`,`DetectorPrecisionEvidence`}, `doc.go` | production/test consumers traced |
| DELETE (ports) | the 12 dead groups (agent_mail, ci_status, citation, context_compiler, corpus_reader, corpus_writer, finding_compiler-dead-part, finding_recurrence, freshness_policy, harness, llm.go, wiki_index) | zero external `ports.X` refs each |
| EXCLUDED (LIVE) | `internal/evidence` + flywheel compute block | aliased `ratchet.LoadCitations` at `flywheel_metrics.go:142`; grep-invisible |
