#!/usr/bin/env bash
# preamble-exempt: standalone deployment teardown; intentionally runs outside a repo checkout.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  teardown.sh --city PATH [options]

Required:
  --city PATH       City previously created by deploy/gc/bootstrap.sh

Options:
  --gc-bin PATH     Gas City CLI; must match the managed-city marker
  --wait-timeout N  Bounded seconds to wait for supervisor shutdown (default: 60)
  -h, --help        Show this help

This quiesces the managed city without deleting its durable state. It stops the
private supervisor selected by <city>/.gc-home, reaps the city's private agent
socket, performs a final managed-Dolt stop, and refuses success while the
socket or scoped processes are still live.
EOF
}

die() {
  printf 'gc-agentops teardown: %s\n' "$*" >&2
  exit 1
}

city=""
gc_bin=""
wait_timeout=60

while [ "$#" -gt 0 ]; do
  case "$1" in
    --city)
      [ "$#" -ge 2 ] || die "--city requires a path"
      city="$2"
      shift 2
      ;;
    --gc-bin)
      [ "$#" -ge 2 ] || die "--gc-bin requires a path"
      gc_bin="$2"
      shift 2
      ;;
    --wait-timeout)
      [ "$#" -ge 2 ] || die "--wait-timeout requires a value"
      wait_timeout="$2"
      shift 2
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
[[ "$wait_timeout" =~ ^[1-9][0-9]*$ ]] || die "--wait-timeout must be a positive integer"
[ "$wait_timeout" -le 600 ] || die "--wait-timeout must be at most 600"
command -v python3 >/dev/null 2>&1 || die "python3 is required"

canonical_path() {
  python3 - "$1" <<'PY'
import os
import sys

print(os.path.realpath(os.path.expanduser(sys.argv[1])))
PY
}

city="$(canonical_path "$city")"
marker="$city/.gc/agentops-bootstrap.json"
[ -d "$city" ] || die "city directory does not exist: $city"
[ -f "$marker" ] || die "refusing unmanaged city without bootstrap marker: $city"

marker_toolchain="$(python3 - "$marker" "$city" <<'PY'
import json
import os
import sys

marker_path, expected_city = sys.argv[1:]
with open(marker_path, encoding="utf-8") as handle:
    marker = json.load(handle)
schema_version = marker.get("schema_version")
if schema_version not in {2, 3, 4, 5}:
    raise SystemExit("managed-city marker schema_version must be 2, 3, 4, or 5")
ao_bin = ""
ao_sha256 = ""
actual_city = os.path.realpath(os.path.expanduser(str(marker.get("city", ""))))
if actual_city != expected_city:
    raise SystemExit(f"managed-city marker city mismatch: {actual_city!r} != {expected_city!r}")
if schema_version == 2:
    gc_bin = marker.get("gc_bin")
    gc_sha256 = ""
    bd_bin = ""
    bd_sha256 = ""
else:
    toolchain = marker.get("toolchain")
    if not isinstance(toolchain, dict):
        raise SystemExit("managed-city marker has no toolchain")
    gc = toolchain.get("gc")
    bd = toolchain.get("bd")
    if not isinstance(gc, dict) or not isinstance(bd, dict):
        raise SystemExit("managed-city marker toolchain must contain gc and bd")
    gc_bin = gc.get("path")
    gc_sha256 = gc.get("sha256")
    bd_bin = bd.get("path")
    bd_sha256 = bd.get("sha256")
    ao = marker.get("ao_reducer", {}) if schema_version in {4, 5} else {}
    ao_bin = ao.get("path", "")
    ao_sha256 = ao.get("binary_sha256", "")
if not isinstance(gc_bin, str) or not gc_bin.strip():
    raise SystemExit("managed-city marker has no gc path")
for label, value in (("gc", gc_sha256), ("bd", bd_sha256)):
    if schema_version >= 3 and (
        not isinstance(value, str)
        or len(value) != 64
        or any(char not in "0123456789abcdef" for char in value)
    ):
        raise SystemExit(f"managed-city marker has invalid {label} sha256")
if schema_version >= 3 and (not isinstance(bd_bin, str) or not bd_bin.strip()):
    raise SystemExit("managed-city marker has no bd path")
if schema_version >= 4:
    if not isinstance(ao_bin, str) or not ao_bin.strip():
        raise SystemExit("managed-city marker has no ao reducer path")
    if not isinstance(ao_sha256, str) or len(ao_sha256) != 64 or any(char not in "0123456789abcdef" for char in ao_sha256):
        raise SystemExit("managed-city marker has invalid ao reducer sha256")
