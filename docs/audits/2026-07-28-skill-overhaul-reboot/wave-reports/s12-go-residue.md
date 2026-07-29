# S12 — Go CLI audit residue reconciliation

- Bead: `age-skill-overhaul-reboot-sjv7v.12`
- Date: 2026-07-28
- Branch: `claude/sor-s12-go-residue`
- Source audit: `docs/audits/2026-07-24-go-cli-deep-audit.md` (8 findings, one G0/G1/G2 release program)
- Method: reconcile each audit finding against today's `main` (audit ran on an
  older tree; `main` has since merged the eval hardening wave — LAW-0 spawn
  guard, crash-safe 0600 burn-ledger writes — the CLI "residue wave", and skill
  PRs #998–#1006). Verdict per finding: FIXED (already landed), FIXED-HERE,
  NOT-APPLICABLE, or OPEN. Still-present findings that are small and bounded were
  fixed here with a focused test; findings whose honest fix is a cross-package
  refactor are recorded OPEN with a reproduction and a fix sketch.

## Result: 4 fixed here, 1 already landed, 3 open

| # | Finding (severity) | Verdict | Evidence |
|---|---|---|---|
| 1 | eval IDs permit filesystem escape (High) | **FIXED-HERE** | Was still-present: `task_service.go` / `suite_service.go` / `evalsubstrate/modelspec.go` joined caller-supplied YAML IDs into store paths with only a non-empty check, and `adapters/eval/runtime.go` `ReadFile`=`os.ReadFile`, `Root()`=`~/.agents/evals`. Added `evalsubstrate.ValidateID` (rejects separators, `.`/`..`, absolute paths, Windows volume names, control chars; permits colons for `ms:*` IDs) and applied it at `TaskService.Add`, `TaskService.loadTask`, `TaskService.loadSuite` (id branch), `SuiteService.loadSuite` (id branch), `CaptureModelSpec`, `LoadModelSpec`. Tests prove `../../escape` is rejected before any write escapes. |
| 2a | global `--dry-run` ignored — `provenance add` (High) | **FIXED-HERE** | `provenance_composition.go` never wired `DryRun`; `runAdd` unconditionally called `ledgerStore().Append`. Wired `DryRun: GetDryRun` and made `runAdd` honor it (emit the would-be edge, perform no write). Test asserts the ledger file is not created under `--dry-run`. |
| 2b | global `--dry-run` ignored — `gate check` (High) | **OPEN** | `gate_composition.go:26` wires `DryRun`, but `commands/gate/module.go` `newCheckCommand` RunE never consults `module.host.DryRun`; `ao gate check --dry-run` still executes the check registry (process/filesystem effects). Fix crosses into the check service (a plan-only "which checks would run" mode) — larger than a bounded leaf fix. |
| 3 | `--json` and `-o json` not equivalent across read commands (Medium) | **FIXED** (already landed) | The carve-out's `emitStructured` (`commands/provenance/module.go:106-116`) and the skills module now honor **both** the local `--json` and global `-o json`/`-o yaml`. Runtime probes on this tree: `provenance list`, `skills list`, and `skills check` emit byte-identical JSON for `--json` and `-o json`. Residuals (minor, not fixed): `config --json` and `config -o json` both print human help with exit 0 (`config` is a pure parent group, now consistent, not a data leaf); and `eval scenario-ab`/`scenario-moat`/`session-outcome` still define a **local** `--output` that means a file path for two of them and a text/json format for the third (`commands/eval/module.go:787,834,863`) — a cosmetic shadow of the global `--output`, not an automation break. |
| 4 | output limits applied after unbounded buffering (Medium) | **OPEN** | Still-present: `goals/measure.go:159-161`, `gates/scriptrunner.go:147-149`, `eval/engine.go:224-227`, `eval/expectations.go:527` each capture into an unbounded `bytes.Buffer` and truncate/tail only after `Wait`/`Run`. Cross-package; fix is a shared bounded tail/ring writer. |
| 5 | eval cancellation orphans descendants / ignores caller ctx (Medium) | **OPEN** | Still-present: `eval/core_service.go:78` `Run(_ context.Context, …)` discards the caller context; `eval/engine.go:20` `RunSuite` has no context parameter; `engine.go:203` and `expectations.go:512` derive timeouts from `context.Background()`; `engine.go:211` and `expectations.go:515` use `exec.CommandContext` with no process-group cleanup or `WaitDelay`. Cross-package subprocess-runner refactor (goals already has the reference pattern). |
| 6 | auto-created live-runtime isolation dirs leak (Medium) | **FIXED-HERE** | Was still-present: `eval/runtime.go` `liveRuntimeEnv` created `agentops-eval-runtime-*` via `os.MkdirTemp` when no `IsolationRoot` was supplied and returned only `(env, notes, err)`; the caller had no cleanup path. `liveRuntimeEnv` now reports an `ownedRoot` (empty for caller-supplied roots), the caller defers `removeOwnedRoot`, and partial-setup errors clean up the temp dir. Tests prove ownership detection and that a caller-supplied root is never claimed. |
| 7 | eval help references a nonexistent command (Low) | **FIXED-HERE** | Was still-present: `commands/eval/module.go:492` `--model-spec` help said "already captured via `ao eval models capture`" — a command that is not registered. Replaced with accurate text ("resolved from `<evals-root>/models/<id>/spec.yaml`") and regenerated `cli/docs/COMMANDS.md` through its owning generator (`scripts/generate-cli-reference.sh`). |

