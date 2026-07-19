#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  BOOTSTRAP="$REPO_ROOT/deploy/gc/bootstrap.sh"
  TMP="$(mktemp -d "${TMPDIR:-/tmp}/gc-agentops-bootstrap.XXXXXX")"
  CITY="$TMP/city"
  RIG="$TMP/rig"
  REMOTE="$TMP/origin.git"
  PACK="$TMP/agentops-pack"
  BIN="$TMP/bin"
  FAKE_STATE="$TMP/fake-state"
  FAKE_LOG="$TMP/gc.log"
  CODEX_AUTH="$TMP/codex/auth.json"

  mkdir -p "$RIG" "$PACK" "$BIN" "$FAKE_STATE" "$(dirname "$CODEX_AUTH")"
  git init -q -b main "$RIG"
  git -C "$RIG" config user.name "GC Bootstrap Test"
  git -C "$RIG" config user.email "gc-bootstrap@example.invalid"
  printf '%s\n' 'bootstrap fixture' >"$RIG/README.md"
  git -C "$RIG" add README.md
  git -C "$RIG" commit -qm "bootstrap fixture"
  git init -q --bare "$REMOTE"
  git -C "$RIG" remote add origin "$REMOTE"
  git -C "$RIG" push -q -u origin main
  git -C "$REMOTE" update-ref refs/dolt/data "$(git -C "$RIG" rev-parse HEAD)"
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
fake_root="$(cd "$(dirname "$0")/.." && pwd)"
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
  --debug-file)
    debug_file="${2:?}"
    shift 2
    printf '%s\n' 'fake Claude debug trace' >"$debug_file"
    printf 'ARGS' >"$fake_root/claude-interactive.log"
    printf ' <%s>' "$@" >>"$fake_root/claude-interactive.log"
    printf '\n' >>"$fake_root/claude-interactive.log"
    exit "${FAKE_CLAUDE_INTERACTIVE_EXIT:-0}"
    ;;
  *) exit 2 ;;
esac
EOF
  chmod +x "$BIN/claude"

  cat >"$BIN/bd" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[ "${1:-}" = "version" ]
printf '%s\n' 'bd version 1.1.0 (8e4e59d39: test-build)'
EOF
  chmod +x "$BIN/bd"

  FAKE_GC="$BIN/gc"
  cat >"$FAKE_GC" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

fake_root="$(cd "$(dirname "$0")/.." && pwd)"
FAKE_GC_LOG="$fake_root/gc.log"
FAKE_GC_STATE="$fake_root/fake-state"

