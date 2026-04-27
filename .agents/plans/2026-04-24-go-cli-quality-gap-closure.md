---
id: plan-2026-04-24-go-cli-quality-gap-closure
type: plan
date: 2026-04-24
source: "[[.agents/research/2026-04-24-go-cli-quality-gap-analysis.md]]"
ranked_packet: "[[.agents/rpi/ranked-packet-2026-04-24-go-cli-quality.md]]"
---

# Plan: Go CLI Quality Gap Closure

## Context

The `ao` CLI is already useful: it is the repo-native control plane for AgentOps setup, retrieval, RPI, Codex lifecycle, factory startup, goals, findings, and overnight operation. The gap to best-in-class Go CLIs is consistency at scale: stricter automation output contracts, safer live integration boundaries, richer completion/docs, thinner command layers, and a command surface that is easier to understand.

Applied findings:
- `f-2026-04-14-001` - Every issue touching `cli/cmd/ao` production files includes paired command tests.
- `f-2026-04-14-002` - Child beads must cite this durable plan/research packet and checked-in paths, not ephemeral seed files.
- `.agents/learnings/2026-04-14-command-refactors-need-paired-tests.md` - Command refactors must pair production helper splits with direct tests in the same issue.
- `.agents/learnings/2026-04-12-yagni-bridge-not-clone.md` - Do not rewrite working systems just to make the CLI feel cleaner; bridge and standardize existing paths first.

## Files to Modify

| File | Change |
|------|--------|
| `cli/cmd/ao/gc_bridge.go` | Harden `gcBridgeVersion` against wrong `gc` binaries and empty stdout. |
| `cli/cmd/ao/gc_bridge_test.go` | Fix live test panic and add deterministic unit cases. |
| `cli/internal/bridge/gc.go` | Add parser or compatibility helper only if it belongs in shared bridge logic. |
| `scripts/check-cmdao-surface-parity.sh` | Emit or consume command-surface classification. |
| `scripts/cmdao-surface-allowlist.txt` | Reclassify allowlisted commands with explicit reasons. |
| `scripts/generate-cli-reference.sh` | Enrich generated docs from command help metadata without losing parity. |
| `cli/docs/COMMANDS.md` | Regenerated reference output. |
| `docs/cli-surface.md` | NEW - public/internal/deprecated/stateful command classification and ownership notes. |
| `docs/cli-surface.json` | NEW - machine-readable command-surface sidecar for tests and gates. |
| `tests/cli/test-json-flag-consistency.sh` | Turn invalid non-empty JSON from warning into failure for supported commands. |
| `cli/cmd/ao/flag_matrix_test.go` | Expand binary-level JSON/output matrix from the classified command surface. |
| `cli/cmd/ao/json_validity_test.go` | Add or adjust focused JSON contract cases for commands touched by the plan. |
| `cli/cmd/ao/completion.go` | Add PowerShell completion generation and route output through command writers. |
| `cli/cmd/ao/completion_test.go` | Add PowerShell and writer-injection tests. |
| `cli/cmd/ao/completion_values.go` | Add shared enum completion helpers if needed. |
| `cli/cmd/ao/completion_values_test.go` | Assert enum-like flags have completion coverage. |
| `cli/cmd/ao/rpi_phased.go` | Register completions for enum-like flags such as runtime and phase selectors. |
| `cli/cmd/ao/rpi_loop.go` | Register completions for enum-like loop policy flags if present. |
| `cli/cmd/ao/contradict.go` | Pilot writer/App or Options migration for a small command. |
| `cli/cmd/ao/contradict_test.go` | Add direct JSON/table tests that do not require global stdout capture. |
| `cli/cmd/ao/notebook.go` | Pilot Options/writer migration for a stateful command. |
| `cli/cmd/ao/notebook_test.go` | Add quiet, no-memory, and writer-injection coverage. |
| `cli/cmd/ao/app.go` | Extend helper accessors only if the pilot needs shared App defaults. |
| `docs/standards/golang-style-guide.md` | Align complexity guidance with enforced policy or document ratchet path. |
| `cli/.golangci.yml` | Align gocyclo threshold only if policy changes. |
| `scripts/check-go-complexity.sh` | Add reporting/ratchet mode if needed for policy reconciliation. |
| `.github/workflows/validate.yml` | Update complexity thresholds only if policy changes. |

