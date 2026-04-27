---
id: research-2026-04-24-go-cli-local-code-explore
type: research
date: 2026-04-24
---

# Research: Go CLI Local Code Exploration

**Backend:** inline
**Scope:** Local-only exploration of `cli/cmd/ao`, `cli/internal`, `cli/docs/COMMANDS.md`, `cli/README.md`, Go tests, Makefile, and local validation scripts. No web browsing.

## Summary

The `ao` CLI is useful: it is the repo-native control plane for AgentOps bookkeeping, retrieval, health checks, compounding, goals, and terminal workflows, and the public docs map that value directly to commands like `ao quick-start`, `ao doctor`, `ao search`, `ao context assemble`, `ao rpi phased`, `ao evolve`, `ao overnight setup`, and `ao metrics health` (`README.md:234`, `README.md:236`, `README.md:239`, `README.md:248`). The quality picture is mixed: the repo has strong tests, generated docs, race/coverage CI, and some good internal-package Options patterns, but much of `cli/cmd/ao` still depends on package-level Cobra globals, direct `os.Stdout` writes, broad allowlists, and post-hoc reset helpers.

## Key Files And Architectural Map

| File | Role |
|---|---|
| `cli/cmd/ao/main.go` | Tiny binary entrypoint; `version` is ldflag-set and `main` calls `Execute` (`cli/cmd/ao/main.go:4`, `cli/cmd/ao/main.go:7`). |
| `cli/cmd/ao/root.go` | Root Cobra command, global flags, command groups, pre-run setup, and App context injection (`cli/cmd/ao/root.go:22`, `cli/cmd/ao/root.go:41`, `cli/cmd/ao/root.go:57`, `cli/cmd/ao/root.go:78`, `cli/cmd/ao/root.go:89`). |
| `cli/cmd/ao/app.go` | Intended shared state / DI container; comments explicitly call it a Terraform Meta + kubectl Options hybrid (`cli/cmd/ao/app.go:11`, `cli/cmd/ao/app.go:15`, `cli/cmd/ao/app.go:24`). |
| `cli/internal/*` | Domain packages that hold reusable logic: examples include `internal/goals` Options-based command logic (`cli/internal/goals/commands.go:74`, `cli/internal/goals/commands.go:87`), `internal/rpi` typed RPI contracts (`cli/internal/rpi/types.go:19`, `cli/internal/rpi/types.go:53`), `internal/config` precedence-aware config (`cli/internal/config/config.go:1`, `cli/internal/config/config.go:357`), and `internal/quality` doctor rendering (`cli/internal/quality/doctor.go:32`, `cli/internal/quality/doctor.go:39`). |
| `cli/docs/COMMANDS.md` | Generated command reference, explicitly marked auto-generated and not manually edited (`cli/docs/COMMANDS.md:1`, `cli/docs/COMMANDS.md:3`). It documents the broad surface from core commands through RPI, knowledge, search, and scenario commands (`cli/docs/COMMANDS.md:17`, `cli/docs/COMMANDS.md:1498`, `cli/docs/COMMANDS.md:2413`, `cli/docs/COMMANDS.md:2613`). |
| `scripts/generate-cli-reference.sh` | Builds a temporary `ao` and extracts help into `COMMANDS.md` (`scripts/generate-cli-reference.sh:4`, `scripts/generate-cli-reference.sh:32`, `scripts/generate-cli-reference.sh:248`). |
| `cli/Makefile` | Standard build/test/lint entrypoints; build syncs embedded hooks first, tests run build then shuffled `go test`, lint delegates to the repo wrapper (`cli/Makefile:14`, `cli/Makefile:23`, `cli/Makefile:31`, `cli/Makefile:40`). |
| `.github/workflows/validate.yml` | CI verifies docs parity, builds the CLI, runs race+shuffle+coverage tests, enforces the `cmd/ao` coverage floor, checks embedded hook sync, and runs JSON flag consistency (`.github/workflows/validate.yml:118`, `.github/workflows/validate.yml:525`, `.github/workflows/validate.yml:551`, `.github/workflows/validate.yml:562`, `.github/workflows/validate.yml:614`, `.github/workflows/validate.yml:704`). |

