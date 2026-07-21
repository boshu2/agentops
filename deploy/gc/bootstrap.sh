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
  --gc-bin PATH     Absolute Gas City CLI path (required; never resolved from PATH)
  --ao-bin PATH     Absolute built AgentOps reducer path (required; never resolved from PATH)
  --replace-gc-bin  Permit a managed city to move to the supplied --gc-bin path
  --codex-auth PATH Existing Codex auth.json to link into the private home
  --max-active-sessions N  City-wide concurrent session cap (default: 1)
  --start-timeout N Bounded seconds to wait for a usable started city (default: 120)
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
gc_bin=""
ao_bin=""
replace_gc_bin=0
codex_auth=""
source_codex_home="${CODEX_HOME:-${HOME:?HOME is required}/.codex}"
start=0
max_active_sessions=1
start_timeout=120

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
    --ao-bin)
      [ "$#" -ge 2 ] || die "--ao-bin requires a path"
      ao_bin="$2"
      shift 2
      ;;
    --replace-gc-bin)
      replace_gc_bin=1
      shift
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
    --start-timeout)
      [ "$#" -ge 2 ] || die "--start-timeout requires a value"
      start_timeout="$2"
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
[ -n "$gc_bin" ] || die "--gc-bin is required"
[ -n "$ao_bin" ] || die "--ao-bin is required"
[[ "$rig_name" =~ ^[A-Za-z0-9_-]+$ ]] || die "--rig-name must contain only letters, digits, underscore, or hyphen"
[[ "$binding" =~ ^[A-Za-z0-9_-]+$ ]] || die "--binding must contain only letters, digits, underscore, or hyphen"
[[ "$max_active_sessions" =~ ^[1-9][0-9]*$ ]] || die "--max-active-sessions must be a positive integer"
[ "$max_active_sessions" -le 64 ] || die "--max-active-sessions must be at most 64"
[[ "$start_timeout" =~ ^[1-9][0-9]*$ ]] || die "--start-timeout must be a positive integer"
[ "$start_timeout" -le 600 ] || die "--start-timeout must be at most 600"
[ "$replace_gc_bin" -eq 0 ] || [ "$start" -eq 1 ] || \
  die "--replace-gc-bin requires --start so the running supervisor can be replaced and verified"
command -v python3 >/dev/null 2>&1 || die "python3 is required"

canonical_path() {
  python3 - "$1" <<'PY'
import os
import sys

print(os.path.realpath(os.path.expanduser(sys.argv[1])))
PY
}

require_beads_safe_path() {
  local label="$1"
  local path="$2"
  local reason=""
  if ! reason="$(python3 - "$path" <<'PY'
import os
import pwd
import sys
import tempfile

path = os.path.realpath(os.path.abspath(os.path.expanduser(sys.argv[1])))


def within(candidate, root):
    root = os.path.realpath(root).rstrip(os.sep) or os.sep
    return candidate == root or candidate.startswith(root + os.sep)


# Match Beads' SEC-003 path-boundary contract. Its OS-designated temp
# directory is an explicit exception, but arbitrary paths under /private or
# /var are not. Catch this before gc creates or starts a city so an invalid
# deployment cannot spend the entire readiness timeout failing `bd context`.
temp_root = os.path.realpath(tempfile.gettempdir())
if within(path, temp_root) or within(path, "/var/home"):
    raise SystemExit(0)

for root in (
    "/etc",
    "/usr",
    "/var",
    "/root",
    "/System",
    "/Library",
    "/bin",
    "/sbin",
    "/opt",
    "/private",
):
    if within(path, root):
        print(f"system boundary {root} is rejected by Beads SEC-003")
        raise SystemExit(1)

if within(path, "/Users/Shared"):
    raise SystemExit(0)

try:
    home = os.path.realpath(pwd.getpwuid(os.getuid()).pw_dir)
except (KeyError, OSError):
    home = os.path.realpath(os.path.expanduser("~"))

if any(within(path, root) for root in ("/Users", "/home", "/var/home")) and not within(path, home):
    print(f"peer home is outside the current user home {home}")
    raise SystemExit(1)
PY
)"; then
    die "$label path is unsafe for Beads: $path${reason:+ ($reason)}"
  fi
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
require_beads_safe_path "city" "$city"
require_beads_safe_path "rig" "$rig"

script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
city_template="$script_dir/city.toml"
claude_wrapper_template="$script_dir/claude-interactive.sh"
claude_settings_template="$script_dir/claude-settings.json"
toolchain_lock="$script_dir/toolchain.lock.json"
[ -f "$city_template" ] || die "city template not found: $city_template"
[ -f "$claude_wrapper_template" ] || die "Claude wrapper template not found: $claude_wrapper_template"
[ -f "$claude_settings_template" ] || die "Claude settings template not found: $claude_settings_template"
[ -f "$toolchain_lock" ] || die "qualified toolchain lock not found: $toolchain_lock"

