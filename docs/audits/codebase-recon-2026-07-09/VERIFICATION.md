# Verification ledger — 2026-07-09 codebase recon

## Identity and boundary

- Pinned base: `fbba8af5ace635104775ef18f34fef362ba368ce`
- Base refs at audit start: `HEAD == main == origin/main`
- Dedicated worktree: `/Users/bo/dev/agentops-wt/age-fresh-model-codebase-recon-toqd`
- Bead: `age-fresh-model-codebase-recon-toqd`
- Product source changes: none; this run is audit artifacts only
- External mutation: no push, release, landing, issue creation, or remote state change

## Specialist completion receipts

| Lane | Artifact | Receipt validation | Focused evidence |
|---|---|---|---|
| `codebase-archaeology` | `codebase-archaeology.md` | PASS | 411 lines; pinned SHA; generated-schema parity and six targeted Go packages pass |
| `codebase-audit` | `codebase-audit.md` | PASS with lead correction | 296 lines; eight source findings; raw severity/reachability independently normalized below |
| `codebase-pattern-extraction` | `codebase-pattern-extraction.md` | PASS | 465 lines; eleven patterns; three-or-more-instance evidence and validation plans |
| `codebase-report` | `codebase-report.md` | PASS | 421 lines; 15 required sections; targeted architecture packages pass |

All four strict worker receipts under `.agents/swarm/results/` reported one
owned file, no commit, and no write-scope conflict. Across the four tracked
reports, 366 backticked full-path `file:line` citations were mechanically
checked; none were missing or out of bounds.

## Independent risk disposition

A fresh evidence-review subagent read the audit claims and cited source/tests
without constructing new payloads or editing files. Its result materially
changed the raw producer report:

| ID | Disposition | Normalized severity | Reachability | Independent evidence |
|---|---|---:|---|---|
| A-01 | CONFIRMED | High | Default build | `cli/cmd/ao/tick.go:739-945` uses three default Scanners without `Buffer` or `Err`; council gate is registered in the default CLI. |
| A-02 | ADJUST | Medium | Manual tracked script | `bin/ralph:165-179` calls forbidden `claude -p`, but it has no current callers and is not a default `ao` path. |
| A-03 | ADJUST | Medium | Legacy-only | `cli/cmd/ao/codex.go:1` and its tests are guarded by `//go:build legacy`; the lexical containment gap is source-confirmed. |
| A-04 | ADJUST | Medium | Legacy-only | Host `sh -c` and presence-only receipt validation are source-confirmed; the producer's untagged test command ran zero Codex tests. |
| A-05 | CONFIRMED | Medium | Default fast gate | Changelog read errors become SKIP; blocking SKIP is non-failing. |
| A-06 | CONFIRMED | Medium | Explicit scripts | Two scripts call removed commands and are deliberately grandfathered by the invocation baseline. |
| A-07 | ADJUST | Low | Legacy-only | Output/process-tree gaps exist, but the dispatcher is excluded from default builds. |
| A-08 | CONFIRMED | Low | Default `ao eval chaos` | Built-in council fixtures omit required `context_id`. |

No A-series claim was factually refuted. Four were adjusted for reachability or
severity, and one producer test assertion was refuted: an untagged Go command
cannot prove legacy-tagged Codex tests ran.

## Lead reproductions

### 1. Pinned base fails the canonical fast gate

The baseline command was run before audit artifacts were added:

```text
ao gate check --fast --scope head
68 checks: 66 PASS, 1 FAIL, 1 SKIP
blocking failure: skill.schema
exit=1
```

The direct backing check reproduces the only failure:

```text
$ bash scripts/validate-skill-schema.sh
FAIL goal-design: 'explicit' is not one of ['questions', 'task', 'none']
Total: 59 | Pass: 58 | Fail: 1 | Warn: 0
exit=1
```

The invalid value is at `skills/goal-design/SKILL.md:20-24`. This failure was
present at the pinned base and was not introduced by the audit documents.

### 2. Pinned tip has no bound verdict

`docs/provenance/ledger.jsonl` contains no binding for the full or abbreviated
`fbba8af5…` commit. GitHub Actions run
[29033274027](https://github.com/boshu2/agentops/actions/runs/29033274027)
recorded:

```text
commit fbba8af5ace6 lacks a bound verdict
1 commit(s) in 1-commit range lack proof — report-only, not failing
```

The workflow sets `enforce` false by default and only appends `--enforce` when
the input is true (`.github/workflows/verdict-backstop.yml:18-24`,
`.github/workflows/verdict-backstop.yml:50-64`). The run succeeded with an
empty `ENFORCE` value. This is confirmed governance state, not an inference
from missing local files alone.

### 3. Legacy build-tag correction

The raw audit listed an untagged focused test command as evidence for Codex
dispatcher behavior. The lead reran the exact test selector both ways:

```text
$ go test ./cmd/ao -run 'TestCodexDispatch(...)' -count=1 -v
testing: warning: no tests to run
PASS [no tests to run]

$ go test -tags legacy ./cmd/ao -run 'TestCodexDispatch(...)' -count=1 -v
TestCodexDispatchRejectsPathEscapes PASS
TestCodexDispatchExecutesRequiredCommandsIntoReceipt PASS
TestCodexDispatchRecordsFailingRequiredCommandExitCode PASS
```

This confirms the cited legacy behavior while refuting the claim that the
untagged focused command exercised it.

## Mechanical artifact validation

The following checks are required after synthesis and machine evidence are
complete:

| Check | Result |
|---|---|
| Four worker receipt JSON files are structurally complete | PASS |
| Every source report contains the pinned SHA | PASS |
| Full-path report citations exist and line numbers are in bounds | PASS — 354 citations |
| `findings.json` parses and its summary matches its finding array | PASS — 15 findings: 3 High, 9 Medium, 3 Low |
| Relative Markdown links in the audit directory resolve | PASS; repository doc-release validator checked 3,626 links with 0 broken |
| `git diff --check` | PASS |
| `check-docs-no-retired-tech`, architecture drift, and strict doc-skill references | PASS |
| `make docs-check` | FAIL — pinned-base target calls deleted `scripts/validate-hook-preflight.sh` and exits 127 |
| `tests/docs/validate-doc-release.sh` run directly | PASS |
| `ao gate check --fast --scope staged --json` on all 10 audit artifacts | PASS — 27 selected, 27 passed, 0 warned/failed/skipped/unknown |
| `ao gate check --fast --scope head --json` on the initial immutable audit commit | PASS — 27 selected, 27 passed, 0 warned/failed/skipped/unknown |

The `docs-check` failure is not caused by these artifacts. `Makefile:29-32`
unconditionally calls a path absent from the pinned tree; the legacy pre-push
script already documents that the validator was intentionally removed. This is
promoted as F-15 rather than waived as an audit-introduced failure.

## Evidence limits

- The audit did not run a full release rehearsal, mutate source to demonstrate
  symlink writes, induce OOM/orphan processes, or land/push against the remote.
- Legacy-only defects are retained because the profile is buildable source,
  but they are excluded from default-spine severity.
- The prior 2026-07-02 base object is absent from this rewritten/shallow local
  history; delta classifications use current executable evidence plus the
  prior durable report, not a raw two-base Git diff.
- A passing final docs-scoped gate would prove this artifact change, not erase
  the separately recorded gate-red state of its pinned parent.
