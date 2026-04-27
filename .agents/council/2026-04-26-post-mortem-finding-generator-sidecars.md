---
id: post-mortem-2026-04-26-finding-generator-sidecars
type: post-mortem
date: 2026-04-26
source: "PR #155"
---

# Post-Mortem: Finding Generator Sidecars

> RPI streak: unavailable | Sessions: unavailable | Last verdict: unavailable.
> `.agents/rpi/rpi-state.json` was absent in this worktree.

**Epic:** recent / PR #155
**PR:** https://github.com/boshu2/agentops/pull/155
**Merge commit:** `08e3a38eb8b2394e27a3ccd0335761b79f7f3fc4`
**Duration:** 24m active post-merge retrospective
**Cycle-Time Trend:** stable; this is the first structured `.agents/retro/` entry in this worktree.

## Summary

PR #155 fully landed RFC 0001 Proposal 1: Dream can now run bounded read-only
finding generators during INGEST, persist per-generator sidecars, and route
candidates through a single serialized REDUCE writer. `/evolve` remains serial,
and generator workers still cannot mutate `.agents/rpi/next-work.jsonl`
directly.

Proof collected:

- PR #155 merged on 2026-04-26 at 22:38:02Z.
- GitHub checks for PR #155 all passed, including `go-build`, `cli-integration`,
  `contract-compatibility-gate`, `codex-runtime-sections`, `windows-smoke`, and
  `summary`.
- Local review reran `cd cli && go test -race ./internal/overnight ./internal/mine`.
- Prior implementation validation ran `cd cli && go test ./internal/overnight ./internal/mine`,
  `cd cli && go test ./cmd/ao ./internal/mine ./internal/overnight`,
  `cd cli && make build`, `bash scripts/check-next-work-schema-rows.sh`,
  `bash scripts/validate-next-work-contract-parity.sh`,
  `bash scripts/validate-codex-generated-artifacts.sh --scope worktree`,
  `bash scripts/validate-codex-rpi-contract.sh`, and
  `bash scripts/validate-codex-lifecycle-guards.sh`.

## Checkpoint Policy

| Check | Status | Detail |
|---|---|---|
| Chain loaded | SKIP | `.agents/ao/chain.jsonl` is absent, so this is a standalone post-mortem |
| Prior phases locked | SKIP | No ratchet chain rows were available to replay |
| No FAIL verdicts | SKIP | Chain replay unavailable; relied on PR review, local tests, and CI |
| Artifacts exist | PASS | PR #155, merge commit `08e3a38e`, and changed files are present |
| Idempotency | PASS | No existing post-mortem batch for `post-mortem-finding-generator-sidecars` was present |

## Council Verdict: PASS

| Judge | Verdict | Key Finding |
|---|---|---|
| Plan-Compliance | PASS | Proposal 1's sidecar-plus-single-writer shape shipped without parallelizing `/evolve` |
| Tech-Debt | WARN | Queue dedup is safe, but the INGEST duplicate-rate metric can undercount aggregator-sourced duplicates |
| Learnings | PASS | The reusable pattern is sidecar fanout plus one reducer-owned durable write |

### Implementation Assessment

The implementation matches the accepted RFC direction. `RunIngest` now documents
that corpus substages stay serial while bounded read-only generators may fan out
and write sidecars only (`cli/internal/overnight/ingest.go:90-115`). The only
registered generator is `mine-findings`, which runs `mine.Run` with
`EmitWorkItems:false` and a 26h source window (`cli/internal/overnight/ingest.go:327-395`).

The queue write stays serialized in REDUCE. The sidecar aggregator reads
`OutputDir/generator-results/*.json`, skips soft-failed sidecars, dedups by ID
and `dedup_key`, reconciles duplicate candidates by severity, and appends one
batch under `source_epic:"dream-generator-aggregator"`
(`cli/internal/overnight/generator_sidecars.go:222-301`,
`cli/internal/overnight/generator_sidecars.go:424-459`). REDUCE calls that
aggregator after `findings-router` and before `inject-refresh`, inside the
checkpoint staging tree (`cli/internal/overnight/reduce.go:115-145`,
`cli/internal/overnight/reduce.go:242-252`, `cli/internal/overnight/reduce.go:360-374`).

`mine.Run` now accepts a context and its git/gocyclo shellouts use
`exec.CommandContext`, so the current generator respects cancellation
(`cli/internal/mine/mine.go:219-280`, `cli/internal/mine/mine.go:314-353`,
`cli/internal/mine/mine.go:389-459`, `cli/internal/mine/mine.go:502-573`).

### Concerns

No blocking defect was found in the merged PR. The residual issue is metrics
accuracy, not queue safety: the aggregator's dedup state scans all next-work
entries and keys (`cli/internal/overnight/generator_sidecars.go:237-301`,
`cli/internal/overnight/generator_sidecars.go:368-421`), but the sidecar
builder still asks `mine.LoadExistingMineIDs`, which only marks unconsumed
`source_epic:"compile-mine"` rows as duplicates (`cli/internal/mine/work_items.go:92-123`).
As generator-aggregator rows accumulate, `GeneratorDuplicateRate` can undercount
duplicates even though REDUCE will still suppress repeated queue writes.