printf 'ENV GC_HOME=%s GC_ISOLATED=%s CODEX_HOME=%s OTEL_SDK_DISABLED=%s GC_BEADS=%s GC_DOLT_PORT=%s BEADS_DOLT_SERVER_PORT=%s GC_CITY=%s BD_DOLT_SYNC_CLI_REMOTES=%s BEADS_DOLT_SYNC_CLI_REMOTES=%s\n' \
  "${GC_HOME-<unset>}" "${GC_ISOLATED-<unset>}" "${CODEX_HOME-<unset>}" \
  "${OTEL_SDK_DISABLED-<unset>}" \
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
  version)
    [ "${2:-}" = "--json" ] || exit 2
    printf '{"ok":true,"schema_version":"1","version":"1.3.5","commit":"%s","date":"2026-07-12T00:00:00Z"}\n' \
      "${FAKE_GC_COMMIT:-8ffc009d}"
    ;;
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
    if [ "${FAKE_GC_REJECT_INIT_PATCHES:-0}" = "1" ] && \
        grep -Fq '[[patches.agent]]' "$template"; then
      printf '%s\n' 'gc init: patches cannot target scaffold agents before imports resolve' >&2
      exit 9
    fi
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
        adopt=0
        rig_path="${3:?}"
        shift 3
        rig_name=""
        while [ "$#" -gt 0 ]; do
          case "$1" in
            --name)
              rig_name="$2"
              shift 2
              ;;
            --adopt)
              adopt=1
              shift
              ;;
            *) shift ;;
          esac
        done
        origin_url="$(git -C "$rig_path" config --get remote.origin.url 2>/dev/null || true)"
        printf 'RIG_ORIGIN <%s> ADOPT <%s>\n' "${origin_url:-<unset>}" "$adopt" >>"$FAKE_GC_LOG"
        if [ "$adopt" -eq 0 ] && [ -n "$origin_url" ] && \
            [ -n "$(git ls-remote "$origin_url" refs/dolt/data 2>/dev/null)" ]; then
          mkdir -p "$rig_path/.beads"
          printf '%s\n' '{"backend":"dolt"}' >"$rig_path/.beads/metadata.json"
          printf '%s\n' 'issue_prefix: test' >"$rig_path/.beads/config.yaml"
          printf '%s\n' "bd init refuses: remote 'origin' already has Dolt history (refs/dolt/data)." >&2
          exit 10
        fi
        if [ "${FAKE_GC_FAIL_RIG_ADD_ONCE:-0}" = "1" ] && [ ! -e "$FAKE_GC_STATE/rig-add-failed" ]; then
          touch "$FAKE_GC_STATE/rig-add-failed"
          mkdir -p "$rig_path/.beads"
          printf '%s\n' '{"backend":"dolt"}' >"$rig_path/.beads/metadata.json"
          printf '%s\n' 'issue_prefix: test' >"$rig_path/.beads/config.yaml"
          exit 7
        fi
        mkdir -p "$rig_path/.beads"
        printf '%s\n' '{"backend":"dolt"}' >"$rig_path/.beads/metadata.json"
        printf '%s\n' 'issue_prefix: test' >"$rig_path/.beads/config.yaml"
        if [ "${FAKE_GC_COMMIT_ON_RIG_ADD:-0}" = "1" ] || \
            [ "${FAKE_GC_STAGE_ON_RIG_ADD:-0}" = "1" ]; then
		  if [ "${FAKE_GC_APPEND_GITIGNORE:-0}" = "1" ]; then
		    printf '\n%s\n%s\n%s\n%s\n%s\n' \
		      '# Beads / Dolt files (added by bd init)' \
		      '.dolt/' \
		      '*.db' \
		      '.beads-credential-key' \
		      '.beads/proxieddb/' >>"$rig_path/.gitignore"
          fi
          git -C "$rig_path" add .beads/metadata.json .gitignore
          if [ "${FAKE_GC_APPEND_RUNTIME_GITIGNORE:-0}" = "1" ]; then
            printf '\n%s\n%s\n%s\n' \
              '# Gas City' \
              '.beads/*' \
              '!.beads/identity.toml' >>"$rig_path/.gitignore"
          fi
          if [ "${FAKE_GC_COMMIT_ON_RIG_ADD:-0}" = "1" ]; then
            git -C "$rig_path" commit -qm "bd init: initialize beads issue tracking"
          fi
          printf '%s\n' '{"backend":"dolt"}' >"$rig_path/.beads/metadata.json"
        fi
        if [ "${FAKE_GC_REWRITE_GITIGNORE:-0}" = "1" ]; then
          python3 - "$rig_path/.gitignore" <<'PY'
import sys

path = sys.argv[1]
with open(path, "rb") as handle:
    before = handle.read()
lines = [line for line in before.split(b"\n") if line.strip() != b".beads/"]
cleaned = b"\n".join(lines)
if cleaned and not cleaned.endswith(b"\n"):
    cleaned += b"\n"
if cleaned and not cleaned.endswith(b"\n\n"):
    cleaned += b"\n"
with open(path, "wb") as handle:
    handle.write(cleaned + b"# Gas City\n.beads/*\n!.beads/identity.toml\n")
PY
        fi
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
  agent)
    [ "${2:-}" = "list" ] || exit 2
    [ "${3:-}" = "--json" ] || exit 2
    python3 - "$city/city.toml" <<'PY'
import json
import sys
import tomllib

with open(sys.argv[1], "rb") as handle:
    config = tomllib.load(handle)
rigs = config.get("rigs", [])
if not rigs:
    raise SystemExit("fake gc expected at least one rig")