marker="$city/.gc/agentops-bootstrap.json"
city_has_content=0
if [ -d "$city" ] && [ -n "$(find "$city" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
  city_has_content=1
fi
if [ "$city_has_content" -eq 1 ] && [ ! -f "$marker" ]; then
  die "refusing existing unmanaged city: $city"
fi

[[ "$gc_bin" == */* ]] || die "--gc-bin must be an absolute or explicit path; PATH resolution is forbidden"
[[ "$ao_bin" == */* ]] || die "--ao-bin must be an absolute or explicit path; PATH resolution is forbidden"
[ -x "$gc_bin" ] || die "Gas City CLI is not executable: $gc_bin"
[ -x "$ao_bin" ] || die "AgentOps reducer is not executable: $ao_bin"
gc_bin="$(canonical_path "$gc_bin")"
ao_bin="$(canonical_path "$ao_bin")"

# Development builds commonly place the matched gc and bd binaries together
# outside the ambient PATH. Keep Gas City's Beads subprocesses on that pinned
# toolchain so an unrelated Homebrew bd is neither required nor selected.
gc_bin_dir="$(dirname "$gc_bin")"
export PATH="$gc_bin_dir:$PATH"
bd_bin="$gc_bin_dir/bd"
[ -x "$bd_bin" ] || die "paired Beads CLI is not executable beside gc: $bd_bin"
bd_bin="$(canonical_path "$bd_bin")"
[ "$(dirname "$bd_bin")" = "$gc_bin_dir" ] || \
  die "paired Beads CLI must resolve beside gc: $bd_bin"

ao_reducer_json=""

toolchain_json=""
if ! toolchain_json="$(python3 - "$gc_bin" "$bd_bin" <<'PY'
import hashlib
import json
import os
import re
import subprocess
import sys

gc_bin, bd_bin = map(os.path.realpath, sys.argv[1:])


def run_json(command, label):
    result = subprocess.run(command, check=False, capture_output=True, text=True)
    if result.returncode != 0:
        detail = (result.stderr or result.stdout).strip()
        raise SystemExit(f"{label} failed ({result.returncode}): {detail}")
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        raise SystemExit(f"{label} did not return JSON: {exc}") from exc


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


gc = run_json([gc_bin, "version", "--json"], "gc version --json")
if gc.get("ok") is not True:
    raise SystemExit("gc version --json did not report ok=true")
gc_version = str(gc.get("version", "")).strip()
gc_commit = str(gc.get("commit", "")).strip()
if gc_version == "dev":
    if not re.fullmatch(r"[0-9a-f]{7,40}", gc_commit):
        raise SystemExit("development gc build must embed a hexadecimal commit")
else:
    match = re.fullmatch(r"v?(\d+)\.(\d+)\.(\d+)(?:[-+].*)?", gc_version)
    if match is None:
        raise SystemExit(f"cannot parse gc version {gc_version!r}")
    if tuple(map(int, match.groups())) < (1, 3, 5):
        raise SystemExit(f"gc {gc_version} is unsupported; AgentOps requires >=1.3.5")

bd_result = subprocess.run(
    [bd_bin, "version"], check=False, capture_output=True, text=True
)
if bd_result.returncode != 0:
    detail = (bd_result.stderr or bd_result.stdout).strip()
    raise SystemExit(f"bd version failed ({bd_result.returncode}): {detail}")
bd_match = re.search(r"^bd version (\S+) \(([^:)]+)", bd_result.stdout, re.MULTILINE)
if bd_match is None:
    raise SystemExit(f"cannot parse bd version output: {bd_result.stdout.strip()!r}")
bd_version, bd_commit = bd_match.groups()
if bd_version != "1.1.0":
    raise SystemExit(
        f"bd {bd_version} is unsupported; this Gas City deployment requires 1.1.0"
    )

print(json.dumps({
    "gc": {
        "path": gc_bin,
        "sha256": sha256(gc_bin),
        "version": gc_version,
        "commit": gc_commit,
        "date": str(gc.get("date", "")).strip(),
    },
    "bd": {
        "path": bd_bin,
        "sha256": sha256(bd_bin),
        "version": bd_version,
        "commit": bd_commit,
    },
}, sort_keys=True))
PY
)"; then
  die "invalid paired gc/bd toolchain: $toolchain_json"
fi

qualified_toolchain_json=""
if ! qualified_toolchain_json="$(python3 - "$toolchain_lock" "$toolchain_json" <<'PY'
import json
import sys

lock_path, runtime_json = sys.argv[1:]
with open(lock_path, encoding="utf-8") as handle:
    lock = json.load(handle)
runtime = json.loads(runtime_json)
if lock.get("schema_version") != 1:
    raise SystemExit("toolchain lock schema_version must be 1")
entries = lock.get("accepted_pairs")
if not isinstance(entries, list) or not entries:
    raise SystemExit("toolchain lock must contain accepted_pairs")


def commit_matches(actual, expected):
    return (
        isinstance(actual, str)
        and isinstance(expected, str)
        and len(actual) >= 7
        and expected.startswith(actual)
    )


selected = None
for entry in entries:
    if not isinstance(entry, dict):
        continue
    gc = entry.get("gc", {})
    bd = entry.get("bd", {})
    if (
        runtime["gc"]["version"] == gc.get("version")
        and commit_matches(runtime["gc"]["commit"], gc.get("source_commit"))
        and runtime["bd"]["version"] == bd.get("version")
        and commit_matches(runtime["bd"]["commit"], bd.get("source_commit"))
    ):
        selected = entry
        break
if selected is None:
    raise SystemExit(
        "pair is not in toolchain.lock.json: "
        f"gc {runtime['gc']['version']}@{runtime['gc']['commit']} + "
        f"bd {runtime['bd']['version']}@{runtime['bd']['commit']}"
    )
runtime["qualification"] = {
    "id": selected["id"],
    "status": selected["status"],
    "gc_source_commit": selected["gc"]["source_commit"],
    "bd_source_commit": selected["bd"]["source_commit"],
}
print(json.dumps(runtime, sort_keys=True))
PY
)"; then
  die "unsupported paired gc/bd toolchain: $qualified_toolchain_json"
fi
toolchain_json="$qualified_toolchain_json"

# AO is admitted only from the same materialized toolchain receipt as gc/bd.
# The receipt, not this checkout's current HEAD, is the authority for the
# reducer source commit and committed CLI tree.
toolchain_receipt="$(dirname "$gc_bin_dir")/toolchain.json"
if ! ao_reducer_json="$(python3 - "$toolchain_receipt" "$toolchain_lock" "$toolchain_json" "$ao_bin" "$gc_bin" "$bd_bin" "$pack" "$city_template" "$script_dir/../.." <<'PY'
import hashlib
import json
import os
import re
import subprocess
import sys

receipt_path, lock_path, runtime_json, ao_bin, gc_bin, bd_bin, pack, city_template, repository = sys.argv[1:]
receipt_path, lock_path, ao_bin, gc_bin, bd_bin, pack, city_template, repository = map(
    os.path.realpath,
    (receipt_path, lock_path, ao_bin, gc_bin, bd_bin, pack, city_template, repository),
)
if not os.path.isfile(receipt_path) or os.path.islink(receipt_path):
    raise SystemExit("admitted gc/bd pair has no regular toolchain.json receipt beside bin")

def digest(path):
    value = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            value.update(chunk)
    return value.hexdigest()

def portable_path(value, label):
    if not isinstance(value, str) or not value or os.path.isabs(value):
        raise SystemExit(f"toolchain receipt has invalid {label}.path")
    normalized = os.path.normpath(value)
    if normalized.startswith(".." + os.sep) or normalized == "..":
        raise SystemExit(f"toolchain receipt has escaping {label}.path")
    return normalized

def exact_digest(value, label):
    if not isinstance(value, str) or re.fullmatch(r"[0-9a-f]{64}", value) is None:
        raise SystemExit(f"toolchain receipt has invalid {label} sha256")
    return value

try:
    receipt = json.load(open(receipt_path, encoding="utf-8"))
    lock = json.load(open(lock_path, encoding="utf-8"))
    runtime = json.loads(runtime_json)
except (OSError, json.JSONDecodeError) as exc:
    raise SystemExit(f"cannot read toolchain receipt: {exc}") from exc
if receipt.get("schema_version") != 2:
    raise SystemExit("toolchain receipt schema_version must be 2")
qualification = runtime.get("qualification", {})
pair_id = qualification.get("id")
selected = next((entry for entry in lock.get("accepted_pairs", []) if entry.get("id") == pair_id), None)
if not isinstance(selected, dict) or receipt.get("pair") != selected:
    raise SystemExit("toolchain receipt pair does not match admitted gc/bd pair")
entries = receipt.get("runtime")
if not isinstance(entries, dict) or set(entries) != {"gc", "bd", "ao"}:
    raise SystemExit("toolchain receipt runtime must contain exactly gc, bd, and ao")
