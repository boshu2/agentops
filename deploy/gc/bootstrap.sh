#!/usr/bin/env bash
# preamble-exempt: standalone deployment bootstrap; intentionally runs outside a repo checkout.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  bootstrap.sh --city PATH --rig PATH --pack PATH [options]

Required:
  --city PATH       Fresh city directory, or a city managed by this bootstrap
  --rig PATH        Existing repository to register as the sole initial rig
  --pack PATH       AgentOps Gas City pack directory containing pack.toml

Options:
  --rig-name NAME   Logical rig name (default: agentops)
  --binding NAME    Pack import binding at city and rig scopes (default: agentops)
  --gc-bin PATH     Gas City CLI (default: gc from PATH)
  --codex-auth PATH Existing Codex auth.json to link into the private home
  --max-active-sessions N  City-wide concurrent session cap (default: 1)
  --start           Start the city after all preflight checks pass
  -h, --help        Show this help
EOF
}

die() {
  printf 'gc-agentops bootstrap: %s\n' "$*" >&2
  exit 1
}

city=""
rig=""
pack=""
rig_name="agentops"
binding="agentops"
gc_bin="gc"
codex_auth=""
source_codex_home="${CODEX_HOME:-${HOME:?HOME is required}/.codex}"
start=0
max_active_sessions=1

while [ "$#" -gt 0 ]; do
  case "$1" in
    --city)
      [ "$#" -ge 2 ] || die "--city requires a path"
      city="$2"
      shift 2
      ;;
    --rig)
      [ "$#" -ge 2 ] || die "--rig requires a path"
      rig="$2"
      shift 2
      ;;
    --pack)
      [ "$#" -ge 2 ] || die "--pack requires a path"
      pack="$2"
      shift 2
      ;;
    --rig-name)
      [ "$#" -ge 2 ] || die "--rig-name requires a name"
      rig_name="$2"
      shift 2
      ;;
    --binding)
      [ "$#" -ge 2 ] || die "--binding requires a name"
      binding="$2"
      shift 2
      ;;
    --gc-bin)
      [ "$#" -ge 2 ] || die "--gc-bin requires a path"
      gc_bin="$2"
      shift 2
      ;;
    --codex-auth)
      [ "$#" -ge 2 ] || die "--codex-auth requires a path"
      codex_auth="$2"
      shift 2
      ;;
    --max-active-sessions)
      [ "$#" -ge 2 ] || die "--max-active-sessions requires a value"
      max_active_sessions="$2"
      shift 2
      ;;
    --start)
      start=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

[ -n "$city" ] || die "--city is required"
[ -n "$rig" ] || die "--rig is required"
[ -n "$pack" ] || die "--pack is required"
[[ "$rig_name" =~ ^[A-Za-z0-9_-]+$ ]] || die "--rig-name must contain only letters, digits, underscore, or hyphen"
[[ "$binding" =~ ^[A-Za-z0-9_-]+$ ]] || die "--binding must contain only letters, digits, underscore, or hyphen"
[[ "$max_active_sessions" =~ ^[1-9][0-9]*$ ]] || die "--max-active-sessions must be a positive integer"
[ "$max_active_sessions" -le 64 ] || die "--max-active-sessions must be at most 64"
command -v python3 >/dev/null 2>&1 || die "python3 is required"

canonical_path() {
  python3 - "$1" <<'PY'
import os
import sys

print(os.path.realpath(os.path.expanduser(sys.argv[1])))
PY
}

city="$(canonical_path "$city")"
rig="$(canonical_path "$rig")"
pack="$(canonical_path "$pack")"
if [ -z "$codex_auth" ]; then
  codex_auth="$source_codex_home/auth.json"
fi
codex_auth="$(canonical_path "$codex_auth")"