## Boundaries

**Always:** Preserve existing CLI behavior unless an issue explicitly changes the contract; keep generated docs generated; run paired tests for every production command file touched; cite durable plan/research artifacts in bead closure notes.

**Ask First:** Renaming commands, hiding commands from public docs, changing exit-code semantics, or tightening complexity fail thresholds below the current CI policy.

**Never:** Rewrite the whole CLI, replace Cobra, clone external systems, or make broad RPI runtime behavior changes as part of this quality pass.

## Baseline Audit

| Metric | Command | Result |
|--------|---------|--------|
| Go files under `cli/` | `find cli -name '*.go' -not -path '*/vendor/*' \| wc -l` | 762 |
| Production Go LOC under `cli/` | `find cli -name '*.go' -not -name '*_test.go' -print0 \| xargs -0 wc -l` | 100,180 |
| Test Go LOC under `cli/` | `find cli -name '*_test.go' -print0 \| xargs -0 wc -l` | 196,122 |
| Production Go LOC under `cli/cmd/ao` | `find cli/cmd/ao -name '*.go' -not -name '*_test.go' -print0 \| xargs -0 wc -l` | 57,765 |
| Test Go LOC under `cli/cmd/ao` | `find cli/cmd/ao -name '*_test.go' -print0 \| xargs -0 wc -l` | 140,955 |
| Documented top-level commands | `awk '/^### \`ao / {count++} END {print count}' cli/docs/COMMANDS.md` | 55 |
| Largest command files | `find cli/cmd/ao -name '*.go' -not -name '*_test.go' -print0 \| xargs -0 wc -l \| sort -nr \| head` | `overnight.go` 1,587; `rpi_loop.go` 1,347; `codex.go` 1,344; `hooks.go` 1,180; `rpi_loop_supervisor.go` 1,140 |
| Production functions over CC 15 | `gocyclo -over 15 cli/cmd/ao cli/internal` | 38 production functions; max CC 19 |
| Build | `cd cli && make build` | PASS |
| Vet | `cd cli && go vet ./...` | PASS |
| CLI docs parity | `./scripts/generate-cli-reference.sh --check` | PASS |
| Focused JSON tests | `cd cli && go test ./cmd/ao -run 'TestJSONValidity|TestFlagMatrix_JSONOutput'` | PASS |
| Full Go suite | `cd cli && go test ./...` | FAIL: `TestGCBridgeVersion_Live` panics on empty version output |

## Implementation

### 1. Harden the live `gc` bridge boundary

In `cli/cmd/ao/gc_bridge.go`:

- Modify `gcBridgeVersion(execCommand gcExecFn) (string, error)` at `cli/cmd/ao/gc_bridge.go:60` so it rejects empty stdout and wrong-tool output with a contextual error. Use `CombinedOutput` or another stderr-aware path so `/usr/bin/gc version` returning stderr-only text cannot be treated as a valid version.
- Add helper if useful:

```go
func parseGCVersionOutput(output []byte) (string, error) {
    version := strings.TrimSpace(string(output))
    if version == "" {
        return "", fmt.Errorf("gc version returned empty output")
    }
    if !gcBridgeCompatible(version) {
        return "", fmt.Errorf("gc version %q below minimum %s or not the expected gc tool", version, gcMinVersion)
    }
    return version, nil
}
```

In `cli/cmd/ao/gc_bridge_test.go`:

- Change `TestGCBridgeVersion_Live` at `cli/cmd/ao/gc_bridge_test.go:748` so wrong-tool or empty-version cases call `t.Skipf` with the error, not `t.Error` followed by `v[0]`.
- Add unit tests for empty output, stderr-only output, semver output, and incompatible output.

Key functions to reuse:
- `gcBridgeCompatible` at `cli/cmd/ao/gc_bridge.go:68`.
- `bridge.GCBridgeCompatible` through the alias at `cli/cmd/ao/gc_bridge.go:68`.

### 2. Add command-surface taxonomy and richer generated docs

