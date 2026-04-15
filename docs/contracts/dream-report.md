# Dream Report Contract

`summary.json` and `summary.md` are the canonical Dream report artifacts.

All future Dream layers must consume this contract instead of inventing parallel formats.

## Required Files

- `summary.json`
- `summary.md`

## Required JSON Fields

| Field | Type | Meaning |
|------|------|---------|
| `schema_version` | integer | Report contract version (v1 = 1, v2 = 2; see Schema v2 section) |
| `mode` | string | Run mode identifier |
| `run_id` | string | Stable run identifier |
| `goal` | string | Optional goal carried into the run |
| `repo_root` | string | Absolute repo path |
| `output_dir` | string | Absolute artifact directory |
| `status` | string | `planned`, `dry-run`, `done`, or `failed` |
| `dry_run` | boolean | Whether execution mutated state |
| `started_at` | string | RFC3339 UTC start time |
| `finished_at` | string | RFC3339 UTC finish time |
| `duration` | string | Human-readable duration |
| `runtime` | object | Runtime/process metadata |
| `steps` | array | Ordered step outcomes |
| `artifacts` | object | Named artifact paths |
| `recommended` | array | Recommended follow-up commands |
| `next_action` | string | Single highest-signal first move |

## Runtime Object

Required fields:

- `keep_awake`
- `keep_awake_mode`
- `requested_timeout`
- `effective_timeout`
- `lock_path`
- `log_path`
- `process_contract_doc`
- `report_contract_doc`

Optional fields:

- `keep_awake_note`

## Step Object

Each step entry must include:

- `name`
- `status`

Optional fields:

- `command`
- `artifact`
- `note`

## Optional Embedded Data

These may be embedded when available:

- `close_loop`
- `metrics_health`
- `retrieval_live`
- `briefing`
- `morning_packets`
- `council`
- `dreamscape`
- `degraded`

The JSON contract must tolerate their absence when the corresponding step soft-fails.

## Markdown Contract

`summary.md` is the human-readable rendering of the same run.

It must answer four questions fast:

1. What state did I wake up to?
2. What ran overnight?
3. What degraded or failed?
4. What should I do first?

Minimum sections:

- status header
- DreamScape, when available
- terrain or health section
- what ran
- morning packets, when available
- Dream Council, when available
- degraded, when applicable
- first move
- recommended commands

## Downstream Consumers

Future consumers of this contract:

- `ao overnight report`
- Dream Setup validation
- Dream Council synthesis
- DreamScape terrain visualization
- morning queue/bead handoff layers

Those layers must extend the report additively instead of replacing these fields.

## Schema v2

Schema v2 (2026-04-09) introduces the compounding iteration loop. It is **additive only**: all v1 fields remain in place with identical semantics. v1 readers that tolerate unknown fields continue to work unchanged. This backward compatibility is verified by `TestRunOvernight_SchemaV2IsV1BackwardCompatible` in `cli/cmd/ao/overnight_test.go`.

### Bump Policy

- `schema_version` is bumped to `2` when any of the new v2 fields are present.
- New fields must be optional and ignored by v1 consumers.
- Never rename, retype, or remove a v1 field.

### New Optional Top-Level Fields

| Field | Type | Example | Meaning |
|-------|------|---------|---------|
| `iterations` | array of IterationSummary | `[{...}, {...}]` | Per-iteration sub-summaries, ordered by `index` |
| `fitness_delta` | object | `{"composite": 0.042, "retrieval_bench": 0.018}` | Composite delta across all iterations (final minus initial) |
| `plateau_reason` | string | `"2 consecutive iterations below epsilon 0.01"` | Present iff halted on plateau |
| `regression_reason` | string | `"retrieval_bench dropped 0.08 below floor 0.05"` | Present iff halted on regression |
| `morning_packets` | array of MorningPacket | `[{...}, {...}]` | Ranked executable morning work packets emitted from Dream's queue/fallback synthesis |

### MorningPacket Shape

| Field | Type | Example | Meaning |
|-------|------|---------|---------|
| `id` | string | `"dream-0d9fa4d5f2e0d7a1"` | Stable packet ID for queue and bead dedupe |
| `rank` | int | `1` | 1-based priority rank within this Dream run |
| `title` | string | `"Repair Dream retrieval coverage"` | Human title for the packet |
| `type` | string | `"bug"` | Queue/work type used for morning routing |
| `severity` | string | `"high"` | Priority signal reused from next-work semantics |
| `confidence` | string | `"high"` | Dream's confidence in the packet |
| `why_now` | string | `"Dream ranked this..."` | Short reason this packet surfaced now |
| `evidence` | array of strings | `["retrieval coverage=0.33"]` | Concrete supporting evidence |
| `target_files` | array of strings | `["cli/cmd/ao/overnight.go"]` | Candidate files to inspect first |
| `likely_tests` | array of strings | `["cli/cmd/ao/overnight_test.go"]` | Likely tests to run or extend |
| `morning_command` | string | `"ao rpi phased \"Repair Dream retrieval coverage\""` | Exact morning execution command |
| `bead_id` | string | `"na-1234"` | Linked bead when bd sync succeeds |
| `artifact_path` | string | `".../morning-packets/01-....json"` | Per-packet artifact path |

