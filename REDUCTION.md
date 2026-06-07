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
| KEEP | 467 |
| RELOCATE | 110 |
| ARCHIVE | 0 |
| RESEARCH | 39 |
| Total | 616 |

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

## Extracted Orchestration Select Surface

The `ao orchestrate select` production selector moved behind the `mto-fleet`
route. AO keeps the public command wrapper and backend-selection contract in
place, while the exec-backed NTM probe adapter and trace renderer now live
under `cli/internal/adapters/mto/orchestrationselect`.

## Extracted Worktree Config Surface

The root pre-run Git worktree repair moved behind the `mto-fleet` adapter
boundary. AO still runs the startup repair for local safety, but the Git
environment sanitizer and shared `core.worktree` migration now live under
`cli/internal/adapters/worktreeconfig` instead of `cli/cmd/ao`.

## Extracted Agents Lint Adapter Surface

The `.agents/` write-surface lint script runner moved behind the `mto-fleet`
adapter boundary. AO keeps the `ao agents lint` public command wrapper and
exit-code contract in place, while script invocation, stdout/stderr forwarding,
JSON flag forwarding, and non-zero exit mapping now live under
`cli/internal/adapters/agentslint`.

## Extracted Agents Inspect Adapter

The `.agents/` write-surface inventory renderer moved behind the `mto-fleet`
adapter boundary. AO keeps the `ao agents inspect` public command wrapper,
default repo-root path resolution, and flags in place, while contract loading,
inventory construction, and text/JSON rendering now live under
`cli/internal/adapters/agentsinspect`. Shared `.agents` allowlist parsing and
active skill discovery remain under `cli/internal/adapters/agentsurface`.

## Extracted Turn Verify Adapter Surface

The Evidenced-Turn verification evaluator moved behind a local assurance
adapter boundary. AO keeps the `ao turn verify` public command wrapper and
verdict contract in place, while input decoding, provenance ledger loading,
trace-graph orphan checks, predicate evaluation, and verdict rendering now live
under `cli/internal/adapters/turnverify`.

## Extracted MCP Transport Adapter Surface

The MCP JSON-RPC stdio transport moved behind the `mto-fleet` adapter boundary.
AO keeps the `ao mcp serve` public command wrapper, curated tool descriptors,
holdout-denial policy, and real executor in place, while newline-delimited
JSON-RPC serving, dispatch, initialize response handling, tools/list response
handling, tools/call response handling, and protocol error shaping now live
under `cli/internal/adapters/mcptransport`.

## Extracted Session Spawn Adapter Surface

The `ao session spawn` template launcher moved behind the `mto-fleet` adapter
boundary. AO keeps the public command wrapper and flags in place, while TOML
template loading, required-field validation, variable expansion, hostname
sanitization, init-step execution, tmux session creation, dry-run rendering, and
no-tmux behavior now live under `cli/internal/adapters/sessionspawn`.

## Extracted MCP Surface Adapter

The curated `ao mcp serve` tool catalog, holdout-denial policy, print-tools JSON
renderer, shell-backed executor, and transport wiring moved behind the
`mto-fleet` adapter boundary. AO keeps the public `ao mcp serve` wrapper and
flags in place, while MCP surface behavior now lives under
`cli/internal/adapters/mcpsurface`.

## Extracted Agents Doctor Adapter

The combined `.agents/` inspect/lint/orphan report moved behind the `mto-fleet`
adapter boundary. AO keeps the public `ao agents doctor` wrapper and flags in
place, while the report builder, lint subprocess contract, orphan skill scan,
undocumented-dir scan, strict-mode error, and text/JSON rendering now live under
`cli/internal/adapters/agentsdoctor`. Shared `.agents` surface parsing now lives
under `cli/internal/adapters/agentsurface`.

## Extracted Agents Reference Scanner

The `.agents/` production-reference scanner and shell-parity contract tests
moved behind the `mto-fleet` adapter boundary. AO keeps the public `ao agents`
wrappers focused on command registration and flags, while the read-side
write-surface scanner now lives under
`cli/internal/adapters/agentsreferences`.

## Extracted Codex Runtime Adapter

The Codex runtime detection, hook-capability probing, session-index lookup,
history fallback transcript synthesis, and archived transcript discovery moved
behind the `vendor-image-adapter` boundary. AO keeps the public `ao codex`
command/state machine in `cli/cmd/ao/codex.go`, while runtime-specific
discovery now lives under
`cli/internal/adapters/vendorimage/codexruntime`.

## Reversibility

This is a plan artifact. The next beads may move or remove code, but each move
must cite the row(s) from `docs/reduction/ao-file-buckets.tsv`, update this
document if a bucket changes, and pass the normal generated-artifact and CI
gates before merge.
