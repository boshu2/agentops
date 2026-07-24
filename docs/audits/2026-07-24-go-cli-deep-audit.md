# Go CLI Deep Audit

- Date: 2026-07-24
- Scope: `/Users/bo/dev/agentops/cli`
- Mode: read-only audit; no CLI files modified

## Executive assessment

The Go CLI is generally well engineered: it has strong test density, explicit
architecture boundaries, generated-documentation checks, honest handling of
unknown capability metadata, and good safety implementations in doctor,
storage, and much of goals.

The principal risks are inconsistent cross-cutting policy enforcement.
Filesystem containment, `--dry-run`, output negotiation, subprocess lifecycle,
and temporary-resource ownership are each implemented locally, producing
several verified gaps.

## Inventory

- 478 Go files, including 231 test files.
- 101,561 total Go lines; 49,255 production Go lines.
- 68 Go packages: 65 under `internal` and 3 command entry points.
- 17 published top-level commands and 84 published command nodes.
- Go 1.26.5; the CLI module declares Go 1.26 and toolchain 1.26.5.

## Findings

### High: eval IDs permit filesystem escape

- Type: verified security and correctness defect
- Confidence: high

Evidence:

- `cli/internal/eval/task_service.go:64-87`: `TaskService.Add` accepts a
  YAML-provided ID, checks only non-emptiness and sample count, then constructs
  the destination with
  `filepath.Join(root, "tasks", task.ID, "task.yaml")` at line 83 and writes it
  at line 84.
- `cli/internal/eval/task_service.go:188-198`: `loadTask` joins caller-supplied
  IDs the same way for reads.
- `cli/internal/eval/suite_service.go:87-100`: `SuiteService.loadSuite` repeats
  this for suite IDs.
- `cli/internal/evalsubstrate/modelspec.go:12-20`: `ModelSpecPath` accepts an
  arbitrary spec ID, while capture checks only for an empty ID.
- `cli/internal/commands/eval/module.go:405-413` and `450-454`: public CLI input
  reaches the path through `ao eval task add` and `task show`.
- Existing task tests cover valid IDs but not traversal or symlink cases.

An ID such as `../../escape` is cleaned by `filepath.Join` and escapes the
declared eval subtree. Writes have a fixed `task.yaml` suffix, but can still
create or overwrite that file outside the store. Reads can likewise cross the
intended boundary.

Recommendation:

- Centralize validation for task, suite, run, and model-spec IDs; reject
  separators, absolute paths, `.` and `..` components, control characters, and
  platform-specific volume syntax.
- Use containment-safe operations rooted with `os.OpenRoot`, not only a lexical
  prefix check.
- Add traversal, Windows-volume, and symlink-parent tests at every service
  boundary.

### High: global `--dry-run` does not reliably suppress effects

- Type: verified safety and contract defect
- Confidence: high

Evidence:

- `cli/cmd/ao/root.go:135`: the persistent flag promises “Show what would
  happen without executing” for every command.
- `cli/internal/commands/provenance/module.go:126-175`: `ao provenance add`
  unconditionally calls `ledgerStore().Append`; its host options contain no
  dry-run seam.
- `cli/internal/commands/gate/module.go:19-24`: the gate module receives
  `HostOptions.DryRun`, but `gate check` never consults it and always executes
  the check service at lines 108-116.
- Doctor, init, config, and selected eval paths do honor dry-run, confirming
  that behavior is implemented family by family.

Impact: users can request `--dry-run` and still write a provenance ledger or
launch repository check processes. This directly violates an advertised safety
control.

Recommendation:

- Either make `--dry-run` local only to supported commands or centrally reject
  it on unsupported effectful leaves.
- Add dry-run support and effect metadata to `CommandContract`.
- Generate a test matrix that exercises every filesystem and process command
  under dry-run and proves no state or process effect occurs.

### Medium: `--json` and `-o json` are not equivalent across read commands

- Type: verified automation and API defect
- Confidence: high

Evidence:

- `cli/cmd/ao/root.go:137-138`: root help defines `--json` as shorthand for
  `-o json`.
