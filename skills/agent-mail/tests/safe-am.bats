#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  RUN="$SKILL_DIR/scripts/safe-am.sh"
  FIX="$(mktemp -d)"
  FIX="$(cd "$FIX" && pwd -P)"
  mkdir -p "$FIX/project/.git" "$FIX/backups"
  printf 'bounded body\n' >"$FIX/body.md"
  MOCK="$FIX/am"
  cat >"$MOCK" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  --version) printf 'am 9.9.9\n'; exit 0 ;;
  capabilities)
    if [[ -n "${MOCK_CAPABILITIES:-}" ]]; then
      printf '%s\n' "$MOCK_CAPABILITIES"
    else
      printf '%s\n' '{"schema_version":"am.capabilities.v1","tool":"am","commands":[{"name":"mail","direct_subcommands":["send"]},{"name":"file_reservations","direct_subcommands":["conflicts","reserve","release"]},{"name":"guard","direct_subcommands":["install"]},{"name":"doctor","direct_subcommands":["repair"]}]}'
    fi
    exit 0
    ;;
  clear-and-reset-everything)
    printf 'reset\n' >>"$MOCK_ACTIONS"
    exit 0
    ;;
esac
printf '%s\n' "$*" >>"$MOCK_ACTIONS"
if [[ " $* " == *' doctor repair '* && " $* " == *' --dry-run '* ]]; then printf 'dry run\n'; fi
if [[ " $* " == *' mail send '* ]]; then printf '{"message_id":7}\n'; fi
SH
  chmod +x "$MOCK"
  export AM_BIN="$MOCK"
  export MOCK_ACTIONS="$FIX/actions"
  BODY_DIGEST=$(printf '%s' "$(cat "$FIX/body.md")" | shasum -a 256 | awk '{print $1}')
  HANDOFF_SUBJECT_DIGEST=$(printf '%s' 'bounded handoff' | shasum -a 256 | awk '{print $1}')
  TEST_SUBJECT_DIGEST=$(printf '%s' test | shasum -a 256 | awk '{print $1}')
}

teardown() { rm -rf "$FIX"; }

@test "authorized message sends once through an attested surface" {
  run "$RUN" send --project "$FIX/project" --from GreenCastle --to BlueLake --thread task-7 --subject 'bounded handoff' --body-file "$FIX/body.md" --approve "am:send:$FIX/project:GreenCastle:BlueLake:task-7:$HANDOFF_SUBJECT_DIGEST:$BODY_DIGEST:false"
  [ "$status" -eq 0 ]
  [ "$(grep -c 'mail send' "$MOCK_ACTIONS")" -eq 1 ]
}

@test "raw destructive baseline runs while package reset remains blocked" {
  run "$MOCK" clear-and-reset-everything --force --no-archive
  [ "$status" -eq 0 ]
  [ -s "$MOCK_ACTIONS" ]
  rm -f "$MOCK_ACTIONS"
  run "$RUN" reset --project "$FIX/project" --approve anything
  [ "$status" -eq 4 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "missing send capability and missing approval stop before a message" {
  export MOCK_CAPABILITIES='{"schema_version":"am.capabilities.v1","tool":"am","commands":[]}'
  run "$RUN" send --project "$FIX/project" --from GreenCastle --to BlueLake --thread task-7 --subject test --body-file "$FIX/body.md" --approve "am:send:$FIX/project:GreenCastle:BlueLake:task-7:$TEST_SUBJECT_DIGEST:$BODY_DIGEST:false"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
  unset MOCK_CAPABILITIES
  run "$RUN" send --project "$FIX/project" --from GreenCastle --to BlueLake --thread task-7 --subject test --body-file "$FIX/body.md"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "approval binds subject body and acknowledgement semantics" {
  run "$RUN" send --project "$FIX/project" --from GreenCastle --to BlueLake --thread task-7 --subject changed --body-file "$FIX/body.md" --approve "am:send:$FIX/project:GreenCastle:BlueLake:task-7:$HANDOFF_SUBJECT_DIGEST:$BODY_DIGEST:false"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
  run "$RUN" send --project "$FIX/project" --from GreenCastle --to BlueLake --thread task-7 --subject 'bounded handoff' --body-file "$FIX/body.md" --ack-required --approve "am:send:$FIX/project:GreenCastle:BlueLake:task-7:$HANDOFF_SUBJECT_DIGEST:$BODY_DIGEST:false"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "hook install is repo-bound and exact-approval-bound" {
  run "$RUN" guard-install --project "$FIX/project" --repo "$FIX/project" --approve "am:guard-install:$FIX/project:$FIX/project"
  [ "$status" -eq 0 ]
  grep -q 'guard install' "$MOCK_ACTIONS"
  rm -f "$MOCK_ACTIONS"
  run "$RUN" guard-install --project "$FIX/project" --repo "$FIX" --approve "am:guard-install:$FIX/project:$FIX"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}