## Fixes landed in this branch

- `cli/internal/evalsubstrate/id.go` (new) — `ValidateID` containment helper.
- `cli/internal/evalsubstrate/id_test.go` (new) — rejection + acceptance table (incl. `ms:stable` colon IDs).
- `cli/internal/eval/task_service.go` — `ValidateID` at `Add`, `loadTask`, `loadSuite`.
- `cli/internal/eval/suite_service.go` — `ValidateID` at `loadSuite`.
- `cli/internal/evalsubstrate/modelspec.go` — `ValidateID` at `CaptureModelSpec`, `LoadModelSpec`.
- `cli/internal/eval/task_service_test.go`, `cli/internal/evalsubstrate/modelspec_test.go` — traversal-rejection boundary tests (assert no escaping write).
- `cli/cmd/ao/provenance_composition.go` + `cli/internal/commands/provenance/module.go` (+ test) — `--dry-run` now suppresses the provenance-add ledger write.
- `cli/internal/eval/runtime.go` (+ test) — internally created live-runtime isolation roots are owned and removed after the run.
- `cli/internal/commands/eval/module.go` + `cli/docs/COMMANDS.md` — corrected `--model-spec` help.

## OPEN items — reproductions and fix sketches

### 2b — `ao gate check --dry-run` runs the checks
Reproduction: `ao gate check --dry-run` executes the full deterministic check
registry (process spawns, `go build`, git ops) exactly as without the flag,
because `commands/gate/module.go` `newCheckCommand` never reads
`module.host.DryRun`. Sketch: add a `Plan`/dry-run request field to
`gateapp.CheckRequest`; when set, the check service returns the selected check
IDs and their declared effects **without** executing script runners, and the
module renders that plan. This is the honest form of "show what would happen"
and it also unblocks the audit's effect-metadata roadmap item.

### 4 — unbounded subprocess output buffering
Reproduction: a check/goal/eval command whose subprocess writes gigabytes to
stdout is buffered in full (`bytes.Buffer`) before the 4 KiB / `truncateOutput`
trim, so peak CLI memory is unbounded regardless of the post-trim cap. Sketch:
a shared `boundedTailWriter` (fixed ring capacity + a "truncated, original N
bytes" flag) used as `cmd.Stdout`/`cmd.Stderr` across `goals/measure.go`,
`gates/scriptrunner.go`, and `eval/engine.go` + `eval/expectations.go`. Test: a
child that emits > cap bytes; assert bounded retained size and a truncation
marker.

### 5 — caller cancellation ignored, grandchildren orphaned
Reproduction: cancelling the context passed to `CoreService.Run` (or any
`engine.RunSuite` path) does not stop the eval subprocess, and a timed-out
command that spawned a grandchild leaves it running with pipes open, because the
timeout comes from `context.Background()` and `exec.CommandContext` kills only
the direct child. Sketch: thread the caller context through `RunSuite` and the
`core_service` use cases, and route all eval subprocess launches through the
same runner `goals/measure.go` already uses (`configureProcGroup` +
`cmd.WaitDelay` + PID tracking + signal-based process-group cleanup). Test: a
child that starts a grandchild keeping stdout open; assert the grandchild is
reaped on timeout.

## Gates (this branch)

- `cd cli && go build ./...` — Success.
- `cd cli && go vet ./...` — no issues.
- `cd cli && go test ./...` — 2870 passed across 70 packages.
- `scripts/golangci-lint-v2.sh run` (touched packages) — 0 issues.
- `scripts/generate-cli-reference.sh --check` — `cli/docs/COMMANDS.md` up to date.