root = os.path.dirname(receipt_path)
for label, actual in (("gc", gc_bin), ("bd", bd_bin), ("ao", ao_bin)):
    item = entries.get(label)
    if not isinstance(item, dict):
        raise SystemExit(f"toolchain receipt has no {label} runtime")
    path = portable_path(item.get("path"), label)
    if os.path.realpath(os.path.join(root, path)) != actual:
        raise SystemExit(f"toolchain receipt {label} path does not match admitted binary")
    if exact_digest(item.get("sha256"), label) != digest(actual):
        raise SystemExit(f"toolchain receipt {label} digest does not match admitted binary")
for label in ("gc", "bd"):
    item = entries[label]
    observed = runtime[label]
    if item.get("version") != observed.get("version") or item.get("commit") != observed.get("commit"):
        raise SystemExit(f"toolchain receipt {label} runtime identity does not match admitted binary")
ao = entries["ao"]
source_commit = ao.get("source_commit")
cli_tree = ao.get("cli_tree")
build_version = ao.get("build_version")
if not isinstance(source_commit, str) or re.fullmatch(r"[0-9a-f]{40}", source_commit) is None:
    raise SystemExit("toolchain receipt has invalid ao source_commit")
if not isinstance(cli_tree, str) or re.fullmatch(r"[0-9a-f]{40}", cli_tree) is None:
    raise SystemExit("toolchain receipt has invalid ao cli_tree")
if not isinstance(build_version, str) or not build_version:
    raise SystemExit("toolchain receipt has invalid ao build_version")
result = subprocess.run(["git", "-C", repository, "rev-parse", "--verify", source_commit + ":cli"], capture_output=True, text=True)
if result.returncode or result.stdout.strip() != cli_tree:
    raise SystemExit("toolchain receipt ao cli_tree does not resolve from its source_commit")
inputs = [city_template, lock_path]
pack_inputs = []
for current_root, _dirs, files in os.walk(pack):
    for name in files:
        path = os.path.join(current_root, name)
        if not os.path.isfile(path) or os.path.islink(path):
            raise SystemExit(f"ao reducer pack input is not a regular file: {path}")
        pack_inputs.append(path)
        if name.endswith((".toml", ".json", ".py")):
            inputs.append(path)
config_entries = [(path, digest(path)) for path in sorted(set(inputs))]
pack_entries = [(os.path.relpath(path, pack).replace(os.sep, "/"), digest(path)) for path in sorted(pack_inputs)]
print(json.dumps({
    "path": ao_bin,
    "binary_sha256": digest(ao_bin),
    "source_commit": source_commit,
    "cli_tree": cli_tree,
    "build_version": build_version,
    "pack_content_sha256": hashlib.sha256(json.dumps(pack_entries, separators=(",", ":"), ensure_ascii=True).encode()).hexdigest(),
    "schema_config_sha256": hashlib.sha256(json.dumps(config_entries, separators=(",", ":"), ensure_ascii=True).encode()).hexdigest(),
}, sort_keys=True))
PY
)"; then
  die "invalid admitted ao reducer identity: $ao_reducer_json"
fi

previous_gc_bin=""
toolchain_replacement=0
if [ -f "$marker" ]; then
  marker_identity=""
  if ! marker_identity="$(python3 - "$marker" "$city" "$rig" "$pack" "$rig_name" "$binding" "$codex_auth" "$max_active_sessions" "$replace_gc_bin" "$toolchain_json" "$ao_reducer_json" <<'PY'
import json
import os
import sys

marker_path, city, rig, pack, rig_name, binding, codex_auth, max_active_sessions, replace_gc_bin, toolchain_json, ao_reducer_json = sys.argv[1:]
with open(marker_path, encoding="utf-8") as handle:
    marker = json.load(handle)
schema_version = marker.get("schema_version")
if schema_version not in {2, 3, 4}:
    raise SystemExit(f"managed city marker has unsupported schema_version {schema_version!r}")
expected = {
    "city": city,
    "rig": rig,
    "pack": pack,
    "rig_name": rig_name,
    "binding": binding,
    "codex_auth": codex_auth,
    "max_active_sessions": int(max_active_sessions),
}
for key, value in expected.items():
    actual = marker.get(key, 1 if key == "max_active_sessions" else None)
    if actual != value:
        raise SystemExit(
            f"managed city mismatch for {key}: {actual!r} != {value!r}"
        )
requested_toolchain = json.loads(toolchain_json)
if schema_version == 2:
    actual_gc = marker.get("gc_bin")
    requested_gc = requested_toolchain["gc"]["path"]
    replacement = actual_gc != requested_gc
    if replace_gc_bin != "1" and replacement:
        raise SystemExit(
            f"managed city mismatch for gc_bin: {actual_gc!r} != {requested_gc!r}"
        )
else:
    actual_gc = marker.get("toolchain", {}).get("gc", {}).get("path")
    replacement = marker.get("toolchain") != requested_toolchain
    if replace_gc_bin != "1" and replacement:
        raise SystemExit("managed city mismatch for paired gc/bd toolchain identity")
if schema_version != 4 or marker.get("ao_reducer") != json.loads(ao_reducer_json):
    raise SystemExit("managed city mismatch for ao reducer identity")
packs_lock = os.path.join(city, "packs.lock")
recorded_lock = marker.get("packs_lock_sha256")
if not isinstance(recorded_lock, str) or len(recorded_lock) != 64:
    raise SystemExit("managed city marker has no packs.lock digest")
if not os.path.isfile(packs_lock) or os.path.islink(packs_lock):
    raise SystemExit("managed city packs.lock is missing or unsafe")
import hashlib
actual_lock = hashlib.sha256(open(packs_lock, "rb").read()).hexdigest()
if actual_lock != recorded_lock:
    raise SystemExit("managed city mismatch for packs.lock identity")
if not isinstance(actual_gc, str) or not actual_gc.strip():
    raise SystemExit("managed city marker has no prior gc binary path")
print(f"{os.path.realpath(os.path.expanduser(actual_gc))}\t{int(replacement)}")
PY
)"; then
    die "invalid or mismatched managed-city marker: $marker_identity"
  fi
  IFS=$'\t' read -r previous_gc_bin toolchain_replacement <<<"$marker_identity"
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
export AO_BIN="$ao_bin"
# A managed test/production city must not inherit desktop-wide OTLP endpoints.
# Role sessions also receive this through city.toml, but bootstrap and the
# supervisor perform foreground Beads/Dolt work before any role is launched.
export OTEL_SDK_DISABLED=true
# A Git remote is not necessarily a Dolt remote (local qualification clones are
# the sharp edge). Never let rig registration synthesize or sync Dolt remotes
# from Git URLs; off-box Beads replication is an explicit operator concern.
export BD_DOLT_SYNC_CLI_REMOTES=false
export BEADS_DOLT_SYNC_CLI_REMOTES=false

"$gc_bin" lint "$pack" --json >/dev/null