## Plan Vs Delivered

Planned in RFC 0001 Proposal 1
(`docs/rfcs/0001-finding-generator-parallelism.md`):

- start with one existing generator adapter, `mine.Run`
- write per-run sidecar JSON
- keep workers away from direct `next-work.jsonl` writes
- merge sidecars through a deterministic aggregator
- dedup and reconcile candidates before appending one queue row
- keep `/evolve` serial
- update the Dream contract and skill text to permit only bounded in-process read-side fanout

Delivered:

- `mine.Run` became a context-aware internal entrypoint and `runMineFindingGenerator`
  uses it with `EmitWorkItems:false`
- `FindingGeneratorSidecar` and `FindingGeneratorCandidate` define the sidecar
  envelope and queue-safe candidate payload
- `writeFindingGeneratorSidecar` writes atomic per-generator JSON files
- `AggregateFindingGeneratorSidecars` performs single-writer queue aggregation
- REDUCE includes `generator-aggregator` as a serialized checkpoint stage
- INGEST and REDUCE summaries include generator candidate, duplicate, sidecar,
  and routing counters
- Dream shared/Codex skills and the Dream run contract now allow only bounded
  in-process read-side generators, with REDUCE as the one durable queue writer

Adjusted / deferred scope:

- No external watchlist generator was added; that remains RFC Proposal 2.
- No parallel evaluator/corroborator lane was added; that remains RFC Proposal 3.
- No next-work schema promotion for `status`, `requires`, or `goal_weight` was
  required for Proposal 1.

## Prediction Accuracy

| Prior Finding | Prediction | Result |
|---|---|---|
| `f-2026-04-14-001` | Production command refactors can miss paired tests | HIT avoided; no `cli/cmd/ao/` production command file changed, and internal code changes landed with direct tests |
| `f-2026-04-14-002` | Closed work can cite non-durable seed paths | HIT avoided; closure evidence is PR #155, merge commit `08e3a38e`, CI, and changed-file tests |
| RFC 0001 risk | Fanout can create queue write races | HIT avoided; generators emit sidecars and only REDUCE writes next-work |

## Four-Surface Closure

| Surface | Result | Evidence |
|---|---|---|
| Code | PASS | `cli/internal/overnight` and `cli/internal/mine` implement sidecars, context-aware generation, aggregation, and staged REDUCE writes |
| Documentation | PASS | `docs/contracts/dream-run-contract.md:241-249`, `skills/dream/SKILL.md:50-58`, and `skills-codex/dream/SKILL.md:14-22` document the concurrency boundary |
| Examples | PASS | No CLI command examples changed; generated summaries and tests cover the new report fields |
| Proof | PASS | Targeted local tests, race test, build, schema/contract validators, Codex artifact validators, and green PR CI |

## Closure Integrity

| Check | Result | Details |
|---|---|---|
| Evidence precedence | PASS | Closure is commit-backed by merge commit `08e3a38e` and PR #155 |
| Phantom bead detection | SKIP | No bead or epic ID was supplied for this post-mortem |
| Orphaned children | SKIP | No bead hierarchy was in scope |
| Multi-wave regression | PASS | The merge commit contains one cohesive PR; no later wave removed the sidecar aggregator |
| Stretch goals | PASS | Proposals 2 and 3 remain explicit future work in RFC 0001 and next-work |

## Metadata Verification

Mechanical checks:

- 14 changed files in `1bd2f082..08e3a38e`; all 14 exist on disk.
- Changed docs and skill files still resolve in CI; `markdownlint` and the doc/contract parity gates passed before merge.
- No ASCII box diagrams were introduced in the changed files.
- PR #155 status is `MERGED`, not draft, with merge commit `08e3a38e`.

Metadata warnings:

- The post-mortem branch inherited an unrelated dirty `docs/INDEX.md` change from
  the local worktree environment. It is not staged, not part of this report, and
  must not be treated as evidence for PR #155.

## Test Pyramid Assessment

| Scope | Planned | Actual | Gaps | Action |
|---|---|---|---|---|
| `mine.Run` context behavior | L1/L2 | L1: `TestRun_ContextCanceledSkipsSourcesAndWrites`; L2-ish command paths via `go test ./cmd/ao ./internal/mine ./internal/overnight` | none | keep |
| INGEST generator sidecar | L1/L2/BF4 | L2: `TestRunIngest_FindingGeneratorEmitsRealSidecarCandidates`; BF4: `TestRunFindingGenerator_StalledGeneratorWritesSoftFailSidecar` | future generators need the same stall contract | next-work item 1 |
| REDUCE single writer | L1/L2 | L2: `TestAggregateFindingGeneratorSidecarsAppendsOneDedupedBatch`; checkpoint proof: `TestRunReduce_AggregatesGeneratorSidecarsIntoStagedNextWork` | duplicate-rate metric can undercount already-aggregated rows | next-work item 2 |

