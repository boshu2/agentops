#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  BOOTSTRAP="$REPO_ROOT/deploy/gc/bootstrap.sh"
  TMP="$(mktemp -d "${TMPDIR:-/tmp}/gc-agentops-bootstrap.XXXXXX")"
  CITY="$TMP/city"
  RIG="$TMP/rig"
  PACK="$TMP/agentops-pack"
  BIN="$TMP/bin"
  FAKE_STATE="$TMP/fake-state"
  FAKE_LOG="$TMP/gc.log"
  CODEX_AUTH="$TMP/codex/auth.json"

  mkdir -p "$RIG" "$PACK" "$BIN" "$FAKE_STATE" "$(dirname "$CODEX_AUTH")"
  printf '%s\n' '[pack]' 'name = "agentops-executor"' 'schema = 2' >"$PACK/pack.toml"
  printf '%s\n' '{"fake":"auth"}' >"$CODEX_AUTH"

  cat >"$BIN/codex" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[ "${1:-}" = "login" ] && [ "${2:-}" = "status" ]
[ -L "${CODEX_HOME:?}/auth.json" ]
printf '%s\n' 'Logged in'
EOF
  chmod +x "$BIN/codex"

cat >"$BIN/claude" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  --version)
    printf '%s\n' '2.1.211 (Claude Code)'
    ;;
  auth)
    [ "${2:-}" = "status" ]
    [ "${3:-}" = "--json" ]
    printf '%s\n' '{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty"}'
    ;;
  auto-mode)
    [ "${2:-}" = "defaults" ]
    printf '%s\n' '{"allow":[],"soft_deny":[],"hard_deny":[]}'
    ;;
  *) exit 2 ;;
esac
EOF
  chmod +x "$BIN/claude"

  FAKE_GC="$BIN/gc"
  cat >"$FAKE_GC" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

fake_root="$(cd "$(dirname "$0")/.." && pwd)"
FAKE_GC_LOG="$fake_root/gc.log"
FAKE_GC_STATE="$fake_root/fake-state"

printf 'ENV GC_HOME=%s GC_ISOLATED=%s CODEX_HOME=%s GC_BEADS=%s GC_DOLT_PORT=%s BEADS_DOLT_SERVER_PORT=%s GC_CITY=%s BD_DOLT_SYNC_CLI_REMOTES=%s BEADS_DOLT_SYNC_CLI_REMOTES=%s\n' \
  "${GC_HOME-<unset>}" "${GC_ISOLATED-<unset>}" "${CODEX_HOME-<unset>}" \
  "${GC_BEADS-<unset>}" "${GC_DOLT_PORT-<unset>}" \
  "${BEADS_DOLT_SERVER_PORT-<unset>}" "${GC_CITY-<unset>}" \
  "${BD_DOLT_SYNC_CLI_REMOTES-<unset>}" "${BEADS_DOLT_SYNC_CLI_REMOTES-<unset>}" >>"$FAKE_GC_LOG"
printf 'ARGS' >>"$FAKE_GC_LOG"
printf ' <%s>' "$@" >>"$FAKE_GC_LOG"
printf '\n' >>"$FAKE_GC_LOG"

city=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --city)
      city="$2"
      shift 2
      ;;
    --rig)
      shift 2
      ;;
    *) break ;;
  esac
done

