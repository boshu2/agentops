#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  CHECKER="$REPO_ROOT/scripts/check-go-cli-architecture.sh"
}

run_architecture_fixture() {
  local fixture="$REPO_ROOT/tests/fixtures/go-cli-architecture/$1"
  run bash -c "cd '$REPO_ROOT/cli' && go run ./internal/archcheck/cmd --root '$fixture'"
}

@test "architecture checker induced fixtures cover every forbidden boundary" {
  run "$CHECKER" --self-test
  [ "$status" -eq 0 ]
  [[ "$output" == *"self-test PASS"* ]]
}

@test "architecture checker accepts the pre-migration command-module tree" {
  run "$CHECKER"
  [ "$status" -eq 0 ]

  run "$CHECKER" --all-migrated
  [ "$status" -eq 0 ]
}

@test "architecture checker emits a deterministic literal family inventory" {
  scope="$BATS_TEST_TMPDIR/family-beads-scope.json"
  run "$CHECKER" --inventory --family beads --out "$scope"
  [ "$status" -eq 0 ]
  test -s "$scope"
  run jq -e '.schema_version == 1 and .family == "beads" and (.owner_files | length > 0) and (.legacy_symbols | length > 0) and (.allowed_paths | length > 0)' "$scope"
  [ "$status" -eq 0 ]
}

@test "architecture checker rejects an induced command process effect" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/cli/internal/commands/demo"
  cat >"$fixture/cli/internal/commands/demo/module.go" <<'GO'
package demo
import "os/exec"
func run() { _ = exec.Command("forbidden") }
GO
  run bash -c "cd '$REPO_ROOT/cli' && go run ./internal/archcheck/cmd --root '$fixture'"
  [ "$status" -eq 1 ]
  [[ "$output" == *"effect.process"* ]]
}

@test "semantic seal rejects an uncancellable command module" {
  fixture="$BATS_TEST_TMPDIR/uncancellable-repo"
  mkdir -p "$fixture/cli/internal/commands/demo"
  cat >"$fixture/cli/internal/commands/demo/module.go" <<'GO'
package demo

import "context"

type Executor interface {
	Execute(context.Context, []string) error
}

func Run(executor Executor, args []string) error {
	return executor.Execute(context.Background(), args)
}
GO

  run bash -c "cd '$REPO_ROOT/cli' && go run ./internal/archcheck/cmd --root '$fixture'"
  [ "$status" -eq 1 ]
  [[ "$output" == *"context"* ]]
}

@test "semantic seal rejects divergent tracker execution" {
  run_architecture_fixture "tracker-execution/mutated"
  [ "$status" -eq 1 ]
  [[ "$output" == *"semantic.tracker-execution"* ]]

  run_architecture_fixture "tracker-execution/valid"
  [ "$status" -eq 0 ]
}

@test "semantic seal rejects undeclared universal effects" {
  run_architecture_fixture "effects/mutated"
  [ "$status" -eq 1 ]
  [[ "$output" == *"semantic.effects"* ]]

  run_architecture_fixture "effects/valid"
  [ "$status" -eq 0 ]
}

@test "semantic seal rejects untruthful structured output" {
  run_architecture_fixture "output/mutated"
  [ "$status" -eq 1 ]
  [[ "$output" == *"semantic.output"* ]]

  run_architecture_fixture "output/valid"
  [ "$status" -eq 0 ]
}

@test "semantic seal rejects a missing recursive runnable contract" {
  run_architecture_fixture "recursive-contracts/mutated"
  [ "$status" -eq 1 ]
  [[ "$output" == *"semantic.recursive-contracts"* ]]

  run_architecture_fixture "recursive-contracts/valid"
  [ "$status" -eq 0 ]
}

@test "semantic seal rejects stale generated evidence" {
  run_architecture_fixture "generated-evidence/mutated"
  [ "$status" -eq 1 ]
  [[ "$output" == *"semantic.generated-evidence"* ]]

  run_architecture_fixture "generated-evidence/valid"
  [ "$status" -eq 0 ]
}

@test "semantic seal rejects evidence not bound to the candidate" {
  run_architecture_fixture "evidence-binding/mutated"
  [ "$status" -eq 1 ]
  [[ "$output" == *"semantic.evidence-binding"* ]]

  run_architecture_fixture "evidence-binding/valid"
  [ "$status" -eq 0 ]
}

@test "architecture checker ignores comments strings tests and adapter effects" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/cli/internal/commands/demo" "$fixture/cli/internal/adapters/raw_exec"
  cat >"$fixture/cli/internal/commands/demo/module.go" <<'GO'
package demo
// exec.Command and os.ReadFile are documentation, not effects.
const docs = "exec.Command os.ReadFile http.Get time.Now"
GO
  cat >"$fixture/cli/internal/commands/demo/module_test.go" <<'GO'
package demo
import "os/exec"
func example() { _ = exec.Command("test-only") }
GO
  cat >"$fixture/cli/internal/adapters/raw_exec/adapter.go" <<'GO'
package raw_exec
import "os/exec"
func Run() { _ = exec.Command("declared-adapter") }
GO
  run bash -c "cd '$REPO_ROOT/cli' && go run ./internal/archcheck/cmd --root '$fixture'"
  [ "$status" -eq 0 ]
}