In `scripts/check-cmdao-surface-parity.sh` and `scripts/cmdao-surface-allowlist.txt`:

- Convert allowlist rows into explicit categories: `public-tested`, `public-stateful-fixture-needed`, `internal-hidden`, `deprecated`, `unsafe-live`, or `manual-only`.
- Fail when an allowlist row has no category/reason.

In `docs/cli-surface.md`:

- Create a generated-or-maintained table with command, category, owner area, JSON contract, completion status, docs status, and test status.

In `docs/cli-surface.json`:

- Emit the same command-surface inventory in a machine-readable shape so Issue 3 and Issue 4 consume structured data rather than parsing prose Markdown.
- Include at least these fields per command: `command`, `category`, `owner_area`, `json_contract`, `completion_status`, `docs_status`, `test_status`, and `reason`.
- Treat `docs/cli-surface.md` as the human rendering and `docs/cli-surface.json` as the script/test contract.

In `scripts/generate-cli-reference.sh`:

- Keep the current generated docs parity behavior.
- Add generated sections only from stable Cobra/help metadata: aliases, examples, output modes, and failure notes when present.
- Do not hand-edit `cli/docs/COMMANDS.md`; regenerate it.

Validation:

```bash
./scripts/generate-cli-reference.sh --check
bash scripts/check-cmdao-surface-parity.sh
```

### 3. Harden machine-readable output contracts

In `tests/cli/test-json-flag-consistency.sh`:

- Preserve unknown `--json` as failure.
- Convert "stdout exists but is not valid JSON" from `warn` to `fail` for commands classified as supporting JSON.
- Keep empty stdout as warning only for commands classified as stateful or args-required.
- Consume `docs/cli-surface.json` from Issue 2 instead of duplicating classification in the script or parsing `docs/cli-surface.md`.

In `cli/cmd/ao/flag_matrix_test.go`:

- Expand `TestFlagMatrix_JSONOutput` beyond the current curated list at `cli/cmd/ao/flag_matrix_test.go:33` using the Issue 2 classification.
- Add negative-path tests for commands that accept `--json` but require state/arguments, so they either emit valid structured error JSON or are classified out of the supported set.

In `cli/cmd/ao/json_validity_test.go`:

- Add command-level cases for any command touched by this plan.

Validation:

```bash
cd cli && make build
bash tests/cli/test-json-flag-consistency.sh ./cli/bin/ao
cd cli && go test ./cmd/ao -run 'TestJSONValidity|TestFlagMatrix_JSONOutput'
```

### 4. Expand shell completion coverage

In `cli/cmd/ao/completion.go`:

- Add `powershell` to `Use`, `ValidArgs`, and `RunE`.
- Route generator output to `cmd.OutOrStdout()` rather than `os.Stdout`, preserving existing generated output behavior.

In `cli/cmd/ao/completion_test.go`:

- Add PowerShell generation test.
- Assert completion output can be captured via command writers.

In `cli/cmd/ao/completion_values.go` and `cli/cmd/ao/completion_values_test.go`:

- Add tests for enum-like flags discovered from `cli/docs/COMMANDS.md`.
- Register completions for high-value enum flags in `cli/cmd/ao/rpi_phased.go` and `cli/cmd/ao/rpi_loop.go`, starting with flags whose help already enumerates values.

Validation:

```bash
cd cli && go test ./cmd/ao -run 'Test.*Completion'
cd cli && go run ./cmd/ao completion powershell >/tmp/ao.ps1
./scripts/generate-cli-reference.sh --check
```

### 5. Pilot thin command Options/writer migration

Use two commands as pilots, not a package-wide rewrite.

In `cli/cmd/ao/contradict.go`:

- Change `runContradict(_ *cobra.Command, _ []string) error` at `cli/cmd/ao/contradict.go:88` to accept the command, derive `app := AppFromContext(cmd.Context())`, and write JSON/table output to `cmd.OutOrStdout()` or `app.Stdout`.
- Keep `ContradictResult` unchanged.
- Add helper `renderContradictResult(w io.Writer, result ContradictResult, format string) error`.

In `cli/cmd/ao/notebook.go`:

- Introduce a small options struct for `runNotebookUpdate`:

```go
type notebookUpdateOptions struct {
    CWD        string
    MemoryFile string
    Quiet      bool
    MaxLines   int
    Source     string
    SessionID  string
    Stdout     io.Writer
}
```

- Keep Cobra flag names unchanged.
- Route user-visible output at `cli/cmd/ao/notebook.go:71`, `cli/cmd/ao/notebook.go:82`, `cli/cmd/ao/notebook.go:90`, and `cli/cmd/ao/notebook.go:122` through the injected writer path.

Validation:

```bash
cd cli && go test ./cmd/ao -run 'Test.*Contradict|Test.*Notebook'
cd cli && go test ./cmd/ao -run 'TestJSONValidity_(AntiPatterns|Config|Status)'
```

### 6. Reconcile complexity policy and hot-file budget

In `docs/standards/golang-style-guide.md`:

- Resolve drift between the aspirational CC <= 10 guidance at `docs/standards/golang-style-guide.md:446` and the current CI policy.

In `cli/.golangci.yml`, `scripts/check-go-complexity.sh`, and `.github/workflows/validate.yml`:

- Either keep current warn 15/fail 25 and update docs to match, or add a ratchet mode that preserves fail 25 while making touched functions over 15 require a refactor note.
- Do not lower the fail threshold without a separate baseline and migration plan.

Add a hot-file budget section to `docs/cli-surface.md` or a dedicated `docs/cli-quality.md`:

- List top command files by LOC.
- State "when touching files >500 LOC, keep net production LOC neutral unless adding a feature, and move reusable logic into `cli/internal`."

Validation:

```bash
bash scripts/check-go-complexity.sh --base HEAD~1 --warn 15 --fail 25
cd cli && make lint
```

## Tests

| Test | Level | Rationale |
|------|-------|-----------|
| `TestGCBridgeVersion_EmptyOutput` | L1 | Helper behavior in isolation. |
| `TestGCBridgeVersion_Live` | L2 | External binary boundary with skip-on-wrong-tool semantics. |
| `TestFlagMatrix_JSONOutput` expanded cases | L2 | Built binary output contract. |
| `tests/cli/test-json-flag-consistency.sh` | L0 | Contract gate over supported JSON commands. |
| `TestCompletionPowerShell` | L1 | Completion command generation branch. |
| `TestCompletionEnumFlagCoverage` | L0/L1 | Static contract over enum-like flags and registered completions. |
| `TestContradict_WriterInjection` | L1 | Command rendering no longer depends on process stdout. |
| `TestNotebookUpdate_WriterInjection` | L1 | Stateful command uses injected writer and tempdir state. |
| `scripts/generate-cli-reference.sh --check` | L0 | Generated docs parity. |
| `scripts/check-go-complexity.sh --base HEAD~1 --warn 15 --fail 25` | L0 | Complexity policy gate. |

Test levels metadata:

```json
{
  "test_levels": {
    "required": ["L0", "L1", "L2"],
    "recommended": [],
    "rationale": "The plan changes CLI contracts, command tests, generated docs, completion generation, and one external binary boundary. It does not require L3 unless RPI runtime orchestration behavior changes."
  }
}
```

## Conformance Checks

| Issue | Check Type | Check |
|-------|------------|-------|
| Issue 1 | tests | `cd cli && go test ./cmd/ao -run 'TestGCBridge'` |
| Issue 1 | command | `cd cli && go test ./...` no longer fails because of `TestGCBridgeVersion_Live` on a wrong `/usr/bin/gc` binary. |
| Issue 2 | files_exist | `["docs/cli-surface.md", "docs/cli-surface.json"]` |
| Issue 2 | command | `./scripts/generate-cli-reference.sh --check` |
| Issue 2 | command | `bash scripts/check-cmdao-surface-parity.sh` |
| Issue 3 | command | `bash tests/cli/test-json-flag-consistency.sh ./cli/bin/ao` |
| Issue 3 | tests | `cd cli && go test ./cmd/ao -run 'TestJSONValidity|TestFlagMatrix_JSONOutput'` |
| Issue 4 | tests | `cd cli && go test ./cmd/ao -run 'Test.*Completion'` |
| Issue 4 | command | `cd cli && go run ./cmd/ao completion powershell >/tmp/ao.ps1` |
| Issue 5 | tests | `cd cli && go test ./cmd/ao -run 'Test.*Contradict|Test.*Notebook'` |
| Issue 5 | content_check | `{file: "cli/cmd/ao/contradict.go", pattern: "renderContradictResult"}` |
| Issue 6 | command | `bash scripts/check-go-complexity.sh --base HEAD~1 --warn 15 --fail 25` |
| Issue 6 | lint | `cd cli && make lint` |