command_name="${1:-}"
case "$command_name" in
  lint)
    printf '%s\n' '{"ok":true}'
    ;;
  init)
    target="${!#}"
    template=""
    while [ "$#" -gt 0 ]; do
      case "$1" in
        --file)
          template="$2"
          shift 2
          ;;
        *) shift ;;
      esac
    done
    mkdir -p "$target/.gc"
    printf '%s\n' \
      '[pack]' \
      'name = "generated-city"' \
      'schema = 2' \
      '' \
      '[imports.core]' \
      'source = "builtin:core"' \
      'version = "sha:generated"' >"$target/pack.toml"
    cp "$template" "$target/city.toml"
    printf '%s\n' 'workspace_name = "generated-city"' >"$target/.gc/site.toml"
    ;;
  rig)
    case "${2:-}" in
      add)
        if [ "${FAKE_GC_FAIL_RIG_ADD_ONCE:-0}" = "1" ] && [ ! -e "$FAKE_GC_STATE/rig-add-failed" ]; then
          touch "$FAKE_GC_STATE/rig-add-failed"
          exit 7
        fi
        rig_path="${3:?}"
        shift 3
        rig_name=""
        while [ "$#" -gt 0 ]; do
          case "$1" in
            --name)
              rig_name="$2"
              shift 2
              ;;
            *) shift ;;
          esac
        done
        if [ ! -e "$FAKE_GC_STATE/rig-added" ]; then
          printf '\n[[rigs]]\nname = "%s"\nsuspended_on_start = true\n' \
            "$rig_name" >>"$city/city.toml"
          printf '\n[rigs.%s]\npath = "%s"\n' "$rig_name" "$rig_path" >>"$city/.gc/site.toml"
        fi
        touch "$FAKE_GC_STATE/rig-added"
        ;;
      resume)
        touch "$FAKE_GC_STATE/rig-resumed"
        ;;
      *) exit 2 ;;
    esac
    ;;
  config)
    printf '%s\n' 'resolved config'
    ;;
  import)
    [ "${2:-}" = "status" ] || exit 2
    printf '%s\n' '{"ok":true}'
    ;;
  start)
    touch "$FAKE_GC_STATE/started"
    ;;
  *)
    printf 'unexpected fake gc invocation: %s\n' "$command_name" >&2
    exit 2
    ;;
esac
EOF
  chmod +x "$FAKE_GC"

}

teardown() {
  rm -rf "$TMP"
}

run_bootstrap() {
  PATH="$BIN:$PATH" "$BOOTSTRAP" \
    --city "$CITY" \
    --rig "$RIG" \
    --pack "$PACK" \
    --gc-bin "$FAKE_GC" \
    --codex-auth "$CODEX_AUTH" \
    "$@"
}

@test "bootstrap creates an isolated dual-provider city without starting it" {
  export GC_BEADS=file
  export GC_DOLT_PORT=39999
  export BEADS_DOLT_SERVER_PORT=39998
  export GC_CITY=/old/city
  export CODEX_HOME=/old/codex-home

  run run_bootstrap
  [ "$status" -eq 0 ]
  [ -f "$CITY/.gc/agentops-bootstrap.json" ]
  [ -f "$CITY/.gc-home/supervisor.toml" ]
  [ ! -e "$FAKE_STATE/started" ]

  grep -Fq '[supervisor]' "$CITY/.gc-home/supervisor.toml"
  grep -Fq 'bind = "127.0.0.1"' "$CITY/.gc-home/supervisor.toml"
  supervisor_port="$(python3 - "$CITY/.gc-home/supervisor.toml" <<'PY'
import sys
import tomllib

with open(sys.argv[1], "rb") as handle:
    port = tomllib.load(handle)["supervisor"]["port"]
print(port)
PY
)"
  [ "$supervisor_port" -gt 0 ]
  [ "$supervisor_port" -le 65535 ]

  grep -Fq '[imports.agentops]' "$CITY/pack.toml"
  grep -Fq '[rigs.imports.agentops]' "$CITY/city.toml"
  grep -Fq '[providers.codex]' "$CITY/city.toml"
  grep -Fq '[providers.claude]' "$CITY/city.toml"
  grep -Fq 'print_args = []' "$CITY/city.toml"
  ! grep -Eq '^provider[[:space:]]*=' "$CITY/city.toml"
  python3 - "$CITY/city.toml" <<'PY'
import os
import sys
import tomllib

with open(sys.argv[1], "rb") as handle:
    config = tomllib.load(handle)
patches = config["patches"]["agent"]
assert [
    patch for patch in patches
    if patch.get("name") == "bd.dog" and not patch.get("dir") and patch.get("suspended") is True
] == [{"name": "bd.dog", "suspended": True}]
assert [
    patch for patch in patches
    if patch.get("name") == "core.control-dispatcher" and not patch.get("dir") and patch.get("suspended") is True
] == [{"name": "core.control-dispatcher", "suspended": True}]
assert [
    patch for patch in patches
    if patch.get("name") == "codex" and not patch.get("dir") and patch.get("suspended") is True
] == [{"name": "codex", "suspended": True}]
assert [
    patch for patch in patches
    if patch.get("name") == "claude" and not patch.get("dir") and patch.get("suspended") is True
] == [{"name": "claude", "suspended": True}]
assert [
    patch for patch in patches
    if patch.get("name") == "codex" and patch.get("dir") == "agentops" and patch.get("suspended") is True
] == [{"dir": "agentops", "name": "codex", "suspended": True}]
assert [
    patch for patch in patches
    if patch.get("name") == "claude" and patch.get("dir") == "agentops" and patch.get("suspended") is True
] == [{"dir": "agentops", "name": "claude", "suspended": True}]
assert [
    patch for patch in patches
    if patch.get("name") == "core.control-dispatcher" and patch.get("dir") == "agentops" and patch.get("suspended") is True
] == [{"dir": "agentops", "name": "core.control-dispatcher", "suspended": True}]
city = os.path.realpath(os.path.dirname(sys.argv[1]))
rig = os.path.realpath(os.path.join(city, "..", "rig"))
scope_roots = [os.path.join(city, ".gc"), os.path.join(city, ".gc-home"), rig]
expected_codex_args = []
for root in scope_roots:
    expected_codex_args.extend(["--add-dir", root])