write_marker() {
  local state="$1"
  mkdir -p "$city/.gc"
  python3 - "$marker" "$city" "$rig" "$pack" "$rig_name" "$binding" "$codex_auth" "$max_active_sessions" "$state" "$toolchain_json" "$ao_reducer_json" "$city/packs.lock" <<'PY'
import json
import os
import sys
import tempfile

path, city, rig, pack, rig_name, binding, codex_auth, max_active_sessions, state, toolchain_json, ao_reducer_json, packs_lock = sys.argv[1:]
if not os.path.isfile(packs_lock) or os.path.islink(packs_lock):
    raise SystemExit("managed city must have a regular packs.lock generated by Gas City")
with open(packs_lock, "rb") as handle:
    packs_lock_sha256 = __import__("hashlib").sha256(handle.read()).hexdigest()
payload = {
    "schema_version": 4,
    "state": state,
    "city": city,
    "rig": rig,
    "pack": pack,
    "rig_name": rig_name,
    "binding": binding,
    "codex_auth": codex_auth,
    "max_active_sessions": int(max_active_sessions),
    "toolchain": json.loads(toolchain_json),
    "ao_reducer": json.loads(ao_reducer_json),
    "packs_lock_sha256": packs_lock_sha256,
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
  # Released Gas City builds resolve city patches before the scaffold imports
  # have materialized their agents. Passing the final policy here therefore
  # makes valid patches for bd.dog, core.control-dispatcher, codex, and claude
  # fail as "agent not found". Initialize from the same provider/workspace
  # policy without agent patches; after gc init has installed the SDK-owned
  # scaffold, the normal reconciliation below writes the final fail-closed
  # patch set.
  init_template="$(mktemp "${TMPDIR:-/tmp}/gc-agentops-city-init.XXXXXX")"
  trap 'rm -f "$init_template"' EXIT
  python3 - "$city_template" "$init_template" <<'PY'
import os
import sys
import tomllib

source, destination = sys.argv[1:]
with open(source, encoding="utf-8") as handle:
    lines = handle.readlines()

rendered = []
skipping_agent_patch = False
for line in lines:
    stripped = line.strip()
    if stripped == "[[patches.agent]]":
        skipping_agent_patch = True
        continue
    if skipping_agent_patch and stripped.startswith("["):
        skipping_agent_patch = False
    if not skipping_agent_patch:
        rendered.append(line)

text = "".join(rendered).rstrip() + "\n"
config = tomllib.loads(text)
if config.get("patches", {}).get("agent"):
    raise SystemExit("gc init policy must not contain agent patches")
with open(destination, "w", encoding="utf-8") as handle:
    handle.write(text)
    handle.flush()
    os.fsync(handle.fileno())
PY
  "$gc_bin" init \
    --file "$init_template" \
    --no-start \
    --skip-provider-readiness \
    "$city" >/dev/null
  rm -f "$init_template"
  trap - EXIT
  [ -f "$city/pack.toml" ] || die "gc init did not create pack.toml"
  [ -f "$city/city.toml" ] || die "gc init did not create city.toml"
  write_marker configuring
fi

# Managed Claude sessions must not inherit the operator's optional Remote
# Control startup preference. A second interactive Claude process can supersede
# that account-wide remote epoch and terminate a healthy GC worker. Gas City
# owns this city-level source and projects it into .gc/settings.json together
# with its hooks when the city starts or reloads.
python3 - "$claude_settings_template" "$city/.claude/settings.json" <<'PY'
import json
import os
import sys
import tempfile

source, destination = sys.argv[1:]
with open(source, "rb") as handle:
    data = handle.read()
value = json.loads(data)
if value != {"remoteControlAtStartup": False}:
    raise SystemExit("managed Claude settings must disable Remote Control startup exactly")
if os.path.islink(destination):
    raise SystemExit(f"refusing managed Claude settings symlink: {destination}")
os.makedirs(os.path.dirname(destination), mode=0o700, exist_ok=True)
fd, temporary = tempfile.mkstemp(prefix=".settings.", dir=os.path.dirname(destination))
try:
    os.fchmod(fd, 0o600)
    with os.fdopen(fd, "wb") as handle:
        handle.write(data)
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, destination)
finally:
    if os.path.exists(temporary):
        os.unlink(temporary)
PY

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
# The city owns this isolated Codex profile. Pre-trust the caller-selected Git
# root before the first managed session starts; otherwise Codex presents a
# first-use workspace dialog. Prompt delivery is also configured through Gas
# City's post-readiness nudge, which protects dynamic non-Git role directories.
python3 - "$CODEX_HOME/config.toml" "$rig" <<'PY'
import json
import os
import re
import sys
import tempfile
import tomllib

path, rig = sys.argv[1:]
rig = os.path.realpath(rig)
header = f"[projects.{json.dumps(rig)}]"
text = ""
if os.path.exists(path):
    with open(path, encoding="utf-8") as handle:
        text = handle.read()
    tomllib.loads(text)

# Managed interactive sessions must never stop at Codex's optional update
# picker.  Keep this city-private so the factory does not mutate the operator's
# normal Codex profile.
first_table = re.search(r"(?m)^\[", text)
root_end = first_table.start() if first_table else len(text)
root = text[:root_end]
tail = text[root_end:]
update_match = re.search(
    r"(?m)^check_for_update_on_startup[ \t]*=[ \t]*(?:true|false)[ \t]*$",
    root,
)
if update_match:
    root = (
        root[:update_match.start()]
        + "check_for_update_on_startup = false"
        + root[update_match.end():]
    )
else:
    root = "check_for_update_on_startup = false\n" + root
text = root + tail

header_match = re.search(rf"(?m)^{re.escape(header)}[ \t]*$", text)
if header_match:
    block_end_match = re.search(r"(?m)^\[", text[header_match.end():])
    block_end = (
        header_match.end() + block_end_match.start()
        if block_end_match
        else len(text)
    )
    block = text[header_match.end():block_end]
    trust_match = re.search(r'(?m)^trust_level[ \t]*=[ \t]*"[^"]*"[ \t]*$', block)
    if trust_match:
        block = (
            block[:trust_match.start()]
            + 'trust_level = "trusted"'
            + block[trust_match.end():]
        )
    else:
        block = '\ntrust_level = "trusted"' + block
    text = text[:header_match.end()] + block + text[block_end:]
else:
    if text and not text.endswith("\n"):
        text += "\n"
    if text:
        text += "\n"
    text += f'{header}\ntrust_level = "trusted"\n'

parsed = tomllib.loads(text)
if parsed.get("check_for_update_on_startup") is not False:
    raise SystemExit("failed to disable the managed Codex startup update prompt")
if parsed.get("projects", {}).get(rig, {}).get("trust_level") != "trusted":
    raise SystemExit(f"failed to pre-trust managed Codex project {rig}")

os.makedirs(os.path.dirname(path), mode=0o700, exist_ok=True)
fd, temporary = tempfile.mkstemp(prefix=".config.toml.", dir=os.path.dirname(path))
try:
    os.fchmod(fd, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        handle.write(text)
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, path)
finally:
    if os.path.exists(temporary):
        os.unlink(temporary)
PY
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
claude_wrapper="$city/.gc/bin/claude-interactive"
python3 - "$claude_wrapper_template" "$claude_wrapper" "$claude_bin" <<'PY'
import os
import sys
import tempfile

template_path, destination, claude_bin = sys.argv[1:]
with open(template_path, encoding="utf-8") as handle:
    text = handle.read()
sentinel = "__GC_AGENTOPS_CLAUDE_BIN__"
if text.count(sentinel) != 1:
    raise SystemExit(f"Claude wrapper template must contain exactly one {sentinel}")
text = text.replace(sentinel, claude_bin)
os.makedirs(os.path.dirname(destination), mode=0o700, exist_ok=True)
fd, temporary = tempfile.mkstemp(prefix=".claude-interactive.", dir=os.path.dirname(destination))
try:
    os.fchmod(fd, 0o700)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        handle.write(text)
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, destination)
finally:
    if os.path.exists(temporary):
        os.unlink(temporary)
