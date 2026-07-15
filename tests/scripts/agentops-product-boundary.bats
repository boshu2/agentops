#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  ACTIVE_AUTHORITY=(
    AGENTS.md
    PRODUCT.md
    README.md
    docs/CI-CD.md
    docs/architecture/operating-loop.md
    cli/cmd/ao/root.go
    cli/cmd/ao/demo.go
  )
}

scan_obsolete_authority() {
  local file="$1"
  grep -Eni \
    'ao[[:space:]]+(land|pawl|plan-pawl|done|close|governor|yield|claim|next-work|worktree|validate|converge|reconcile|membrane|crank)|/pawl-review|semantic[[:space:]]+pre-push|push-as-CI|Validate[[:space:]]+verdict[^[:cntrl:]]*(push|merge|release|delivery)|automatic[^[:cntrl:]]*(repair|retry|replan)' \
    "$file"
}

require_text() {
  grep -Fq -- "$2" "$REPO_ROOT/$1" || {
    echo "$1 is missing: $2" >&2
    return 1
  }
}

@test "active authority teaches one bounded experiment and stop" {
  local file
  for file in "${ACTIVE_AUTHORITY[@]}"; do
    run scan_obsolete_authority "$REPO_ROOT/$file"
    [ "$status" -eq 1 ] || {
      echo "$output" >&2
      return 1
    }
  done

  require_text AGENTS.md \
    "RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop"
  require_text PRODUCT.md \
    "AgentOps is not a new GitLab, CI service, tracker, merge queue, delivery system"
  require_text docs/architecture/operating-loop.md \
    "RPI invokes Plan, Implement, and Validate at most once and then stops."
  require_text docs/CI-CD.md \
    "Repositories own delivery policy for local and cloud agents."
}

@test "optional strategies and adapters are not core dependencies" {
  run python3 - "$REPO_ROOT" <<'PY'
from pathlib import Path
import sys
import yaml

root = Path(sys.argv[1])
actual = {}
for name in ("rpi", "plan", "implement", "validate"):
    data = yaml.safe_load((root / "skills" / name / "SKILL.md").read_text().split("---", 2)[1])
    actual[name] = set(data["metadata"]["dependencies"])
expected = {"rpi": {"plan", "implement", "validate"}, "plan": set(), "implement": set(), "validate": set()}
if actual != expected:
    raise SystemExit(actual)
PY
  [ "$status" -eq 0 ]
}

@test "negative fixtures reject semantic Git authority and automatic continuation" {
  fixture="$BATS_TEST_TMPDIR/obsolete.md"
  printf '%s\n' \
    'Run ao land after the review.' \
    'Require semantic pre-push approval.' \
    'A FAIL triggers automatic repair.' >"$fixture"

  run scan_obsolete_authority "$fixture"
  [ "$status" -eq 0 ]
  [[ "$output" == *"ao land"* ]]
  [[ "$output" == *"semantic pre-push"* ]]
  [[ "$output" == *"automatic repair"* ]]
}