- `cli/internal/capabilities/document.go:96-100`: capabilities promises JSON on
  read-side commands via either form.
- `cli/internal/commands/provenance/module.go:211` and `235-240`: provenance
  defines and renders from a separate local `list --json` boolean. Runtime
  probing confirmed that `provenance list --json` emits JSON while
  `provenance list -o json` emits human text with exit 0.
- `cli/internal/commands/skills/module.go:126,205,281,324,389,416,504,575`:
  skills contains eight more independent local JSON flags. Probes confirmed the
  same mismatch for `skills list` and `skills check`.
- `cli/cmd/ao/flag_matrix_test.go:298-349`: the equivalence matrix tests only
  `status`.
- `cli/cmd/ao/group_json.go:35-45`: `config --json` returns human help with exit
  0 because runnable parents are excluded from group JSON.
- `cli/internal/commands/eval/module.go:747-824`: eval shadows global
  `--output`. It means a file path for `scenario-ab` and `scenario-moat`, but a
  text or JSON format for `session-outcome`.

Impact: automation receives successful non-JSON output after requesting JSON,
and the global option’s meaning changes by subcommand.

Recommendation:

- Use one inherited output enum consumed by every renderer.
- Remove local JSON booleans or derive them from the negotiated root format.
- Rename file destinations to `--out`, `--output-file`, or `--scorecard`.
- Generate valid-JSON and `--json`/`-o json` equivalence tests for every
  advertised read leaf.
- Reject unsupported YAML or JSON explicitly instead of silently producing
  human text.

### Medium: output limits are applied after unbounded buffering

- Type: verified reliability defect
- Confidence: high

Evidence:

- `cli/internal/goals/measure.go:157-175`: goals captures unlimited stdout and
  stderr in a `bytes.Buffer`, then truncates only after `Wait`.
- `cli/internal/gates/scriptrunner.go:133-158`: gate does the same before taking
  a 4 KiB tail.
- `cli/internal/eval/engine.go:224-239`: deterministic eval cases buffer
  unlimited separate streams.
- `cli/internal/eval/runtime.go:431-443`: live runtime execution repeats it.
- `cli/internal/eval/expectations.go:505-541`: eval auto-detection repeats it.

A noisy or malformed check can exhaust CLI memory; post-process truncation does
not bound peak memory.

Recommendation: share a bounded ring or tail writer, or spool to a restricted
temporary file with a hard byte ceiling. Record whether output was truncated
and, where feasible, the original byte count.

### Medium: eval cancellation can orphan descendants or ignore caller cancellation

- Type: verified static lifecycle defect
- Confidence: high

Evidence:

- `cli/internal/eval/core_service.go:78`: `CoreService.Run` discards its caller
  context. Compare, PromoteBaseline, AuditBaseline, Scorecard, and Coverage do
  the same at lines 124, 140, 152, 160, and 188.
- `cli/internal/eval/engine.go:20-64`: `RunSuite` has no context parameter and
  creates each timeout from `context.Background()` at line 203.
- `cli/internal/eval/engine.go:211-229`: commands use `exec.CommandContext`
  without process-group cleanup or `WaitDelay`.
- `cli/internal/eval/expectations.go:512-530`: auto-detect repeats the background
  context.
- `cli/internal/goals/measure.go:147-175` and `245-265`: goals demonstrates the
  stronger existing pattern with caller context, process groups, `WaitDelay`,
  PID tracking, and signal cleanup.

Impact: programmatic cancellation is ignored, and a timed-out shell command can
leave grandchildren running or holding pipes.

Recommendation: introduce a shared subprocess runner with caller-context
propagation, process-group or process-tree termination, `WaitDelay`, and bounded
capture. Add a test where a child starts a grandchild that keeps stdout open.

### Medium: automatically created live-runtime isolation directories leak

- Type: verified resource and privacy defect
- Confidence: high

Evidence:

- `cli/internal/eval/runtime.go:324-365`: `liveRuntimeEnv` creates
  `agentops-eval-runtime-*` with `os.MkdirTemp` when no isolation root is
  supplied.