PY

python3 - "$city/city.toml" "$city_template" "$CODEX_HOME" "$gc_bin" "$ao_bin" "$city" "$rig" "$max_active_sessions" "$claude_wrapper" <<'PY'
import json
import hashlib
import os
import re
import sys
import tempfile
import tomllib

path, template_path, codex_home, gc_bin, ao_bin, city, requested_rig, max_active_sessions, claude_wrapper = sys.argv[1:]
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
    "__GC_AGENTOPS_AO_BIN__": ao_bin,
    "__GC_AGENTOPS_CLAUDE_WRAPPER__": claude_wrapper,
    "__GC_AGENTOPS_TMUX_SOCKET__": (
        "agentops-" + hashlib.sha256(os.path.realpath(city).encode()).hexdigest()[:20]
    ),
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

codex_scope_args = ["--dangerously-bypass-hook-trust"]
for root in canonical_scope_roots:
    codex_scope_args.extend(["--add-dir", root])
claude_scope_args = ["--add-dir", *canonical_scope_roots, "--safe-mode"]
scope_lines = {
    'args_append = ["--dangerously-bypass-hook-trust", "__GC_AGENTOPS_CODEX_SCOPE_ARGS__"]':
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
session = config.get("session", {})
codex_provider = config.get("providers", {}).get("codex", {})
claude_provider = config.get("providers", {}).get("claude", {})
if workspace.get("provider"):
    raise SystemExit("city workspace provider must stay unset; roles select providers explicitly")
if workspace.get("max_active_sessions") != max_active_sessions:
    raise SystemExit(f"city workspace max_active_sessions must be {max_active_sessions}")
if workspace.get("env", {}).get("GC_BIN") != gc_bin:
    raise SystemExit("city workspace must pin GC_BIN for managed sessions")
if workspace.get("env", {}).get("OTEL_SDK_DISABLED") != "true":
    raise SystemExit("city workspace must disable inherited OTLP exporters for managed sessions")
expected_socket = "agentops-" + hashlib.sha256(os.path.realpath(city).encode()).hexdigest()[:20]
if session.get("socket") != expected_socket:
    raise SystemExit("city tmux socket must be bound to the canonical city path")
if session.get("setup_timeout") != "60s":
    raise SystemExit("city session setup timeout must cover cold skill materialization")
if codex_provider.get("base") != "builtin:codex":
    raise SystemExit("city must register the builtin Codex provider")
if codex_provider.get("prompt_mode") != "none":
    raise SystemExit("Codex startup prompts must use Gas City's post-readiness nudge")
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
expected_codex_args = ["--dangerously-bypass-hook-trust"]
for root in canonical_expected_roots:
    expected_codex_args.extend(["--add-dir", root])
if codex_provider.get("args_append") != expected_codex_args:
    raise SystemExit("Codex writable roots must exactly match managed runtime and rig roots")
if claude_provider.get("base") != "builtin:claude":
    raise SystemExit("city must register the builtin Claude provider")
if claude_provider.get("command") != claude_wrapper:
    raise SystemExit("Claude provider must use the city-local interactive diagnostics wrapper")
if claude_provider.get("path_check") != "claude":
    raise SystemExit("Claude provider readiness must check the real Claude CLI")
if claude_provider.get("print_args") != []:
    raise SystemExit("Claude print_args must be empty; interactive sessions are mandatory")
if claude_provider.get("args_append") != ["--add-dir", *canonical_expected_roots, "--safe-mode"]:
    raise SystemExit("Claude must use only managed runtime/rig roots plus safe mode")
for field in ("args", "args_append"):
    if any(value in {"-p", "--print"} for value in claude_provider.get(field, [])):
        raise SystemExit(f"Claude {field} must not contain print-mode flags")
if claude_provider.get("option_defaults", {}).get("permission_mode") != "unrestricted":
    raise SystemExit("Claude permission_mode must bypass the remote auto-mode classifier")
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
    (
        "claude",
        "unrestricted",
        {
            "auto": ["--permission-mode", "auto"],
            "unrestricted": ["--dangerously-skip-permissions"],
        },
    ),
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
for provider_name, expected_default, expected_choices in (
    (
        "codex",
        "gpt-5.6-terra",
        {
            "gpt-5.6-terra": ["--model", "gpt-5.6-terra"],
            "gpt-5.6-sol": ["--model", "gpt-5.6-sol"],
        },
    ),
    (
        "claude",
        "opus-4.8",
        {
            "opus-4.8": ["--model", "claude-opus-4-8"],
            "fable-5": ["--model", "claude-fable-5"],
        },
    ),
):
    provider = config["providers"][provider_name]
    if provider.get("option_defaults", {}).get("model") != expected_default:
        raise SystemExit(f"{provider_name} model default must be {expected_default}")
    selected = [
        option for option in provider.get("options_schema", [])
        if option.get("key") == "model"
    ]
    if len(selected) != 1 or selected[0].get("default") != expected_default:
        raise SystemExit(f"{provider_name} must declare exactly one role-pinned model schema")
    choices = {choice.get("value"): choice for choice in selected[0].get("choices", [])}
    if set(choices) != set(expected_choices):
        raise SystemExit(f"{provider_name} model schema exposes unexpected choices")
    for choice, expected_flags in expected_choices.items():
        if choices.get(choice, {}).get("flag_args") != expected_flags:
            raise SystemExit(f"{provider_name} {choice} model flags are incompatible with the installed CLI")
for provider_name, expected_default, expected_choices in (
    ("codex", "high", {"high": ["-c", "model_reasoning_effort=high"]}),
    (
        "claude",
        "medium",
        {
            "medium": ["--effort", "medium"],
            "adaptive": [],
        },
    ),
):
    provider = config["providers"][provider_name]
    if provider.get("option_defaults", {}).get("effort") != expected_default:
        raise SystemExit(f"{provider_name} effort default must be {expected_default}")
    selected = [
        option for option in provider.get("options_schema", [])
        if option.get("key") == "effort"
    ]
    if len(selected) != 1 or selected[0].get("default") != expected_default:
        raise SystemExit(f"{provider_name} must declare exactly one role-pinned effort schema")
    choices = {choice.get("value"): choice for choice in selected[0].get("choices", [])}
    if set(choices) != set(expected_choices):
        raise SystemExit(f"{provider_name} effort schema exposes unexpected choices")
    for choice, expected_flags in expected_choices.items():
        if choices.get(choice, {}).get("flag_args") != expected_flags:
            raise SystemExit(f"{provider_name} {choice} effort flags are incompatible with the installed CLI")
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

# `bd init` treats a Git origin advertising refs/dolt/data as an instruction to
# bootstrap from that remote.  This bootstrap owns an isolated, local Beads
# store, so expose no Git origin while GC registers the rig.  Rename the whole
# remote section instead of rewriting only its URL so fetch refspecs, push URLs,
# and custom remote configuration survive byte-for-byte.  The reserved section
# also lets a later invocation recover if this process was killed after the
# rename but before the EXIT trap ran.
add_rig_with_local_beads() (
  local parked_remote="agentops-bootstrap-origin"
  local origin_parked=0
  local pre_add_head=""
  local post_add_head=""
  local unexpected_commit=0
  local unexpected_index=0
  local metadata="$rig/.beads/metadata.json"
  local config="$rig/.beads/config.yaml"
	local project_gitignore="$rig/.gitignore"
	local gitignore_backup=""
	local gitignore_existed=0
  local -a add_args=(
    --city "$city" rig add "$rig"
    --name "$rig_name"
    --start-suspended
  )

  remote_section_exists() {
    git -C "$rig" config --local --get-regexp "^remote\\.$1\\." >/dev/null 2>&1
  }

  restore_origin() {
    local status="$?"
    trap - EXIT HUP INT TERM
    if [ "$origin_parked" -eq 1 ]; then
      if remote_section_exists origin; then
        printf '%s\n' \
          "gc-agentops bootstrap: cannot restore remote.origin because rig registration created a replacement" >&2
        exit 1
      fi
      git -C "$rig" config --local --rename-section \
        "remote.$parked_remote" remote.origin || exit 1
    fi
		[ -z "$gitignore_backup" ] || rm -f "$gitignore_backup"
    exit "$status"
  }

  trap restore_origin EXIT
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 143' TERM

  if remote_section_exists "$parked_remote"; then
    if remote_section_exists origin; then
      die "rig has both remote.origin and reserved remote.$parked_remote; refusing ambiguous recovery"
    fi
    git -C "$rig" config --local --rename-section \
      "remote.$parked_remote" remote.origin
  fi
  if remote_section_exists origin; then
    git -C "$rig" config --local --rename-section \
      remote.origin "remote.$parked_remote"
    origin_parked=1
  fi

  if git -C "$rig" rev-parse --verify HEAD >/dev/null 2>&1; then
    git -C "$rig" diff --cached --quiet || \
      die "rig index has staged changes; gc rig add may invoke bd init, so commit or unstage them first"
    pre_add_head="$(git -C "$rig" rev-parse HEAD)"
		[ ! -L "$project_gitignore" ] || \
		  die "refusing project .gitignore symlink during gc rig add: $project_gitignore"
		gitignore_backup="$(mktemp "$city/.gc/project-gitignore-before.XXXXXX")"
		if [ -e "$project_gitignore" ]; then
			cp -p "$project_gitignore" "$gitignore_backup"
			gitignore_existed=1
		fi
  fi

  if [ -e "$metadata" ] || [ -e "$config" ]; then
    [ -f "$metadata" ] && [ -f "$config" ] || \
      die "rig has an incomplete Beads initialization; expected both $metadata and $config"
    add_args+=(--adopt)
  fi

  "$gc_bin" "${add_args[@]}" >/dev/null

  # `bd init --init-if-missing` currently creates a host-endpoint metadata
  # commit even on --adopt, after which GC correctly normalizes that metadata
  # back to its portable form. The runtime must not advance the caller's base
  # branch merely to attach a store. Rewind only commits created during this
  # exact registration call, reset the previously-clean index to the original
  # tree, and leave the normalized working files untouched.
  if [ -n "$pre_add_head" ]; then
    post_add_head="$(git -C "$rig" rev-parse HEAD)"
    if [ "$post_add_head" != "$pre_add_head" ]; then
      while IFS= read -r subject; do
        [ "$subject" = "bd init: initialize beads issue tracking" ] || unexpected_commit=1
      done <<EOF
$(git -C "$rig" log --format=%s "$pre_add_head..$post_add_head")
EOF
      while IFS= read -r changed_path; do
        case "$changed_path" in
          .beads/*) ;;
			  .gitignore) ;;
          *) unexpected_commit=1 ;;
        esac
      done <<EOF
$(git -C "$rig" diff --name-only "$pre_add_head" "$post_add_head")
EOF
		  if ! git -C "$rig" diff --quiet "$pre_add_head" "$post_add_head" -- .gitignore; then
			  python3 - "$gitignore_backup" "$rig" "$post_add_head" <<'PY' || unexpected_commit=1
import subprocess
import sys

backup, rig, commit = sys.argv[1:]
with open(backup, "rb") as handle:
    before = handle.read()
after = subprocess.check_output(
    ["git", "-C", rig, "show", f"{commit}:.gitignore"]
)
header = b"# Beads / Dolt files (added by bd init)\n"
variants = (
    (b".dolt/", b"*.db", b".beads-credential-key"),
    (b".dolt/", b"*.db", b".beads-credential-key", b".beads/proxieddb/"),
)
lines = {line.strip() for line in before.splitlines()}
for patterns in variants:
    missing = [pattern for pattern in patterns if pattern not in lines]
    if not missing:
        continue
    expected = before
    if expected and not expected.endswith(b"\n"):
        expected += b"\n"
    expected += b"\n" + header + b"".join(pattern + b"\n" for pattern in missing)
    if after == expected:
        raise SystemExit(0)
raise SystemExit("bd init commit changed .gitignore outside its canonical append-only stanza")
PY
		  fi
      git -C "$rig" update-ref -m "gc rig add: discard transient bd init commit" \
        HEAD "$pre_add_head" "$post_add_head"
      git -C "$rig" read-tree "$pre_add_head^{tree}"
      [ "$unexpected_commit" -eq 0 ] || \
        die "gc rig add created an unexpected commit; HEAD was restored and its file changes were left in the worktree"
		  if [ "$gitignore_existed" -eq 1 ]; then
			  cp -p "$gitignore_backup" "$project_gitignore"
		  else
			  rm -f "$project_gitignore"
		  fi
    fi

    # Newer Beads releases can leave the same canonical initialization files
    # staged without creating the historical bd-init commit. The caller's
    # index was proven clean above, so accept only Beads-owned paths, verify
    # the exact append-only .gitignore stanza, and restore the original index.
    if ! git -C "$rig" diff --cached --quiet; then
      while IFS= read -r changed_path; do
        case "$changed_path" in
          .beads/*) ;;
          .gitignore) ;;
          *) unexpected_index=1 ;;
        esac
      done <<EOF
$(git -C "$rig" diff --cached --name-only)
EOF
      if ! git -C "$rig" diff --cached --quiet -- .gitignore; then
        python3 - "$gitignore_backup" "$rig" <<'PY' || unexpected_index=1
import subprocess
import sys

backup, rig = sys.argv[1:]
with open(backup, "rb") as handle:
    before = handle.read()
# Gas City may append its own runtime stanza to the working copy after Beads
# stages initialization. Validate the Beads-owned index projection, which is
# the exact content this cleanup is about, instead of conflating later
# unstaged Gas City bytes with the staged bd-init change.
after = subprocess.check_output(["git", "-C", rig, "show", ":.gitignore"])
header = b"# Beads / Dolt files (added by bd init)\n"
variants = (
    (b".dolt/", b"*.db", b".beads-credential-key"),
    (b".dolt/", b"*.db", b".beads-credential-key", b".beads/proxieddb/"),
)
lines = {line.strip() for line in before.splitlines()}
for patterns in variants:
    missing = [pattern for pattern in patterns if pattern not in lines]
    if not missing:
        continue
    expected = before
    if expected and not expected.endswith(b"\n"):
        expected += b"\n"
    expected += b"\n" + header + b"".join(pattern + b"\n" for pattern in missing)
    if after == expected:
        raise SystemExit(0)
raise SystemExit("bd init staged .gitignore outside its canonical append-only stanza")
PY
      fi
      git -C "$rig" read-tree "$pre_add_head^{tree}"
      [ "$unexpected_index" -eq 0 ] || \
        die "gc rig add staged unexpected paths; the index was restored and its file changes were left in the worktree"
		  if [ "$gitignore_existed" -eq 1 ]; then
		    cp -p "$gitignore_backup" "$project_gitignore"
		  else
		    rm -f "$project_gitignore"
		  fi
    fi

    # `gc rig add` also upgrades legacy whole-directory Beads ignores in the
    # working copy after any bd-init commit/index activity. That projection is
    # correct for a GC-owned repository, but registration must not rewrite a
    # caller-owned branch. Accept only the exact upstream rig projection from
    # the bytes captured immediately before this call, then restore those
    # original bytes and mode. Any other .gitignore mutation is left in place
    # and fails closed for inspection.
    if [ -e "$project_gitignore" ] || [ "$gitignore_existed" -eq 1 ]; then
      if ! cmp -s "$gitignore_backup" "$project_gitignore"; then
        python3 - "$gitignore_backup" "$project_gitignore" <<'PY'
import sys

before_path, after_path = sys.argv[1:]
with open(before_path, "rb") as handle:
    before = handle.read()
with open(after_path, "rb") as handle:
    after = handle.read()

legacy = {b".beads", b".beads/", b"/.beads", b"/.beads/"}
obsolete = {
    b"!.beads/config.yaml", b"!/.beads/config.yaml", b"!**/.beads/config.yaml",
    b"!.beads/metadata.json", b"!/.beads/metadata.json", b"!**/.beads/metadata.json",
}
lines = before.split(b"\n")
cleaned_lines = []
present = set()
removed = False
for line in lines:
    trimmed = line.strip()
    if trimmed in legacy or trimmed in obsolete:
        removed = True
        continue
    cleaned_lines.append(line)
    present.add(trimmed)
cleaned = b"\n".join(cleaned_lines)
entries = (b".beads/*", b"!.beads/identity.toml")
new_entries = [entry for entry in entries if entry not in present]
if not new_entries:
    expected = cleaned if removed else before
else:
    expected = cleaned
    if expected:
        if not expected.endswith(b"\n"):
            expected += b"\n"
        if not expected.endswith(b"\n\n"):
            expected += b"\n"
    expected += b"# Gas City\n" + b"".join(entry + b"\n" for entry in new_entries)
if after != expected:
    raise SystemExit(
        "gc rig add changed .gitignore outside its canonical upstream rig projection"
    )
PY
        if [ "$gitignore_existed" -eq 1 ]; then
          cp -p "$gitignore_backup" "$project_gitignore"
        else
          rm -f "$project_gitignore"
        fi
      fi
    fi
  fi
)