assert config["providers"]["codex"]["args_append"] == expected_codex_args
assert config["providers"]["claude"]["args_append"] == ["--add-dir", *scope_roots]
for provider_name, expected_default, expected_choices in (
    (
        "codex",
        "auto-edit",
        {
            "auto-edit": [
                "--sandbox",
                "workspace-write",
                "--ask-for-approval",
                "never",
                "-c",
                "sandbox_workspace_write.network_access=true",
            ],
            "unrestricted": ["--dangerously-bypass-approvals-and-sandbox"],
        },
    ),
    ("claude", "auto", {"auto": ["--permission-mode", "auto"]}),
):
    provider = config["providers"][provider_name]
    assert provider["option_defaults"]["permission_mode"] == expected_default
    assert provider["options_schema_merge"] == "replace"
    options = {option["key"]: option for option in provider["options_schema"]}
    option = options["permission_mode"]
    actual_choices = {
        choice["value"]: choice["flag_args"] for choice in option["choices"]
    }
    assert actual_choices == expected_choices
PY
  canonical_gc="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$FAKE_GC")"
  grep -Fq "GC_BIN = \"$canonical_gc\"" "$CITY/city.toml"
  ! grep -Eq 'antigravity|agy|named_session|\[\[agent\]\]' "$CITY/city.toml"
  ! grep -Fq "path = \"$RIG\"" "$CITY/city.toml"
  grep -Fq 'path = "' "$CITY/.gc/site.toml"

  canonical_city="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$CITY")"
  expected_home="GC_HOME=$canonical_city/.gc-home"
  expected_codex="CODEX_HOME=$canonical_city/.gc/codex-home"
  grep -Fq "$expected_home" "$FAKE_LOG"
  grep -Fq 'GC_ISOLATED=1' "$FAKE_LOG"
  grep -Fq "$expected_codex" "$FAKE_LOG"
  grep -Fq 'BD_DOLT_SYNC_CLI_REMOTES=false BEADS_DOLT_SYNC_CLI_REMOTES=false' "$FAKE_LOG"
  ! grep -Ev 'GC_BEADS=<unset>.*GC_DOLT_PORT=<unset>.*BEADS_DOLT_SERVER_PORT=<unset>.*GC_CITY=<unset>' \
    < <(grep '^ENV ' "$FAKE_LOG")

  grep -Fq 'ARGS <init> <--file>' "$FAKE_LOG"
  grep -Fq 'ARGS <--city>' "$FAKE_LOG"
  grep -Fq '<rig> <add>' "$FAKE_LOG"
  grep -Fq 'ARGS <lint>' "$FAKE_LOG"
  grep -Fq '<config> <show>' "$FAKE_LOG"
  grep -Fq '<config> <explain>' "$FAKE_LOG"
  grep -Fq '<config> <explain> <--provider> <codex> <--json>' "$FAKE_LOG"
  grep -Fq '<config> <explain> <--provider> <claude> <--json>' "$FAKE_LOG"
  grep -Fq '<import> <status> <--json>' "$FAKE_LOG"
}

@test "bootstrap is idempotent after successful configuration" {
  run run_bootstrap
  [ "$status" -eq 0 ]
  supervisor_config_digest="$(shasum -a 256 "$CITY/.gc-home/supervisor.toml" | awk '{print $1}')"

  run run_bootstrap
  [ "$status" -eq 0 ]

  [ "$(grep -c 'ARGS <init>' "$FAKE_LOG")" -eq 1 ]
  [ "$(grep -c '<rig> <add>' "$FAKE_LOG")" -eq 2 ]
  [ "$(grep -c '^\[imports.agentops\]$' "$CITY/pack.toml")" -eq 1 ]
  [ "$(grep -c '^\[rigs.imports.agentops\]$' "$CITY/city.toml")" -eq 1 ]
  [ "$(grep -c '^\[\[patches.agent\]\]$' "$CITY/city.toml")" -eq 7 ]
  [ "$(shasum -a 256 "$CITY/.gc-home/supervisor.toml" | awk '{print $1}')" = "$supervisor_config_digest" ]
}