- The function returns only environment and notes, not ownership information or
  cleanup.
- The caller at `cli/internal/eval/runtime.go:172-197` has no cleanup path.
- `cli/internal/eval/runtime_test.go:101-123`: isolation tests supply
  `t.TempDir()` explicitly, so they do not cover the internally owned
  directory.

Impact: runtime-written HOME and CODEX_HOME data remains in temporary storage
after success, error, and timeout.

Recommendation: return an owned-resource cleanup function or structured
environment object; defer cleanup only for internally created roots. Test
success, failure, timeout, and partial-setup errors.

### Low: eval help references a nonexistent command

- Type: verified UX correctness defect
- Confidence: high

`cli/internal/commands/eval/module.go:468-477` tells users that a model spec is
“already captured via `ao eval models capture`”. That command is not registered
and exits as an unknown command. Generated documentation faithfully repeats the
stale source text.

Recommendation: restore the command if it remains supported, or replace the
help with the actual capture workflow. Add an executable-reference test for
command paths named in help text.

## Improvement opportunities, not current defects

- Capabilities is deliberately honest but incomplete: of 84 command nodes, 74
  have unknown args, 77 report no output, 74 have unknown effects, and 74 have
  no exit-code map. `cli/internal/clicontract/inspect.go:11-108` intentionally
  avoids fabrication. Incrementally attach leaf contracts and add format and
  dry-run support metadata.
- `cli/internal/evalsubstrate/atomic.go:17-52` uses a fixed `<path>.tmp`. That
  matches the repository’s one-writer default, but concurrent CLI invocations
  targeting one artifact can collide. A unique same-directory temporary plus
  rename would make the assumption unnecessary.
- Gate execution is intentionally serial. Once checks declare resource and
  conflict metadata, a bounded deterministic worker pool could improve
  full-gate latency.
- `cli/internal/commands/eval/module.go` is 973 lines and
  `cli/cmd/ao/config.go` is 1,157 lines. Further leaf-level extraction would
  reduce flag-state and policy drift.
- `cmd/skill-frontmatter-json` and `cmd/witness-crosscheck` have no direct
  package tests. Their main domain logic is tested elsewhere, but small
  exit-code and stdio integration tests would close the shell boundary.
- `-V` is not defined, while `--version` works. This is only conventional CLI
  polish.

## Top-level command and owning-package ledger

All 17 command modules were inspected for registration, flags, contracts,
effects, rendering, and tests.

| Command | Owning package | Assessment |
|---|---|---|
| `capabilities` | `internal/commands/capabilities` | Stable JSON and YAML projection with tests; useful, but only 10 of 84 nodes have rich contracts. |
| `config` | `internal/commands/config` | Layered source reporting, model writes, locking, validation, and dry-run are tested; parent `config --json` violates the broad JSON promise. |
| `demo` | `internal/commands/demo` | Pure product-boundary output with focused tests; no defect found. |
| `doctor` | `internal/commands/doctor` | Strongest safety surface: read and mutate separation, dry-run union tests, scoped mutation, locks, undo and GC, and artifacts. |
| `eval` | `internal/commands/eval` | Broad deterministic and live substrate with extensive tests; source of containment, subprocess, isolation, flag-collision, and stale-help findings. |
| `flywheel` | `internal/commands/flywheel` | Status and compare formats and metrics tested; contract remains intentionally unattached. |
| `gate` | `internal/commands/gate` | Clear deterministic-check boundary and exit mapping; ignores inherited dry-run. |
| `goals` | `internal/commands/goals` | Rich measure, validate, drift, history, export, and scenario surface; mature tests and process cleanup; unbounded output remains. |
| `init` | `internal/commands/init` | Creates evidence storage without Git mutation; dry-run tested; no defect found. |
| `provenance` | `internal/commands/provenance` | Strong hash-chain, verify, trace, show, and export tests; ignores dry-run and fragments output flags. |
| `quick-start` | `internal/commands/quickstart` | Pure generated orientation text with boundary tests; no defect found. |
| `redact` | `internal/commands/redact` | Small pure stdin-to-stdout canonical redactor; tested; no defect found. |
| `robot-docs` | `internal/commands/robotdocs` | Live command-tree projection and generated-doc check are strong; semantic stale help is propagated faithfully. |
| `session` | `internal/commands/session` | Local bootstrap, handoff, and rehydrate boundary is tested; local JSON flags contribute to output fragmentation. |
| `skills` | `internal/commands/skills` | Strong query, graph, link, health, and resolver tests; eight local JSON flags diverge from `-o json`. |
| `status` | `internal/commands/status` | Evidence-only status, artifact validation, and JSON equivalence are strong. |
| `version` | `internal/commands/version` | Human and JSON metadata are deterministic and tested; only minor missing `-V` convention. |

