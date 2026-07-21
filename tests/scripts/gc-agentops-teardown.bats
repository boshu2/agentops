#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  TEARDOWN="$REPO_ROOT/deploy/gc/teardown.sh"
  TMP="$(mktemp -d "${TMPDIR:-/tmp}/gc-agentops-teardown.XXXXXX")"
  CITY="$TMP/city"
  BIN="$TMP/bin"
  LOG="$TMP/gc.log"
  STATE="$TMP/state"
  mkdir -p "$CITY/.gc" "$CITY/.gc-home" "$BIN" "$STATE"

  FAKE_GC="$BIN/gc"
  cat >"$FAKE_GC" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
log="$root/gc.log"
state="$root/state"
printf 'ENV GC_HOME=<%s> GC_ISOLATED=<%s> OTEL_SDK_DISABLED=<%s>\n' \
  "${GC_HOME-}" "${GC_ISOLATED-}" "${OTEL_SDK_DISABLED-}" >>"$log"
printf 'ARGS' >>"$log"
printf ' <%s>' "$@" >>"$log"
printf '\n' >>"$log"

case "${1:-} ${2:-}" in
  "supervisor status")
    if [ -e "$state/supervisor-running" ]; then
      printf '%s\n' '{"ok":true,"running":true,"pid":1234}'
    else
      printf '%s\n' '{"ok":true,"running":false,"pid":0}'
    fi
    ;;
  "supervisor stop")
    [ "${3:-}" = "--wait" ]
    [ "${4:-}" = "--wait-timeout" ]
    rm -f "$state/supervisor-running"
    touch "$state/supervisor-stopped"
    ;;
  "dolt-state stop-managed")
    [ "${3:-}" = "--city" ]
    touch "$state/dolt-stopped"
    if [ -e "$state/spawn-late-helper" ] && [ ! -e "$state/late-helper-spawned" ]; then
      touch "$state/late-helper-spawned"
      (
        sleep 1
        python3 -c 'import pathlib,sys,time; time.sleep(0.5); pathlib.Path(sys.argv[2]).touch()' \
          "$4" "$state/late-helper-finished"
      ) </dev/null >/dev/null 2>&1 &
    fi
    ;;
  *)
    printf 'unexpected fake gc invocation:' >&2
    printf ' <%s>' "$@" >&2
    printf '\n' >&2
    exit 2
    ;;
esac
EOF
  chmod +x "$FAKE_GC"

  FAKE_BD="$BIN/bd"
  cat >"$FAKE_BD" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' 'bd version 1.1.0 (test)'
EOF
  chmod +x "$FAKE_BD"

  FAKE_AO="$BIN/ao"
  cat >"$FAKE_AO" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' 'ao fixture reducer'
EOF
  chmod +x "$FAKE_AO"

  cat >"$BIN/tmux" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
printf 'TMUX' >>"$root/gc.log"
printf ' <%s>' "$@" >>"$root/gc.log"
printf '\n' >>"$root/gc.log"
if [ "${3:-}" = "kill-server" ]; then
  rm -f "$root/state/tmux-live"
  exit 0
fi
if [ -e "$root/state/tmux-live" ]; then
  printf '%s\n' 'agentops-live: 1 windows'
  exit 0
fi
exit 1
EOF
  chmod +x "$BIN/tmux"

  cat >"$CITY/city.toml" <<'EOF'
[session]
socket = "agentops-teardown-test"
EOF
	CANONICAL_CITY="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$CITY")"
  python3 - "$CITY/.gc/agentops-bootstrap.json" "$CITY" "$FAKE_GC" "$FAKE_BD" "$FAKE_AO" <<'PY'
import hashlib
import json
import os
import sys

path, city, gc_bin, bd_bin, ao_bin = sys.argv[1:]


def sha256(filename):
    return hashlib.sha256(open(filename, "rb").read()).hexdigest()


with open(path, "w", encoding="utf-8") as handle:
    json.dump({
        "schema_version": 4,
        "state": "ready",
        "city": os.path.realpath(city),
        "toolchain": {
            "gc": {
                "path": os.path.realpath(gc_bin),
                "sha256": sha256(gc_bin),
            },
            "bd": {
                "path": os.path.realpath(bd_bin),
                "sha256": sha256(bd_bin),
            },
        },
        "ao_reducer": {
            "path": os.path.realpath(ao_bin),
            "binary_sha256": sha256(ao_bin),
            "source_commit": "0" * 40,
            "source_tree": "0" * 40,
            "schema_config_sha256": "0" * 64,
        },
    }, handle)
    handle.write("\n")
