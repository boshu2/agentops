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
  invoke.sh --city PATH dashboard [--open]
  invoke.sh --city PATH packs list|refresh|search|show|maintainers [ARG...]
  invoke.sh --city PATH mayor start
  invoke.sh --city PATH mayor status
  invoke.sh --city PATH mayor tell "MESSAGE"
  invoke.sh --city PATH mayor attach-hint
  invoke.sh --city PATH -- GC_COMMAND [ARG...]

Gas City 1.4's store-scoped control dispatcher propels formula runs. The Mayor
is an on-demand inspection and manual-dispatch door; `mayor tell` messages must
reference BEAD IDS (e.g. "dispatch testrig-12"), never describe work. `packs`
exposes the registry catalog, including the official `gascity` workflow used by
Maintainer City. Author intent with `create`, then start it with `feed`.
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

# The on-demand city-scoped Mayor is the `agentops`-bound named session.
mayor_alias="agentops.mayor"

# Report the Mayor session state and/or the raw tmux human-door line. Tri-state
# fail-CLOSED (helper death must not read as "no finding"): (a) the gc query
# failed or returned malformed output -> nonzero exit + "status unavailable"; (b)
# the query succeeded but no Mayor session exists -> "not started", exit 0; (c)
# the session is present -> report it. `gc session list --json` carries the tmux
# session_name; the socket is derived exactly as the bootstrap templates it
# (agentops-<sha256(realpath city)[:20]>). Older fallback rows may omit
# running/last_output; an ABSENT field reports as "unknown", never false.
# Returns nonzero only in case (a); the caller propagates that exit code.
mayor_report() {
  local report_mode="$1" listing rc err_tail tmp_err
  tmp_err="$(mktemp "${TMPDIR:-/tmp}/mayor-err.XXXXXX")"
  if listing="$("$gc_bin" session list --state all --json 2>"$tmp_err")"; then
    rc=0
  else
    rc=$?
  fi
  err_tail="$(tail -c 400 "$tmp_err" 2>/dev/null | tr '\n' ' ' || true)"
  rm -f "$tmp_err"
  MAYOR_LISTING="$listing" MAYOR_RC="$rc" MAYOR_ERR="$err_tail" \
    python3 - "$mayor_alias" "$city" "$report_mode" <<'PY'
import hashlib, json, os, sys
alias, city, report_mode = sys.argv[1:4]
socket = "agentops-" + hashlib.sha256(city.encode()).hexdigest()[:20]
raw = os.environ.get("MAYOR_LISTING", "")
gc_rc = os.environ.get("MAYOR_RC", "0")
err = (os.environ.get("MAYOR_ERR", "") or "").strip()

def unavailable(reason):
    detail = (err or reason).strip()
    sys.stderr.write("status unavailable: %s\n" % (detail or "gc session list failed"))
    sys.exit(3)

# (a) fail-closed: the gc query itself failed, or its output is not parseable.
if gc_rc != "0":
    unavailable("gc session list exited %s" % gc_rc)
entry = None
if raw.strip():
    try:
        data = json.loads(raw)
    except (ValueError, TypeError):
        unavailable("gc session list returned malformed JSON")
    sessions = data.get("sessions", []) if isinstance(data, dict) else data
    if not isinstance(sessions, list):
        unavailable("gc session list JSON has no session array")
    for item in sessions:
        if isinstance(item, dict) and (item.get("alias") == alias or item.get("template") == "mayor"):
            entry = item
            break
# Empty stdout on a successful call is a legitimate "no sessions" answer.
session_name = (entry or {}).get("session_name") or ""

def field(key):
    # Absent field -> "unknown" (never coerce a missing bool to false).
    return "unknown" if (entry is None or entry.get(key) is None) else entry.get(key)

if report_mode == "hint":
    if session_name:
        print("tmux -L %s attach -t %s" % (socket, session_name))
    else:
        print("# Mayor not started. Run: invoke.sh --city %s mayor start" % city)
        print("# Then: invoke.sh --city %s mayor status  (prints the tmux attach line)" % city)
    sys.exit(0)
# (b) query succeeded, no Mayor session present.
if entry is None:
    print("mayor (%s): not started" % alias)
    print("  start it:   invoke.sh mayor start")
    print("  socket:     %s (tmux session name appears here once started)" % socket)
    sys.exit(0)
# (c) session present.
print("mayor (%s):" % alias)
print("  state:       %s" % (entry.get("state") or "unknown"))
print("  running:     %s" % field("running"))
print("  attached:    %s" % field("attached"))
print("  last_active: %s" % field("last_active"))
last = entry.get("last_output")
if last:
    print("  last_output: %s" % str(last).strip().replace("\n", " ")[:200])
if session_name:
    print("  human door:  tmux -L %s attach -t %s" % (socket, session_name))
PY
}

