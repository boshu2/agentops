#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  RUN="$SKILL_DIR/scripts/dispatch.sh"
  FIX="$(mktemp -d)"
  FIX="$(cd "$FIX" && pwd -P)"
  mkdir -p "$FIX/work" "$FIX/out"
  printf 'perform one task\n' >"$FIX/prompt"
  MOCK="$FIX/ntm"
  cat >"$MOCK" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  --robot-capabilities)
    name=''; for arg in "$@"; do case "$arg" in --capability-command=*) name=${arg#*=};; esac; done
    if [[ "${MOCK_MISSING_CAP:-}" == "$name" ]]; then printf '{"success":true,"version":"v9.9.9","commands":[]}\n';
    else printf '{"success":true,"version":"v9.9.9","commands":[{"name":"%s"}]}\n' "$name"; fi
    exit 0
    ;;
  --robot-snapshot)
    printf '{"success":true,"session":"session-a","pane":"%%7","workspace":"%s"}\n' "$MOCK_WORKSPACE"
    exit 0
    ;;
  --robot-send=*)
    if [[ " $* " == *' --dry-run '* ]]; then printf '{"success":true,"dry_run":true}\n'; exit 0; fi
    printf '%s\n' "$*" >>"$MOCK_ACTIONS"
    [[ "${MOCK_HANG:-0}" == 1 ]] && sleep 30
    printf '{"success":true,"acknowledged":true}\n'
    exit 0
    ;;
  --robot-tail=*) printf '{"success":true,"tail":["engaged"]}\n'; exit 0 ;;
esac
exit 2
SH
  chmod +x "$MOCK"
  export NTM_BIN="$MOCK"
  export MOCK_ACTIONS="$FIX/actions"
  export MOCK_WORKSPACE="$FIX/work"
  DIGEST=$(shasum -a 256 "$FIX/prompt" | awk '{print $1}')
  APPROVAL="ntm:send:session-a:%7:$FIX/work:$DIGEST"
}

teardown() { rm -rf "$FIX"; }

@test "normal dispatch binds exact existing pane and writes receipt" {
  run "$RUN" --session session-a --pane %7 --workspace "$FIX/work" --prompt-file "$FIX/prompt" --receipt "$FIX/out/receipt.json" --observe 5 --approve "$APPROVAL"
  [ "$status" -eq 0 ]
  jq -e '.acknowledged == true' "$FIX/out/receipt.json" >/dev/null
  [ "$(wc -l <"$MOCK_ACTIONS" | tr -d ' ')" -eq 1 ]
}

@test "raw send baseline dispatches while missing approval stops before action" {
  run "$MOCK" --robot-send=session-a --pane=%7 --msg=raw
  [ "$status" -eq 0 ]
  [ -s "$MOCK_ACTIONS" ]
  rm -f "$MOCK_ACTIONS"
  run "$RUN" --session session-a --pane %7 --workspace "$FIX/work" --prompt-file "$FIX/prompt" --receipt "$FIX/out/receipt.json"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "missing capability and unsafe pane stop before dispatch" {
  export MOCK_MISSING_CAP=snapshot
  run "$RUN" --session session-a --pane %7 --workspace "$FIX/work" --prompt-file "$FIX/prompt" --receipt "$FIX/out/receipt.json" --approve "$APPROVAL"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
  unset MOCK_MISSING_CAP
  run "$RUN" --session session-a --pane '../7' --workspace "$FIX/work" --prompt-file "$FIX/prompt" --receipt "$FIX/out/receipt.json" --approve anything
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "hung tracked send is a timeout rather than completion" {
  export MOCK_HANG=1
  run "$RUN" --session session-a --pane %7 --workspace "$FIX/work" --prompt-file "$FIX/prompt" --receipt "$FIX/out/receipt.json" --observe 1 --approve "$APPROVAL"
  [ "$status" -eq 124 ]
}