add_rig_with_local_beads

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

executor_agents_before="$(mktemp "$city/.gc/executor-agents-before.XXXXXX")"
executor_agents_after="$(mktemp "$city/.gc/executor-agents-after.XXXXXX")"
trap 'rm -f "$executor_agents_before" "$executor_agents_after"' EXIT
"$gc_bin" --city "$city" agent list --json >"$executor_agents_before"

# Direct executor packets carry the candidate directory name in
# gc.pack_workspace. Gas City resolves that value relative to the role's
# work_dir, so the primary rig needs the same parent-root policy as every
# dynamic factory worktree rig. Pointing these roles at the rig itself launches
# them at <rig>/<rig>; leaving work_dir unset has the same effect through GC's
# rig default. Reconcile only the three packet executor routes, then prove the
# effective native projection below.
python3 - "$city/city.toml" "$executor_agents_before" "$rig_name" "$binding" "$rig" <<'PY'
import json
import os
import re
import sys
import tempfile
import tomllib

path, agents_path, rig_name, binding, rig_root = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    text = handle.read()
with open(agents_path, encoding="utf-8") as handle:
    value = json.load(handle)

agents = value.get("agents") if isinstance(value, dict) else None
if not isinstance(agents, list):
    raise SystemExit("gc agent list returned no agent inventory")