## Learnings

### What Went Well

- The RFC's "read side fanout, single writer" framing mapped cleanly into
  Dream's existing checkpoint model.
- Tests were written at the interaction boundary: sidecar creation, stalled
  generator soft-fail behavior, deduped aggregation, and staged queue writes.

### What Was Hard

- The queue safety proof and metric-yield proof are different concerns. Queue
  dedup was correctly centralized in REDUCE, while INGEST's duplicate-rate
  accounting still depends on the older `compile-mine` ID scanner.

### Do Differently Next Time

- Before adding a second generator, write a short generator authoring contract
  that requires context-aware IO, sidecar-only output, stable dedup keys, and a
  stall test.
- Treat generator metrics as first-class fitness signals only after their dedup
  accounting matches the aggregator's real queue policy.

### Patterns to Reuse

- Let parallel workers produce replaceable sidecars; let one reducer own the
  durable queue write.
- Test the failure mode where a worker stalls even if the current production
  worker is context-aware.

### Anti-Patterns to Avoid

- Letting each generator append to `next-work.jsonl` directly.
- Assuming a timeout wrapper proves safety for future plugins unless the plugin
  contract requires context-aware IO.

### Footgun Entries

| Footgun | Impact | Prevention |
|---|---|---|
| In-process generators can ignore context cancellation | The coordinator can write a soft-fail sidecar, but the ignored goroutine can live until process exit | Require context-aware IO and a stall test before registering a generator |
| `gh pr merge` can merge remotely and still exit non-zero when local `main` is checked out in another worktree | A caller might retry merge commands against an already-merged PR | Verify `gh pr view --json state,mergeCommit,mergedAt` before taking any remedial merge action |
| Worktree-local dirty docs can appear on a clean post-mortem branch | Staging broadly would smuggle unrelated doc work into the post-mortem branch | Stage only explicit post-mortem artifact paths |

## Knowledge Lifecycle

### Backlog Processing (Phase 3)

- Scanned: 2 new learnings from this post-mortem
- Merged: 0 duplicates
- Flagged stale: 0
- Score range: 7-7

### Activation (Phase 4)

- Promoted to MEMORY.md: 2
- Constraints compiled: 0
- Next-work items fed: 2

### Retirement (Phase 5)

- Archived: 0 learnings
- MEMORY.md references needing review: 0

## Proactive Improvement Agenda

| # | Area | Improvement | Priority | Horizon | Effort | Evidence |
|---|---|---|---|---|---|---|
| 1 | execution | Document the Dream generator authoring contract | P1 | next-cycle | S | Stalled-generator proof exists, but future generator authors need an explicit checklist before registering new adapters |
| 2 | repo | Align mine sidecar duplicate metrics with aggregator dedup state | P2 | later | S | Queue dedup is correct, but `GeneratorDuplicateRate` can undercount duplicates from `dream-generator-aggregator` rows |

## Prior Findings Resolution Tracking

| Metric | Value |
|---|---|
| Backlog entries analyzed | 67 |
| Prior findings total | 228 |
| Resolved findings | 176 |
| Unresolved findings | 52 |
| Resolution rate | 77.19% |

| Source Epic | Findings | Resolved | Unresolved | Resolution Rate |
|---|---:|---:|---:|---:|
| `rfc-finding-generator-parallelism` | 3 | 1 | 2 | 33.33% |
| `compile-mine` | 9 | 0 | 9 | 0% |
| `dream-findings-router` | 15 | 13 | 2 | 86.67% |
| `2026-04-19-rpi-dag-hardening` | 5 | 1 | 4 | 20% |

## Command-Surface Parity Checklist

| Command File | Run-path Covered by Test? | Evidence | Intentionally Uncovered? | Reason |
|---|---|---|---|---|
| N/A | yes | No `cli/cmd/ao/` command files changed in PR #155 | yes | Implementation was under `cli/internal/mine` and `cli/internal/overnight` |

## BF Assessment

| Level | Exist? | Bugs | Action |
|---|---|---:|---|
| BF1 | n | 0 | Not needed for this slice; deterministic dedup tests cover the data-transform path |
| BF4 | y | 0 | Keep `TestRunFindingGenerator_StalledGeneratorWritesSoftFailSidecar` as the generator stall guard |

## Next Work

| # | Title | Type | Severity | Source | Target Repo |
|---|---|---|---|---|---|
| 1 | Document the Dream generator authoring contract | process-improvement | medium | retro-learning | * |
| 2 | Align mine sidecar duplicate metrics with aggregator dedup state | tech-debt | low | retro-learning | agentops |

### Recommended Next /rpi

```bash
$rpi "Document the Dream generator authoring contract"
```

## Status

- [x] CLOSED - PR #155 merged, learnings captured, next-work fed
- [x] FOLLOW-UP - Two small follow-up items added to `.agents/rpi/next-work.jsonl`