There is no `docs/code-map/` directory in this worktree, so exploration fell back to the newcomer guide, generated CLI docs, scoped `rg`, scoped git history, and source reads. The newcomer guide confirms the CLI layer is `cli/` with `cli/cmd/ao`, `cli/internal`, and generated `cli/docs/COMMANDS.md` (`docs/newcomer-guide.md:29`, `docs/newcomer-guide.md:75`).

## Strengths And Usefulness

1. **The CLI has a clear product job.** Product docs say the Go binary provides repo-native infrastructure that skills depend on, including bookkeeping, goal/issue orchestration, and context/operator surfaces (`PRODUCT.md:64`, `PRODUCT.md:66`, `PRODUCT.md:68`, `PRODUCT.md:70`).
2. **The command surface is broad and operationally meaningful.** Generated docs include RPI lifecycle commands, software factory startup, Codex lifecycle, knowledge activation, lookup/search, retrieval benchmarking, and overnight flows (`cli/docs/COMMANDS.md:744`, `cli/docs/COMMANDS.md:894`, `cli/docs/COMMANDS.md:1498`, `cli/docs/COMMANDS.md:2413`, `cli/docs/COMMANDS.md:2486`, `cli/docs/COMMANDS.md:2590`).
3. **Cobra fundamentals are present.** The root command defines global `--dry-run`, `--verbose`, `--output`, `--json`, and `--config` flags (`cli/cmd/ao/root.go:89`, `cli/cmd/ao/root.go:94`), uses command groups for help organization (`cli/cmd/ao/root.go:78`, `cli/cmd/ao/root.go:87`), and has shell completion generation (`cli/cmd/ao/completion.go:9`, `cli/cmd/ao/completion.go:21`).
4. **The repo has serious validation infrastructure.** CI runs the Go suite with `-race`, shuffled tests, atomic coverage, and a `cmd/ao` coverage floor (`.github/workflows/validate.yml:551`, `.github/workflows/validate.yml:556`, `.github/workflows/validate.yml:562`). CLI docs parity is automated (`.github/workflows/validate.yml:129`, `.github/workflows/validate.yml:132`).
5. **Some domains already follow a high-quality Options pattern.** `internal/goals.RunMeasure` accepts an Options struct with `Stdout` and `Stderr`, defaults those writers only when nil, and renders JSON/table through the provided writer (`cli/internal/goals/commands.go:74`, `cli/internal/goals/commands.go:89`, `cli/internal/goals/commands.go:145`, `cli/internal/goals/commands.go:151`). `internal/quality.RunDoctor` similarly takes `DoctorOptions` and writes to an injected `Stdout` (`cli/internal/quality/doctor.go:32`, `cli/internal/quality/doctor.go:39`, `cli/internal/quality/doctor.go:45`).
6. **Prior local learning is relevant.** Existing learning says changes under `cli/cmd/ao/` must be paired with tests, citing a prior `codex.go` refactor that only cleared the fast gate after helper tests were added (`.agents/learnings/2026-04-14-command-refactors-need-paired-tests.md:15`, `.agents/learnings/2026-04-14-command-refactors-need-paired-tests.md:18`). That matches the current command-heavy risk profile.

## Quality Concerns With Evidence

