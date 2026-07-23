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
# The running pack (this script's own directory) is the trust anchor: its
# toolchain.lock.json pins the accepted pair and its worktree.sh is the
# canonical helper. Expected digests are verified against these, never against
# the mutable managed-city marker (which carries paths for convenience only).
script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
lock="$script_dir/toolchain.lock.json"
worktree_src="$script_dir/worktree.sh"
[ -f "$lock" ] || die "pack toolchain lock is missing: $lock"
[ -f "$worktree_src" ] || die "pack worktree helper source is missing: $worktree_src"

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
identity="$(python3 - "$marker" "$city" "$lock" "$worktree_src" <<'PY'
import hashlib,json,os,sys
marker_path,city,lock_path,worktree_src=sys.argv[1:5]
m=json.load(open(marker_path,encoding="utf-8"))
if m.get("schema_version") != 1 or m.get("state") != "ready" or m.get("city") != city: raise SystemExit("invalid managed city marker")
def sha(path):
 h=hashlib.sha256()
 with open(path,"rb") as f:
  for b in iter(lambda:f.read(1024*1024),b""): h.update(b)
 return h.hexdigest()
# Anchor 1: the running pack's own lock pins the accepted source commits.
lock=json.load(open(lock_path,encoding="utf-8"))
pairs=lock.get("accepted_pairs") or []
if lock.get("schema_version") != 1 or len(pairs) != 1: raise SystemExit("pack toolchain lock is malformed")
gc_commit=pairs[0]["gc"]["source_commit"]; bd_commit=pairs[0]["bd"]["source_commit"]
# Locate the toolchain receipt from the marker's gc path, but trust the RECEIPT
# (cross-checked to the lock), never the marker, for binary paths and digests.
gc_path=os.path.realpath(m["toolchain"]["gc"]["path"])
toolchain_root=os.path.dirname(os.path.dirname(gc_path))
r=json.load(open(os.path.join(toolchain_root,"toolchain.json"),encoding="utf-8"))
if r.get("schema_version") != 2 or set(r.get("runtime",{})) != {"gc","bd"}: raise SystemExit("toolchain receipt is malformed")
if r.get("pair",{}).get("status") != "qualified": raise SystemExit("toolchain receipt pair is not qualified")
if r["pair"]["gc"]["source_commit"] != gc_commit or r["pair"]["bd"]["source_commit"] != bd_commit: raise SystemExit("toolchain receipt does not match the pack lock")
def anchored(name):
 spec=r["runtime"][name]
 return os.path.realpath(os.path.join(toolchain_root,spec["path"])),spec["sha256"]
gc_anchor,gc_anchor_sha=anchored("gc")
if gc_anchor != gc_path: raise SystemExit("marked gc path is not the receipt gc binary")
if sha(gc_path) != gc_anchor_sha: raise SystemExit("gc binary does not match the toolchain receipt")
bd_path,bd_sha=anchored("bd")
if not os.path.exists(bd_path): raise SystemExit("receipt bd binary is missing")
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
# Anchor 2: the installed worktree helper must match the running pack source.
worktree_expected=sha(worktree_src)
t=m["telemetry"]
print("\x1f".join((gc_path,t["metrics_url"],t["logs_url"],str(t.get("sdk_disabled",False)).lower(),m["rig"],m["rig_name"],m["base_ref"],m["worktree_root"],bd_path,bd_sha,m["bead_database"],worktree_expected)))
PY
)" || die "invalid managed city identity"
IFS=$'\x1f' read -r gc_bin metrics_url logs_url otel_disabled rig rig_name base_ref worktree_root bd_bin bd_sha bead_database worktree_expected_sha <<<"$identity"
GC_BIN="$gc_bin"
export GC_BIN
export GC_HOME="$city/.gc-home"
gc_bin_dir="$(dirname "$gc_bin")"
PATH="$gc_bin_dir:$PATH"; export PATH
export GC_OTEL_METRICS_URL="$metrics_url" GC_OTEL_LOGS_URL="$logs_url" OTEL_EXPORTER_OTLP_ENDPOINT="" OTEL_SDK_DISABLED="$otel_disabled"

# Verify a file's sha256 against an expected digest (from the trust anchor).
verify_sha() {
  python3 - "$1" "$2" <<'PY'
import hashlib,sys
h=hashlib.sha256()
with open(sys.argv[1],"rb") as f:
 for b in iter(lambda:f.read(1024*1024),b""): h.update(b)
raise SystemExit(0 if h.hexdigest()==sys.argv[2] else 1)
PY
}
# Verify the pinned bd binary against the receipt-anchored digest.
verify_bd() {
  [ -x "$bd_bin" ] || die "managed bd binary is missing: $bd_bin"
  verify_sha "$bd_bin" "$bd_sha" || die "managed bd binary does not match the toolchain receipt"
}
# Run the pinned bd against the single managed rig Dolt server, mirroring the
# exact environment the bootstrap uses.
run_bd() {
  local port_file="$city/.beads/dolt-server.port" port
  [ -f "$port_file" ] || die "managed Dolt port file is missing: $port_file"
  port="$(tr -d '[:space:]' <"$port_file")"
  { [[ "$port" =~ ^[1-9][0-9]{0,4}$ ]] && [ "$port" -le 65535 ]; } || die "managed Dolt port is invalid"
  env BD_NON_INTERACTIVE=1 BD_EXPORT_AUTO=false BD_BACKUP_ENABLED=false \
    BEADS_DOLT_AUTO_START=0 BEADS_DOLT_SERVER_MODE=1 BEADS_DOLT_SERVER_HOST=127.0.0.1 \
    BEADS_DOLT_SERVER_PORT="$port" BEADS_DOLT_SERVER_DATABASE="$bead_database" \
    "$bd_bin" -C "$rig" "$@"
}

command="$1"; shift
cd "$city"
case "$command" in
  feed)
    [ "$#" -eq 1 ] || die "feed requires exactly one bead id"
    [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,255}$ ]] || die "unsafe bead id"
    # Guard: on GC v1.3.5 `gc sling` treats an unknown id as inline text and
    # auto-creates a task, silently dispatching the wrong work. Require the bead
    # to already exist in the rig store with an exact id match before slinging.
    verify_bd
    lookup="$(run_bd show "$1" --json 2>/dev/null)" || die "bead not found in the rig store: $1"
    printf '%s' "$lookup" | python3 -c 'import json,sys
data=json.load(sys.stdin)
if not isinstance(data,list) or len(data) != 1 or data[0].get("id") != sys.argv[1]: raise SystemExit(1)' "$1" || die "bead lookup did not return an exact single match: $1"
    worktree_script="$city/.gc/scripts/agentops-worktree"
    [ -x "$worktree_script" ] || die "managed worktree helper is missing: $worktree_script"
    verify_sha "$worktree_script" "$worktree_expected_sha" || die "managed worktree helper does not match the pack source"
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
    verify_bd
    create_args=(create --title "$title" --type task --json)
    [ -n "$description" ] && create_args+=(-d "$description")
    receipt="$(run_bd "${create_args[@]}")" || die "bd create failed"
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
