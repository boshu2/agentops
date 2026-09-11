# Known Framework Footguns for Agent Dispatch

Reference doc to include in agent dispatch prompts. Prevents agents from rediscovering known issues.

See also: `skills/agent-native/SKILL.md` for explicit runtime dispatch and disjoint worker scopes.

## Go / Cobra CLI

- **Cobra global state**: `rootCmd` flags are package-level variables. Tests that call `cmd.Execute()` must save/restore flag values and call `cmd.Flags().Set()` to reset Changed state. Use `executeCommand` helper when available.
- **os.Chdir is process-global**: ~160 test sites use `os.Chdir` because production code calls `os.Getwd()`. Cannot use `t.Parallel()` with these tests. Do NOT try to refactor tests to avoid os.Chdir unless also refactoring production code.
- **Go flat package model**: All `_test.go` files in a directory share a namespace. When multiple agents write tests in the same package, they WILL get duplicate symbol errors. Check `cli/cmd/ao/testutil_test.go` for existing shared helpers before declaring new ones.
- **Stale binary**: Go tests never resolve `cli/bin/ao` — `flag_matrix_test.go` builds the exec'd binary from the package source at test time. Shell suites that reuse a prebuilt `cli/bin/ao` when one exists can still pick up a stale build; set `PROOF_FORCE_BUILD=1` (e2e factory) or delete `cli/bin/ao` when in doubt.

## Shell Environment

- **cp alias**: User shell may alias `cp` to `cp -i` (interactive). Always use `/bin/cp -f` in scripts and agent environments.
- **PATH inheritance**: Agent subshells inherit user aliases and functions. For deterministic behavior, use absolute paths (`/usr/bin/git`, `/bin/rm`) or prefix with `command` to bypass aliases.

## Test Patterns

- **Shared helpers in testutil_test.go**: Before declaring a new test helper, check if it already exists in `cli/cmd/ao/testutil_test.go`. Duplicate declarations cause compile errors.
- **t.TempDir() for isolation**: Always use `t.TempDir()` for test directories -- it auto-cleans and provides unique paths.
- **defer restore for globals**: When mutating package-level variables (flags, config), always `defer func() { varName = oldVal }()` immediately after saving.

## Embedded Assets

- **Regenerate owned projections after editing source**: After editing a skill or its references, run `scripts/regen-all.sh` and inspect the generated changes. Edit the canonical source; do not patch runtime copies.
- **CLI docs drift**: After adding/changing CLI commands or flags, run `scripts/generate-cli-reference.sh`. CI checks for drift.

## Scope Overflow

- **Scope-escape template**: When a task exceeds an agent's mandate, use the structured template at `docs/contracts/scope-escape-report.md`. Produce an audit instead of forcing a bad fix. This is the sanctioned behavior for unexpected scope overflow.
- **File count ceiling**: Single-agent tasks should touch ~6 files max. Beyond that, split into subtasks or use scope-escape.

## Maintenance Protocol

This document is a living reference. Update it during every postmortem cycle.

**When to add an entry:**
- A postmortem discovers a framework/platform surprise that wasted agent time
- A swarm worker hits a known limitation not documented here
- A new tool or library introduces a gotcha

**How to add an entry:**
1. Add under the appropriate category header (Go/Cobra CLI, Shell Environment, Test Patterns, Embedded Assets)
2. Create a new category if none fits
3. Format: `- **Bold name**: Description of the footgun and how to avoid it`
4. Include the relevant file path or code reference

**Update cadence:** Every `/postmortem` should check: "Did we discover a new footgun?" If yes, add it here in the same cycle — not next cycle.