rig = rigs[0]
rig_name = rig["name"]
imports = rig.get("imports", {})
if len(imports) != 1:
    raise SystemExit(f"fake gc expected one rig import, found {len(imports)}")
binding = next(iter(imports))
patches = config.get("patches", {}).get("agent", [])
agents = []
for role, provider in (
    ("implementer", "codex"),
    ("implementer-claude", "claude"),
    ("validator", "codex"),
    ("validator-claude", "claude"),
):
    local_name = f"{binding}.{role}"
    workspace_patches = [
        patch for patch in patches
        if patch.get("dir") == rig_name and patch.get("name") == local_name
    ]
    agent = {
        "name": role,
        "qualified_name": f"{rig_name}/{local_name}",
        "dir": rig_name,
        "scope": "rig",
        "provider": provider,
        "suspended": False,
    }
    if len(workspace_patches) == 1 and "work_dir" in workspace_patches[0]:
        agent["work_dir"] = workspace_patches[0]["work_dir"]
    agents.append(agent)
print(json.dumps({"ok": True, "agents": agents}))
PY
    ;;
  import)
    [ "${2:-}" = "status" ] || exit 2
    printf '%s\n' '{"ok":true}'
    ;;
  supervisor)
    case "${2:-}" in
      status)
        if [ -e "$FAKE_GC_STATE/supervisor-running" ]; then
          pid="$(cat "$FAKE_GC_STATE/supervisor-pid")"
          printf '{"ok":true,"running":true,"pid":%s}\n' "$pid"
        else
          printf '%s\n' '{"ok":true,"running":false,"pid":0}'
        fi
        ;;
      stop)
        rm -f "$FAKE_GC_STATE/supervisor-running"
        ;;
      install)
        [ "${3:-}" = "--force" ]
        touch "$FAKE_GC_STATE/supervisor-force-installed"
        ;;
      *) exit 2 ;;
    esac
    ;;
  start)
    mkdir -p "$city/.gc"
    cp "$city/.claude/settings.json" "$city/.gc/settings.json"
    generation=0
    if [ -f "$FAKE_GC_STATE/supervisor-generation" ]; then
      generation="$(cat "$FAKE_GC_STATE/supervisor-generation")"
    fi
    generation="$((generation + 1))"
    printf '%s\n' "$generation" >"$FAKE_GC_STATE/supervisor-generation"
    printf '%s\n' "$((4000 + generation))" >"$FAKE_GC_STATE/supervisor-pid"
    touch "$FAKE_GC_STATE/supervisor-running"
    touch "$FAKE_GC_STATE/started"
    ;;
  status)
    pid=0
    if [ -f "$FAKE_GC_STATE/supervisor-pid" ]; then
      pid="$(cat "$FAKE_GC_STATE/supervisor-pid")"
    fi
    if [ "${FAKE_GC_STATUS_UNUSABLE:-0}" = "1" ]; then
      printf '{"ok":true,"controller":{"running":false,"pid":%s,"status":"starting_bead_store"},"health":{"usable":false},"beads":{"native_store_eligible":true},"rigs":[{"name":"agentops","suspended":false}]}\n' "$pid"
    elif [ "${FAKE_GC_NATIVE_STORE_INELIGIBLE:-0}" = "1" ]; then
      printf '{"ok":true,"controller":{"running":true,"pid":%s},"health":{"usable":true},"beads":{"native_store_eligible":false,"preflight_gate":"bd_context_agreement","preflight_reason":"bd context is unreachable"},"rigs":[{"name":"agentops","suspended":false}]}\n' "$pid"
    else
      printf '{"ok":true,"controller":{"running":true,"pid":%s},"health":{"usable":true},"beads":{"native_store_eligible":true},"rigs":[{"name":"agentops","suspended":false}]}\n' "$pid"
    fi
    ;;
  bd)
    [ "${2:-}" = "list" ] || exit 2
    printf '%s\n' '[]'
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