roles = ("implementer", "implementer-claude", "validator")
expected = {
    f"{rig_name}/{binding}.{role}": f"{binding}.{role}"
    for role in roles
}
for qualified_name in expected:
    matches = [
        agent for agent in agents
        if isinstance(agent, dict) and agent.get("qualified_name") == qualified_name
    ]
    if len(matches) != 1:
        raise SystemExit(
            f"selected AgentOps pack must expose exactly one {qualified_name!r}; "
            f"found {len(matches)}"
        )
    agent = matches[0]
    if agent.get("dir") != rig_name or agent.get("scope") != "rig":
        raise SystemExit(f"executor route {qualified_name!r} is not rig-scoped")
    if agent.get("suspended") is True:
        raise SystemExit(f"executor route {qualified_name!r} is suspended")

markers = list(re.finditer(r"(?m)^\[\[(rigs|patches\.agent)\]\]\s*$", text))
if not markers:
    raise SystemExit("managed city contains neither rig nor agent patch blocks")
base = text[:markers[0].start()].rstrip()
rig_sections = []
preserved_patches = []
managed_names = set(expected.values())
for index, match in enumerate(markers):
    end = markers[index + 1].start() if index + 1 < len(markers) else len(text)
    block = text[match.start():end].strip()
    parsed = tomllib.loads(block)
    if match.group(1) == "rigs":
        rig_sections.append(block)
        continue
    patches = parsed.get("patches", {}).get("agent", [])
    if len(patches) != 1:
        raise SystemExit("managed city contains an invalid agent patch block")
    patch = patches[0]
    if patch.get("dir") == rig_name and patch.get("name") in managed_names:
        continue
    preserved_patches.append(block)