print("\t".join((
    os.path.realpath(os.path.expanduser(gc_bin)),
    gc_sha256,
    os.path.realpath(os.path.expanduser(bd_bin)) if bd_bin else "",
    bd_sha256,
    os.path.realpath(os.path.expanduser(ao_bin)) if ao_bin else "",
    ao_sha256,
)))
PY
)" || die "invalid managed-city marker: $marker"
IFS=$'\t' read -r marker_gc_bin marker_gc_sha256 marker_bd_bin marker_bd_sha256 marker_ao_bin marker_ao_sha256 <<<"$marker_toolchain"

# Detached Beads hooks are launched from the rig and may carry no city path in
# argv. Keep the marker-owned rig as a second exact process-scope identity so a
# hook cannot outlive a short city-only quiet window and restart managed Dolt.
marker_rig="$(python3 - "$marker" <<'PY'
import json
import os
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    marker = json.load(handle)
rig = marker.get("rig", "")
if rig:
    if not isinstance(rig, str):
        raise SystemExit("managed-city marker rig must be a path string")
    print(os.path.realpath(os.path.expanduser(rig)))
PY
)" || die "invalid managed-city rig in marker: $marker"

if [ -z "$gc_bin" ]; then
  gc_bin="$marker_gc_bin"
elif [[ "$gc_bin" == */* ]]; then
  gc_bin="$(canonical_path "$gc_bin")"
else
  die "--gc-bin must be an absolute or explicit path; PATH resolution is forbidden"
fi
[ "$gc_bin" = "$marker_gc_bin" ] || die "--gc-bin does not match managed-city marker: $gc_bin != $marker_gc_bin"
[ -x "$gc_bin" ] || die "Gas City CLI is not executable: $gc_bin"

if [ -n "$marker_gc_sha256" ]; then
  actual_gc_sha256="$(python3 - "$gc_bin" <<'PY'
import hashlib
import sys

digest = hashlib.sha256()
with open(sys.argv[1], "rb") as handle:
    for chunk in iter(lambda: handle.read(1024 * 1024), b""):
        digest.update(chunk)
print(digest.hexdigest())
PY
)"
  [ "$actual_gc_sha256" = "$marker_gc_sha256" ] || \
    die "managed gc binary digest mismatch: $actual_gc_sha256 != $marker_gc_sha256"
  [ -x "$marker_bd_bin" ] || die "managed paired bd binary is not executable: $marker_bd_bin"
  actual_bd_sha256="$(python3 - "$marker_bd_bin" <<'PY'
import hashlib
import sys

digest = hashlib.sha256()
with open(sys.argv[1], "rb") as handle:
    for chunk in iter(lambda: handle.read(1024 * 1024), b""):
        digest.update(chunk)
print(digest.hexdigest())
PY
)"
  [ "$actual_bd_sha256" = "$marker_bd_sha256" ] || \
    die "managed bd binary digest mismatch: $actual_bd_sha256 != $marker_bd_sha256"
  if [ -n "$marker_ao_bin" ]; then
    [ -x "$marker_ao_bin" ] || die "managed ao reducer is not executable: $marker_ao_bin"
    actual_ao_sha256="$(python3 - "$marker_ao_bin" <<'PY'
import hashlib
import sys

digest = hashlib.sha256()
with open(sys.argv[1], "rb") as handle:
    for chunk in iter(lambda: handle.read(1024 * 1024), b""):
        digest.update(chunk)
print(digest.hexdigest())
PY
)"
    [ "$actual_ao_sha256" = "$marker_ao_sha256" ] || \
      die "managed ao reducer binary digest mismatch: $actual_ao_sha256 != $marker_ao_sha256"
  fi
fi

# A teardown must resolve the same private supervisor and store namespace as
# bootstrap. --city selects city commands; GC_HOME selects supervisor identity.
while IFS='=' read -r key _; do
  case "$key" in
    GC_*|BEADS_DOLT_*|DOLT_*) unset "$key" ;;
  esac
done < <(env)
unset BEADS_BACKEND BEADS_DIR BD_DB BD_DATABASE BD_DOLT_PORT BD_HOME || true
export GC_HOME="$city/.gc-home"
export GC_ISOLATED=1
export GC_BIN="$gc_bin"
export OTEL_SDK_DISABLED=true

status_file="$(mktemp "${TMPDIR:-/tmp}/gc-agentops-teardown-status.XXXXXX")"
hook_fence_file="$(mktemp "${TMPDIR:-/tmp}/gc-agentops-teardown-hooks.XXXXXX")"
hook_fence_active=0

restore_gc_hooks() {
  [ "$hook_fence_active" -eq 1 ] || return 0
  python3 - "$hook_fence_file" <<'PY'
import hashlib
import json
import os
import stat
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    entries = json.load(handle)
for entry in entries:
    path = entry["path"]
    info = os.lstat(path)
    if not stat.S_ISREG(info.st_mode) or stat.S_ISLNK(info.st_mode):
        raise SystemExit(f"GC hook changed type while fenced: {path}")
    with open(path, "rb") as handle:
        digest = hashlib.sha256(handle.read()).hexdigest()
    if digest != entry["sha256"]:
        raise SystemExit(f"GC hook changed bytes while fenced: {path}")
for entry in entries:
    os.chmod(entry["path"], entry["mode"])
PY
  hook_fence_active=0
}

cleanup() {
  if [ "$hook_fence_active" -eq 1 ]; then
    restore_gc_hooks || true
  fi
  rm -f "$status_file" "$hook_fence_file"
}
trap cleanup EXIT

# GC's installed Beads hooks intentionally detach their event/autoclose chain.
# Fence only exact GC-stamped hooks before shutdown so closing an in-flight
# tracking bead cannot schedule new work after the supervisor socket is gone.
# Existing detached helpers are still drained below, and original modes are
# restored byte-for-byte after quiescence so a later bootstrap/start is intact.
python3 - "$hook_fence_file" "$city" "$marker_rig" <<'PY'
import hashlib
import json
import os
import stat
import sys

snapshot, city, rig = sys.argv[1:]
entries = []
for root in (city, rig):
    if not root:
        continue
    hooks = os.path.join(root, ".beads", "hooks")
    for name in ("on_create", "on_update", "on_close"):
        path = os.path.join(hooks, name)
        if not os.path.exists(path):
            continue
        info = os.lstat(path)
        if not stat.S_ISREG(info.st_mode) or stat.S_ISLNK(info.st_mode):
            raise SystemExit(f"refusing non-regular managed hook: {path}")
        with open(path, "rb") as handle:
            raw = handle.read()
        if b"# gc-hook-stamp:" not in raw or b"# Installed by gc" not in raw:
            raise SystemExit(f"refusing to fence non-GC hook: {path}")
        entries.append({
            "path": path,
            "mode": stat.S_IMODE(info.st_mode),
            "sha256": hashlib.sha256(raw).hexdigest(),
        })
with open(snapshot, "w", encoding="utf-8") as handle:
    json.dump(entries, handle, sort_keys=True, separators=(",", ":"))
    handle.write("\n")
for entry in entries:
    os.chmod(entry["path"], entry["mode"] & ~0o111)
PY
hook_fence_active=1

supervisor_running() {
  "$gc_bin" supervisor status --json >"$status_file"
  python3 - "$status_file" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    status = json.load(handle)
raise SystemExit(0 if status.get("running") is True else 1)
PY
}

if supervisor_running; then
	"$gc_bin" supervisor stop --wait --wait-timeout "${wait_timeout}s"
fi

# A canceled health/recovery order may have reached managed-Dolt startup before
# its shell process group was terminated. The GC watchdog activation contract
# prevents that race; this final idempotent stop is defense in depth and makes
# teardown safe for cities created by an older binary too.
"$gc_bin" dolt-state stop-managed --city "$city" >/dev/null

if supervisor_running; then
  die "private supervisor still reports running after shutdown"
fi

tmux_socket="$(python3 - "$city/city.toml" <<'PY'
import sys
import tomllib

with open(sys.argv[1], "rb") as handle:
    config = tomllib.load(handle)
print(config.get("session", {}).get("socket", ""))
PY
)"
if [ -n "$tmux_socket" ] && command -v tmux >/dev/null 2>&1; then
  if tmux_sessions="$(tmux -L "$tmux_socket" list-sessions 2>&1)"; then
	# This socket is path-derived, private to the marker-verified managed city,
	# and therefore safe to reap as a deterministic crash-recovery fallback.
	tmux -L "$tmux_socket" kill-server >/dev/null 2>&1 || true
	if tmux_sessions="$(tmux -L "$tmux_socket" list-sessions 2>&1)"; then
	  die "tmux socket $tmux_socket still has live sessions after kill-server: $tmux_sessions"
	fi
  fi
fi

find_residual_processes() {
  python3 - "$city" "$marker_rig" <<'PY'
import os
import subprocess
import sys

scope_roots = [root for root in sys.argv[1:] if root]


def process_rows(arguments):
    rows = []
    for line in subprocess.check_output(arguments, text=True).splitlines():
        fields = line.strip().split(None, 2)
        if len(fields) != 3:
            continue
        try:
            pid, ppid = int(fields[0]), int(fields[1])
        except ValueError:
            continue
        rows.append((pid, ppid, fields[2]))
    return rows


def process_cwds():
    result = {}
    proc_root = "/proc"
    if os.path.isdir(proc_root):
        for name in os.listdir(proc_root):
            if not name.isdigit():
                continue
            try:
                result[int(name)] = os.path.realpath(os.readlink(os.path.join(proc_root, name, "cwd")))
            except OSError:
                pass
        return result
    try:
        lines = subprocess.check_output(
            ["lsof", "-a", "-d", "cwd", "-Fn", "-Fp"],
            text=True,
            stderr=subprocess.DEVNULL,
        ).splitlines()
    except (FileNotFoundError, subprocess.CalledProcessError):
        return result
    pid = None
    for line in lines:
        if line.startswith("p") and line[1:].isdigit():
            pid = int(line[1:])
        elif pid is not None and line.startswith("n"):
            result[pid] = os.path.realpath(line[1:])
    return result


def within(path, root):
    if not path or not root:
        return False
    try:
        return os.path.commonpath((path, root)) == root
    except ValueError:
        return False


# Match over argv plus environment because a GC maintenance helper can carry
# city identity only in GC_CITY/GC_CITY_PATH. Keep an argv-only projection for
# diagnostics so teardown never prints credentials inherited through the
# process environment.
expanded_rows = process_rows(["ps", "eww", "-eo", "pid=,ppid=,command="])
plain_rows = process_rows(["ps", "-eo", "pid=,ppid=,command="])
plain_commands = {pid: command for pid, _ppid, command in plain_rows}
parents = {pid: ppid for pid, ppid, _ in expanded_rows}
cwd_by_pid = process_cwds()
ancestors = set()
pid = os.getpid()
while pid > 0 and pid not in ancestors:
    ancestors.add(pid)
    pid = parents.get(pid, 0)


def descends_from(pid, ancestor):
    seen = set()
    while pid > 0 and pid not in seen:
        if pid == ancestor:
            return True
        seen.add(pid)
        pid = parents.get(pid, 0)
    return False


rows = []
for pid, ppid, expanded_command in expanded_rows:
    # `ps` itself is a short-lived child of this census process and inherits
    # the city environment; exclude this whole diagnostic subtree.
    if pid in ancestors or descends_from(pid, os.getpid()):
        continue
    rows.append((pid, ppid, expanded_command, plain_commands.get(pid, "<command unavailable>")))

for pid, ppid, expanded_command, plain_command in rows:
    cwd = cwd_by_pid.get(pid, "")
    if any(root in expanded_command or within(cwd, root) for root in scope_roots):
        suffix = f" [cwd={cwd}]" if cwd else ""
        print(f"{pid}\t{ppid}\t{plain_command}{suffix}")
PY
}

# Supervisor shutdown terminates its process groups, but a canceled provider
# helper can need a short final interval to reap its own child after the
# supervisor socket is already gone. Match both the city and marker-owned rig,
# then require five consecutive quiet observations so a short argv/env handoff
# cannot be mistaken for quiescence.
# If a late scoped helper appears, repeat GC's supported idempotent Dolt stop;
# never signal an unverified PID here.
residual_deadline="$(( $(date +%s) + wait_timeout ))"
quiet_observations=0
required_quiet_observations=5
while :; do
  residual_processes="$(find_residual_processes)"
  if [ -z "$residual_processes" ]; then
    quiet_observations="$(( quiet_observations + 1 ))"
    [ "$quiet_observations" -lt "$required_quiet_observations" ] || break
  else
    quiet_observations=0
    "$gc_bin" dolt-state stop-managed --city "$city" >/dev/null
  fi
  if [ "$(date +%s)" -ge "$residual_deadline" ]; then
    die "$(printf 'city-scoped processes survived teardown:\n%s' "$residual_processes")"
  fi
  sleep 1
done

restore_gc_hooks || die "failed to restore GC-stamped Beads hook modes"

printf 'Gas City stopped cleanly: %s\n' "$city"