PY
  touch "$STATE/supervisor-running"
}

teardown() {
  rm -rf "$TMP"
}

@test "teardown selects the private supervisor home and proves quiescence" {
  run env PATH="$BIN:$PATH" "$TEARDOWN" --city "$CITY" --wait-timeout 17

  [ "$status" -eq 0 ]
  [[ "$output" == *"Gas City stopped cleanly"* ]]
  [ -e "$STATE/supervisor-stopped" ]
  [ -e "$STATE/dolt-stopped" ]
  grep -Fq "GC_HOME=<$CANONICAL_CITY/.gc-home> GC_ISOLATED=<1> OTEL_SDK_DISABLED=<true>" "$LOG"
  grep -Fq 'ARGS <supervisor> <stop> <--wait> <--wait-timeout> <17s>' "$LOG"
  grep -Fq "ARGS <dolt-state> <stop-managed> <--city> <$CANONICAL_CITY>" "$LOG"
  grep -Fq 'TMUX <-L> <agentops-teardown-test> <list-sessions>' "$LOG"
}

@test "teardown is idempotent when the supervisor is already stopped" {
  rm -f "$STATE/supervisor-running"

  run env PATH="$BIN:$PATH" "$TEARDOWN" --city "$CITY"

  [ "$status" -eq 0 ]
  [ -e "$STATE/dolt-stopped" ]
  ! grep -Fq 'ARGS <supervisor> <stop>' "$LOG"
}

@test "teardown refuses an unmanaged city" {
  rm -f "$CITY/.gc/agentops-bootstrap.json"

  run env PATH="$BIN:$PATH" "$TEARDOWN" --city "$CITY"

  [ "$status" -ne 0 ]
  [[ "$output" == *"refusing unmanaged city"* ]]
  [ ! -e "$STATE/supervisor-stopped" ]
}

@test "teardown refuses changed managed toolchain bytes" {
  printf '\n# replaced\n' >>"$FAKE_GC"

  run env PATH="$BIN:$PATH" "$TEARDOWN" --city "$CITY"

  [ "$status" -ne 0 ]
  [[ "$output" == *"managed gc binary digest mismatch"* ]]
  [ ! -e "$STATE/supervisor-stopped" ]
}

@test "teardown refuses ambient gc substitution and changed reducer bytes" {
  run env PATH="$BIN:$PATH" "$TEARDOWN" --city "$CITY" --gc-bin gc

  [ "$status" -ne 0 ]
  [[ "$output" == *"PATH resolution is forbidden"* ]]
  [ ! -e "$STATE/supervisor-stopped" ]

  printf '%s\n' '# replaced' >>"$FAKE_AO"
  run env PATH="$BIN:$PATH" "$TEARDOWN" --city "$CITY"

  [ "$status" -ne 0 ]
  [[ "$output" == *"ao reducer binary digest mismatch"* ]]
  [ ! -e "$STATE/supervisor-stopped" ]
}

@test "teardown reaps sessions left on the private tmux socket" {
  touch "$STATE/tmux-live"

  run env PATH="$BIN:$PATH" "$TEARDOWN" --city "$CITY"

	[ "$status" -eq 0 ]
	[ ! -e "$STATE/tmux-live" ]
	grep -Fq 'TMUX <-L> <agentops-teardown-test> <kill-server>' "$LOG"
}

@test "teardown waits for a canceled city-scoped helper to finish cleanup" {
  python3 -c 'import time; time.sleep(1)' "$CANONICAL_CITY" &
  helper_pid="$!"

  run env PATH="$BIN:$PATH" "$TEARDOWN" --city "$CITY" --wait-timeout 6
  wait "$helper_pid" 2>/dev/null || true

  [ "$status" -eq 0 ]
  [[ "$output" == *"Gas City stopped cleanly"* ]]
}

@test "teardown does not accept a quiet gap before a late scoped helper" {
  touch "$STATE/spawn-late-helper"

  run env PATH="$BIN:$PATH" "$TEARDOWN" --city "$CITY" --wait-timeout 6

  [ "$status" -eq 0 ]
  [ -e "$STATE/late-helper-finished" ]
  [[ "$output" == *"Gas City stopped cleanly"* ]]
}
