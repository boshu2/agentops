---
id: research-2026-05-03-go-architecture-ai-maintainability
type: research
date: 2026-05-03
backend: inline-policy
---
# Research: Go Architecture AI Maintainability

## Summary

This run revalidated the prior Go CLI quality research against current HEAD. The old high-level findings still apply, but two earlier concrete gaps have already been fixed: `gcBridgeVersion` now uses `CombinedOutput` plus parser validation, and shell completion includes PowerShell through `cmd.OutOrStdout()`.

Current actionable findings:

- Baseline `cd cli && go test ./...` fails on this macOS machine before edits. The failures come from `/var/...` versus `/private/var/...` path spelling in daemon wiki source containment and strict path resolver/readiness test expectations.
- `gofmt -l` reports formatting drift in Go files under `cli/`.
- `bash scripts/check-paths-resolver-coverage.sh` reports `state-path-resolver-coverage total=145`, including production overnight code that reads or writes `.agents/findings`, `.agents/rpi/next-work.jsonl`, and `.agents/overnight/warn-only-budget.json` through direct path construction.

## Prior Knowledge Applied

- `.agents/research/2026-04-24-go-cli-quality-gap-analysis.md`: the CLI gap is consistency at scale, not missing Cobra or missing tests.
- `.agents/research/2026-04-24-go-cli-local-code-explore.md`: `cli/internal/paths` is the intended Go-side path resolver; command-heavy changes require paired tests.
- `.agents/patterns/2026-05-01-state-path-resolver.md`: Go code that reads or writes `.agents` runtime state should use `cli/internal/paths`.

## Evidence

- `cd cli && go test ./...`: FAIL at baseline in `cli/internal/daemon`, `cli/internal/paths`, and `cli/internal/lifecycle`.
- `gocyclo -over 15 cli/cmd/ao cli/internal`: production complexity remains under the current CI fail threshold of 25, with max production functions at 21.
- `gofmt -l $(find cli -name '*.go' -not -path '*/vendor/*')`: non-empty output, so formatting consistency is not currently clean.
- `bash scripts/check-paths-resolver-coverage.sh`: `total=145 by-surface: cli/cmd/ao=49 cli/internal=59 hooks=10 lib=1 scripts=26`.

## Key Files

- `cli/internal/daemon/wiki_jobs.go`: wiki forge source containment rejects real inside-root paths when root and source spell the same macOS temp directory differently.
- `cli/internal/paths/paths.go`: `ResolveFromRoot` returns the git root spelling, which may be symlink-canonicalized on macOS.
- `cli/internal/paths/paths_test.go` and `cli/internal/lifecycle/repo_readiness_test.go`: tests compare raw strings where path equivalence is the real contract.
- `cli/internal/overnight/findings_router.go` and `cli/internal/overnight/warn_only_budget.go`: high-signal production runtime state paths that should route through `cli/internal/paths`.

## Quality Validation

Coverage: `cli/cmd/ao`, `cli/internal`, prior research, active findings/patterns, bd backlog, Go formatting, Go tests, path resolver, and daemon wiki job containment.

Depth ratings:

- Path canonicalization: 3/4. Baseline failures point to exact functions and tests.
- State-path resolver drift: 3/4. Active pattern and warn-only metric identify the target.
- Formatting drift: 4/4. `gofmt -l` is mechanical.
- Full command architecture: 2/4. Prior research is enough for follow-up filing, but this run should not rewrite the full CLI.

## Recommended Scope

Implement three slices:

1. Fix symlink-aware repo-root containment and tests.
2. Route selected overnight runtime state paths through `cli/internal/paths`.
3. Apply `gofmt` to reported Go files.
