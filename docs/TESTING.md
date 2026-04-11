# Testing Guide

Testing ensures AgentOps skills, hooks, and the `ao` CLI work correctly across changes. This guide covers what tests exist, how to run them, and how to write new ones.

## Test Types

| Type | Location | Description |
|------|----------|-------------|
| Unit (Go) | `cli/**/*_test.go` | Go unit tests for CLI internals |
| Integration | `tests/integration/` | Shell scripts testing CLI commands, skill invocation, hook chains |
| Smoke | `tests/smoke-test.sh` | Quick sanity checks that the plugin loads correctly |
| Windows smoke | `tests/windows/test-windows-smoke.ps1` | Native Windows smoke for PowerShell installers, Codex plugin staging, `ao doctor` hints, and focused Windows-sensitive Go tests |
| Contract | `tests/hooks/*.bats`, `scripts/check-contract-compatibility.sh` | Schema and contract validation |
| BATS | `tests/hooks/*.bats`, `tests/scripts/*.bats` | Unit tests for shell hooks and scripts using the BATS framework |
| E2E | `tests/e2e/` | Full pipeline proof runs |
| Skill | `tests/skills/`, `skills/*/scripts/validate.sh` | Skill structure, frontmatter, and behavior validation |
| Doc | `tests/docs/` | Documentation link and count validation |

## Test Tiers

The master runner `tests/run-all.sh` organizes tests into tiers by speed and dependency requirements:

| Tier | Name | Requires | What it covers |
|------|------|----------|----------------|
| 1 | Static Validation | Nothing (runs offline) | Manifest schemas, JSON validity, GOALS.yaml, doc links, skill counts, token budgets, artifact consistency |
| 2 | Smoke Tests | Claude CLI | Plugin load test, `smoke-test.sh`, Codex integration |
| 3 | Functional Tests | Claude CLI, Go | Explicit skill requests, natural language triggering, Claude Code unit tests, release smoke tests, integration tests |

Run a specific tier:

```bash
./tests/run-all.sh              # Tier 1 only (fast, no CLI needed)
./tests/run-all.sh --tier=2     # Tier 1 + 2
./tests/run-all.sh --tier=3     # Tier 1 + 2 + 3
./tests/run-all.sh --all        # All tiers
```

## Running Tests Locally

| Scenario | Command | Approx. Time |
|----------|---------|-------------|
| Quick static validation | `./tests/run-all.sh` | ~10s |
| Full test suite | `./tests/run-all.sh --all` | 2-5 min |
| Go unit tests | `cd cli && make test` | ~15s |
| Push-time local gate | `scripts/pre-push-gate.sh` | ~30-90s |
| Activate repo hooks | `bash scripts/install-dev-hooks.sh` | ~1s |
| Go build + vet + changed-scope race | `scripts/validate-go-fast.sh` | ~20s |
| BATS hook tests | `bats tests/hooks/*.bats` | ~10s |
| BATS script tests | `bats tests/scripts/*.bats` | ~10s |
| Skill validation | `tests/skills/run-all.sh` | ~30s |
| Skill integrity (heal) | `bash skills/heal-skill/scripts/heal.sh --strict` | ~15s |
| Doc validation | `./tests/docs/validate-doc-release.sh` | ~10s |
| Contract compatibility | `./scripts/check-contract-compatibility.sh` | ~10s |
| Full CI gate (local) | `scripts/ci-local-release.sh` | 5-10 min |
| Native Windows smoke | `powershell -ExecutionPolicy Bypass -File .\tests\windows\test-windows-smoke.ps1` | ~1-3 min |

## Writing New Tests

## Local Hooking

Use the repo-managed hooks, not ad hoc `.git/hooks` symlinks:

```bash
bash scripts/install-dev-hooks.sh
```

That activates `.githooks/pre-commit` and `.githooks/pre-push` for the current clone/worktree. The pre-push hook runs `scripts/pre-push-gate.sh`.

### Where to put tests

| Test type | Directory |
|-----------|-----------|
| Go unit tests | Next to the source file in `cli/` (e.g., `cli/internal/goals/measure_test.go`) |
| Hook tests (BATS) | `tests/hooks/` |
| Script tests (BATS) | `tests/scripts/` |
| Skill validation | `skills/<name>/scripts/validate.sh` |
| Integration tests | `tests/integration/test-<name>.sh` |
| Native Windows smoke | `tests/windows/` |
| E2E proof runs | `tests/e2e/` |
| Doc validation | `tests/docs/` |
| Goal validation | `tests/goals/` |
| Lint allowlists | `tests/lint/` |

### Naming conventions

- Go test files: name after the source file they test (e.g., `measure.go` -> `measure_test.go`).
- **No `cov*_test.go` naming.** Test files must not use the `cov*` prefix convention.
- BATS files: `<descriptive-name>.bats` in the appropriate `tests/` subdirectory.
- Shell integration tests: `test-<name>.sh`.

### Assertion rules

