# AgentOps Reduction Plan

This plan categorizes the `ao` command implementation for the lean image
distillation. It does not remove code by itself; it defines the reversible
KEEP / RELOCATE / ARCHIVE / RESEARCH decision surface for the next beads.

Source boundary: `/Users/bo/dev/control-plane/AO-MTO-BOUNDARY.md`.
AgentOps-side relocation gate: [`docs/contracts/ao-mto-seam.md`](docs/contracts/ao-mto-seam.md).

## Boundary Rule

AgentOps (`ao`) must stay self-contained: one image can run the local loop,
local Themis, local corpus, and local flywheel without a fleet. MTO owns the
outer factory: fleet dispatch, leases, fleet Themis/quorum, fleet corpus
aggregation, and cross-instance scheduling.

## Buckets

| Bucket | Meaning | Next action |
|---|---|---|
| KEEP | Belongs in the lean AO image: local loop, mind, local Themis, local flywheel, corpus, evidence, operator ergonomics, and vendor image adapters required for self-contained operation. | Keep in `ao`; slim only by local refactor. |
| RELOCATE | Belongs at the MTO/factory or image-adapter boundary: outer orchestration, runtime supervision, CI/fleet coordination, sidecars, schedulers, or worker dispatch. | Move behind an MTO/factory seam or image-specific adapter. |
| ARCHIVE | Legacy, deprecated, retired, or compatibility-only code that should not ship in the lean image once callers are migrated. | Remove from image; keep recoverable through git history. |
| RESEARCH | Ownership is unclear from filename/import shape. | Inspect before moving or cutting. |

## Per-File Inventory

The complete per-file table is generated at
[`docs/reduction/ao-file-buckets.tsv`](docs/reduction/ao-file-buckets.tsv).
It covers every current `cli/cmd/ao/*.go` file and records:

- file path
- bucket
- bucket rationale
- import dependency clusters

Current inventory count:

| Bucket | Files |
|---|---:|
| KEEP | 458 |
| RELOCATE | 132 |
| ARCHIVE | 0 |
| RESEARCH | 39 |
| Total | 629 |

Validation command:

```bash
test "$(( $(wc -l < docs/reduction/ao-file-buckets.tsv) - 1 ))" -eq "$(find cli/cmd/ao -maxdepth 1 -name '*.go' | wc -l)"
```

## Import-Dependency Shape

All files in `cli/cmd/ao/` compile into the same Go package (`main`), so they
do not import each other directly. The dependency graph that matters for this
reduction is the external package cluster graph captured per file in the TSV:

- `internal/*` clusters are AO-local domain ports and adapters.
- `github.com/...`, `gopkg.in/...`, and other third-party clusters are external
  dependencies that should be minimized in the lean image.
- Files with no non-stdlib imports are usually command glue, tests, or pure
  helpers and are cheaper to keep unless their command surface belongs outside
  AO.

## Initial Decisions

KEEP is the default for local AO capabilities: `beads`, `claim`, `ready`,
`close`, `tick`, `gate`, `goals`, `loop`, `corpus`, `forge`, `mine`, `inject`,
`compile`, `curate`, `flywheel`, `knowledge`, `findings`, `ratchet`,
`validate`, `session`, `skills`, and supporting operator UX.

RELOCATE is the default for outer orchestration and fleet-like surfaces:
`rpi`, `swarm`, `agent(s)`, `ci`, `cron`, `mcp`, `orchestrate`, `worktree`,
`next_work`, runtime supervision, workers, schedulers, sidecars, and daemon
serving surfaces. These are valuable, but they sit one altitude up from the
lean local image.

The initial ARCHIVE rows were retired in the first reduction slice. Anything
not confidently classified is marked RESEARCH rather than cut.

RELOCATE rows are gated by `docs/contracts/ao-mto-seam.md`. A relocation PR
must route each moved surface to `mto-fleet`, `vendor-image-adapter`, or
`defer-load-bearing`, and must keep the local AO flywheel self-contained.

## Retired Archive Surface

The first reduction slice removed `ao pool migrate-legacy` and its dedicated
helpers. Legacy knowledge captures remain supported through the lean local
flywheel path: `ao pool ingest` already scans `.agents/knowledge/*.md` when no
explicit files are provided, and the legacy-capture e2e now exercises that
direct ingest route.

## Relocated Cron-Fire Surface

The cron-fire scheduling renderer moved behind the `mto-fleet` route. AO keeps
`ao cron self-adjust` as a compatibility shim that emits a structured route
notice, but it no longer renders CronCreate prompts, verifies cron templates,
or writes `.agents/evolve/cron-history.jsonl` from the lean local image.

## Extracted Vendor-Image Bundle Surface

The `ao agent bundle` pure builder moved behind the `vendor-image-adapter`
route. AO keeps the public command wrapper and JSON contract in place for
compatibility, while the runtime-specific bundle construction now lives under
`cli/internal/adapters/vendorimage/agentbundle`.

## Extracted Harness Sync Surface

The `ao harness status` filesystem sync adapter moved behind the
`vendor-image-adapter` route. AO keeps the public command wrapper and JSONL
contract in place, while the skill/skills-codex hash scanner now lives under
`cli/internal/adapters/vendorimage/harnesssync`.

## Extracted CI Status Adapter Surface

The GitHub Actions-backed `CIStatusPort` adapter moved behind the `mto-fleet`
route. AO keeps the `ao ci` public command wrapper and JSON-lines contract in
place, while the production `gh run list` adapter now lives under
`cli/internal/adapters/ci_status`.

## Reversibility

This is a plan artifact. The next beads may move or remove code, but each move
must cite the row(s) from `docs/reduction/ao-file-buckets.tsv`, update this
document if a bucket changes, and pass the normal generated-artifact and CI
gates before merge.
