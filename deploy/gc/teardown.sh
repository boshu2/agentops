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
if schema_version not in {2, 3}:
    raise SystemExit("managed-city marker schema_version must be 2 or 3")
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
if not isinstance(gc_bin, str) or not gc_bin.strip():
    raise SystemExit("managed-city marker has no gc path")
for label, value in (("gc", gc_sha256), ("bd", bd_sha256)):
    if schema_version == 3 and (
        not isinstance(value, str)
        or len(value) != 64
        or any(char not in "0123456789abcdef" for char in value)
    ):
        raise SystemExit(f"managed-city marker has invalid {label} sha256")
if schema_version == 3 and (not isinstance(bd_bin, str) or not bd_bin.strip()):
    raise SystemExit("managed-city marker has no bd path")
print("\t".join((
    os.path.realpath(os.path.expanduser(gc_bin)),
    gc_sha256,
    os.path.realpath(os.path.expanduser(bd_bin)) if bd_bin else "",
    bd_sha256,
)))
PY
)" || die "invalid managed-city marker: $marker"
IFS=$'\t' read -r marker_gc_bin marker_gc_sha256 marker_bd_bin marker_bd_sha256 <<<"$marker_toolchain"

if [ -z "$gc_bin" ]; then
  gc_bin="$marker_gc_bin"
elif [[ "$gc_bin" == */* ]]; then
  gc_bin="$(canonical_path "$gc_bin")"
else
  gc_bin="$(command -v "$gc_bin" || true)"
  [ -n "$gc_bin" ] || die "Gas City CLI not found on PATH"
  gc_bin="$(canonical_path "$gc_bin")"
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
trap 'rm -f "$status_file"' EXIT

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
  python3 - "$city" <<'PY'
import os
import subprocess
import sys

city = sys.argv[1]


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


# Match over argv plus environment because a GC maintenance helper can carry
# city identity only in GC_CITY/GC_CITY_PATH. Keep an argv-only projection for
# diagnostics so teardown never prints credentials inherited through the
# process environment.
expanded_rows = process_rows(["ps", "eww", "-axo", "pid=,ppid=,command="])
plain_rows = process_rows(["ps", "-axo", "pid=,ppid=,command="])
plain_commands = {pid: command for pid, _ppid, command in plain_rows}
parents = {pid: ppid for pid, ppid, _ in expanded_rows}
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
    if city in expanded_command:
        print(f"{pid}\t{ppid}\t{plain_command}")
PY
}

# Supervisor shutdown terminates its process groups, but a canceled provider
# helper can need a short final interval to reap its own child after the
# supervisor socket is already gone. Require three consecutive quiet
# observations so a short argv/env handoff cannot be mistaken for quiescence.
# If a late scoped helper appears, repeat GC's supported idempotent Dolt stop;
# never signal an unverified PID here.
residual_deadline="$(( $(date +%s) + wait_timeout ))"
quiet_observations=0
while :; do
  residual_processes="$(find_residual_processes)"
  if [ -z "$residual_processes" ]; then
    quiet_observations="$(( quiet_observations + 1 ))"
    [ "$quiet_observations" -lt 3 ] || break
  else
    quiet_observations=0
    "$gc_bin" dolt-state stop-managed --city "$city" >/dev/null
  fi
  if [ "$(date +%s)" -ge "$residual_deadline" ]; then
    die "$(printf 'city-scoped processes survived teardown:\n%s' "$residual_processes")"
  fi
  sleep 1
done

printf 'Gas City stopped cleanly: %s\n' "$city"