command="$1"; shift
cd "$city"
case "$command" in
  feed)
    [ "$#" -eq 1 ] || die "feed requires exactly one bead id"
    [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,255}$ ]] || die "unsafe bead id"
    # Require an exact existing rig-store id before slinging. This preserves the
    # caller-owned intent boundary even if GC accepts inline work on this surface.
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
    # attach the native formula to the rig-scoped planner. The v1.4 scoped
    # control dispatcher advances the graph; the Mayor is not on this path.
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
  dashboard)
    if [ "$#" -eq 0 ]; then
      exec "$gc_bin" dashboard --no-open
    fi
    [ "$#" -eq 1 ] && [ "$1" = "--open" ] || die "dashboard accepts only --open"
    exec "$gc_bin" dashboard
    ;;
  packs)
    [ "$#" -ge 1 ] || die "packs requires list, refresh, search, show, or maintainers"
    sub="$1"; shift
    case "$sub" in
      list)
        [ "$#" -eq 0 ] || die "packs list accepts no arguments"
        exec "$gc_bin" pack registry list
        ;;
      refresh)
        [ "$#" -le 1 ] || die "packs refresh accepts at most one registry name"
        exec "$gc_bin" pack registry refresh "$@"
        ;;
      search)
        exec "$gc_bin" pack registry search "$@"
        ;;
      show)
        [ "$#" -eq 1 ] || die "packs show requires one pack name"
        exec "$gc_bin" pack registry show "$1"
        ;;
      maintainers)
        [ "$#" -eq 0 ] || die "packs maintainers accepts no arguments"
        exec "$gc_bin" pack registry show main:gascity --refresh
        ;;
      *) die "unknown packs operation: $sub" ;;
    esac
    ;;
  mayor)
    [ "$#" -ge 1 ] || { usage; exit 2; }
    sub="$1"; shift
    case "$sub" in
      start)
        [ "$#" -eq 0 ] || die "mayor start accepts no arguments"
        # The Mayor is on-demand in v1.4; wake opens the human/agent door without
        # creating a standing polling loop. It dispatches only and never claims.
        exec "$gc_bin" session wake "$mayor_alias"
        ;;
      status)
        [ "$#" -eq 0 ] || die "mayor status accepts no arguments"
        # Propagate the tri-state fail-closed exit: nonzero means the gc query
        # failed or was malformed (status unavailable), not "no session".
        mayor_report summary || exit $?
        ;;
      attach-hint)
        [ "$#" -eq 0 ] || die "mayor attach-hint accepts no arguments"
        mayor_report hint || exit $?
        ;;
      tell)
        [ "$#" -eq 1 ] || die "mayor tell requires exactly one quoted message"
        message="$1"
        # Drivers hand the Mayor BEAD IDS, never prose work (e.g. "dispatch
        # testrig-12"). Refuse an empty/whitespace message so a fat-fingered
        # driver never nudges the Mayor with nothing to act on.
        [ -n "$(printf '%s' "$message" | tr -d '[:space:]')" ] || die "mayor tell requires a non-empty message (reference a bead id, e.g. 'dispatch testrig-12')"
        # gc mail send delivers a message bead and --notify nudges the Mayor.
        exec "$gc_bin" mail send --to "$mayor_alias" --notify -m "$message"
        ;;
      *) die "unknown mayor operation: $sub" ;;
    esac
    ;;
  --)
    [ "$#" -gt 0 ] || die "a GC command is required after --"
    exec "$gc_bin" "$@"
    ;;
  *) die "unknown operation: $command" ;;
esac