@test "bootstrap rejects a Beads-unsafe city root before invoking gc" {
  unsafe_city="/private/agentops-gc-bootstrap-test-$BATS_TEST_NUMBER/city"

  run run_bootstrap --city "$unsafe_city"

  [ "$status" -eq 1 ]
  [[ "$output" == *"city path is unsafe for Beads"* ]]
  [[ "$output" == *"Beads SEC-003"* ]]
  [ ! -e "$unsafe_city" ]
  [ ! -s "$FAKE_LOG" ]
}

@test "bootstrap defers scaffold-agent patches until after gc init" {
  export FAKE_GC_REJECT_INIT_PATCHES=1

  run run_bootstrap
  [ "$status" -eq 0 ]
  [ "$(grep -c '^\[\[patches.agent\]\]$' "$CITY/city.toml")" -eq 11 ]
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

  python3 - "$CITY/.gc/agentops-bootstrap.json" "$FAKE_GC" "$BIN/bd" <<'PY'
import json
import os
import re
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    marker = json.load(handle)
assert marker["schema_version"] == 3
assert marker["toolchain"]["gc"]["path"] == os.path.realpath(sys.argv[2])
assert marker["toolchain"]["gc"]["version"] == "1.3.5"
assert marker["toolchain"]["gc"]["commit"] == "8ffc009d"
assert marker["toolchain"]["bd"]["path"] == os.path.realpath(sys.argv[3])
assert marker["toolchain"]["bd"]["version"] == "1.1.0"
assert marker["toolchain"]["bd"]["commit"] == "8e4e59d39"
assert marker["toolchain"]["qualification"]["id"] == "gascity-v1.3.5-sdk-release"
assert marker["toolchain"]["qualification"]["status"] == "compatible"
assert re.fullmatch(r"[0-9a-f]{64}", marker["toolchain"]["gc"]["sha256"])
assert re.fullmatch(r"[0-9a-f]{64}", marker["toolchain"]["bd"]["sha256"])
PY

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
  [ -x "$CITY/.gc/bin/claude-interactive" ]
  [ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["remoteControlAtStartup"])' "$CITY/.claude/settings.json")" = "False" ]
  ! grep -Eq '^provider[[:space:]]*=' "$CITY/city.toml"
  python3 - "$CITY/city.toml" <<'PY'
import os
import hashlib
import sys
import tomllib

with open(sys.argv[1], "rb") as handle:
    config = tomllib.load(handle)
patches = config["patches"]["agent"]
city = os.path.realpath(os.path.dirname(sys.argv[1]))
rig = os.path.realpath(os.path.join(city, "..", "rig"))
workspace_parent = os.path.dirname(rig)
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
for role in ("implementer", "implementer-claude", "validator", "validator-claude"):
    assert [
        patch for patch in patches
        if patch.get("name") == f"agentops.{role}" and patch.get("dir") == "agentops"
    ] == [{"dir": "agentops", "name": f"agentops.{role}", "work_dir": workspace_parent}]
assert config["session"]["socket"] == (
    "agentops-" + hashlib.sha256(city.encode()).hexdigest()[:20]
)
assert config["session"]["setup_timeout"] == "60s"
assert config["session"]["nudge_poll_interval"] == "5s"
scope_roots = [os.path.join(city, ".gc"), os.path.join(city, ".gc-home"), rig]
expected_codex_args = ["--dangerously-bypass-hook-trust"]
for root in scope_roots:
    expected_codex_args.extend(["--add-dir", root])
assert config["providers"]["codex"]["args_append"] == expected_codex_args
assert config["providers"]["codex"]["prompt_mode"] == "none"
assert config["providers"]["claude"]["args_append"] == ["--add-dir", *scope_roots, "--safe-mode"]
assert config["providers"]["claude"]["print_args"] == []
assert config["providers"]["claude"]["command"] == os.path.join(city, ".gc", "bin", "claude-interactive")
assert config["providers"]["claude"]["path_check"] == "claude"
assert "-p" not in config["providers"]["claude"]["args_append"]
assert "--print" not in config["providers"]["claude"]["args_append"]
for provider_name, expected_default, expected_model, expected_choices, expected_models in (
    (
        "codex",
        "auto-edit",
        "gpt-5.6-terra",
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
        {
            "gpt-5.6-terra": ["--model", "gpt-5.6-terra"],
            "gpt-5.6-sol": ["--model", "gpt-5.6-sol"],
        },
    ),
    (
        "claude",
        "unrestricted",
        "opus-4.8",
        {
            "auto": ["--permission-mode", "auto"],
            "unrestricted": ["--dangerously-skip-permissions"],
        },
        {
            "opus-4.8": ["--model", "claude-opus-4-8"],
        },
    ),
):
    provider = config["providers"][provider_name]
    assert provider["option_defaults"]["permission_mode"] == expected_default
    assert provider["option_defaults"]["model"] == expected_model
    assert provider["options_schema_merge"] == "replace"
    options = {option["key"]: option for option in provider["options_schema"]}
    option = options["permission_mode"]
    actual_choices = {
        choice["value"]: choice["flag_args"] for choice in option["choices"]
    }
    assert actual_choices == expected_choices
    model_option = options["model"]
    actual_models = {
        choice["value"]: choice["flag_args"] for choice in model_option["choices"]
    }
    assert actual_models == expected_models
PY
  python3 - "$CITY/.gc/codex-home/config.toml" "$RIG" <<'PY'
import os
import sys
import tomllib

with open(sys.argv[1], "rb") as handle:
    config = tomllib.load(handle)
assert config["check_for_update_on_startup"] is False
assert config["projects"][os.path.realpath(sys.argv[2])]["trust_level"] == "trusted"
PY
  canonical_gc="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$FAKE_GC")"
  grep -Fq "GC_BIN = \"$canonical_gc\"" "$CITY/city.toml"
  grep -Fq 'OTEL_SDK_DISABLED = "true"' "$CITY/city.toml"
  ! grep -Eq 'antigravity|agy|named_session|\[\[agent\]\]' "$CITY/city.toml"
  ! grep -Fq "path = \"$RIG\"" "$CITY/city.toml"
  grep -Fq 'path = "' "$CITY/.gc/site.toml"

  canonical_city="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$CITY")"
  expected_home="GC_HOME=$canonical_city/.gc-home"
  expected_codex="CODEX_HOME=$canonical_city/.gc/codex-home"
  grep -Fq "$expected_home" "$FAKE_LOG"
  grep -Fq 'GC_ISOLATED=1' "$FAKE_LOG"
  grep -Fq "$expected_codex" "$FAKE_LOG"
  grep -Fq 'OTEL_SDK_DISABLED=true' "$FAKE_LOG"
  grep -Fq 'BD_DOLT_SYNC_CLI_REMOTES=false BEADS_DOLT_SYNC_CLI_REMOTES=false' "$FAKE_LOG"
  ! grep -Ev 'GC_BEADS=<unset>.*GC_DOLT_PORT=<unset>.*BEADS_DOLT_SERVER_PORT=<unset>.*GC_CITY=<unset>' \
    < <(grep '^ENV ' "$FAKE_LOG")

  grep -Fq 'ARGS <init> <--file>' "$FAKE_LOG"
  grep -Fq 'ARGS <--city>' "$FAKE_LOG"
  grep -Fq '<rig> <add>' "$FAKE_LOG"
  grep -Fq 'RIG_ORIGIN <<unset>> ADOPT <0>' "$FAKE_LOG"
  [ "$(git -C "$RIG" config --get remote.origin.url)" = "$REMOTE" ]
  grep -Fq 'ARGS <lint>' "$FAKE_LOG"
  grep -Fq '<config> <show>' "$FAKE_LOG"
  grep -Fq '<config> <explain>' "$FAKE_LOG"
  grep -Fq '<config> <explain> <--provider> <codex> <--json>' "$FAKE_LOG"
  grep -Fq '<config> <explain> <--provider> <claude> <--json>' "$FAKE_LOG"
  grep -Fq '<import> <status> <--json>' "$FAKE_LOG"
}

