#!/usr/bin/env bash
# preamble-exempt: standalone managed-city teardown.
set -euo pipefail

die() { printf 'gc-agentops teardown: %s\n' "$*" >&2; exit 1; }
city=""; wait_timeout=60
while [ "$#" -gt 0 ]; do
  case "$1" in
    --city) city="${2:?--city requires a path}"; shift 2 ;;
    --wait-timeout) wait_timeout="${2:?--wait-timeout requires seconds}"; shift 2 ;;
    -h|--help) printf '%s\n' 'Usage: teardown.sh --city PATH [--wait-timeout SECONDS]'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done
[ -n "$city" ] || die "--city is required"
[[ "$wait_timeout" =~ ^[1-9][0-9]*$ ]] || die "wait timeout must be positive"
city="$(python3 - "$city" <<'PY'
import os,sys
print(os.path.realpath(sys.argv[1]))
PY
)"
marker="$city/.gc/agentops-bootstrap.json"
[ -f "$marker" ] || die "managed bootstrap marker is missing"
identity="$(python3 - "$marker" "$city" <<'PY'
import json,sys
m=json.load(open(sys.argv[1],encoding="utf-8"))
if m.get("schema_version") != 1 or m.get("city") != sys.argv[2]: raise SystemExit("invalid managed city marker")
t=m["telemetry"]
print(f'{m["toolchain"]["gc"]["path"]}\t{m["supervisor_port"]}\t{str(t.get("sdk_disabled",False)).lower()}')
PY
)" || die "invalid managed city marker"
IFS=$'\t' read -r gc_bin port otel_disabled <<<"$identity"
[ -x "$gc_bin" ] || die "managed gc binary is unavailable"
gc_bin_dir="$(dirname "$gc_bin")"
GC_HOME="$city/.gc-home"; PATH="$gc_bin_dir:$PATH"; export GC_HOME PATH
export OTEL_SDK_DISABLED="$otel_disabled"
"$gc_bin" --city "$city" stop >/dev/null 2>&1 || true
"$gc_bin" supervisor stop --wait --wait-timeout "${wait_timeout}s" >/dev/null 2>&1 || true
python3 - "$port" <<'PY'
import socket,sys
s=socket.socket(); s.settimeout(.25)
try: s.connect(("127.0.0.1",int(sys.argv[1])))
except OSError: raise SystemExit(0)
raise SystemExit("private supervisor still accepts connections")
PY
python3 - "$city" "$$" "$PPID" <<'PY'
import os,subprocess,sys
root=os.path.realpath(sys.argv[1]); excluded={os.getpid(), *(int(x) for x in sys.argv[2:])}
rows=subprocess.check_output(["ps","-axo","pid=,command="],text=True).splitlines()
live=[]
for row in rows:
    fields=row.strip().split(None,1)
    if len(fields)==2 and int(fields[0]) not in excluded and root in fields[1]: live.append(row.strip())
if live: raise SystemExit("managed city processes remain:\n"+"\n".join(live))
PY
printf 'Gas City stopped cleanly: %s\n' "$city"
