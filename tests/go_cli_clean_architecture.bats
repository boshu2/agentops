#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
}

@test "S5 compatibility survives every vertical migration slice" {
  test -x "$REPO_ROOT/scripts/check-go-cli-compatibility.sh"
  test -d "$REPO_ROOT/cli/testdata/compatibility-baseline"

  run "$REPO_ROOT/scripts/check-go-cli-compatibility.sh"
  [ "$status" -eq 0 ]
}

@test "S1 explicit profiles assemble one deterministic command graph" {
  test -f "$REPO_ROOT/cli/internal/cliapp/root.go"
  test -f "$REPO_ROOT/cli/internal/cliapp/profile.go"
  test -d "$REPO_ROOT/cli/internal/composition"

  run rg -n 'func BuildRoot\(' "$REPO_ROOT/cli/internal/cliapp"
  [ "$status" -eq 0 ]

  run rg -n 'internal/composition' "$REPO_ROOT/cli/cmd/ao" -g '*.go' -g '!**/*_test.go'
  [ "$status" -eq 0 ]

  run rg -n 'internal/(commands|adapters)/' "$REPO_ROOT/cli/internal/cliapp" -g '*.go'
  [ "$status" -eq 1 ]

  run rg -n 'internal/(composition|cliapp)' "$REPO_ROOT/cli/internal/commands" -g '*.go'
  [ "$status" -eq 1 ]

  run rg -n '^func init\(' "$REPO_ROOT/cli/cmd/ao" -g '*.go' -g '!**/*_test.go'
  [ "$status" -eq 1 ]

  [ ! -e "$REPO_ROOT/cli/cmd/ao/zz_args_policy.go" ]
  [ ! -e "$REPO_ROOT/cli/cmd/ao/zzz_default_spine.go" ]
}

@test "S4 contract metadata is explicit before projection" {
  test -f "$REPO_ROOT/cli/internal/clicontract/metadata.go"

  run rg -n 'type CommandContract struct' \
    "$REPO_ROOT/cli/internal/clicontract/metadata.go"
  [ "$status" -eq 0 ]

  for field in ID Profiles Args Output Effects ExitClasses; do
    run rg -n "^[[:space:]]*$field[[:space:]]" \
      "$REPO_ROOT/cli/internal/clicontract/metadata.go"
    [ "$status" -eq 0 ]
  done

  run rg -n 'func (ValidateContract|Attach)\(' \
    "$REPO_ROOT/cli/internal/clicontract"
  [ "$status" -eq 0 ]
}

@test "S1 root contract selects profiles before assembly" {
  test -f "$REPO_ROOT/cli/internal/cliapp/profile.go"
  test -f "$REPO_ROOT/cli/internal/cliapp/module.go"
  test -f "$REPO_ROOT/cli/internal/cliapp/root.go"

  run rg -n 'func BuildRoot\(' "$REPO_ROOT/cli/internal/cliapp/root.go"
  [ "$status" -eq 0 ]

  run rg -n 'clicontract\.CommandContract' \
    "$REPO_ROOT/cli/internal/cliapp/module.go"
  [ "$status" -eq 0 ]
}

@test "S2 command handlers are thin driving adapters" {
  family="${AO_ARCH_FAMILY:-beads}"
  family_dir="${family//-/_}"
  module_dir="$REPO_ROOT/cli/internal/commands/$family_dir"

  test -f "$module_dir/module.go"
  run rg -n 'func NewModule\(' "$module_dir/module.go"
  [ "$status" -eq 0 ]

  run rg -n -g '*.go' -g '!**/*_test.go' \
    'os/exec|exec\.Command|os\.(ReadFile|WriteFile|Mkdir|Remove|Setenv|Getenv)|net/http' \
    "$module_dir"
  [ "$status" -eq 1 ]

  test -f "$REPO_ROOT/cli/testdata/compatibility-baseline/families/$family/ownership.json"
  test -x "$REPO_ROOT/scripts/check-go-cli-architecture.sh"

  run "$REPO_ROOT/scripts/check-go-cli-architecture.sh" --family "$family"
  [ "$status" -eq 0 ]

  run "$REPO_ROOT/scripts/check-go-cli-compatibility.sh" \
    --verify-frozen --profiles default,flywheel,legacy,combined --family "$family"
  [ "$status" -eq 0 ]
}

@test "S3 tracker and workspace resolution has one owner" {
  [ ! -e "$REPO_ROOT/cli/cmd/ao/beads_tracker.go" ]

  run rg -n 'ChildEnv|GitCommonDir|git-common-dir|BEADS_DIR' \
    "$REPO_ROOT/cli/internal/trackerresolve"
  [ "$status" -eq 0 ]

  run rg -n -g '*.go' -g '!**/*_test.go' \
    'func .*resolve.*Tracker|func .*find.*Beads|type trackerResolution' \
    "$REPO_ROOT/cli/cmd/ao"
  [ "$status" -eq 1 ]
}

@test "S4 command contracts are declared once and projected everywhere" {
  run rg -n 'type CommandContract struct' "$REPO_ROOT/cli/internal/clicontract"
  [ "$status" -eq 0 ]

  run rg -n 'type CommandContract struct' "$REPO_ROOT/cli/internal/cliapp"
  [ "$status" -eq 1 ]

  for field in ID Profiles Args Output Effects ExitClasses; do
    run rg -n "^[[:space:]]*$field[[:space:]]" \
      "$REPO_ROOT/cli/internal/clicontract"
    [ "$status" -eq 0 ]
  done

  run rg -n 'internal/cliapp' "$REPO_ROOT/cli/internal/clicontract" -g '*.go'
  [ "$status" -eq 1 ]

  run rg -n \
    'stableID\(path\)|Output:[[:space:]]+"none"|Effects:[[:space:]]+"mixed"|map\[string\]string\{"0": "success", "1": "error"\}' \
    "$REPO_ROOT/cli/internal/clicontract"
  [ "$status" -eq 1 ]
}

@test "S6 the strangler scaffolding is deleted and cannot regrow" {
  production_files="$(find "$REPO_ROOT/cli/cmd/ao" -maxdepth 1 -name '*.go' ! -name '*_test.go' | wc -l | tr -d ' ')"
  production_lines="$(find "$REPO_ROOT/cli/cmd/ao" -maxdepth 1 -name '*.go' ! -name '*_test.go' -print0 | xargs -0 wc -l | tail -1 | awk '{print $1}')"

  [ "$production_files" -le 3 ]
  [ "$production_lines" -le 500 ]
  test -x "$REPO_ROOT/scripts/check-go-cli-architecture.sh"

  run "$REPO_ROOT/scripts/check-go-cli-architecture.sh"
  [ "$status" -eq 0 ]
}