1. **The intended App/Options architecture is not consistently adopted.** `App` exists specifically to replace mutable globals and enable dependency injection (`cli/cmd/ao/app.go:11`, `cli/cmd/ao/app.go:14`), and root injects it into command context (`cli/cmd/ao/root.go:57`, `cli/cmd/ao/root.go:65`). But command handlers still rely heavily on package-level flag variables and getters: `notebook.go` keeps five command globals (`cli/cmd/ao/notebook.go:13`, `cli/cmd/ao/notebook.go:19`), `root.go` exposes `GetOutput`/`GetDryRun` over package globals (`cli/cmd/ao/root.go:99`, `cli/cmd/ao/root.go:117`), and `contradict.go` renders based on `GetOutput()` and writes to `os.Stdout` directly (`cli/cmd/ao/contradict.go:183`, `cli/cmd/ao/contradict.go:189`).
2. **Testing has to compensate for global mutable command state.** `executeCommand` manually saves and restores many package-level vars (`cli/cmd/ao/cobra_commands_test.go:33`, `cli/cmd/ao/cobra_commands_test.go:36`, `cli/cmd/ao/cobra_commands_test.go:126`), resets Cobra flag state recursively (`cli/cmd/ao/cobra_commands_test.go:315`), and captures process-global `os.Stdout` because commands print directly (`cli/cmd/ao/cobra_commands_test.go:329`, `cli/cmd/ao/cobra_commands_test.go:359`). Shared test helpers warn that stdout capture redirects global `os.Stdout` and must serialize on a package mutex (`cli/cmd/ao/testutil_test.go:35`, `cli/cmd/ao/testutil_test.go:38`, `cli/cmd/ao/testutil_test.go:43`).
3. **Complexity policy is internally inconsistent.** The local Go style guide says CC 16-20 "Must refactor", 21+ should block merge, and the summary checklist says complexity should be CC <= 10 (`docs/standards/golang-style-guide.md:453`, `docs/standards/golang-style-guide.md:461`, `docs/standards/golang-style-guide.md:555`, `docs/standards/golang-style-guide.md:562`). The actual Go lint config sets `gocyclo.min-complexity` to 25 (`cli/.golangci.yml:11`, `cli/.golangci.yml:13`), and CI enforces warn 15 / fail 25 (`.github/workflows/validate.yml:626`, `.github/workflows/validate.yml:636`). Local `gocyclo -over 15` found production functions in the 16-19 range, including `serveRPIState`, `runContradict`, `runNotebookUpdate`, `RunMeasure`, and `RunPrune`.
4. **The JSON contract gate can warn instead of fail for non-JSON output.** `tests/cli/test-json-flag-consistency.sh` says it verifies valid JSON (`tests/cli/test-json-flag-consistency.sh:47`, `tests/cli/test-json-flag-consistency.sh:51`), but non-empty invalid JSON is only a warning (`tests/cli/test-json-flag-consistency.sh:78`, `tests/cli/test-json-flag-consistency.sh:83`) and the script exits nonzero only when `ERRORS > 0` (`tests/cli/test-json-flag-consistency.sh:132`, `tests/cli/test-json-flag-consistency.sh:137`). The binary matrix in Go tests covers a small curated set of JSON commands (`cli/cmd/ao/flag_matrix_test.go:33`, `cli/cmd/ao/flag_matrix_test.go:40`, `cli/cmd/ao/flag_matrix_test.go:51`).
5. **Command-surface coverage has a broad allowlist.** The parity gate aims to ensure every leaf command is referenced in smoke tests or Go tests (`scripts/check-cmdao-surface-parity.sh:4`, `scripts/check-cmdao-surface-parity.sh:6`, `scripts/check-cmdao-surface-parity.sh:133`, `scripts/check-cmdao-surface-parity.sh:137`). The allowlist explicitly excludes many high-value or hazardous commands, including `rpi loop`, `rpi parallel`, `rpi phased`, `rpi nudge`, `rpi stream`, and multiple state-modifying goals/pool/gate commands (`scripts/cmdao-surface-allowlist.txt:93`, `scripts/cmdao-surface-allowlist.txt:103`, `scripts/cmdao-surface-allowlist.txt:117`, `scripts/cmdao-surface-allowlist.txt:148`). Some exclusions are justified, but best-in-class CLIs usually still have dry-run/tempdir integration coverage for dangerous surfaces.
6. **Generated docs are comprehensive but shallow.** The generator takes only the first help line as command description (`scripts/generate-cli-reference.sh:139`, `scripts/generate-cli-reference.sh:144`), then emits usage and flags (`scripts/generate-cli-reference.sh:147`, `scripts/generate-cli-reference.sh:165`). This creates a complete reference but loses examples, output contracts, environment behavior, failure modes, and long descriptions that matter for complex commands like `ao rpi phased` and `ao rpi loop`.
7. **Shell completion coverage is present but limited.** The completion command exists (`cli/cmd/ao/completion.go:9`, `cli/cmd/ao/completion.go:21`), and static completions are registered for `--output`, `inject --format`, `inject --session-type`, and template flags (`cli/cmd/ao/root.go:96`, `cli/cmd/ao/inject.go:114`, `cli/cmd/ao/seed.go:64`, `cli/cmd/ao/goals_init.go:58`). Many enum-like flags documented in `COMMANDS.md`, such as RPI runtime, failure policy, gate policy, and landing policy, do not show local completion registration in the scoped search.
8. **The generated docs and root command do not reveal a central error/exit-code contract.** Root sets `SilenceUsage: true` but no local `SilenceErrors` or custom error renderer is visible (`cli/cmd/ao/root.go:23`, `cli/cmd/ao/root.go:40`), and `Execute` exits with code 1 for any returned error (`cli/cmd/ao/root.go:71`, `cli/cmd/ao/root.go:75`). This is workable, but it leaves user-facing error consistency to individual commands and Cobra defaults.