### IterationSummary Shape

| Field | Type | Example | Meaning |
|-------|------|---------|---------|
| `id` | string | `"run-1775776630-iter-1"` | IterationID |
| `index` | int | `1` | 1-based iteration index |
| `started_at` | string (RFC3339) | `"2026-04-09T02:15:03Z"` | Iteration start |
| `finished_at` | string (RFC3339) | `"2026-04-09T02:27:41Z"` | Iteration finish |
| `duration` | string | `"12m38s"` | Human-readable duration |
| `status` | string | `"done"` | One of `done`, `degraded`, `rolled-back-pre-commit`, `halted-on-regression-post-commit`, `failed`. See the Status Precedence Truth Table below. |
| `ingest` | object | `{"learnings_added": 14}` | INGEST stage sub-summary |
| `reduce` | object | `{"deduped": 3, "pruned": 1}` | REDUCE stage sub-summary |
| `measure` | object | `{"retrieval_bench": {...}}` | MEASURE stage sub-summary |
| `fitness_before` | object | `{"composite": 0.612}` | Fitness map before this iteration |
| `fitness_after` | object | `{"composite": 0.631}` | Fitness map after this iteration |
| `fitness_delta` | float | `0.019` | Numeric composite delta for this iteration |
| `degraded` | array of strings | `["retrieval_bench"]` | Soft-failed stages |
| `error` | string (optional) | `"checkpoint integrity failure"` | Present on `failed`, `rolled-back-pre-commit`, or (rare) `halted-on-regression-post-commit` |

### Status Precedence Truth Table

The `IterationSummary.Status` field uses an exhaustive enum. Each value has distinct semantics that downstream consumers (morning report, rehydration logic, invariant tests) depend on.

| Status | Commit succeeded? | Corpus on disk? | Rehydration baseline? | Typical trigger |
|---|---|---|---|---|
| `done` | yes | yes | yes | Happy path — all stages succeeded, fitness delta non-regressing |
| `degraded` | no | no (unchanged) | no | MEASURE failed pre-commit; checkpoint rolled back before the live corpus changed |
| `rolled-back-pre-commit` | no | no (unchanged) | no | REDUCE failed before commit; checkpoint was rolled back |
| `halted-on-regression-post-commit` | yes | yes | yes | Post-commit regression check fired; corpus is in live tree but loop halted |
| `halted-on-regression-pre-commit` | no | no (unchanged) | no | Fitness regression or plateau halt fired before commit; checkpoint was rolled back |
| `failed` | partial/no | indeterminate | no | Unrecoverable error in INGEST/CHECKPOINT/COMMIT; RecoverFromCrash handles partial state on restart |

**Invariant:** an iteration with `status ∈ {done, halted-on-regression-post-commit}` is a valid rehydration baseline for `prevSnapshot` on resume, because the compounded corpus is on disk. Statuses `degraded`, `rolled-back-pre-commit`, `halted-on-regression-pre-commit`, and `failed` are NOT valid baselines — rehydration walks past them.

**Companion marker file:** when an iteration has `status = halted-on-regression-post-commit`, Dream writes a sentinel file `committed-but-flagged.iter-<N>.marker` in the same `<outputDir>/<runID>/iterations/` directory. Operators can find flagged iterations via directory listing without parsing every iter-<N>.json.

**Historical note:** prior to Micro-epic 3 (2026-04-10), `rolled-back-pre-commit` and `halted-on-regression-post-commit` were both represented by a single `"rolled-back"` string. This was a semantic lie: post-commit halts claimed rollback while the corpus stayed committed. The fix is Micro-epic 2 commit d20e21bd (persistence) plus Micro-epic 3 (enum split).

### Per-Iteration Persistence (Micro-epic 2, 2026-04-10)

Each `IterationSummary` is also written to disk atomically as soon as the iteration finishes, under:

```
<outputDir>/<runID>/iterations/iter-<N>.json
```

Writes use `os.CreateTemp` → `f.Sync` → `os.Rename` → directory `fsync` so a crash during a write can never leave a half-written file. On resume, `RunLoop` rehydrates prior iterations from this directory and the morning report's `summary.iterations` array is populated from both in-memory state AND the persisted files on disk — a crashed run that previously returned an empty iteration list now surfaces every completed iteration's history. Pre–Micro-epic 2 code never persisted these files, so "resume after upgrade" has no legacy files to migrate.

### Backward Compatibility Guarantee

v1 consumers parsing new v2 output must not error on unknown fields. Most JSON parsers (Go's `encoding/json` with default behavior, Python's `json.loads`, `jq`, browsers) ignore unknown fields by default. Parsers that explicitly opt into strict mode (`DisallowUnknownFields()` in Go, Pydantic `extra=forbid`) **will break** on v2 output and must be updated to tolerate the additive fields listed above.
