#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  RUN="$SKILL_DIR/scripts/dispatch.sh"
  FIX="$(mktemp -d)"
  FIX="$(cd "$FIX" && pwd -P)"
  mkdir -p "$FIX/work" "$FIX/out"
  printf 'independent judgment packet\n' >"$FIX/packet"
  MOCK="$FIX/codex"
  cat >"$MOCK" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  --version) printf 'codex-cli 9.9.9\n'; exit 0 ;;
  login) printf 'Logged in\n'; exit 0 ;;
  exec)
    if [[ "${2:-}" == --help ]]; then printf '%s\n' '--sandbox --cd --ephemeral --output-last-message'; exit 0; fi
    out=''; while [[ $# -gt 0 ]]; do case "$1" in --output-last-message) out=$2; shift 2;; *) shift;; esac; done
    printf 'real-adapter-path\n' >"$out"
    printf 'dispatch\n' >>"$MOCK_ACTIONS"
    ;;
esac
SH
  chmod +x "$MOCK"
  export CODEX_BIN="$MOCK"
  export MOCK_ACTIONS="$FIX/actions"
  DIGEST=$(shasum -a 256 "$FIX/packet" | awk '{print $1}')
  APPROVAL="agent-native:dispatch:codex:judge:judge-1:$FIX/work:model-a:$DIGEST"
}

teardown() { rm -rf "$FIX"; }

@test "authorized judge routes through the real Codex adapter surface" {
  run "$RUN" --adapter codex --role judge --model-profile model-a --context-id judge-1 --workspace "$FIX/work" --packet "$FIX/packet" --output "$FIX/out/result" --approve "$APPROVAL"
  [ "$status" -eq 0 ]
  [ "$(cat "$FIX/out/result")" = real-adapter-path ]
  [ "$(wc -l <"$MOCK_ACTIONS" | tr -d ' ')" -eq 1 ]
}

@test "raw model fixture is explicitly non-attesting and not the dispatch path" {
  run python3 "$SKILL_DIR/scripts/fake_model_runner.py" council --models a,b --available a,b --output "$FIX/out/fixture.json"
  [ "$status" -eq 0 ]
  jq -e '.runtime_attestation_valid == false and .evidence_class == "fixture-only"' "$FIX/out/fixture.json" >/dev/null
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "missing approval and unsupported NTM model attestation stop before dispatch" {
  run "$RUN" --adapter codex --role judge --model-profile model-a --context-id judge-1 --workspace "$FIX/work" --packet "$FIX/packet" --output "$FIX/out/result"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
  run "$RUN" --adapter ntm --role judge --model-profile model-a --context-id judge-1 --workspace "$FIX/work" --packet "$FIX/packet" --output "$FIX/out/result" --approve anything
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "malformed context and unsafe workspace stop before dispatch" {
  run "$RUN" --adapter codex --role judge --model-profile model-a --context-id '../judge' --workspace "$FIX/work" --packet "$FIX/packet" --output "$FIX/out/result" --approve anything
  [ "$status" -ne 0 ]
  run "$RUN" --adapter codex --role judge --model-profile model-a --context-id judge-1 --workspace / --packet "$FIX/packet" --output "$FIX/out/result" --approve anything
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}
