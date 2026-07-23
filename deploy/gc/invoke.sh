#!/usr/bin/env bash
# preamble-exempt: standalone managed-city invoker.
set -euo pipefail

die() { printf 'gc-agentops invoke: %s\n' "$*" >&2; exit 1; }
usage() {
  cat <<'EOF'
Usage:
  invoke.sh --city PATH feed BEAD
  invoke.sh --city PATH create TITLE [-d DESCRIPTION] [--json]
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
bd=m["toolchain"]["bd"]
print("\x1f".join((gc["path"],t["metrics_url"],t["logs_url"],str(t.get("sdk_disabled",False)).lower(),m["rig"],m["rig_name"],m["base_ref"],m["worktree_root"],bd["path"],bd["sha256"],m["bead_database"])))
PY
)" || die "invalid managed city identity"
IFS=$'\x1f' read -r gc_bin metrics_url logs_url otel_disabled rig rig_name base_ref worktree_root bd_bin bd_sha bead_database <<<"$identity"
GC_BIN="$gc_bin"
export GC_BIN
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
    worktree_script="$city/.gc/scripts/agentops-worktree"
    [ -x "$worktree_script" ] || die "managed worktree helper is missing: $worktree_script"
    worktree_receipt="$("$worktree_script" prepare --repo "$rig" --root "$worktree_root" --bead "$1" --base-ref "$base_ref")" || die "cannot prepare an isolated worktree for $1"
    work_dir="$(printf '%s' "$worktree_receipt" | python3 -c 'import json,sys; print(json.load(sys.stdin)["worktree"])')" || die "worktree helper returned an unreadable receipt"
    # Official single-bead intake: home the source bead in the rig store and
    # attach the native formula to the rig-scoped planner. No city-scoped agent
    # ever claims this rig-homed bead, so the v1.3.5 cross-store claim bug is not
    # exercised. The Mayor is an optional monitor and is not on this path.
    exec "$gc_bin" sling "$rig_name/agentops.plan-reviewer" "$1" \
      --on agentops-experiment --nudge --json \
      --var work_dir="$work_dir" \
      --var plan_target="$rig_name/agentops.plan-reviewer" \
      --var implement_target="$rig_name/agentops.implementer" \
      --var validate_target="$rig_name/agentops.validator" \
      --var refiner_target="$rig_name/agentops.refiner"
    ;;
  create)
    [ "$#" -ge 1 ] || { usage; exit 2; }
    title="$1"; shift
    case "$title" in -*) die "create requires a title as the first argument" ;; esac
    description=""; json_mode=0
    while [ "$#" -gt 0 ]; do
      case "$1" in
        -d|--description) description="${2:?-d requires a value}"; shift 2 ;;
        --json) json_mode=1; shift ;;
        *) die "unknown create argument: $1" ;;
      esac
    done
    [ -x "$bd_bin" ] || die "managed bd binary is missing: $bd_bin"
    python3 - "$bd_bin" "$bd_sha" <<'PY' || die "managed bd binary changed"
import hashlib,sys
h=hashlib.sha256()
with open(sys.argv[1],"rb") as f:
 for b in iter(lambda:f.read(1024*1024),b""): h.update(b)
raise SystemExit(0 if h.hexdigest()==sys.argv[2] else 1)
PY
    port_file="$city/.beads/dolt-server.port"
    [ -f "$port_file" ] || die "managed Dolt port file is missing: $port_file"
    managed_port="$(tr -d '[:space:]' <"$port_file")"
    { [[ "$managed_port" =~ ^[1-9][0-9]{0,4}$ ]] && [ "$managed_port" -le 65535 ]; } || die "managed Dolt port is invalid"
    # Mirror the exact bd environment the bootstrap uses for the managed rig
    # store: the single running Dolt server, no auto-start, no export churn.
    bd_env=(env BD_NON_INTERACTIVE=1 BD_EXPORT_AUTO=false BD_BACKUP_ENABLED=false
      BEADS_DOLT_AUTO_START=0 BEADS_DOLT_SERVER_MODE=1 BEADS_DOLT_SERVER_HOST=127.0.0.1
      BEADS_DOLT_SERVER_PORT="$managed_port" BEADS_DOLT_SERVER_DATABASE="$bead_database")
    create_args=(-C "$rig" create --title "$title" --type task --json)
    [ -n "$description" ] && create_args+=(-d "$description")
    receipt="$("${bd_env[@]}" "$bd_bin" "${create_args[@]}")" || die "bd create failed"
    if [ "$json_mode" -eq 1 ]; then
      printf '%s\n' "$receipt"
    else
      printf '%s' "$receipt" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' || die "bd create returned an unreadable receipt"
    fi
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