[ "$city" != "$rig" ] || die "city and rig must be different directories"
[ -d "$rig" ] || die "rig directory does not exist: $rig"
[ -d "$pack" ] || die "pack directory does not exist: $pack"
[ -f "$pack/pack.toml" ] || die "pack.toml not found in pack directory: $pack"
[ -f "$codex_auth" ] || die "Codex auth file does not exist: $codex_auth"
[ -r "$codex_auth" ] || die "Codex auth file is not readable: $codex_auth"
if [ -e "$city" ] && [ ! -d "$city" ]; then
  die "city path exists but is not a directory: $city"
fi

script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
city_template="$script_dir/city.toml"
[ -f "$city_template" ] || die "city template not found: $city_template"

marker="$city/.gc/agentops-bootstrap.json"
city_has_content=0
if [ -d "$city" ] && [ -n "$(find "$city" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
  city_has_content=1
fi
if [ "$city_has_content" -eq 1 ] && [ ! -f "$marker" ]; then
  die "refusing existing unmanaged city: $city"
fi

if [[ "$gc_bin" == */* ]]; then
  [ -x "$gc_bin" ] || die "Gas City CLI is not executable: $gc_bin"
  gc_bin="$(canonical_path "$gc_bin")"
else
  gc_bin="$(command -v "$gc_bin" || true)"
  [ -n "$gc_bin" ] || die "Gas City CLI not found on PATH"
fi

if [ -f "$marker" ]; then
  python3 - "$marker" "$city" "$rig" "$pack" "$rig_name" "$binding" "$gc_bin" "$codex_auth" "$max_active_sessions" <<'PY'
import json
import sys

marker_path, city, rig, pack, rig_name, binding, gc_bin, codex_auth, max_active_sessions = sys.argv[1:]
with open(marker_path, encoding="utf-8") as handle:
    marker = json.load(handle)
expected = {
    "schema_version": 2,
    "city": city,
    "rig": rig,
    "pack": pack,
    "rig_name": rig_name,
    "binding": binding,
    "gc_bin": gc_bin,
    "codex_auth": codex_auth,
    "max_active_sessions": int(max_active_sessions),
}
for key, value in expected.items():
    actual = marker.get(key, 1 if key == "max_active_sessions" else None)
    if actual != value:
        raise SystemExit(
            f"managed city mismatch for {key}: {actual!r} != {value!r}"
        )
PY
fi

# A city must not inherit another city's discovery, store, or Dolt endpoint.
while IFS='=' read -r key _; do
  case "$key" in
    GC_*|BEADS_DOLT_*|DOLT_*) unset "$key" ;;
  esac
done < <(env)
unset BEADS_BACKEND BEADS_DIR BD_DB BD_DATABASE BD_DOLT_PORT BD_HOME || true
export GC_HOME="$city/.gc-home"
export GC_ISOLATED=1
export CODEX_HOME="$city/.gc/codex-home"
export GC_BIN="$gc_bin"
# A Git remote is not necessarily a Dolt remote (local qualification clones are
# the sharp edge). Never let rig registration synthesize or sync Dolt remotes
# from Git URLs; off-box Beads replication is an explicit operator concern.
export BD_DOLT_SYNC_CLI_REMOTES=false
export BEADS_DOLT_SYNC_CLI_REMOTES=false

"$gc_bin" lint "$pack" --json >/dev/null

write_marker() {
  local state="$1"
  mkdir -p "$city/.gc"
  python3 - "$marker" "$city" "$rig" "$pack" "$rig_name" "$binding" "$gc_bin" "$codex_auth" "$max_active_sessions" "$state" <<'PY'
import json
import os
import sys
import tempfile

path, city, rig, pack, rig_name, binding, gc_bin, codex_auth, max_active_sessions, state = sys.argv[1:]
payload = {
    "schema_version": 2,
    "state": state,
    "city": city,
    "rig": rig,
    "pack": pack,
    "rig_name": rig_name,
    "binding": binding,
    "gc_bin": gc_bin,
    "codex_auth": codex_auth,
    "max_active_sessions": int(max_active_sessions),
}
fd, tmp = tempfile.mkstemp(prefix=".agentops-bootstrap.", dir=os.path.dirname(path))
with os.fdopen(fd, "w", encoding="utf-8") as handle:
    json.dump(payload, handle, indent=2, sort_keys=True)
    handle.write("\n")
    handle.flush()
    os.fsync(handle.fileno())
os.replace(tmp, path)
dir_fd = os.open(os.path.dirname(path), os.O_RDONLY)
try:
    os.fsync(dir_fd)
finally:
    os.close(dir_fd)
PY
}

if [ ! -f "$marker" ]; then
  mkdir -p "$(dirname "$city")"
  "$gc_bin" init \
    --file "$city_template" \
    --no-start \
    --skip-provider-readiness \
    "$city" >/dev/null
  [ -f "$city/pack.toml" ] || die "gc init did not create pack.toml"
  [ -f "$city/city.toml" ] || die "gc init did not create city.toml"
  write_marker configuring
fi

# GC_ISOLATED is consumed by the foreground CLI, but a launchd-managed
# supervisor does not inherit that opt-in flag. Seed an explicit loopback port
# in the private GC_HOME so a fresh city never falls back to the machine-wide
# default (8372) when the service manager launches it. Preserve the selected
# port on every managed rerun.
supervisor_port="$(python3 - "$GC_HOME/supervisor.toml" <<'PY'
import os
import socket
import sys
import tempfile
import tomllib

path = sys.argv[1]
if os.path.exists(path):
    with open(path, "rb") as handle:
        config = tomllib.load(handle)
    supervisor = config.get("supervisor", {})
    port = supervisor.get("port")
    bind = supervisor.get("bind", "127.0.0.1")
    if not isinstance(port, int) or isinstance(port, bool) or not 1 <= port <= 65535:
        raise SystemExit("managed supervisor config must declare a valid port")
    if bind != "127.0.0.1":
        raise SystemExit("managed supervisor config must bind to 127.0.0.1")
    print(port)
    raise SystemExit(0)

os.makedirs(os.path.dirname(path), mode=0o700, exist_ok=True)
with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as listener:
    listener.bind(("127.0.0.1", 0))
    port = listener.getsockname()[1]
payload = f'[supervisor]\nport = {port}\nbind = "127.0.0.1"\n'
fd, temporary = tempfile.mkstemp(prefix=".supervisor.toml.", dir=os.path.dirname(path))
try:
    os.fchmod(fd, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        handle.write(payload)
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, path)
    directory_fd = os.open(os.path.dirname(path), os.O_RDONLY)
    try:
        os.fsync(directory_fd)
    finally:
        os.close(directory_fd)
finally:
    if os.path.exists(temporary):
        os.unlink(temporary)
print(port)
PY
)"

mkdir -p "$CODEX_HOME"
chmod 700 "$CODEX_HOME"
private_auth="$CODEX_HOME/auth.json"
if [ -e "$private_auth" ] || [ -L "$private_auth" ]; then
  [ "$(canonical_path "$private_auth")" = "$codex_auth" ] || \
    die "private Codex auth points somewhere else: $private_auth"
else
  ln -s "$codex_auth" "$private_auth"
fi
codex_bin="$(command -v codex || true)"
[ -n "$codex_bin" ] || die "codex CLI is required for provider readiness"
CODEX_HOME="$CODEX_HOME" "$codex_bin" login status >/dev/null 2>&1 || \
  die "private Codex home is not authenticated through $codex_auth"
claude_bin="$(command -v claude || true)"
[ -n "$claude_bin" ] || die "Claude CLI is required for provider readiness"
"$claude_bin" --version >/dev/null 2>&1 || \
  die "Claude CLI failed its non-session readiness check"
claude_auth_status="$("$claude_bin" auth status --json 2>/dev/null)" || \
  die "Claude CLI authentication status is unavailable"
python3 - "$claude_auth_status" <<'PY'
import json
import sys

try:
    status = json.loads(sys.argv[1])
except json.JSONDecodeError as exc:
    raise SystemExit(f"Claude auth status is not JSON: {exc}") from exc
if status.get("loggedIn") is not True:
    raise SystemExit("Claude CLI is not logged in")
if status.get("authMethod") not in {"claude.ai", "oauth_token"}:
    raise SystemExit("Claude CLI must use first-party interactive authentication")
if status.get("apiProvider") != "firstParty":
    raise SystemExit("Claude CLI apiProvider must be firstParty")
PY
"$claude_bin" auto-mode defaults >/dev/null 2>&1 || \
  die "Claude CLI auto mode is unavailable; interactive AgentOps workers require it"

python3 - "$city/city.toml" "$city_template" "$CODEX_HOME" "$gc_bin" "$city" "$rig" "$max_active_sessions" <<'PY'
import json
import os
import re
import sys
import tempfile
import tomllib

path, template_path, codex_home, gc_bin, city, requested_rig, max_active_sessions = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    text = handle.read()
with open(template_path, encoding="utf-8") as handle:
    policy = handle.read()
max_active_sessions = int(max_active_sessions)
policy = re.sub(
    r"(?m)^max_active_sessions = [0-9]+$",
    f"max_active_sessions = {max_active_sessions}",
    policy,
    count=1,
)
replacements = {
    "__GC_AGENTOPS_CODEX_HOME__": codex_home,
    "__GC_AGENTOPS_GC_BIN__": gc_bin,
}
for sentinel, value in replacements.items():
    policy = policy.replace(json.dumps(sentinel), json.dumps(value))

live_config = tomllib.loads(text)
site_path = os.path.join(city, ".gc", "site.toml")
with open(site_path, "rb") as handle:
    live_site = tomllib.load(handle)
scope_roots = [os.path.join(city, ".gc"), os.path.join(city, ".gc-home")]
scope_roots.extend(
    rig.get("path") for rig in live_site.get("rig", [])
    if isinstance(rig.get("path"), str) and rig.get("path")
)
scope_roots.append(requested_rig)
canonical_scope_roots = []
for root in scope_roots:
    canonical = os.path.realpath(root)
    if canonical not in canonical_scope_roots:
        canonical_scope_roots.append(canonical)

codex_scope_args = []
for root in canonical_scope_roots:
    codex_scope_args.extend(["--add-dir", root])
claude_scope_args = ["--add-dir", *canonical_scope_roots]
scope_lines = {
    'args_append = ["__GC_AGENTOPS_CODEX_SCOPE_ARGS__"]':
        f"args_append = {json.dumps(codex_scope_args)}",
    'args_append = ["__GC_AGENTOPS_CLAUDE_SCOPE_ARGS__"]':
        f"args_append = {json.dumps(claude_scope_args)}",
}
for marker_line, rendered_line in scope_lines.items():
    if policy.count(marker_line) != 1:
        raise SystemExit(f"city template must contain exactly one {marker_line!r}")
    policy = policy.replace(marker_line, rendered_line)
tomllib.loads(policy)

policy_patch = re.search(r"(?m)^\[\[patches\.agent\]\]\s*$", policy)
if policy_patch is None:
    raise SystemExit("city template must declare provider suspension patches")
policy = policy[:policy_patch.start()].rstrip()

markers = list(re.finditer(r"(?m)^\[\[(rigs|patches\.agent)\]\]\s*$", text))
rig_sections = []
preserved_patches = []
for index, match in enumerate(markers):
    end = markers[index + 1].start() if index + 1 < len(markers) else len(text)
    block = text[match.start():end].strip()
    if match.group(1) == "rigs":
        rig_sections.append(block)
        continue
    parsed = tomllib.loads(block).get("patches", {}).get("agent", [])
    if len(parsed) != 1:
        raise SystemExit("managed city contains an invalid agent patch block")
    managed_name = parsed[0].get("name")
    managed_dir = parsed[0].get("dir")
    is_managed = (
        managed_name in {"codex", "claude", "core.control-dispatcher"}
        or (managed_name == "bd.dog" and not managed_dir)
    )
    if not is_managed:
        preserved_patches.append(block)

managed_patches = [
    '[[patches.agent]]\nname = "bd.dog"\nsuspended = true',
    '[[patches.agent]]\nname = "core.control-dispatcher"\nsuspended = true',
]
for provider_name in ("codex", "claude"):
    managed_patches.append(
        f'[[patches.agent]]\nname = "{provider_name}"\nsuspended = true'
    )
for rig in live_config.get("rigs", []):
    rig_name = rig.get("name")
    if not isinstance(rig_name, str) or not rig_name:
        raise SystemExit("managed city contains a rig without a name")
    for provider_name in ("core.control-dispatcher", "codex", "claude"):
        managed_patches.append(
            "[[patches.agent]]\n"
            f'dir = "{rig_name}"\n'
            f'name = "{provider_name}"\n'
            "suspended = true"
        )

parts = [policy]
parts.extend(rig_sections)
parts.extend(preserved_patches)
parts.extend(managed_patches)
rendered = "\n\n".join(parts) + "\n"
if rendered != text:
    fd, temporary = tempfile.mkstemp(prefix=".city.toml.", dir=os.path.dirname(path))
    os.fchmod(fd, os.stat(path).st_mode & 0o777)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        handle.write(rendered)
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, path)
    directory_fd = os.open(os.path.dirname(path), os.O_RDONLY)
    try:
        os.fsync(directory_fd)
    finally:
        os.close(directory_fd)
text = rendered
config = tomllib.loads(text)
patches = config.get("patches", {}).get("agent", [])
city_maintenance = {
    patch.get("name"): patch for patch in patches
    if not patch.get("dir") and patch.get("name") in {"bd.dog", "core.control-dispatcher"}
}
if city_maintenance != {
    "bd.dog": {"name": "bd.dog", "suspended": True},
    "core.control-dispatcher": {"name": "core.control-dispatcher", "suspended": True},
}:
    raise SystemExit("city must suspend scaffold maintenance workers")
for provider_name in ("codex", "claude"):
    city_matches = [
        patch for patch in patches
        if patch.get("name") == provider_name and not patch.get("dir")
    ]
    if city_matches != [{"name": provider_name, "suspended": True}]:
        raise SystemExit(f"city must suspend the provider-derived generic {provider_name} target")
    for rig in config.get("rigs", []):
        rig_matches = [
            patch for patch in patches
            if patch.get("name") == provider_name and patch.get("dir") == rig["name"]
        ]
        if rig_matches != [{"dir": rig["name"], "name": provider_name, "suspended": True}]:
            raise SystemExit(
                f"rig {rig['name']!r} must suspend the generic {provider_name} target"
            )
for rig in config.get("rigs", []):
    control_matches = [
        patch for patch in patches
        if patch.get("name") == "core.control-dispatcher" and patch.get("dir") == rig["name"]
    ]
    if control_matches != [{"dir": rig["name"], "name": "core.control-dispatcher", "suspended": True}]:
        raise SystemExit(f"rig {rig['name']!r} must suspend the scaffold control dispatcher")
workspace = config.get("workspace", {})
codex_provider = config.get("providers", {}).get("codex", {})
claude_provider = config.get("providers", {}).get("claude", {})
if workspace.get("provider"):
    raise SystemExit("city workspace provider must stay unset; roles select providers explicitly")
if workspace.get("max_active_sessions") != max_active_sessions:
    raise SystemExit(f"city workspace max_active_sessions must be {max_active_sessions}")
if workspace.get("env", {}).get("GC_BIN") != gc_bin:
    raise SystemExit("city workspace must pin GC_BIN for managed sessions")
if codex_provider.get("base") != "builtin:codex":
    raise SystemExit("city must register the builtin Codex provider")
if codex_provider.get("option_defaults", {}).get("permission_mode") != "auto-edit":
    raise SystemExit("Codex permission_mode must be auto-edit")
if codex_provider.get("env", {}).get("CODEX_HOME") != codex_home:
    raise SystemExit("Codex provider must use the city-private CODEX_HOME")
expected_scope_roots = [os.path.join(city, ".gc"), os.path.join(city, ".gc-home")]
expected_scope_roots.extend(
    rig.get("path") for rig in live_site.get("rig", [])
    if isinstance(rig.get("path"), str) and rig.get("path")
)
expected_scope_roots.append(requested_rig)
canonical_expected_roots = []
for root in expected_scope_roots:
    canonical = os.path.realpath(root)
    if canonical not in canonical_expected_roots:
        canonical_expected_roots.append(canonical)
expected_codex_args = []
for root in canonical_expected_roots:
    expected_codex_args.extend(["--add-dir", root])
if codex_provider.get("args_append") != expected_codex_args:
    raise SystemExit("Codex writable roots must exactly match managed runtime and rig roots")
if claude_provider.get("base") != "builtin:claude":
    raise SystemExit("city must register the builtin Claude provider")
if claude_provider.get("print_args") != []:
    raise SystemExit("Claude print_args must be empty; interactive sessions are mandatory")
if claude_provider.get("args_append") != ["--add-dir", *canonical_expected_roots]:
    raise SystemExit("Claude additional directories must exactly match managed runtime and rig roots")
for field in ("args", "args_append"):
    if any(value in {"-p", "--print"} for value in claude_provider.get(field, [])):
        raise SystemExit(f"Claude {field} must not contain print-mode flags")
if claude_provider.get("option_defaults", {}).get("permission_mode") != "auto":
    raise SystemExit("Claude permission_mode must be auto")
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
    if config["providers"][provider_name].get("options_schema_merge") != "replace":
        raise SystemExit(f"{provider_name} must replace the inherited options schema")
    selected = [
        option for option in config["providers"][provider_name].get("options_schema", [])
        if option.get("key") == "permission_mode"
    ]
    if len(selected) != 1 or selected[0].get("default") != expected_default:
        raise SystemExit(f"{provider_name} must replace the inherited permission_mode schema")
    choices = {choice.get("value"): choice for choice in selected[0].get("choices", [])}
    if set(choices) != set(expected_choices):
        raise SystemExit(f"{provider_name} permission schema exposes unexpected choices")
    for choice, expected_flags in expected_choices.items():
        if choices.get(choice, {}).get("flag_args") != expected_flags:
            raise SystemExit(f"{provider_name} {choice} permission flags are incompatible with the installed CLI")
for forbidden in ("agent", "named_session"):
    if config.get(forbidden):
        raise SystemExit(f"city must not declare {forbidden}")
PY

# Local, possibly uncommitted packs are intentionally read in place. This is
# the SDK-documented case where direct TOML is safer than promoting HEAD.
python3 - "$city/pack.toml" "$binding" "$pack" <<'PY'
import json
import os
import sys
import tempfile
import tomllib

path, binding, source = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    text = handle.read()
config = tomllib.loads(text)
current = config.get("imports", {}).get(binding)
if current is None:
    if text and not text.endswith("\n"):
        text += "\n"
    text += f"\n[imports.{binding}]\nsource = {json.dumps(source)}\n"
    fd, temporary = tempfile.mkstemp(prefix=".pack.toml.", dir=os.path.dirname(path))
    os.fchmod(fd, os.stat(path).st_mode & 0o777)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        handle.write(text)
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, path)
elif current.get("source") != source or current.get("version"):
    raise SystemExit(f"city import {binding!r} already has a different source or pin")
PY

"$gc_bin" --city "$city" rig add "$rig" \
  --name "$rig_name" \
  --start-suspended >/dev/null

python3 - "$city/city.toml" "$rig_name" "$binding" "$pack" <<'PY'
import json
import os
import re
import sys
import tempfile
import tomllib

path, rig_name, binding, source = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    text = handle.read()
config = tomllib.loads(text)
matches = [rig for rig in config.get("rigs", []) if rig.get("name") == rig_name]
if len(matches) != 1:
    raise SystemExit(f"expected exactly one rig named {rig_name!r}, found {len(matches)}")
current = matches[0].get("imports", {}).get(binding)
if current is not None:
    if current.get("source") != source or current.get("version"):
        raise SystemExit(f"rig import {binding!r} already has a different source or pin")
    raise SystemExit(0)

starts = [match.start() for match in re.finditer(r"(?m)^\[\[rigs\]\]\s*$", text)]
for index, start in enumerate(starts):
    end = starts[index + 1] if index + 1 < len(starts) else len(text)
    block = text[start:end]
    parsed = tomllib.loads(block)
    rig = parsed.get("rigs", [{}])[0]
    if rig.get("name") != rig_name:
        continue
    addition = f"\n[rigs.imports.{binding}]\nsource = {json.dumps(source)}\n"
    text = text[:end].rstrip() + "\n" + addition + text[end:]
    fd, temporary = tempfile.mkstemp(prefix=".city.toml.", dir=os.path.dirname(path))
    os.fchmod(fd, os.stat(path).st_mode & 0o777)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        handle.write(text)
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, path)
    break
else:
    raise SystemExit(f"could not locate TOML block for rig {rig_name!r}")
PY

# The scaffold control dispatcher and provider catalog entries inject generic
# targets into every rig. Keep those SDK surfaces visible but non-routable: only
# roles explicitly declared by the selected AgentOps pack remain active.
python3 - "$city/city.toml" "$rig_name" <<'PY'
import os
import sys
import tempfile
import tomllib

path, rig_name = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    text = handle.read()
config = tomllib.loads(text)
changed = False
for provider_name in ("core.control-dispatcher", "codex", "claude"):
    matches = [
        patch for patch in config.get("patches", {}).get("agent", [])
        if patch.get("name") == provider_name and patch.get("dir") == rig_name
    ]
    if matches:
        if len(matches) != 1 or matches[0].get("suspended") is not True:
            raise SystemExit(
                f"rig {rig_name!r} has an invalid managed {provider_name} suspension patch"
            )
        continue
    if text and not text.endswith("\n"):
        text += "\n"
    text += (
        "\n[[patches.agent]]\n"
        f'dir = "{rig_name}"\n'
        f'name = "{provider_name}"\n'
        "suspended = true\n"
    )
    changed = True
    config = tomllib.loads(text)
if not changed:
    raise SystemExit(0)
fd, temporary = tempfile.mkstemp(prefix=".city.toml.", dir=os.path.dirname(path))
os.fchmod(fd, os.stat(path).st_mode & 0o777)
with os.fdopen(fd, "w", encoding="utf-8") as handle:
    handle.write(text)
    handle.flush()
    os.fsync(handle.fileno())
os.replace(temporary, path)
PY

"$gc_bin" --city "$city" config show >/dev/null
"$gc_bin" --city "$city" config explain >/dev/null
for provider_name in codex claude; do
  "$gc_bin" --city "$city" config explain --provider "$provider_name" --json >/dev/null
done
"$gc_bin" --city "$city" import status --json >/dev/null
write_marker ready

if [ "$start" -eq 1 ]; then
  "$gc_bin" --city "$city" start >/dev/null
  "$gc_bin" --city "$city" rig resume "$rig_name" --json >/dev/null
fi

printf 'Gas City AgentOps deployment ready\n'
printf '  city: %s\n' "$city"
printf '  rig:  %s (%s)\n' "$rig_name" "$rig"
printf '  pack: %s as %s\n' "$pack" "$binding"
printf '  supervisor: 127.0.0.1:%s\n' "$supervisor_port"
if [ "$start" -eq 0 ]; then
  printf '  state: configured, not started\n'
else
  printf '  state: started, rig resumed\n'
fi