- **No coverage-padding tests.** Every test must assert behavioral correctness, not just presence. Tests that use trivial `!= ""` or `!= nil` assertions solely to inflate coverage metrics are banned.
- If a function's coverage is low, write a real test that validates behavior or accept the metric gap.

## Go Testing Rules

### Coverage floor

The Go coverage floor is **84%**. CI enforces this. Run coverage locally:

```bash
cd cli && go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
```

### Command / test pairing

Each CLI command file in `cli/cmd/ao/` should have a corresponding `*_test.go` file. Tests should exercise:

- Flag parsing and defaults
- JSON output mode (`--json`)
- Error paths (missing args, invalid input)
- Behavioral correctness of the command's core logic

### Assertion density

Tests must make meaningful assertions about output content, exit codes, and side effects. A test that only checks `err == nil` without validating the result is insufficient.

## Hook Testing (BATS)

Hooks are tested using the [BATS](https://github.com/bats-core/bats-core) framework. Test files live in `tests/hooks/`.

### Existing BATS test files

| File | Covers |
|------|--------|
| `test-hooks.bats` | All hook categories (prompt-nudge, session-start, kill switch, etc.) |
| `hook-output-schema.bats` | Hook output JSON schema contracts |
| `hook-stdin-contracts.bats` | Hook stdin JSON input contracts |
| `constraint-compiler.bats` | Constraint compiler logic |
| `lib-hook-helpers.bats` | Unit tests for `lib/hook-helpers.sh` functions |

### Writing a BATS test

```bash
#!/usr/bin/env bats

setup() {
    load helpers/test_helper
    _helper_setup
    export CLAUDE_SESSION_ID="bats-test-$$"
}

teardown() {
    _helper_teardown
}

@test "my-hook: does the expected thing" {
    RESULT=$(bash "$HOOKS_DIR/my-hook.sh" 2>/dev/null)
    echo "$RESULT" | jq -e '.hookSpecificOutput.someField == "expected"'
}

@test "my-hook: kill switch suppresses output" {
    OUTPUT=$(AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/my-hook.sh" 2>&1 || true)
    [ -z "$OUTPUT" ]
}
```

### Running BATS tests

```bash
# All hook tests
bats tests/hooks/*.bats

# Single file
bats tests/hooks/test-hooks.bats

# Verbose output
bats --verbose-run tests/hooks/*.bats
```

## Skill Testing

### Per-skill validation

Each skill can have a `scripts/validate.sh` that checks skill-specific invariants. The runner `tests/skills/run-all.sh` iterates over all skills in `skills/` and:

1. Verifies `SKILL.md` exists with YAML frontmatter and a `name:` field.
2. Checks declared dependencies exist as sibling skill directories.
3. Runs `scripts/validate.sh` if present.
4. Runs lint checks (`lint-skills.sh`), Claude feature coverage, and alias collision detection.

### Running skill tests

```bash
# Full skill validation suite
tests/skills/run-all.sh

# Skill integrity check (references, orphan files, structure)
bash skills/heal-skill/scripts/heal.sh --strict
```

### heal.sh --strict

The heal script validates that every file in `skills/<name>/references/` is linked from the skill's `SKILL.md`. Missing links break CI.

## Quarantine Policy

Tests requiring external services (API calls, network access, live Claude/Codex sessions) that cannot be mocked are placed in `tests/_quarantine/`.

**Promotion path:** To move a quarantined test into the main suite:

1. Replace external calls with mocks or API stubs so the test runs headlessly.
2. Move the test file to the appropriate `tests/` subdirectory.
3. Verify it passes in CI without network access.

Quarantined tests are excluded from the default `run-all.sh` tiers and CI. Standalone runtime smoke tests that do not require live runtimes belong in `tests/skills/` or `tests/scripts/` and are expected to run in CI.

## Test Directory Map

| Directory | Purpose |
|-----------|---------|
| `tests/hooks/` | BATS unit tests for hook scripts (`hooks/*.sh`) |
| `tests/skills/` | Skill validation scripts plus standalone runtime smoke tests (Claude Code, Codex, OpenCode) |
| `tests/spec-consistency/` | Spec consistency gates across manifests and docs |
| `tests/goals/` | Goal validation and measurement (`GOALS.yaml` / `GOALS.md`) |
| `tests/lint/` | Lint allowlists and code style checks |
| `tests/explicit-skill-requests/` | Tests for explicit skill trigger patterns |
| `tests/cli/` | CLI flag consistency and behavior tests |
| `tests/windows/` | Native Windows installer, plugin staging, and focused CLI portability smoke tests |
| `tests/e2e/` | End-to-end proof runs (full pipeline) |
| `tests/docs/` | Documentation validation (links, skill counts, goal counts) |
| `tests/scripts/` | BATS tests for repo scripts (`scripts/*.sh`) |
| `tests/integration/` | Integration tests (CLI commands, skill invocation, hook chains) |
| `tests/fixtures/` | Shared test fixtures and sample data |
| `tests/lib/` | Shared test helpers and color utilities |
| `tests/_quarantine/` | Quarantined tests requiring external services |