## Verification

1. Build and focused tests:

```bash
cd cli && make build
cd cli && go vet ./...
cd cli && go test ./cmd/ao -run 'TestGCBridge|TestJSONValidity|TestFlagMatrix_JSONOutput|Test.*Completion|Test.*Contradict|Test.*Notebook'
```

2. Generated docs and CLI gates:

```bash
./scripts/generate-cli-reference.sh --check
bash scripts/check-cmdao-surface-parity.sh
bash tests/cli/test-json-flag-consistency.sh ./cli/bin/ao
bash scripts/check-go-complexity.sh --base HEAD~1 --warn 15 --fail 25
```

3. Full validation before closing the epic:

```bash
cd cli && go test ./...
scripts/pre-push-gate.sh --fast
```

## Issues

### Issue 1: Harden live `gc` bridge detection and tests

**Dependencies:** None

**Description:** Fix the reproduced full-suite failure where `/usr/bin/gc version` exits 0 with empty stdout and stderr text, causing `TestGCBridgeVersion_Live` to panic. Update `gcBridgeVersion`, add deterministic parser tests, and make the live test skip or report incompatibility instead of indexing an empty string.

**Acceptance:** `cd cli && go test ./cmd/ao -run 'TestGCBridge'` passes; `cd cli && go test ./...` is no longer blocked by `TestGCBridgeVersion_Live` on hosts where `gc` is the wrong binary.

**Files:** `cli/cmd/ao/gc_bridge.go`, `cli/cmd/ao/gc_bridge_test.go`, optionally `cli/internal/bridge/gc.go`.

**Validation:** L1 unit parser tests plus L2 live boundary test.

### Issue 2: Classify the CLI command surface and enrich generated docs

**Dependencies:** None

**Description:** Create a durable command-surface taxonomy and make generated docs more useful without hand-editing `cli/docs/COMMANDS.md`. Classify public/internal/stateful/deprecated/manual-only commands, require reasons for allowlist rows, emit `docs/cli-surface.json` as the machine-readable sidecar, and extend the docs generator to include stable metadata such as examples and aliases.

**Acceptance:** `docs/cli-surface.md` and `docs/cli-surface.json` exist; every `scripts/cmdao-surface-allowlist.txt` row has a category and reason; JSON output/completion gates can consume the sidecar without parsing Markdown; `./scripts/generate-cli-reference.sh --check` and `bash scripts/check-cmdao-surface-parity.sh` pass.

**Files:** `scripts/check-cmdao-surface-parity.sh`, `scripts/cmdao-surface-allowlist.txt`, `scripts/generate-cli-reference.sh`, `cli/docs/COMMANDS.md`, `docs/cli-surface.md`, `docs/cli-surface.json`.

**Validation:** L0 generated-docs and command-surface gates.

### Issue 3: Harden machine-readable output contracts

**Dependencies:** Issue 2

**Description:** Use the command classification sidecar from Issue 2 to make JSON output failures mechanical. Invalid non-empty JSON from commands that claim `--json` support should fail the gate. Stateful commands should be explicitly classified instead of silently tolerated.

**Acceptance:** Invalid JSON is a failure for supported commands; stateful commands have explicit classification; focused JSON tests pass.

**Files:** `tests/cli/test-json-flag-consistency.sh`, `cli/cmd/ao/flag_matrix_test.go`, `cli/cmd/ao/json_validity_test.go`, `docs/cli-surface.json`.

**Validation:** L0 shell contract gate plus L2 binary-level JSON matrix.

### Issue 4: Add PowerShell and enum-like shell completion coverage

