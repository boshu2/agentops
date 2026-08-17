#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  RUN="$SKILL_DIR/scripts/run.sh"
  FIX="$(mktemp -d)"
  mkdir -p "$FIX/work" "$FIX/out"
  printf 'review this exact subject\n' >"$FIX/prompt.txt"
  MOCK="$FIX/codex"
  cat >"$MOCK" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  --version) printf 'codex-cli 9.9.9\n'; exit 0 ;;
  login) [[ "${2:-}" == status ]]; printf 'Logged in\n'; exit 0 ;;
  exec)
    if [[ "${2:-}" == --help ]]; then
      printf '%s\n' '--sandbox --cd --ephemeral --output-last-message'
      exit 0
    fi
    out=''
    sandbox=''
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --output-last-message) out=$2; shift 2 ;;
        --sandbox) sandbox=$2; shift 2 ;;
        *) shift ;;
      esac
    done
    printf '%s\n' "$sandbox" >"$MOCK_ACTION_LOG"
    prompt=$(cat)
    if [[ "$prompt" == *HANG* ]]; then
      sleep 30 &
      printf '%s\n' "$!" >"$MOCK_CHILD_PID"
      wait
    fi
    printf 'bounded result\n' >"$out"
    ;;
esac
SH
  chmod +x "$MOCK"
  export MOCK_ACTION_LOG="$FIX/action"
  export MOCK_CHILD_PID="$FIX/child.pid"
}

teardown() {
  rm -rf "$FIX"
}

@test "normal read-only invocation produces a nonempty artifact" {
  run env CODEX_BIN="$MOCK" "$RUN" --workspace "$FIX/work" --prompt "$FIX/prompt.txt" --output "$FIX/out/result.txt" --deadline 5
  [ "$status" -eq 0 ]
  [ "$(cat "$FIX/out/result.txt")" = 'bounded result' ]
  [ "$(cat "$MOCK_ACTION_LOG")" = 'read-only' ]
}

@test "raw baseline executes write mode while bounded surface rejects missing approval" {
  run bash -c 'printf x | "$1" exec --sandbox workspace-write --output-last-message "$2" -' _ "$MOCK" "$FIX/raw.out"
  [ "$status" -eq 0 ]
  [ "$(cat "$MOCK_ACTION_LOG")" = 'workspace-write' ]
  rm -f "$MOCK_ACTION_LOG"
  run env CODEX_BIN="$MOCK" "$RUN" --workspace "$FIX/work" --prompt "$FIX/prompt.txt" --output "$FIX/out/result.txt" --sandbox workspace-write
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTION_LOG" ]
}

@test "missing runtime capability stops before execution" {
  sed -i.bak 's/--sandbox --cd --ephemeral --output-last-message/--sandbox --cd --ephemeral/' "$MOCK"
  run env CODEX_BIN="$MOCK" "$RUN" --workspace "$FIX/work" --prompt "$FIX/prompt.txt" --output "$FIX/out/result.txt"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTION_LOG" ]
}

@test "deadline kills and reaps the entire child process group" {
  printf 'HANG\n' >"$FIX/prompt.txt"
  run env CODEX_BIN="$MOCK" "$RUN" --workspace "$FIX/work" --prompt "$FIX/prompt.txt" --output "$FIX/out/result.txt" --deadline 1
  [ "$status" -eq 124 ]
  [ -s "$MOCK_CHILD_PID" ]
  run kill -0 "$(cat "$MOCK_CHILD_PID")"
  [ "$status" -ne 0 ]
}
