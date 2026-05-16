# BC Ports Inventory

> **Status:** 20 ports scaffolded (12 from the cycle 78-106 wire-up
> arc + FactoryAdmissionPort cycle 139 / soc-2klg.1 +
> ClaimEvidencePort cycle 141 / soc-2klg.2 + four AgentOps 3.0
> hookless replacement ports from soc-m6v5.9.8.2 + two BC3 Loop
> evidence/stop ports from soc-y5vh.3). 16 of 20 have production
> adapters delivered (cycles 83 + 108-118 + 140 + 142 + 193-194).
> The 4 remaining ports have in-memory adapters and tests; production
> adapters land as their hook leases and loop call-sites are migrated.
> Every BC port has an `InMemoryX` test double in
> `cli/internal/ports/` (compile-time `var _ XPort = (*InMemoryX)(nil)`
> assertions). Next-phase work continues call-site migration through
> these adapters (per-BC follow-up bds: `soc-pm5t` for BC1, sibling
> bds for BC2-BC5).

The 5 bounded contexts (Corpus, Validation, Loop, Factory, Runtime)
each declare a small set of typed Go interfaces ("ports") at
`cli/internal/ports/`. This contract is the catalog index — for the
detailed BC1 semantics see
[`bc1-corpus-ports.md`](bc1-corpus-ports.md). Each BC follows the
same file triplet shape (`<port>.go` + `inmemory_<port>.go` +
`inmemory_<port>_test.go`) and includes a compile-time
`var _ <Port> = (*InMemoryX)(nil)` assertion as a drift guard.

