#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  RUN="$SKILL_DIR/scripts/safe-gc.sh"
  FIX="$(mktemp -d)"
  FIX="$(cd "$FIX" && pwd -P)"
  mkdir -p "$FIX/city/rigs/demo" "$FIX/out"
  printf '[workspace]\n' >"$FIX/city/city.toml"
  printf 'Launch build-basic with push=false.\n' >"$FIX/message"
  GC_MOCK="$FIX/gc"
  AO_MOCK="$FIX/ao"
  cat >"$GC_MOCK" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
args=" $* "
if [[ "$args" == *' version --json '* ]]; then
  if [[ "${MOCK_EDGE:-0}" == 1 ]]; then printf '{"ok":true,"version":"edge"}\n'; else printf '{"ok":true,"version":"1.4.2"}\n'; fi
elif [[ "${1:-}" == --help ]]; then
  printf '%s\n' 'bd mail version --city --rig'
elif [[ "$args" == *' mail send --help '* ]]; then
  printf '%s\n' '--notify --json --subject --message'
elif [[ "$args" == *' bd show '* ]]; then
  printf '[{"id":"ago-123","status":"open","title":"Bounded work","description":"Acceptance: tests pass"}]\n'
elif [[ "$args" == *' mail send mayor '* ]]; then
  printf 'mail\n' >>"$MOCK_ACTIONS"
  if [[ "${MOCK_NOTIFY_FAIL:-0}" == 1 ]]; then
    printf '{"schema_version":"1","ok":true,"command":"mail.send","action":"send","id":"m-1","count":1,"notified":false}\n'
  else
    printf '{"schema_version":"1","ok":true,"command":"mail.send","action":"send","id":"m-1","count":1,"notified":true}\n'
  fi
elif [[ "${1:-}" == sling ]]; then
  printf 'sling\n' >>"$MOCK_ACTIONS"
  printf 'slung\n'
else
  exit 2
fi
SH
  cat >"$AO_MOCK" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
args=" $* "
if [[ "$args" == *' version --json '* ]]; then printf '{"version":"3.5.0"}\n'
elif [[ "$args" == *' gc --help '* ]]; then
  if [[ "${MOCK_NO_GC:-0}" == 1 ]]; then printf 'check\n'; else printf 'prepare check recover-affinity\n'; fi
elif [[ "$args" == *' gc check '* ]]; then [[ "${MOCK_CHECK_FAIL:-0}" != 1 ]]
elif [[ "$args" == *' gc prepare '* ]]; then printf 'prepare\n' >>"$MOCK_ACTIONS"
else exit 2
fi
SH
  chmod +x "$GC_MOCK" "$AO_MOCK"
  export GC_BIN="$GC_MOCK" AO_BIN="$AO_MOCK" MOCK_ACTIONS="$FIX/actions"
  PREPARE_TOKEN=$("$RUN" token prepare --city "$FIX/city" --rig "$FIX/city/rigs/demo")
  DISPATCH_TOKEN=$("$RUN" token dispatch --city "$FIX/city" --rig "$FIX/city/rigs/demo" --bead ago-123 --message-file "$FIX/message")
}

teardown() { rm -rf "$FIX"; }

@test "approved prepare post-checks and approved source intent reaches Mayor once" {
  run "$RUN" prepare --city "$FIX/city" --rig "$FIX/city/rigs/demo" --approve "$PREPARE_TOKEN"
  [ "$status" -eq 0 ]
  run "$RUN" dispatch --city "$FIX/city" --rig "$FIX/city/rigs/demo" --bead ago-123 --message-file "$FIX/message" --receipt "$FIX/out/receipt.json" --approve "$DISPATCH_TOKEN"
  [ "$status" -eq 0 ]
  [ "$(wc -l <"$MOCK_ACTIONS" | tr -d ' ')" -eq 2 ]
  jq -e '.status == "dispatched_to_mayor" and .not_checked == ["gas-city-runtime-implementation","mayor-processing","model-dispatch"]' "$FIX/out/receipt.json" >/dev/null
}

@test "raw sling baseline acts while wrapper has no sling and missing approval cannot dispatch" {
  run "$GC_MOCK" sling worker ago-123
  [ "$status" -eq 0 ]
  [ -s "$MOCK_ACTIONS" ]
  rm -f "$MOCK_ACTIONS"
  run "$RUN" sling --city "$FIX/city" --rig "$FIX/city/rigs/demo" --approve anything
  [ "$status" -ne 0 ]
  run "$RUN" dispatch --city "$FIX/city" --rig "$FIX/city/rigs/demo" --bead ago-123 --message-file "$FIX/message" --receipt "$FIX/out/receipt.json"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "unreleased or incomplete runtime stops before prepare or mail" {
  export MOCK_EDGE=1
  run "$RUN" prepare --city "$FIX/city" --rig "$FIX/city/rigs/demo" --approve "$PREPARE_TOKEN"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
  unset MOCK_EDGE
  export MOCK_NO_GC=1
  run "$RUN" dispatch --city "$FIX/city" --rig "$FIX/city/rigs/demo" --bead ago-123 --message-file "$FIX/message" --receipt "$FIX/out/receipt.json" --approve "$DISPATCH_TOKEN"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "failed qualification and unsafe city or rig stop before dispatch" {
  export MOCK_CHECK_FAIL=1
  run "$RUN" dispatch --city "$FIX/city" --rig "$FIX/city/rigs/demo" --bead ago-123 --message-file "$FIX/message" --receipt "$FIX/out/receipt.json" --approve "$DISPATCH_TOKEN"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
  unset MOCK_CHECK_FAIL
  ln -s "$FIX/city" "$FIX/city-link"
  run "$RUN" token prepare --city "$FIX/city-link" --rig "$FIX/city/rigs/demo"
  [ "$status" -ne 0 ]
  run "$RUN" token prepare --city "$FIX/city" --rig "$FIX/out"
  [ "$status" -ne 0 ]
}

@test "failed Mayor notify preserves a non-success receipt and does not claim dispatch" {
  export MOCK_NOTIFY_FAIL=1
  run "$RUN" dispatch --city "$FIX/city" --rig "$FIX/city/rigs/demo" --bead ago-123 --message-file "$FIX/message" --receipt "$FIX/out/receipt.json" --approve "$DISPATCH_TOKEN"
  [ "$status" -eq 3 ]
  jq -e '.status == "mail_unattested" and (.not_checked | index("mayor-notify"))' "$FIX/out/receipt.json" >/dev/null
  [ "$(grep -c '^mail$' "$MOCK_ACTIONS")" -eq 1 ]
}