@test "cities with the same basename receive different tmux sockets" {
  run run_bootstrap
  [ "$status" -eq 0 ]

  first_socket="$(python3 - "$CITY/city.toml" <<'PY'
import sys
import tomllib

with open(sys.argv[1], "rb") as handle:
    print(tomllib.load(handle)["session"]["socket"])
PY
)"

  second_city="$TMP/second/city"
  rm -f "$FAKE_STATE/rig-added"
  run env PATH="$BIN:$PATH" "$BOOTSTRAP" \
    --city "$second_city" \
    --rig "$RIG" \
    --pack "$PACK" \
    --gc-bin "$FAKE_GC" \
    --codex-auth "$CODEX_AUTH"
  [ "$status" -eq 0 ]

  second_socket="$(python3 - "$second_city/city.toml" <<'PY'
import sys
import tomllib

with open(sys.argv[1], "rb") as handle:
    print(tomllib.load(handle)["session"]["socket"])
PY
)"
  [ "$first_socket" != "$second_socket" ]
}

@test "bootstrap is idempotent after successful configuration" {
  run run_bootstrap
  [ "$status" -eq 0 ]
  printf '\n[projects."/unrelated/project"]\ntrust_level = "trusted"\n' \
    >>"$CITY/.gc/codex-home/config.toml"
  supervisor_config_digest="$(shasum -a 256 "$CITY/.gc-home/supervisor.toml" | awk '{print $1}')"

  run run_bootstrap
  [ "$status" -eq 0 ]

  [ "$(grep -c 'ARGS <init>' "$FAKE_LOG")" -eq 1 ]
  [ "$(grep -c '<rig> <add>' "$FAKE_LOG")" -eq 2 ]
  [ "$(grep -c 'ADOPT <1>' "$FAKE_LOG")" -eq 1 ]
  [ "$(grep -c '^\[imports.agentops\]$' "$CITY/pack.toml")" -eq 1 ]
  [ "$(grep -c '^\[rigs.imports.agentops\]$' "$CITY/city.toml")" -eq 1 ]
  [ "$(grep -c '^\[\[patches.agent\]\]$' "$CITY/city.toml")" -eq 11 ]
  [ "$(shasum -a 256 "$CITY/.gc-home/supervisor.toml" | awk '{print $1}')" = "$supervisor_config_digest" ]
  python3 - "$CITY/.gc/codex-home/config.toml" "$RIG" <<'PY'
import os
import sys
import tomllib

with open(sys.argv[1], "rb") as handle:
    config = tomllib.load(handle)
assert config["check_for_update_on_startup"] is False
assert config["projects"][os.path.realpath(sys.argv[2])]["trust_level"] == "trusted"
assert config["projects"]["/unrelated/project"]["trust_level"] == "trusted"
PY
}