See also: [`ubiquitous-language.md`](ubiquitous-language.md)
(canonical naming per BC),
[`finding-compiler.md`](finding-compiler.md) (the existing
compile-side contract `FindingCompilerPort` mirrors), bd epics
[`soc-2c1p`](https://github.com/boshu2/agentops/issues) (BC1),
`soc-wxh5` (BC2), `soc-y5vh` (BC3), `soc-2klg` (BC4),
`soc-zd7c` (BC5).

## Roster

### BC1 Corpus / Context Compiler (5 ports)

| Port | File | Responsibility |
|---|---|---|
| `CorpusReaderPort` | `corpus_reader.go` | Decay-ranked retrieval (Lookup) |
| `CorpusWriterPort` | `corpus_writer.go` | Typed capture (Capture, idempotent) |
| `FindingCompilerPort` | `finding_compiler.go` | Promote finding → plan/pre-mortem/constraint outputs |
| `CitationPort` | `citation.go` | Verify per-citation freshness against HEAD |
| `ContextCompilerPort` | `context_compiler.go` | Assemble bounded phase context explicitly, replacing startup/prompt context hooks |

Detailed semantics: [`bc1-corpus-ports.md`](bc1-corpus-ports.md).

### BC2 Validation / Evidence and Trust (4 ports)

| Port | File | Responsibility |
|---|---|---|
| `GateRunnerPort` | `gate_runner.go` | Run a named gate; return PASS/WARN/FAIL/SKIP/UNKNOWN verdict |
| `CIStatusPort` | `ci_status.go` | Read CI history (Latest(sha), Recent(limit)) |
| `ClaimEvidenceBinderPort` | `claim_evidence_binder.go` | Bind claim→evidence at a promotion-gate level (PG1-PG4, upgrade-only) |
| `SafetyPolicyPort` | `safety_policy.go` | Evaluate deterministic safety policies for mutation/read operations |

Adapter contracts:

- `GateRunnerPort.Run` returns non-nil verdict; empty Name → UNKNOWN.
  Unknown-name policy is adapter-defined (in-memory: UNKNOWN; flag
  `UnknownIsFail` flips to FAIL).
- `CIStatusPort.Latest` returns zero-value `CIRun` for unknown sha
  (not an error). Empty sha → error.
- `ClaimEvidenceBinderPort.Bind` is idempotent; allows level upgrade;
  rejects downgrade with error containing "downgrade".

### BC3 Loop / Explicit Lifecycle (5 ports)

| Port | File | Responsibility |
|---|---|---|
| `LoopReaderPort` | `loop_reader.go` | Read evolve cycle ledger (Latest, Range, IdleStreak) |
| `LoopWriterPort` | `loop_writer.go` | Append cycle entries (auto-assign Number; reject duplicates) |
| `CloseoutPort` | `closeout.go` | Run explicit end-of-cycle/session closeout actions |
| `HypothesisLedgerPort` | `hypothesis_ledger.go` | Append/read empirical evolve hypotheses without coupling callers to JSONL |
| `ConvergenceCheckPort` | `convergence_check.go` | Evaluate the structural evolve stop predicate from typed evidence |

Adapter contracts:

- `LoopReaderPort.IdleStreak` counts trailing entries whose
  `Result` is `"idle"` or `"unchanged"` — the dormancy signal
  evolve's Step 3 uses.
- `LoopWriterPort.Append` auto-assigns `Number` when it's 0
  (next = max+1); honors explicit Number; rejects duplicates with
  error containing "duplicate".
- `HypothesisLedgerPort.Append` rejects empty and duplicate IDs.
  `List` returns append order. `Find` returns `(zero, false, nil)`
  for unknown IDs. Returned evidence slices are defensive copies.
- `ConvergenceCheckPort.Check` counts the leading most-recent CI
  success streak only. Default criteria are CI green streak >=3,
  HIGH+MEDIUM unconsumed <=1, and fitness baseline captured.
  Non-success, queued, or in-progress runs break the streak.

### BC4 Factory (4 ports)

| Port | File | Responsibility |
|---|---|---|
| `OperatorPort` | `operator.go` | Record human-in-loop intents (Record, List most-recent-first) |
| `EventBusPort` | `event_bus.go` | Pub/sub for factory events (Publish, Subscribe with cancel) |
| `FactoryAdmissionPort` | `factory_admission.go` | Probe repo, PR, and CI evidence for factory admission decisions |
| `ClaimEvidencePort` | `claim_evidence.go` | Derive claim-to-evidence promotion from gate verdicts without downgrades |

Adapter contracts:

- `OperatorPort.Record` rejects empty `Kind`. List returns
  most-recent first.
- `EventBusPort` uses **exact** topic match (no globbing).
  Subscribe returns a cancel function that blocks until in-flight
  dispatch completes. Handler errors do not stop sibling
  subscribers. Empty Topic on Publish rejected.
- `FactoryAdmissionPort` returns Known-bearing evidence slices so
  callers can distinguish unavailable evidence from a clean result.
- `ClaimEvidencePort.Derive` enforces upgrade-only promotion:
  PASS promotes to PG2 minimum, WARN to PG1 minimum, FAIL/SKIP/UNKNOWN
  keep the existing level.

### BC5 Runtime Shell (2 ports)

| Port | File | Responsibility |
|---|---|---|
| `HarnessPort` | `harness.go` | Report skill↔harness sync state (Status, StatusForSkill) |
| `WorkspacePort` | `workspace.go` | Setup and cleanup runtime workspaces/worktrees |

Adapter contracts:

- `HarnessPort.Status` returns a fresh defensive copy.
- `HarnessPort.StatusForSkill` rejects empty skill; unknown skill
  returns non-nil empty slice.
- `WorkspacePort.Setup` and `WorkspacePort.Cleanup` reject empty
  workspace ids and return typed lifecycle results.

## AgentOps 3.0 Hookless Replacement Ports

The hookless-first 3.0 migration adds four ports specifically to
absorb hidden runtime hook behavior before deletion:

| Replacement port | Replaces hook class |
|---|---|
| `ContextCompilerPort` | SessionStart context injection, context guard, standards injector, context monitor |
| `SafetyPolicyPort` | destructive git, worker authority, edit scope, holdout isolation, team lifecycle guards |
| `CloseoutPort` | SessionEnd/Stop maintenance, handoff, flywheel closeout, compile defrag |
| `WorkspacePort` | WorktreeCreate/WorktreeRemove setup and cleanup |

These complement existing ports already used by the hook lease
inventory:

- `GateRunnerPort` for deterministic validation hooks
- `EventBusPort` for non-resident event subscribers
- `HarnessPort` for Codex/Claude projection refresh and parity
- `OperatorPort` for prompt routing as explicit operator intent

Drift check: `scripts/check-hook-port-replacements.sh` verifies the
lease inventory references real port files and that every required
hook replacement port is referenced by at least one non-remove hook.

## Adapter Construction Pattern (universal across BCs)

```
cli/internal/ports/
  <name>.go                  # interface + types + doc-comments
  inmemory_<name>.go         # InMemoryX adapter (test double)
  inmemory_<name>_test.go    # 5-9 focused tests covering contract
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

## Per-BC Wire-Up Order

Each BC has its own follow-up bd that tracks production-adapter +
caller-refactor work:

- **BC1** (`soc-pm5t`): start with CitationPort (smallest callers,
  cycle 75's 100%-covered helpers). Cycle 83 landed
  `productionCitationAdapter` as the first wire-up commit.
- **BC2** (future bd): start with GateRunnerPort — existing gate
  invocations live in `cli/internal/evalsubstrate/gates.go`.
- **BC3** (future bd): start with LoopReaderPort — evolve Step 0
  bootstrap reads cycle-history.jsonl inline today.
- **BC4** (future bd): EventBusPort needs a real transport
  (NATS/Kafka) before wire-up is useful; OperatorPort can wire to
  the existing /halt + /rescue skills first.
- **BC5** (future bd): HarnessPort wraps the existing
  `scripts/regen-codex-hashes.sh` + `scripts/audit-codex-parity.sh`
  flow.

## What This Contract Does NOT Specify

- **Persistence format.** Adapters decide.
- **Decay-ranking, retry, or backpressure algorithms.** Adapters
  decide; ports are kind-agnostic.
- **Cross-port composition.** No declared cross-port dependencies
  — callers compose them externally.
- **Concurrent-mutation guarantees.** Adapters document their own
  thread-safety posture (in-memory writers are mutex-guarded;
  in-memory readers are read-only at construction).

## Drift-Blocking Surfaces

- Compile-time `var _ XPort = (*InMemoryX)(nil)` assertions in
  every BC `inmemory_<name>.go` file (20 assertions total).
- Compile-time `var _ XPort = (*productionX)(nil)` assertions in
  every `cli/cmd/ao/<x>_adapter.go` file (14 assertions total).
- 100+ Go tests in `cli/internal/ports/*_test.go` and 100+ Go tests
  across the production-adapter files in `cli/cmd/ao/*_adapter_test.go`.
- This contract doc is linked in `docs/documentation-index.md`.

## Tracked Scaffold Commits (chronological)

| Cycle | Commit | Port |
|---|---|---|
| 78 | `e362f427` | BC1 CorpusReaderPort + scaffold |
| 79 | `4a1cf5f0` | BC1 CorpusWriterPort |
| 80 | `39c98335` | BC1 FindingCompilerPort |
| 81 | `3791568f` | BC1 CitationPort |
| 99 | `06b63ba6` | BC2 GateRunnerPort |
| 100 | `2bf4c8d7` | BC2 CIStatusPort |
| 101 | `e59b8802` | BC2 ClaimEvidenceBinderPort |
| 102 | `54bca279` | BC3 LoopReaderPort |
| 103 | `7fd9466e` | BC3 LoopWriterPort |
| 104 | `d10ae648` | BC4 OperatorPort |
| 105 | `8cd646e5` | BC4 EventBusPort |
| 106 | `a6754235` | BC5 HarnessPort — 12-port surface complete |
| 139 | `adafc08d` | BC4 FactoryAdmissionPort — 13th port (soc-2klg.1) |
| 141 | _this commit_ | BC4 ClaimEvidencePort — 14th port (soc-2klg.2) |
| 3.0 S2 | _this branch_ | BC1/BC2/BC3/BC5 hookless replacement ports: ContextCompilerPort, SafetyPolicyPort, CloseoutPort, WorkspacePort |
| 192 | _this commit_ | BC3 HypothesisLedgerPort + ConvergenceCheckPort: 19th/20th ports (soc-y5vh.3) |

## Production Adapters (chronological, completed cycle 118)

| Cycle | Commit | Adapter | BC | Shape |
|---|---|---|---|---|
| 83 | `4e91ab58` | `productionCitationAdapter` | BC1 | bd CLI wrapper |
| 108 | `bb78cdb3` | `productionLoopReader` | BC3 | JSONL read |
| 109 | `b0fa7dfe` | `productionLoopWriter` | BC3 | JSONL append |
| 110 | `c511214b` | `productionOperator` | BC4 | JSONL append + reverse |
| 111 | `c851ab8a` | `productionHarness` | BC5 | tree walk + hash join |
| 112 | `f27b0bec` | `productionCorpusReader` | BC1 | tree walk + ranker |
| 113 | `0be3f00b` | `productionCorpusWriter` | BC1 | idempotent file write + frontmatter |
| 114 | `fd9dc598` | `productionFindingCompiler` | BC1 | pure-Go transform |
| 115 | `006ad286` | `productionGateRunner` | BC2 | subprocess (exec.Command) |
| 116 | `96318d7b` | `productionClaimEvidenceBinder` | BC2 | JSONL append + upgrade-only rule |
| 117 | `8669b15e` | `productionCIStatus` | BC2 | external CLI + JSON parse (pluggable runner) |
| 118 | `57ad553d` | `productionEventBus` | BC4 | sync in-memory pubsub |
| 140 | `f4f05324` | `productionFactoryAdmission` | BC4 | daemon.FactoryAdmissionEvidenceProvider wrapper (3 probe methods + type translation) |
| 142 | _this commit_ | `productionClaimEvidence` | BC4 | composer over GateRunnerPort (policy enforced via productionPromoteEvidenceLevel) |
| 193 | _this commit_ | `productionHypothesisLedger` | BC3 | JSONL append/read for `.agents/evolve/hypotheses.jsonl` |
| 194 | _this commit_ | `productionConvergenceCheck` | BC3 | Pure STOP predicate over typed CI/finding/baseline evidence |

All adapters in `cli/cmd/ao/<x>_adapter.go` with paired
`<x>_adapter_test.go`. Each carries a compile-time port assertion
and follows the file-triplet pattern. Next-phase work is call-site
migration through these adapters.

## See Also

- [`bc1-corpus-ports.md`](bc1-corpus-ports.md) — detailed BC1 contract
- [`finding-compiler.md`](finding-compiler.md) — prevention ladder
  that `FindingCompilerPort` formalizes
- [`finding-registry.md`](finding-registry.md) — upstream registry
- [`ubiquitous-language.md`](ubiquitous-language.md) — canonical
  naming per BC
- `docs/plans/2026-05-12-rescope-evolve-and-architecture.md` —
  Wave 2 rescoping rationale + BC epic anchors
