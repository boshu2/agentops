#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  RUN="$SKILL_DIR/scripts/rotate.sh"
  FIX="$(mktemp -d)"
  MOCK="$FIX/caam"
  cat >"$MOCK" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  --help) printf 'activate status version\n' ;;
  version) printf 'caam 9.9.9 (fixture)\n' ;;
  status)
    if [[ -s "$MOCK_STATE" ]]; then profile=$(cat "$MOCK_STATE"); else profile=old-profile; fi
    printf '{"profile":"%s"}\n' "$profile"
    ;;
  activate)
    [[ "${MOCK_HANG:-0}" == 1 ]] && sleep 30
    printf '%s\n' "$3" >"$MOCK_STATE"
    printf '%s\n' action >>"$MOCK_ACTIONS"
    printf '{"activated":"%s"}\n' "$3"
    ;;
esac
SH
  chmod +x "$MOCK"
  export ACCOUNT_ROTATION_CAAM_BIN="$MOCK"
  export MOCK_STATE="$FIX/state"
  export MOCK_ACTIONS="$FIX/actions"
}

teardown() { rm -rf "$FIX"; }

@test "authorized normal rotation uses the bounded surface once" {
  run "$RUN" --family codex --profile work --tool caam --approve rotate:codex:work
  [ "$status" -eq 0 ]
  [ "$(wc -l <"$MOCK_ACTIONS" | tr -d ' ')" -eq 1 ]
  [ "$(cat "$MOCK_STATE")" = work ]
}

@test "raw baseline mutates credentials while missing approval stops before action" {
  run "$MOCK" activate codex raw --json
  [ "$status" -eq 0 ]
  [ "$(cat "$MOCK_STATE")" = raw ]
  rm -f "$MOCK_ACTIONS"
  run "$RUN" --family codex --profile work --tool caam
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
  [ "$(cat "$MOCK_STATE")" = raw ]
}

@test "missing capability fails closed before credential mutation" {
  sed -i.bak 's/activate status version/status version/' "$MOCK"
  run "$RUN" --family codex --profile work --tool caam --approve rotate:codex:work
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "unsupported family and malformed profile are rejected" {
  run "$RUN" --family shell --profile work --tool caam --approve rotate:shell:work
  [ "$status" -ne 0 ]
  run "$RUN" --family codex --profile '../work' --tool caam --approve 'rotate:codex:../work'
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "hung credential switch reaches a typed deadline without reporting success" {
  export MOCK_HANG=1
  run "$RUN" --family codex --profile work --tool caam --deadline 1 --approve rotate:codex:work
  [ "$status" -eq 124 ]
  [ ! -e "$MOCK_ACTIONS" ]
  [ ! -e "$MOCK_STATE" ]
}