@test "bootstrap repairs primary executor workspace roots" {
  run run_bootstrap
  [ "$status" -eq 0 ]

  python3 - "$CITY/city.toml" "$RIG" <<'PY'
import json
import os
import sys

path, rig = sys.argv[1:]
expected = (
    "[[patches.agent]]\n"
    'dir = "agentops"\n'
    'name = "agentops.validator"\n'
    f"work_dir = {json.dumps(os.path.dirname(os.path.realpath(rig)))}"
)
wrong = expected.rsplit("\n", 1)[0] + f"\nwork_dir = {json.dumps(os.path.realpath(rig))}"
with open(path, encoding="utf-8") as handle:
    text = handle.read()
if text.count(expected) != 1:
    raise SystemExit("fixture has no unique managed validator workspace patch")
with open(path, "w", encoding="utf-8") as handle:
    handle.write(text.replace(expected, wrong))
PY

  run run_bootstrap
  [ "$status" -eq 0 ]
  python3 - "$CITY/city.toml" "$RIG" <<'PY'
import os
import sys
import tomllib

with open(sys.argv[1], "rb") as handle:
    config = tomllib.load(handle)
matches = [
    patch for patch in config["patches"]["agent"]
    if patch.get("dir") == "agentops" and patch.get("name") == "agentops.validator"
]
assert matches == [{
    "dir": "agentops",
    "name": "agentops.validator",
    "work_dir": os.path.dirname(os.path.realpath(sys.argv[2])),
}]
PY
}