Cobra-generated `help` and `completion` were also smoke-tested; zsh completion
generated successfully.

## Remaining internal-package ledger

Every internal package was inspected at least at source and test-manifest and
public-boundary level. High-risk packages received line-by-line tracing.

- `adapters/capabilities`: thin Cobra and platform projection; no direct tests,
  exercised through capabilities integration.
- `adapters/config`: thin gateway over config; no direct tests, exercised by
  config services and modules.
- `adapters/doctor`: well-tested request and runtime mappings, including
  dry-run persistence protection.
- `adapters/eval`: suite and runtime bridges plus agentic sandbox; strong
  sandbox-denial and runtime tests.
- `adapters/gate`: repo-root and validated-range mapping are tested.
- `archcheck`: architecture inventory and semantic escape classification are
  well tested.
- `archcheck/cmd`: thin internal checker entry point; no direct tests.
- `capabilities`: stable document construction; tested.
- `clicontract`: explicit metadata validation and projection; tested and
  intentionally fail-honest.
- `config`: precedence, validation, locking, patch preview, and dry-run tested.
- `constraintindex`: locking, stale filtering, fail-closed parsing, and publish
  sanitization tested.
- `doctor`: large but strongly partitioned engine, fix, read, and mutation
  implementation with broad tests.
- `drrebuild`: deterministic ledger rebuild and tamper and reorder verification
  tested.
- `drwitness`: Dolt-to-JSONL witness re-derivation and cross-check tested.
- `eval`: deepest review; strong functional breadth, with findings above.
- `evalsubstrate`: canonicalization, gates, manifests, holdout, refusal, and
  rubric logic tested; ID containment missing.
- `evidence`: small citation path and load layer; no direct package tests.
- `flywheelapp`: identity, namespace, metrics, golden signals, and all output
  formats tested.
- `gate`: application service mapping and workflow-parity behavior tested.
- `gates`: routing, reporting, orchestration, workflow coverage, and
  architecture checks tested; subprocess capture issue above.
- `gates/checks`: constraints, Git hygiene, builds, workflow installation,
  parity, and routing tested.
- `goals`: broad, mature test suite; process-group handling is a notable
  strength; capture remains unbounded.
- `goalsfitness`: threshold parsing, aggregation, and unknown and skipped
  semantics tested.
- `initapp`: small directory initializer; no direct test, covered by command
  tests.
- `openclaw`: compatibility snapshot schema and provenance validation tested.
- `parser`: Claude and Codex transcript parsing, usage accounting, and malformed
  and truncated input tested.
- `paths`: environment precedence, repo resolution, shell agreement, and
  validation tested.
- `ports`: narrow interfaces; in-memory gate runner cancellation and defaults
  tested.
- `provenanceapp`: session mining, filtering, and schema conformance tested.
- `provenancegraph`: ledger, graph, hashing, storage, verification, and locking
  inspected; command tests exercise it heavily.
- `quality`: validation, scoring, and reporting utilities inspected with
  focused tests; no defect found.
- `redact`: canonical secret scrubbing tested.
- `runtimecmd`: command splitting and normalization tested; no defect found.
- `scenario`: scenario persistence and answer-key model inspected and tested.
- `scenarioresults`: result aggregation and storage inspected and tested.
- `sessionapp`: thin application boundary with no direct tests; command
  integration covers primary behavior.