## Gaps Versus High-Quality Go CLI Conventions

1. **Thin Cobra layer gap.** The repo already knows the desired direction: `App` says command state should move out of globals into a testable Options/DI pattern (`cli/cmd/ao/app.go:11`, `cli/cmd/ao/app.go:14`). Best-in-class Go CLIs keep Cobra handlers thin and route into internal packages; this repo does that in `internal/goals` and `internal/quality`, but many command files still contain business logic plus output formatting.
2. **Stable machine-output gap.** The global `--json` flag exists (`cli/cmd/ao/root.go:92`, `cli/cmd/ao/root.go:93`), but the consistency gate tolerates non-JSON output as warnings (`tests/cli/test-json-flag-consistency.sh:78`, `tests/cli/test-json-flag-consistency.sh:84`). A stronger CLI treats machine output as a contract and fails CI on invalid JSON for supported commands.
3. **Integration confidence gap.** Broad command-surface parity exists, but many key commands are allowlisted due to statefulness or long runtimes (`scripts/cmdao-surface-allowlist.txt:93`, `scripts/cmdao-surface-allowlist.txt:115`). Best-in-class CLIs usually provide deterministic `--dry-run`, tempdir, fixture, or fake-executor integration tests for those paths.
4. **Docs UX gap.** Generated docs are kept in sync, which is strong (`cli/docs/COMMANDS.md:3`, `.github/workflows/validate.yml:129`), but the generator emits a skeletal API reference from help text rather than a user-oriented reference with examples and output/error contracts (`scripts/generate-cli-reference.sh:139`, `scripts/generate-cli-reference.sh:165`).
5. **Completion and discoverability gap.** Completion generation is available, but enum completion registration is sparse compared with the number of enum-like flags in generated docs (`cli/cmd/ao/completion_values.go:10`, `cli/cmd/ao/completion_values.go:18`, `cli/docs/COMMANDS.md:1559`, `cli/docs/COMMANDS.md:1581`, `cli/docs/COMMANDS.md:1659`).
6. **Complexity ratchet gap.** Local style guidance wants much lower complexity than CI currently enforces (`docs/standards/golang-style-guide.md:453`, `docs/standards/golang-style-guide.md:562`; `cli/.golangci.yml:12`, `cli/.golangci.yml:13`). The codebase is below the current fail threshold, but not below the repo's own aspirational standard.

## Recommended Issue Decomposition Hints