@test "bootstrap requires explicit authority to replace a managed gc binary path" {
  run run_bootstrap
  [ "$status" -eq 0 ]

  upgraded_gc="$BIN/gc-upgraded"
  cp "$FAKE_GC" "$upgraded_gc"

  run run_bootstrap --gc-bin "$upgraded_gc"
  [ "$status" -ne 0 ]
  [[ "$output" == *"managed city mismatch for paired gc/bd toolchain identity"* ]]

  run run_bootstrap --gc-bin "$upgraded_gc" --replace-gc-bin --start
  [ "$status" -eq 0 ]
  [ -e "$FAKE_STATE/supervisor-force-installed" ]
  [ "$(cat "$FAKE_STATE/supervisor-generation")" -eq 1 ]

  canonical_upgraded="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$upgraded_gc")"
  python3 - "$CITY/.gc/agentops-bootstrap.json" "$CITY/city.toml" "$canonical_upgraded" <<'PY'
import json
import sys
import tomllib

marker_path, city_config_path, expected = sys.argv[1:]
with open(marker_path, encoding="utf-8") as handle:
    marker = json.load(handle)
with open(city_config_path, "rb") as handle:
    city_config = tomllib.load(handle)
assert marker["schema_version"] == 3
assert marker["toolchain"]["gc"]["path"] == expected
assert city_config["workspace"]["env"]["GC_BIN"] == expected
PY
}

@test "bootstrap detects same-path binary replacement by digest" {
  run run_bootstrap
  [ "$status" -eq 0 ]

  printf '\n# replacement bytes\n' >>"$FAKE_GC"

  run run_bootstrap
  [ "$status" -ne 0 ]
  [[ "$output" == *"managed city mismatch for paired gc/bd toolchain identity"* ]]

  run run_bootstrap --replace-gc-bin --start
  [ "$status" -eq 0 ]
  [ -e "$FAKE_STATE/supervisor-force-installed" ]
}

