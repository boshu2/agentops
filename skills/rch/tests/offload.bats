#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  RUN="$SKILL_DIR/scripts/offload.sh"
  FIX="$(mktemp -d)"
  FIX="$(cd "$FIX" && pwd -P)"
  mkdir -p "$FIX/work" "$FIX/out"
  git -C "$FIX/work" init -q
  MOCK="$FIX/rch"
  cat >"$MOCK" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  --version) printf 'rch 1.0.18\n' ;;
  --capabilities)
    if [[ "${MOCK_BAD_CAPS:-0}" == 1 ]]; then printf '{"version":"1.0.18","commands":["check"],"runtimes":["rust"]}\n'
    else printf '{"version":"1.0.18","commands":["check","exec"],"runtimes":["rust","bun"]}\n'; fi
    ;;
  --help-json) printf '{"command":"%s"}\n' "${2:-}" ;;
  check) [[ "${MOCK_NOT_READY:-0}" != 1 ]] ;;
  exec)
    printf '%s\n' "$*" >>"$MOCK_ACTIONS"
    printf 'compiled\n'
    if [[ "${MOCK_LOCAL:-0}" == 1 ]]; then printf '[RCH] local (no healthy workers)\n' >&2
    else printf '[RCH] remote worker-a (12ms)\n' >&2; fi
    ;;
  *) exit 2 ;;
esac
SH
  chmod +x "$MOCK"
  export RCH_BIN="$MOCK"
  export MOCK_ACTIONS="$FIX/actions"
  APPROVAL=$("$RUN" approval --workspace "$FIX/work" -- cargo check --workspace)
}

teardown() { rm -rf "$FIX"; }

@test "authorized safe build is offloaded once and records remote proof" {
  run "$RUN" run --workspace "$FIX/work" --receipt "$FIX/out/receipt.json" --deadline 5 --approve "$APPROVAL" -- cargo check --workspace
  [ "$status" -eq 0 ]
  [ "$(wc -l <"$MOCK_ACTIONS" | tr -d ' ')" -eq 1 ]
  jq -e '.status == "remote" and .command == ["cargo","check","--workspace"] and .not_checked == ["remote-worker-implementation"]' "$FIX/out/receipt.json" >/dev/null
}

@test "raw exec baseline acts while missing approval stops before offload" {
  run "$MOCK" exec -- cargo check
  [ "$status" -eq 0 ]
  [ -s "$MOCK_ACTIONS" ]
  rm -f "$MOCK_ACTIONS"
  run "$RUN" run --workspace "$FIX/work" --receipt "$FIX/out/receipt.json" -- cargo check --workspace
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "missing capability and failed readiness stop before action" {
  export MOCK_BAD_CAPS=1
  run "$RUN" run --workspace "$FIX/work" --receipt "$FIX/out/receipt.json" --approve "$APPROVAL" -- cargo check --workspace
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
  unset MOCK_BAD_CAPS
  export MOCK_NOT_READY=1
  run "$RUN" run --workspace "$FIX/work" --receipt "$FIX/out/receipt.json" --approve "$APPROVAL" -- cargo check --workspace
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "local fallback and unsafe command cannot be reported as remote success" {
  export MOCK_LOCAL=1
  run "$RUN" run --workspace "$FIX/work" --receipt "$FIX/out/receipt.json" --approve "$APPROVAL" -- cargo check --workspace
  [ "$status" -eq 3 ]
  jq -e '.status == "local_fallback"' "$FIX/out/receipt.json" >/dev/null
  run "$RUN" approval --workspace "$FIX/work" -- sh -c 'touch /tmp/nope'
  [ "$status" -ne 0 ]
  run "$RUN" approval --workspace "$FIX/work" -- cargo check --manifest-path=/tmp/elsewhere
  [ "$status" -ne 0 ]
}

@test "existing output paths stop before remote execution" {
  printf 'keep\n' >"$FIX/out/receipt.json"
  run "$RUN" run --workspace "$FIX/work" --receipt "$FIX/out/receipt.json" --approve "$APPROVAL" -- cargo check --workspace
  [ "$status" -ne 0 ]
  [ "$(cat "$FIX/out/receipt.json")" = keep ]
  [ ! -e "$MOCK_ACTIONS" ]
}