@test "bootstrap preserves interleaved extra rigs and refreshes provider directory grants" {
  run run_bootstrap
  [ "$status" -eq 0 ]

  extra_rig="$TMP/extra-rig"
  mkdir -p "$extra_rig"
  printf '\n[[rigs]]\nname = "extra"\nprefix = "ex"\nsuspended_on_start = true\n\n[rigs.imports.agentops]\nsource = "%s"\n' \
    "$PACK" >>"$CITY/city.toml"
  printf '\n[[rig]]\nname = "extra"\npath = "%s"\n' "$extra_rig" >>"$CITY/.gc/site.toml"

  run run_bootstrap
  [ "$status" -eq 0 ]
  python3 - "$CITY/city.toml" "$extra_rig" <<'PY'
import os
import sys
import tomllib

with open(sys.argv[1], "rb") as handle:
    config = tomllib.load(handle)
extra = os.path.realpath(sys.argv[2])
assert [rig["name"] for rig in config["rigs"]] == ["agentops", "extra"]
patches = config["patches"]["agent"]
for provider in ("codex", "claude"):
    assert [
        patch for patch in patches
        if patch.get("name") == provider and patch.get("dir") == "extra"
    ] == [{"dir": "extra", "name": provider, "suspended": True}]
assert [
    patch for patch in patches
    if patch.get("name") == "core.control-dispatcher" and patch.get("dir") == "extra"
] == [{"dir": "extra", "name": "core.control-dispatcher", "suspended": True}]
assert extra in config["providers"]["codex"]["args_append"]
assert extra in config["providers"]["claude"]["args_append"]
PY
}

@test "bootstrap repairs a managed city after rig registration fails" {
  export FAKE_GC_FAIL_RIG_ADD_ONCE=1
  run run_bootstrap
  [ "$status" -ne 0 ]
  [ -f "$CITY/.gc/agentops-bootstrap.json" ]
  [ ! -e "$FAKE_STATE/rig-added" ]

  unset FAKE_GC_FAIL_RIG_ADD_ONCE
  run run_bootstrap
  [ "$status" -eq 0 ]
  [ -e "$FAKE_STATE/rig-added" ]
  [ "$(grep -c '^\[\[rigs\]\]$' "$CITY/city.toml")" -eq 1 ]
  [ "$(grep -c '^\[rigs.imports.agentops\]$' "$CITY/city.toml")" -eq 1 ]
}

@test "bootstrap refuses an existing unmanaged city" {
  mkdir -p "$CITY"
  printf '%s\n' '[pack]' 'name = "historical"' 'schema = 2' >"$CITY/pack.toml"

  run run_bootstrap
  [ "$status" -ne 0 ]
  [[ "$output" == *"refusing existing unmanaged city"* ]]
  [ ! -s "$FAKE_LOG" ]
}

@test "start is opt-in" {
  run run_bootstrap --start
  [ "$status" -eq 0 ]
  [ -e "$FAKE_STATE/started" ]
  [ -e "$FAKE_STATE/rig-resumed" ]
  [ "$(grep -c '<start>$' "$FAKE_LOG")" -eq 1 ]
  grep -Fq '<rig> <resume> <agentops> <--json>' "$FAKE_LOG"
}

@test "factory bootstrap can raise the bounded city concurrency cap" {
  run run_bootstrap --max-active-sessions 8
  [ "$status" -eq 0 ]

  python3 - "$CITY/city.toml" "$CITY/.gc/agentops-bootstrap.json" <<'PY'
import json
import sys
import tomllib

with open(sys.argv[1], "rb") as handle:
    config = tomllib.load(handle)
with open(sys.argv[2], encoding="utf-8") as handle:
    marker = json.load(handle)
assert config["workspace"]["max_active_sessions"] == 8
assert marker["max_active_sessions"] == 8
PY
}

@test "portable deployment sources contain no host-specific path" {
  run grep '/Users/bo' "$REPO_ROOT/deploy/gc/bootstrap.sh" "$REPO_ROOT/deploy/gc/city.toml"
  [ "$status" -eq 1 ]
}
