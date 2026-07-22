#!/usr/bin/env bash
# preamble-exempt: standalone managed-city invoker.
set -euo pipefail

die() { printf 'gc-agentops invoke: %s\n' "$*" >&2; exit 1; }
usage() {
  cat <<'EOF'
Usage:
  invoke.sh --city PATH feed BEAD
  invoke.sh --city PATH status
  invoke.sh --city PATH doctor
  invoke.sh --city PATH -- GC_COMMAND [ARG...]
EOF
}
city=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --city) city="${2:?--city requires a path}"; shift 2; break ;;
    -h|--help) usage; exit 0 ;;
    *) die "expected --city PATH" ;;
  esac
done
[ -n "$city" ] && [ "$#" -gt 0 ] || { usage; exit 2; }
city="$(python3 - "$city" <<'PY'
import os,sys
print(os.path.realpath(sys.argv[1]))
PY
)"
marker="$city/.gc/agentops-bootstrap.json"
[ -f "$marker" ] || die "managed bootstrap marker is missing: $marker"
identity="$(python3 - "$marker" "$city" <<'PY'
import hashlib,json,os,sys
m=json.load(open(sys.argv[1],encoding="utf-8"))
if m.get("schema_version") != 1 or m.get("state") != "ready" or m.get("city") != sys.argv[2]: raise SystemExit("invalid managed city marker")
def sha(path):
 h=hashlib.sha256()
 with open(path,"rb") as f:
  for b in iter(lambda:f.read(1024*1024),b""): h.update(b)
 return h.hexdigest()
gc=m["toolchain"]["gc"]
if sha(gc["path"]) != gc["sha256"]: raise SystemExit("managed gc binary changed")
def tree_sha(root):
 h=hashlib.sha256()
 for parent,dirs,files in os.walk(root):
  dirs.sort()
  for name in sorted(files):
   path=os.path.join(parent,name); rel=os.path.relpath(path,root)
   h.update(rel.encode()+b"\0"+open(path,"rb").read()+b"\0")
 return h.hexdigest()
if tree_sha(os.path.dirname(m["pack_snapshot"])) != m.get("pack_sha256"): raise SystemExit("managed pack snapshot changed")
if sha(os.path.join(m["city"],"city.toml")) != m.get("city_config_sha256"): raise SystemExit("managed city config changed")
t=m["telemetry"]
print("\x1f".join((gc["path"],t["metrics_url"],t["logs_url"],str(t.get("sdk_disabled",False)).lower())))
PY
)" || die "invalid managed city identity"
IFS=$'\x1f' read -r gc_bin metrics_url logs_url otel_disabled <<<"$identity"
export GC_HOME="$city/.gc-home"
gc_bin_dir="$(dirname "$gc_bin")"
PATH="$gc_bin_dir:$PATH"; export PATH
export GC_OTEL_METRICS_URL="$metrics_url" GC_OTEL_LOGS_URL="$logs_url" OTEL_EXPORTER_OTLP_ENDPOINT="" OTEL_SDK_DISABLED="$otel_disabled"
command="$1"; shift
cd "$city"
case "$command" in
  feed)
    [ "$#" -eq 1 ] || die "feed requires exactly one bead id"
    [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,255}$ ]] || die "unsafe bead id"
    exec "$gc_bin" sling agentops.mayor "$1" --nudge --json
    ;;
  status)
    [ "$#" -eq 0 ] || die "status accepts no arguments"
    exec "$gc_bin" status --json
    ;;
  doctor)
    [ "$#" -eq 0 ] || die "doctor accepts no arguments"
    exec "$gc_bin" doctor --json
    ;;
  --)
    [ "$#" -gt 0 ] || die "a GC command is required after --"
    exec "$gc_bin" "$@"
    ;;
  *) die "unknown operation: $command" ;;
esac