@test "Claude wrapper preserves interactive Opus launch and records the exit" {
  run run_bootstrap
  [ "$status" -eq 0 ]

  run env FAKE_CLAUDE_INTERACTIVE_EXIT=17 \
    "$CITY/.gc/bin/claude-interactive" \
    --model claude-opus-4-8 --dangerously-skip-permissions
  [ "$status" -eq 17 ]

  grep -Fq 'ARGS <--model> <claude-opus-4-8> <--dangerously-skip-permissions>' \
    "$TMP/claude-interactive.log"
  diagnostics="$CITY/.gc/runtime/claude-diagnostics"
  [ "$(find "$diagnostics" -name '*.debug.log' | wc -l | tr -d ' ')" -eq 1 ]
  [ "$(find "$diagnostics" -name '*.exit' | wc -l | tr -d ' ')" -eq 1 ]
  grep -Fq 'exit_code=17' "$diagnostics"/*.exit
  ! grep -Eq '(^|[[:space:]])(-p|--print)($|[[:space:]])' "$TMP/claude-interactive.log"
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
  grep -Fq 'ADOPT <1>' "$FAKE_LOG"
  [ "$(git -C "$RIG" config --get remote.origin.url)" = "$REMOTE" ]
  [ "$(grep -c '^\[\[rigs\]\]$' "$CITY/city.toml")" -eq 1 ]
  [ "$(grep -c '^\[rigs.imports.agentops\]$' "$CITY/city.toml")" -eq 1 ]
}

@test "bootstrap removes the transient bd init commit from the caller branch" {
  export FAKE_GC_COMMIT_ON_RIG_ADD=1
	export FAKE_GC_APPEND_GITIGNORE=1
  before="$(git -C "$RIG" rev-parse HEAD)"
	[ ! -e "$RIG/.gitignore" ]

  run run_bootstrap
  [ "$status" -eq 0 ]
  [ "$(git -C "$RIG" rev-parse HEAD)" = "$before" ]
  git -C "$RIG" diff --cached --quiet
	[ ! -e "$RIG/.gitignore" ]
  [ "$(git -C "$RIG" log -1 --format=%s)" = "bootstrap fixture" ]
}

@test "bootstrap clears canonical staged bd init files without a commit" {
  export FAKE_GC_STAGE_ON_RIG_ADD=1
	export FAKE_GC_APPEND_GITIGNORE=1
  export FAKE_GC_APPEND_RUNTIME_GITIGNORE=1
  before="$(git -C "$RIG" rev-parse HEAD)"

  run run_bootstrap
  [ "$status" -eq 0 ]
  [ "$(git -C "$RIG" rev-parse HEAD)" = "$before" ]
  git -C "$RIG" diff --cached --quiet
	[ ! -e "$RIG/.gitignore" ]
  [ -f "$RIG/.beads/metadata.json" ]

  run run_bootstrap
  [ "$status" -eq 0 ]
  git -C "$RIG" diff --cached --quiet
}

@test "bootstrap restores the exact caller gitignore after Gas City upgrades it" {
  printf '%s\n' '# caller rules' '.beads/' 'dist/' >"$RIG/.gitignore"
  git -C "$RIG" add .gitignore
  git -C "$RIG" commit -qm "add caller gitignore"
  before="$(shasum -a 256 "$RIG/.gitignore" | awk '{print $1}')"
  export FAKE_GC_REWRITE_GITIGNORE=1

  run run_bootstrap
  [ "$status" -eq 0 ]
  [ "$(shasum -a 256 "$RIG/.gitignore" | awk '{print $1}')" = "$before" ]
  git -C "$RIG" diff --quiet -- .gitignore
}

@test "bootstrap refuses an existing unmanaged city" {
  mkdir -p "$CITY"
  printf '%s\n' '[pack]' 'name = "historical"' 'schema = 2' >"$CITY/pack.toml"

  run run_bootstrap
  [ "$status" -ne 0 ]
  [[ "$output" == *"refusing existing unmanaged city"* ]]
  [ ! -s "$FAKE_LOG" ]
}

@test "bootstrap refuses a same-version toolchain outside the lock" {
  export FAKE_GC_COMMIT=deadbeef

  run run_bootstrap

  [ "$status" -ne 0 ]
  [[ "$output" == *"pair is not in toolchain.lock.json"* ]]
  [ ! -e "$CITY/.gc/agentops-bootstrap.json" ]
}

@test "start is opt-in" {
  run run_bootstrap --start
  [ "$status" -eq 0 ]
  [ -e "$FAKE_STATE/started" ]
  [ -e "$FAKE_STATE/rig-resumed" ]
  [ "$(grep -c '<start> <--no-auto-restart>$' "$FAKE_LOG")" -eq 1 ]
  [ "$(grep -c '<status> <--json>' "$FAKE_LOG")" -ge 1 ]
  grep -Fq '<bd> <list> <--json> <--limit> <1>' "$FAKE_LOG"
  grep -Fq '<--rig> <agentops> <bd> <list> <--json> <--limit> <1>' "$FAKE_LOG"
  grep -Fq '<rig> <resume> <agentops> <--json>' "$FAKE_LOG"
}

@test "start fails closed when the city never becomes usable" {
  export FAKE_GC_STATUS_UNUSABLE=1
  run run_bootstrap --start --start-timeout 1

  [ "$status" -ne 0 ]
  [[ "$output" == *"did not become usable within 1s"* ]]
  [ -e "$FAKE_STATE/started" ]
  [ -e "$FAKE_STATE/rig-resumed" ]
}

@test "start fails closed when Gas City cannot use its native Beads store" {
  export FAKE_GC_NATIVE_STORE_INELIGIBLE=1
  run run_bootstrap --start --start-timeout 1

  [ "$status" -ne 0 ]
  [[ "$output" == *"did not become usable within 1s"* ]]
  [ -e "$FAKE_STATE/started" ]
  [ -e "$FAKE_STATE/rig-resumed" ]
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
  run grep '/Users/bo' \
    "$REPO_ROOT/deploy/gc/bootstrap.sh" \
    "$REPO_ROOT/deploy/gc/city.toml" \
    "$REPO_ROOT/deploy/gc/claude-interactive.sh" \
    "$REPO_ROOT/deploy/gc/claude-settings.json" \
    "$REPO_ROOT/deploy/gc/toolchain.lock.json"
  [ "$status" -eq 1 ]
}
