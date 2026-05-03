---
id: plan-2026-05-03-go-architecture-ai-maintainability
type: plan
date: 2026-05-03
source: ".agents/research/2026-05-03-go-architecture-ai-maintainability.md"
epic_id: soc-lb4e
---
# Plan: Go Architecture AI Maintainability

## Context And Applied Findings

Applied findings:
- `.agents/patterns/2026-05-01-state-path-resolver.md`
- `.agents/research/2026-04-24-go-cli-quality-gap-analysis.md`
- `.agents/research/2026-04-24-go-cli-local-code-explore.md`

The current run is full-complexity discovery but standard implementation scope: fix mechanically verified Go consistency issues without renaming commands, changing user-facing CLI contracts, or doing a package-wide Cobra rewrite.

## Baseline Audit

- `PROGRAM.md` validates with `ao autodev validate --json --file PROGRAM.md`.
- `cd cli && go test ./...` fails at baseline due macOS `/var` versus `/private/var` path spelling in daemon wiki containment plus strict path string tests.
- `gofmt -l` over `cli` Go files is non-empty.
- `bash scripts/check-paths-resolver-coverage.sh` reports `total=145`.
- `gocyclo -over 15 cli/cmd/ao cli/internal` shows production functions under the current fail threshold of 25.

## Issues

### soc-lb4e.2 - Fix macOS path canonicalization in Go repo containment

Files:
- `cli/internal/daemon/wiki_jobs.go`
- `cli/internal/daemon/wiki_jobs_test.go`
- `cli/internal/paths/paths_test.go`
- `cli/internal/lifecycle/repo_readiness_test.go`

Acceptance:
- Containment accepts inside-root paths when symlink spellings differ.
- Traversal and outside-root paths still fail.
- Focused daemon, paths, and lifecycle tests pass.

### soc-lb4e.3 - Route overnight Go state paths through canonical resolver

Files:
- `cli/internal/overnight/findings_router.go`
- `cli/internal/overnight/warn_only_budget.go`
- `cli/internal/overnight/*_test.go`

Acceptance:
- `RouteFindings` and warn-only budget path helpers honor `AO_AGENTS_DIR`, `AO_RPI_DIR`, and `AO_FINDINGS_DIR`.
- Existing default tests still pass.
- State-path resolver coverage count decreases.

### soc-lb4e.4 - Normalize Go formatting drift under cli

Files:
- Go files reported by `gofmt -l` under `cli/`.

Acceptance:
- `gofmt -l` over `cli` Go files returns empty.
- Formatting-only diffs do not change behavior.

## Execution Order

Wave 1: `soc-lb4e.2` and `soc-lb4e.3` can be implemented together because they touch disjoint packages.

Wave 2: `soc-lb4e.4` runs after semantic edits so gofmt normalizes both existing drift and new code.

## Verification Commands

- `cd cli && go test ./internal/daemon ./internal/paths ./internal/lifecycle ./internal/overnight`
- `gofmt -l $(find cli -name '*.go' -not -path '*/vendor/*')`
- `bash scripts/check-paths-resolver-coverage.sh`
- `cd cli && go test ./...`
- `cd cli && make build`
- `cd cli && make test` if the focused suite is green and time permits.

## Boundaries

No command renames, no hidden/deprecated surface changes, no broad `cli/cmd/ao` extraction, and no unrelated doc rewrites.