**Dependencies:** Issue 2

**Description:** Add PowerShell completion generation and register completion functions for high-value enum-like flags. Start with flags already advertising finite value sets in help output, especially RPI runtime/phase selectors.

**Acceptance:** `ao completion powershell` works; completion output is capturable through Cobra writers; enum-like flags selected in the issue have tests proving completion coverage.

**Files:** `cli/cmd/ao/completion.go`, `cli/cmd/ao/completion_test.go`, `cli/cmd/ao/completion_values.go`, `cli/cmd/ao/completion_values_test.go`, `cli/cmd/ao/rpi_phased.go`, `cli/cmd/ao/rpi_loop.go`, `cli/docs/COMMANDS.md`.

**Validation:** L1 completion tests plus generated-doc parity.

### Issue 5: Pilot thin command Options/writer migration

**Dependencies:** Issue 3

**Description:** Migrate `contradict` and `notebook update` as pilot commands for the intended App/Options pattern. Keep command behavior and flags stable, but route output through injected writers and reduce reliance on process-global stdout and package-global state in tests.

**Acceptance:** Pilot commands have writer-injection tests; JSON/table behavior remains stable; production command changes are paired with direct tests in the same issue.

**Files:** `cli/cmd/ao/contradict.go`, `cli/cmd/ao/contradict_test.go`, `cli/cmd/ao/notebook.go`, `cli/cmd/ao/notebook_test.go`, optionally `cli/cmd/ao/app.go`.

**Validation:** L1 command tests and focused JSON tests.

### Issue 6: Reconcile Go complexity policy and hot-file budget

**Dependencies:** None

**Description:** Align the Go style guide, golangci config, CI complexity gate, and contributor expectations. Do not lower the fail threshold without a separate migration plan. Add a hot-file budget so future CLI work knows how to treat 500+ LOC command files.

**Acceptance:** Docs and gates agree on warn/fail semantics; the hot-file policy is documented; current complexity gate still passes.

**Files:** `docs/standards/golang-style-guide.md`, `cli/.golangci.yml`, `scripts/check-go-complexity.sh`, `.github/workflows/validate.yml`, optionally `docs/cli-quality.md`.

**Validation:** L0 complexity gate plus lint.

## Execution Order

**Wave 1** (parallel): Issue 1, Issue 2, Issue 6

**Wave 2** (after Issue 2): Issue 3, Issue 4

**Wave 3** (after Issue 3): Issue 5

## Planning Rules Compliance

| Rule | Status | Justification |
|------|--------|---------------|
| PR-001: Mechanical Enforcement | PASS | Every issue has command, test, content, or file-existence conformance checks. |
| PR-002: External Validation | PASS | Validation uses Go tests, shell gates, generated-doc checks, and CI-aligned scripts, not implementer self-assessment. |
| PR-003: Feedback Loops | PASS | Output contracts feed JSON gates; command classification feeds docs/tests/completion; complexity policy feeds future CLI work. |
| PR-004: Separation Over Layering | PASS | The plan pilots Options/writer migration in two commands instead of layering more globals. |
| PR-005: Process Gates First | PASS | Issues 2, 3, and 6 establish classification/gates before broader command refactors. |
| PR-006: Cross-Layer Consistency | PASS | CLI docs, scripts, tests, and CI policy are updated in the same issue where their contract changes. |
| PR-007: Phased Rollout | PASS | Wave 1 fixes baseline and classification; Wave 2 tightens output/completion; Wave 3 pilots architecture after gates exist. |

Unchecked rules: 0

## File Dependency Matrix

