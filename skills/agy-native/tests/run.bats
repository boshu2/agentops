#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  RUN="$SKILL_DIR/scripts/run.sh"
  FIX="$(mktemp -d)"
  FIX="$(cd "$FIX" && pwd -P)"
  mkdir -p "$FIX/work" "$FIX/out"
  printf 'judge the packet\n' >"$FIX/packet"
  MOCK="$FIX/agy"
  cat >"$MOCK" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  --version) printf '1.1.13\n'; exit 0 ;;
  --help) printf '%s\n' '--print --print-timeout --sandbox --mode --model --disable-slash-commands'; exit 0 ;;
  models) printf 'model-a\nmodel-b\n'; exit 0 ;;
esac
printf '%s\n' "$*" >"$MOCK_ACTIONS"
if [[ "${MOCK_HANG:-0}" == 1 ]]; then sleep 30 & printf '%s\n' "$!" >"$MOCK_CHILD_PID"; wait; fi
printf 'agy result\n'
SH
  chmod +x "$MOCK"
  export AGY_BIN="$MOCK"
  export MOCK_ACTIONS="$FIX/actions"
  export MOCK_CHILD_PID="$FIX/child.pid"
  DIGEST=$(shasum -a 256 "$FIX/packet" | awk '{print $1}')
}

teardown() { rm -rf "$FIX"; }

@test "normal validator run is sandboxed and bounded" {
  run "$RUN" --role validator --model model-a --context-id validator-1 --workspace "$FIX/work" --packet "$FIX/packet" --output "$FIX/out/result" --deadline 5
  [ "$status" -eq 0 ]
  [ "$(cat "$FIX/out/result")" = 'agy result' ]
  grep -q -- '--sandbox --mode plan' "$MOCK_ACTIONS"
}

@test "raw write-capable baseline runs while wrapper requires exact approval" {
  run "$MOCK" --print raw --mode accept-edits
  [ "$status" -eq 0 ]
  [ -s "$MOCK_ACTIONS" ]
  rm -f "$MOCK_ACTIONS"
  run "$RUN" --role implementer --model model-a --context-id impl-1 --workspace "$FIX/work" --packet "$FIX/packet" --output "$FIX/out/result"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "missing capability and missing model stop before session start" {
  sed -i.bak 's/--sandbox//' "$MOCK"
  run "$RUN" --role validator --model model-a --context-id validator-1 --workspace "$FIX/work" --packet "$FIX/packet" --output "$FIX/out/result"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
  mv "$MOCK.bak" "$MOCK"
  chmod +x "$MOCK"
  run "$RUN" --role validator --model absent --context-id validator-1 --workspace "$FIX/work" --packet "$FIX/packet" --output "$FIX/out/result"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "outer deadline reports timeout instead of a completed result" {
  export MOCK_HANG=1
  run "$RUN" --role validator --model model-a --context-id validator-1 --workspace "$FIX/work" --packet "$FIX/packet" --output "$FIX/out/result" --deadline 1
  [ "$status" -eq 124 ]
  [ -s "$MOCK_CHILD_PID" ]
  run kill -0 "$(cat "$MOCK_CHILD_PID")"
  [ "$status" -ne 0 ]
}