1. **Adopt a command factory / Options contract for `cli/cmd/ao`.**
   - Owner files: `cli/cmd/ao/root.go`, `cli/cmd/ao/app.go`, one pilot command such as `cli/cmd/ao/contradict.go` or `cli/cmd/ao/notebook.go`.
   - Test surfaces: `cli/cmd/ao/cobra_commands_test.go`, `cli/cmd/ao/testutil_test.go`, command-specific tests.
   - Acceptance shape: a pilot command takes dependencies from `cmd.Context()` or an Options struct, writes only to injected writers, and needs less global reset in tests.

2. **Create shared output helpers for JSON/table/yaml and migrate direct printers.**
   - Owner files: `cli/internal/formatter/*`, `cli/cmd/ao/root.go`, representative command files with `fmt.Print`/`os.Stdout`.
   - Test surfaces: `cli/cmd/ao/json_validity_test.go`, `cli/cmd/ao/golden_output_test.go`, `tests/cli/test-json-flag-consistency.sh`.
   - Acceptance shape: supported commands write JSON through one helper, human output through injected writers, and invalid JSON under `--json` fails tests.

3. **Harden the JSON flag consistency gate.**
   - Owner files: `tests/cli/test-json-flag-consistency.sh`, `tests/cli/README.md`, `.github/workflows/validate.yml`.
   - Test surfaces: `tests/cli/test-json-flag-consistency-tempdir.sh`, `cli/cmd/ao/flag_matrix_test.go`.
   - Acceptance shape: invalid non-empty JSON output is an error for commands that accept `--json`; unsupported state-dependent commands are explicitly classified.

4. **Reduce command-surface allowlist by adding tempdir/dry-run integration smoke tests.**
   - Owner files: `scripts/check-cmdao-surface-parity.sh`, `scripts/cmdao-surface-allowlist.txt`, `scripts/release-smoke-test.sh`, selected `cli/cmd/ao/*_test.go`.
   - Test surfaces: RPI dry-run/tempdir tests, goals/pool/gate fixture tests, release smoke.
   - Acceptance shape: each removed allowlist entry has either a safe integration test or a documented fake executor path.

5. **Enrich generated CLI docs without losing parity.**
   - Owner files: `scripts/generate-cli-reference.sh`, `cli/docs/COMMANDS.md`, command `Long`/`Example` fields.
   - Test surfaces: `scripts/generate-cli-reference.sh --check`, docs parity CI.
   - Acceptance shape: generated docs include examples, output modes, and failure notes for complex commands while staying generated.

6. **Expand shell completion coverage for enum-like flags.**
   - Owner files: `cli/cmd/ao/completion_values.go`, commands with enum flags such as RPI loop/phased/evolve/search/overnight.
   - Test surfaces: `cli/cmd/ao/completion_values_test.go`, command-specific completion tests.
   - Acceptance shape: enum flags advertised as `a|b|c` in help have `RegisterFlagCompletionFunc` coverage.

7. **Reconcile complexity policy with the repo Go standard.**
   - Owner files: `cli/.golangci.yml`, `scripts/check-go-complexity.sh`, `docs/standards/golang-style-guide.md`, high-complexity command files.
   - Test surfaces: `cd cli && make lint`, CI go-build complexity step.
   - Acceptance shape: either the style guide matches the enforced warn/fail thresholds, or the gates ratchet toward CC <= 10 with an allowlist/waiver mechanism for existing functions.

## Coverage And Validation Notes

- Explored docs/index/newcomer, CLI README, root command/app entrypoint, generated command reference, Makefile, CI workflow, JSON consistency tests, command-surface parity scripts, completion code, representative command handlers, internal Options-pattern packages, and local Go style guide.
- Scoped git history over CLI paths shows recent activity in CLI tests, command docs/surface gates, shell completions, doctor, compile, overnight, and performance work; this supports treating the CLI as active and evolving rather than static.
- No tests were run for this research subtask; this was a local code exploration and evidence capture.