| Task | File | Access | Notes |
|------|------|--------|-------|
| Issue 1 | `cli/cmd/ao/gc_bridge.go` | write | Version parsing and empty-output error handling. |
| Issue 1 | `cli/cmd/ao/gc_bridge_test.go` | write | Unit/live bridge tests. |
| Issue 1 | `cli/internal/bridge/gc.go` | write | Optional shared parser/compat helper. |
| Issue 2 | `scripts/check-cmdao-surface-parity.sh` | write | Classification enforcement. |
| Issue 2 | `scripts/cmdao-surface-allowlist.txt` | write | Add categories/reasons. |
| Issue 2 | `scripts/generate-cli-reference.sh` | write | Enrich generated docs. |
| Issue 2 | `cli/docs/COMMANDS.md` | write | Regenerated output. |
| Issue 2 | `docs/cli-surface.md` | write | Human command-surface inventory. |
| Issue 2 | `docs/cli-surface.json` | write | Machine-readable inventory consumed by tests and gates. |
| Issue 3 | `tests/cli/test-json-flag-consistency.sh` | write | Harden JSON gate. |
| Issue 3 | `cli/cmd/ao/flag_matrix_test.go` | write | Expand binary matrix. |
| Issue 3 | `cli/cmd/ao/json_validity_test.go` | write | Focused cases. |
| Issue 3 | `docs/cli-surface.json` | read | Consume command classification. |
| Issue 4 | `cli/cmd/ao/completion.go` | write | PowerShell and writer routing. |
| Issue 4 | `cli/cmd/ao/completion_test.go` | write | Completion tests. |
| Issue 4 | `cli/cmd/ao/completion_values.go` | write | Completion helpers. |
| Issue 4 | `cli/cmd/ao/completion_values_test.go` | write | Enum coverage tests. |
| Issue 4 | `cli/cmd/ao/rpi_phased.go` | write | Register enum completions. |
| Issue 4 | `cli/cmd/ao/rpi_loop.go` | write | Register enum completions. |
| Issue 4 | `cli/docs/COMMANDS.md` | write | Regenerated if help changes. |
| Issue 4 | `docs/cli-surface.json` | read | Use classification. |
| Issue 5 | `cli/cmd/ao/contradict.go` | write | Writer/App pilot. |
| Issue 5 | `cli/cmd/ao/contradict_test.go` | write | New focused tests. |
| Issue 5 | `cli/cmd/ao/notebook.go` | write | Options/writer pilot. |
| Issue 5 | `cli/cmd/ao/notebook_test.go` | write | New focused tests. |
| Issue 5 | `cli/cmd/ao/app.go` | write | Optional helper extension. |
| Issue 6 | `docs/standards/golang-style-guide.md` | write | Policy alignment. |
| Issue 6 | `cli/.golangci.yml` | write | Optional threshold alignment. |
| Issue 6 | `scripts/check-go-complexity.sh` | write | Optional ratchet/report mode. |
| Issue 6 | `.github/workflows/validate.yml` | write | Optional threshold alignment. |
| Issue 6 | `docs/cli-quality.md` | write | Optional hot-file budget doc. |

## File-Conflict Matrix

| File | Issues | Mitigation |
|------|--------|------------|
| `docs/cli-surface.md` | Issue 2, Issue 3, Issue 4 | Issue 3 and 4 depend on Issue 2 and only read the human inventory unless updating their own status fields. |
| `docs/cli-surface.json` | Issue 2, Issue 3, Issue 4 | Issue 2 owns the schema/shape. Issue 3 and 4 consume it read-only unless they first update the Issue 2 contract and regenerate both renderings. |
| `cli/docs/COMMANDS.md` | Issue 2, Issue 4 | Issue 4 depends on Issue 2 and must regenerate from the post-Issue-2 generator. |
| `cli/cmd/ao/app.go` | Issue 5 only | No same-wave conflict. |
| `scripts/check-go-complexity.sh` | Issue 6 only | No same-wave conflict. |

## Cross-Wave Shared Files

| File | Wave 1 Issues | Wave 2+ Issues | Mitigation |
|------|---------------|----------------|------------|
| `docs/cli-surface.md` | Issue 2 | Issue 3, Issue 4 | Later issues depend on Issue 2 and branch from post-Wave-1 SHA. |
| `docs/cli-surface.json` | Issue 2 | Issue 3, Issue 4 | Later issues consume the sidecar read-only unless the taxonomy contract is explicitly expanded. |
| `cli/docs/COMMANDS.md` | Issue 2 | Issue 4 | Issue 4 depends on Issue 2 and regenerates docs from current generator. |

## Scaffold Decision

No new project, package, module, or service is required. Step 4.5 scaffold is skipped.
