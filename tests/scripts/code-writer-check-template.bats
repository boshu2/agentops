#!/usr/bin/env bats
# The code-writer agent (agents/code-writer.md) runs a caller's check ONCE and
# must keep that check's original exit status, even when the check calls
# `exit` or turns on errexit. This runs the agent's own status-capture template
# exactly as written there, with a counter proving the check ran once.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  WORK="$BATS_TEST_TMPDIR/work"
  mkdir -p "$WORK"
}

# run_template CHECK: substitute CHECK into the template and run it in $WORK.
run_template() {
  local script
  script="$(python3 - "$REPO_ROOT/agents/code-writer.md" "$1" <<'PY'
import re
import sys

source = open(sys.argv[1], encoding="utf-8").read()
match = re.search(r"       set \+e\n[\s\S]*?       printf[^\n]+", source)
assert match, "status-capture template not found in agents/code-writer.md"
template = re.sub(r"(?m)^       ", "", match.group(0))
assert "SUPPLIED_CHECK_COMMAND" in template, template
print(template.replace("SUPPLIED_CHECK_COMMAND", sys.argv[2]))
PY
)" || return 1
  rm -f "$WORK/check-count"
  run bash -c "cd '$WORK' && $script"
}

@test "code-writer template: a silent passing check runs once and reports 0" {
  run_template 'printf "called\n" >> check-count; exit 0'
  [ "$status" -eq 0 ]
  [ "$output" = $'\nAGENTOPS_CHECK_STATUS=0' ]
  [ "$(cat "$WORK/check-count")" = "called" ]
}

@test "code-writer template: a failing check that calls exit keeps its status" {
  run_template 'printf "called\n" >> check-count; exit 7'
  [ "$status" -eq 0 ]
  [ "$output" = $'\nAGENTOPS_CHECK_STATUS=7' ]
  [ "$(cat "$WORK/check-count")" = "called" ]
}

@test "code-writer template: an errexit check stops inside the subshell and reports 1" {
  run_template 'set -e; printf "called\n" >> check-count; false; printf "UNREACHABLE"'
  [ "$status" -eq 0 ]
  [ "$output" = $'\nAGENTOPS_CHECK_STATUS=1' ]
  [ "$(cat "$WORK/check-count")" = "called" ]
}
