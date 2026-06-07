# Migration Tracker: os.Getwd() → resolveProjectDir()

**Status:** In Progress (5/N migrated)
**Started:** 2026-02-26 (Cycle 3)
**Pattern:** cli/cmd/ao/projectdir.go — resolveProjectDir()

## Why

Production code calls `os.Getwd()` directly, forcing tests to use `os.Chdir()` (process-global). This blocks `t.Parallel()` and makes the 46s test suite grow linearly with new tests.

`resolveProjectDir()` reads `testProjectDir` when set, allowing tests to override the directory without `os.Chdir`.

## Migrated (5)

| File | Migrated In |
|------|-------------|
| badge.go | Cycle 3 (0a145dd) |
| metrics_baseline.go | Cycle 3 (0a145dd) |
| metrics_cite.go | Cycle 3 (0a145dd) |
| metrics_report.go | Cycle 3 (0a145dd) |
| trace.go | Cycle 3 (0a145dd) |

## Remaining

| File | os.Getwd() calls | Priority | Notes |
|------|:-:|----------|-------|
| doctor.go | 4 | P1 | Good test coverage; high value target |
| store.go | 4 | P1 | Good test coverage; high value target |
| forge.go | 1 | P1 | Good test coverage; high value target |
| gate.go | 4 | P2 | Moderate coverage; multiple call sites |
| maturity.go | 5 | P2 | Moderate coverage; most calls in file |
| pool.go | 6 | P2 | Moderate coverage; highest call count |
| temper.go | 3 | P2 | Moderate coverage |
| task_sync.go | 3 | P2 | Moderate coverage |
| rpi_phased.go | 3 | P2 | Complex; includes os.Chdir workaround comment |
| feedback.go | 2 | P2 | Moderate coverage |
| feedback_loop.go | 2 | P2 | Moderate coverage |
| context.go | 2 | P2 | Moderate coverage |
| plans.go | 2 | P2 | Mixed: one `err`, one `_` pattern |
| batch_forge.go | 1 | P3 | Low coverage |
| batch_promote.go | 1 | P3 | Low coverage |
| config.go | 1 | P3 | Uses `_` error pattern |
| context_assemble.go | 1 | P3 | Low coverage |
| contradict.go | 1 | P3 | Low coverage |
| dedup.go | 1 | P3 | Low coverage |
| extract.go | 1 | P3 | Low coverage |
| fire.go | 1 | P3 | Low coverage |
| flywheel_close_loop.go | 1 | P3 | Low coverage |
| goals_init.go | 1 | P3 | Uses `dir` variable name instead of `cwd` |
| index.go | 1 | P3 | Low coverage |
| init.go | 1 | P3 | Low coverage |
| inject.go | 1 | P3 | Low coverage |
| lookup.go | 1 | P3 | Low coverage |
| memory.go | 1 | P3 | Low coverage |
| metrics_cite_report.go | 1 | P3 | Uses `baseDir` variable name instead of `cwd` |
| metrics_flywheel.go | 1 | P3 | Low coverage |
| metrics_health.go | 1 | P3 | Low coverage |
| metrics_nudge.go | 1 | P3 | Low coverage |
| mind.go | 1 | P3 | Low coverage |
| notebook.go | 1 | P3 | Low coverage |
| pool_ingest.go | 1 | P3 | Low coverage |
| quickstart.go | 1 | P3 | Low coverage |
| ratchet_check.go | 1 | P3 | Low coverage |
| ratchet_find.go | 1 | P3 | Low coverage |
| ratchet_migrate.go | 1 | P3 | Low coverage |
| ratchet_next.go | 1 | P3 | Low coverage |
| ratchet_promote.go | 1 | P3 | Low coverage |
| ratchet_record.go | 1 | P3 | Low coverage |
| ratchet_skip.go | 1 | P3 | Low coverage |
| ratchet_spec.go | 1 | P3 | Low coverage |
| ratchet_status.go | 1 | P3 | Low coverage |
| ratchet_trace.go | 1 | P3 | Low coverage |
| ratchet_validate.go | 1 | P3 | Low coverage |
| rpi_cancel.go | 1 | P3 | Low coverage |
| rpi_cleanup.go | 1 | P3 | Low coverage |
| rpi_loop.go | 1 | P3 | Low coverage |
| rpi_parallel.go | 1 | P3 | Uses `baseCwd` variable name instead of `cwd` |
| rpi_status.go | 1 | P3 | Low coverage |
| rpi_verify.go | 1 | P3 | Low coverage |
| search.go | 1 | P3 | Low coverage |
| session_close.go | 1 | P3 | Low coverage |
| status.go | 1 | P3 | Low coverage |
| vibe_check.go | 1 | P3 | Low coverage |
| worktree.go | 1 | P3 | Low coverage |

## Migration Approach

For each file, the standard migration is:

1. Replace `cwd, err := os.Getwd()` with `cwd, err := resolveProjectDir()`
2. For variant variable names (e.g. `dir`, `baseDir`, `baseCwd`, `origDir`), replace the full `os.Getwd()` call while preserving the variable name, e.g. `dir, err := resolveProjectDir()`
3. Remove the `os` import if no other `os.*` symbols are used in the file — check before removing
4. Update corresponding `_test.go` to set `testProjectDir` in a `t.Cleanup` instead of calling `os.Chdir`, where applicable

### Special cases

- **config.go** — uses `cwd, _ := os.Getwd()` (ignores error). Keep `_` pattern: `cwd, _ := resolveProjectDir()`
- **plans.go** — has both `cwd, err := os.Getwd()` and `cwd, _ := os.Getwd()` patterns; migrate each independently
- **rpi_phased.go** — line 95 has a comment explicitly mentioning `os.Getwd()` to explain an `os.Chdir` workaround; after migration this comment and the `os.Chdir` block should be re-evaluated

## Acceptance Criteria

- All production `os.Getwd()` calls in `cli/cmd/ao/*.go` (excluding `projectdir.go` itself) are replaced
- `grep -rn "os.Getwd" cli/cmd/ao/*.go | grep -v _test.go | grep -v "projectdir.go"` returns empty
- Tests can set `testProjectDir` in `t.Cleanup` instead of `os.Chdir`, enabling `t.Parallel()` across those tests