session_work_dir = os.path.dirname(os.path.realpath(rig_root))
managed_patches = [
    "[[patches.agent]]\n"
    f"dir = {json.dumps(rig_name)}\n"
    f"name = {json.dumps(local_name)}\n"
    f"work_dir = {json.dumps(session_work_dir)}"
    for local_name in expected.values()
]
rendered = "\n\n".join([base, *rig_sections, *preserved_patches, *managed_patches]) + "\n"
tomllib.loads(rendered)
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
PY

"$gc_bin" --city "$city" config show >/dev/null
"$gc_bin" --city "$city" agent list --json >"$executor_agents_after"
python3 - "$executor_agents_after" "$rig_name" "$binding" "$rig" <<'PY'
import json
import os
import sys

path, rig_name, binding, rig_root = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    value = json.load(handle)
agents = value.get("agents") if isinstance(value, dict) else None
if not isinstance(agents, list):
    raise SystemExit("gc agent list returned no agent inventory after workspace reconciliation")
expected_work_dir = os.path.dirname(os.path.realpath(rig_root))
for role in ("implementer", "implementer-claude", "validator"):
    qualified_name = f"{rig_name}/{binding}.{role}"
    matches = [
        agent for agent in agents
        if isinstance(agent, dict) and agent.get("qualified_name") == qualified_name
    ]
    if len(matches) != 1 or matches[0].get("work_dir") != expected_work_dir:
        actual = [agent.get("work_dir") for agent in matches]
        raise SystemExit(
            f"executor route {qualified_name!r} must resolve packet workspaces from "
            f"{expected_work_dir!r}; found {actual!r}"
        )
PY
rm -f "$executor_agents_before" "$executor_agents_after"
trap - EXIT

"$gc_bin" --city "$city" config explain >/dev/null
for provider_name in codex claude; do
  "$gc_bin" --city "$city" config explain --provider "$provider_name" --json >/dev/null
done
"$gc_bin" --city "$city" import status --json >/dev/null
if [ "$toolchain_replacement" -eq 0 ]; then
  write_marker ready
fi

if [ "$start" -eq 1 ]; then
  previous_supervisor_pid=0
  if [ "$toolchain_replacement" -eq 1 ]; then
    stop_bin="$gc_bin"
    if [ -x "$previous_gc_bin" ]; then
      stop_bin="$previous_gc_bin"
    fi
    replacement_status="$(mktemp "$city/.gc/replacement-status.XXXXXX")"
    if "$stop_bin" supervisor status --json >"$replacement_status" 2>/dev/null; then
      previous_supervisor_pid="$(python3 - "$replacement_status" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    status = json.load(handle)
pid = status.get("pid", 0)
print(pid if isinstance(pid, int) and not isinstance(pid, bool) and pid > 0 else 0)
PY
)"
    fi
    rm -f "$replacement_status"
    if [ "$previous_supervisor_pid" -gt 0 ]; then
      "$stop_bin" supervisor stop --wait --wait-timeout "${start_timeout}s" >/dev/null
    fi
    "$gc_bin" supervisor install --force >/dev/null
  fi

  "$gc_bin" --city "$city" start --no-auto-restart >/dev/null
  "$gc_bin" --city "$city" rig resume "$rig_name" --json >/dev/null

  start_status="$(mktemp "$city/.gc/start-status.XXXXXX")"
  start_error="$(mktemp "$city/.gc/start-status-error.XXXXXX")"
  start_deadline="$(( $(date +%s) + start_timeout ))"
  started_usable=0
  while [ "$(date +%s)" -le "$start_deadline" ]; do
    if "$gc_bin" --city "$city" status --json >"$start_status" 2>"$start_error" && \
        python3 - "$start_status" "$rig_name" <<'PY'
import json
import sys

path, rig_name = sys.argv[1:]
try:
    with open(path, encoding="utf-8") as handle:
        status = json.load(handle)
except (OSError, json.JSONDecodeError):
    raise SystemExit(1)
matches = [
    rig for rig in status.get("rigs", [])
    if isinstance(rig, dict) and rig.get("name") == rig_name
]
beads = status.get("beads")
healthy = (
    status.get("ok") is True
    and status.get("controller", {}).get("running") is True
    and status.get("health", {}).get("usable") is True
    and isinstance(beads, dict)
    and beads.get("native_store_eligible") is True
    and len(matches) == 1
    and matches[0].get("suspended") is False
)
raise SystemExit(0 if healthy else 1)
PY
    then
      # `status.health.usable` proves the controller can open its stores, but a
      # cold Dolt provider may still make the first agent-side read stall. Do
      # one bounded read in both scopes before declaring the deployment ready.
      if "$gc_bin" --city "$city" bd list --json --limit 1 >/dev/null 2>>"$start_error" && \
          "$gc_bin" --city "$city" --rig "$rig_name" bd list --json --limit 1 >/dev/null 2>>"$start_error"; then
        started_usable=1
        break
      fi
    fi
    sleep 1
  done
  if [ "$started_usable" -ne 1 ]; then
    printf 'last gc status:\n' >&2
    python3 -m json.tool "$start_status" >&2 2>/dev/null || cat "$start_status" >&2
    if [ -s "$start_error" ]; then
      printf 'last gc status error:\n' >&2
      cat "$start_error" >&2
    fi
    rm -f "$start_status" "$start_error"
    die "started city did not become usable within ${start_timeout}s"
  fi
  started_supervisor_pid="$(python3 - "$start_status" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    status = json.load(handle)
pid = status.get("controller", {}).get("pid")
if not isinstance(pid, int) or isinstance(pid, bool) or pid <= 0:
    raise SystemExit("started city status has no positive controller pid")
print(pid)
PY
)"
  if [ "$toolchain_replacement" -eq 1 ] && \
      [ "$previous_supervisor_pid" -gt 0 ] && \
      [ "$started_supervisor_pid" -eq "$previous_supervisor_pid" ]; then
    die "replacement supervisor retained old pid $previous_supervisor_pid"
  fi
  rm -f "$start_status" "$start_error"
  python3 - "$city/.gc/settings.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    settings = json.load(handle)
if settings.get("remoteControlAtStartup") is not False:
    raise SystemExit("started city did not project remoteControlAtStartup=false")
PY
  if [ "$toolchain_replacement" -eq 1 ]; then
    write_marker ready
  fi
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