- `shellutil`: sanitized shell construction tested.
- `skills`: catalog, frontmatter, and domain operations inspected and tested.
- `skillsapp`: application listing and query operations inspected and tested.
- `skillshealth`: frontmatter health validation tested.
- `skillsresolve`: overlap and coverage-gap resolution tested.
- `statusapp`: rejects corrupt and arbitrary artifacts and detects subject
  mutation; strong tests.
- `storage`: atomic files, locking, search index, and persistence tested,
  including Unix behavior.
- `testsupport`: Git discovery environment scrubbing tested.
- `types`: shared durable models, JSON round trips, token deduplication, and
  supersession depth tested.
- `types/quest`: atomic writes and fault behavior tested, including requested
  modes.
- `verdictcheck`: canonical JSON, digest parity, verdict shape, and golden
  corpus tested.
- `wiki`: frontmatter codec and corpus location tested.

## Command entry points

- `cmd/ao`: deeply inspected; extensive root, flag, output, help, exit, and
  composition tests.
- `cmd/skill-frontmatter-json`: inspected; simple YAML-to-JSON adapter, but no
  direct tests.
- `cmd/witness-crosscheck`: inspected; thin exit-code and file adapter over
  tested `drwitness`, but no direct tests.

## Strengths

- Broad checks are fully green, including race instrumentation.
- Architecture enforcement is unusually explicit: `archcheck`, module
  contracts, generated docs, and truthfulness tests constrain drift.
- Doctor’s mutation safety and goals’ process-group cleanup are reusable
  reference implementations.
- Storage and verdict validation emphasize durability, canonicalization,
  tamper detection, and fail-closed behavior.
- Stdout and stderr separation, typo hints, help behavior, no-argument
  behavior, and completion generation were clean in smoke probes.
- Unknown capabilities are reported as unknown rather than fabricated.

## Recommended roadmap

1. Before the next release, close eval path containment and make global dry-run
   fail-safe.
2. Unify subprocess execution across eval, gate, and goals with bounded capture,
   caller cancellation, tree cleanup, and explicit truncation telemetry.
3. Normalize output semantics and generate an all-read-command JSON equivalence
   matrix.
4. Fix live-isolation ownership and cleanup and the stale model-spec help.
5. Expand leaf capability contracts, including effect, dry-run, and supported
   output metadata.
6. Then address unique atomic temporary names, helper-main integration tests,
   eval module decomposition, and optional deterministic gate parallelism.

## Checked

- `rtk go test -shuffle=on ./...`: passed; 2,650 tests reported across 68
  packages.
- `rtk go test -race ./...`: passed; 2,650 tests reported.
- `rtk go vet ./...`: passed.
- `scripts/golangci-lint-v2.sh run ./...`: 0 issues.
- `govulncheck ./...`: no known vulnerabilities.
- `scripts/generate-cli-reference.sh --check`: `cli/docs/COMMANDS.md` current.
- `go build ./cmd/ao`: passed.
- CLI probes: root and no-args help, `-h`, `--help`, `--version`, invalid command
  and flag hints, capabilities JSON and YAML, group JSON, output conflict,
  `NO_COLOR`, zsh completion, stdout and stderr separation, and representative
  output-equivalence cases.
- All 17 top-level command modules, all 65 internal packages, and all 3 command
  packages inventoried and assessed.
- Final `git status`, `git diff --stat`, and `git diff --check` for `cli`: clean.

## Not checked and residual risk

- No mutating command was run against the repository, home directory, or an
  external system.
- No live Codex or model eval, network-dependent doctor action, doctor fix or
  undo, real provenance write, or external runtime integration was executed.
- No Linux or Windows runtime execution was performed; platform build-tag
  implementations were inspected and covered only by available tests.
- Fixtures, schemas, embedded assets, and generated Markdown were checked
  through their owning tests and generators, not audited line by line.
- No hostile-output memory benchmark or actual orphan-grandchild reproduction
  was run; those findings are based on direct implementation tracing.
- Existing unrelated non-CLI worktree changes were untouched.
