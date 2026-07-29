#!/usr/bin/env bash
# preamble-exempt: standalone deployment bootstrap.
set -euo pipefail

die() { printf 'gc-agentops bootstrap: %s\n' "$*" >&2; exit 1; }
usage() {
  cat <<'EOF'
Usage: bootstrap.sh --city PATH --rig PATH --gc-bin PATH [options]

Options:
  --pack PATH              Factory pack (default: repository pack)
  --rig-name NAME          Logical rig name (default: repository basename)
  --delivery-mode MODE     auto (default) or manual
  --telemetry-mode MODE    auto (default), required, or off
  --otel-metrics-url URL   Default: local VictoriaMetrics OTLP endpoint
  --otel-logs-url URL      Default: local VictoriaLogs OTLP endpoint
  --max-active-sessions N  Default: 2
  --start                  Start the private supervisor and resume the rig
EOF
}

script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
registry_lock="$script_dir/pack-registry.lock.json"
city=""; rig=""; gc_bin=""; pack="$script_dir/../../packs/agentops-factory"
rig_name=""; delivery_mode="auto"; telemetry_mode="auto"; start=0
metrics_url="http://localhost:8428/opentelemetry/api/v1/push"
logs_url="http://localhost:9428/insert/opentelemetry/v1/logs"
max_active_sessions=2
while [ "$#" -gt 0 ]; do
  case "$1" in
    --city) city="${2:?--city requires a path}"; shift 2 ;;
    --rig) rig="${2:?--rig requires a path}"; shift 2 ;;
    --gc-bin) gc_bin="${2:?--gc-bin requires a path}"; shift 2 ;;
    --pack) pack="${2:?--pack requires a path}"; shift 2 ;;
    --rig-name) rig_name="${2:?--rig-name requires a value}"; shift 2 ;;
    --delivery-mode) delivery_mode="${2:?--delivery-mode requires a value}"; shift 2 ;;
    --telemetry-mode) telemetry_mode="${2:?--telemetry-mode requires a value}"; shift 2 ;;
    --otel-metrics-url) metrics_url="${2:?--otel-metrics-url requires a URL}"; shift 2 ;;
    --otel-logs-url) logs_url="${2:?--otel-logs-url requires a URL}"; shift 2 ;;
    --max-active-sessions) max_active_sessions="${2:?--max-active-sessions requires a value}"; shift 2 ;;
    --start) start=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done
[ -n "$city" ] && [ -n "$rig" ] && [ -n "$gc_bin" ] || die "--city, --rig, and --gc-bin are required"
case "$delivery_mode" in auto|manual) ;; *) die "delivery mode must be auto or manual" ;; esac
case "$telemetry_mode" in auto|required|off) ;; *) die "telemetry mode must be auto, required, or off" ;; esac
[[ "$max_active_sessions" =~ ^[1-9][0-9]*$ ]] || die "max sessions must be positive"
[ "$max_active_sessions" -le 16 ] || die "max sessions must be at most 16"

canonical() { python3 - "$1" <<'PY'
import os, sys
print(os.path.realpath(os.path.abspath(os.path.expanduser(sys.argv[1]))))
PY
}
city="$(canonical "$city")"; rig="$(canonical "$rig")"; pack="$(canonical "$pack")"; gc_bin="$(canonical "$gc_bin")"
gc_bin_dir="$(dirname "$gc_bin")"
GC_BIN="$gc_bin"
export GC_BIN
[ -d "$rig/.git" ] || git -C "$rig" rev-parse --git-dir >/dev/null 2>&1 || die "rig is not a Git repository: $rig"
[ -f "$rig/.beads/metadata.json" ] && [ -f "$rig/.beads/config.yaml" ] || die "rig must already contain a GC-compatible Beads store and the source bead"
bead_prefix="$(python3 - "$rig/.beads/config.yaml" <<'PY'
import re, sys
for line in open(sys.argv[1], encoding="utf-8"):
    match = re.match(r"^(?:issue_prefix|issue-prefix):\s*(.*?)\s*(?:#.*)?$", line)
    if not match: continue
    value = match.group(1).strip().strip("\"'").lower()
    if value and re.fullmatch(r"[a-z0-9][a-z0-9_-]*", value):
        print(value); break
PY
)"
[ -n "$bead_prefix" ] || die "rig Beads config must contain a valid top-level issue_prefix"
bead_database="$(python3 - "$rig/.beads/metadata.json" "$bead_prefix" <<'PY'
import json, re, sys
metadata = json.load(open(sys.argv[1], encoding="utf-8"))
value = str(metadata.get("dolt_database") or sys.argv[2]).strip()
if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]*", value):
    raise SystemExit("invalid Beads dolt_database")
print(value)
PY
)" || die "rig Beads metadata has no valid dolt_database"
[ -f "$pack/pack.toml" ] || die "factory pack is missing pack.toml: $pack"
executor="$(canonical "$pack/../agentops-executor")"
[ -f "$executor/pack.toml" ] || die "factory pack requires sibling agentops-executor"
[ -x "$gc_bin" ] || die "gc binary is not executable: $gc_bin"
toolchain_root="$(dirname "$(dirname "$gc_bin")")"
receipt="$toolchain_root/toolchain.json"
[ -f "$receipt" ] || die "toolchain receipt is missing beside gc: $receipt"
[ -f "$registry_lock" ] || die "pack registry lock is missing: $registry_lock"
bd_bin="$toolchain_root/bin/bd"
[ -x "$bd_bin" ] || die "paired bd binary is missing: $bd_bin"

toolchain_identity="$(python3 - "$receipt" "$gc_bin" "$bd_bin" <<'PY'
import hashlib, json, os, sys
receipt_path, gc_path, bd_path = sys.argv[1:]
r = json.load(open(receipt_path, encoding="utf-8"))
if r.get("schema_version") != 2 or set(r.get("runtime", {})) != {"gc", "bd"}:
    raise SystemExit("toolchain receipt must contain only schema-2 gc and bd runtimes")
def sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for b in iter(lambda: f.read(1024 * 1024), b""): h.update(b)
    return h.hexdigest()
for name, path in (("gc", gc_path), ("bd", bd_path)):
    expected = r["runtime"][name]
    if os.path.realpath(path) != os.path.realpath(os.path.join(os.path.dirname(receipt_path), expected["path"])) or sha(path) != expected["sha256"]:
        raise SystemExit(f"{name} binary differs from toolchain receipt")
p = r.get("pair", {})
if p.get("status") != "qualified": raise SystemExit("toolchain pair is not qualified")
print("\t".join((p["id"], p["gc"]["source_commit"], p["bd"]["source_commit"], r["runtime"]["gc"]["sha256"], r["runtime"]["bd"]["sha256"])))
PY
)" || die "invalid toolchain receipt"
IFS=$'\t' read -r pair_id gc_commit bd_commit gc_sha bd_sha <<<"$toolchain_identity"
[ "$gc_commit" = "a7297c511d637a3609947386f3389d76ddb2f23b" ] || die "unsupported Gas City commit: $gc_commit"
[ "$bd_commit" = "8e4e59d39f3459a43cf21a3236a13eca4dd874f7" ] || die "unsupported Beads commit: $bd_commit"

[ -n "$rig_name" ] || rig_name="$(basename "$rig")"
[[ "$rig_name" =~ ^[A-Za-z0-9_-]+$ ]] || die "unsafe rig name: $rig_name"
base_ref="$(git -C "$rig" symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's#^origin/##' || true)"
[ -n "$base_ref" ] || base_ref="main"
worktree_root="$(dirname "$rig")/$(basename "$rig")-gc-workers"

telemetry_status="off"
if [ "$telemetry_mode" != "off" ]; then
  if python3 - "$metrics_url" "$logs_url" <<'PY'
import socket, sys
from urllib.parse import urlparse
for value in sys.argv[1:]:
    u = urlparse(value)
    if u.scheme not in {"http", "https"} or not u.hostname: raise SystemExit(1)
    with socket.create_connection((u.hostname, u.port or (443 if u.scheme == "https" else 80)), timeout=.5): pass
PY
  then telemetry_status="enabled"
  elif [ "$telemetry_mode" = "required" ]; then die "required OTEL endpoints are unavailable"
  else telemetry_status="degraded"; metrics_url=""; logs_url=""
  fi
else metrics_url=""; logs_url=""
fi
otel_disabled="true"
[ "$telemetry_status" = "enabled" ] && otel_disabled="false"
export GC_OTEL_METRICS_URL="$metrics_url" GC_OTEL_LOGS_URL="$logs_url" OTEL_EXPORTER_OTLP_ENDPOINT="" OTEL_SDK_DISABLED="$otel_disabled"

request_digest="$(python3 - "$pack" "$executor" "$script_dir/city.toml" "$script_dir/worktree.sh" "$script_dir/refine.sh" "$registry_lock" "$pair_id" "$rig_name" "$bead_prefix" "$bead_database" "$base_ref" "$worktree_root" "$delivery_mode" "$telemetry_mode" "$telemetry_status" "$metrics_url" "$logs_url" "$max_active_sessions" <<'PY'
import hashlib, json, os, sys
factory, executor, template, worktree, refine, registry, pair, rig_name, prefix, database, base, workers, delivery, tmode, tstatus, metrics, logs, maximum = sys.argv[1:]
def tree_sha(root):
    h=hashlib.sha256()
    for parent,dirs,files in os.walk(root):
        dirs.sort()
        for name in sorted(files):
            path=os.path.join(parent,name); rel=os.path.relpath(path,root)
            h.update(rel.encode()+b"\0"+open(path,"rb").read()+b"\0")
    return h.hexdigest()
request={"pair":pair,"factory":tree_sha(factory),"executor":tree_sha(executor),"template":hashlib.sha256(open(template,"rb").read()).hexdigest(),"worktree":hashlib.sha256(open(worktree,"rb").read()).hexdigest(),"refine":hashlib.sha256(open(refine,"rb").read()).hexdigest(),"registry":hashlib.sha256(open(registry,"rb").read()).hexdigest(),"rig_name":rig_name,"beads":{"prefix":prefix,"database":database},"base":base,"workers":workers,"delivery":delivery,"telemetry":{"mode":tmode,"status":tstatus,"metrics":metrics,"logs":logs},"max_sessions":int(maximum)}
print(hashlib.sha256(json.dumps(request,sort_keys=True,separators=(",",":")).encode()).hexdigest())
PY
)" || die "cannot derive requested configuration identity"

marker="$city/.gc/agentops-bootstrap.json"
if [ -f "$marker" ]; then
  python3 - "$marker" "$city" "$rig" "$pack" "$pair_id" "$delivery_mode" "$bead_prefix" "$bead_database" "$request_digest" "$gc_bin" "$gc_sha" "$bd_bin" "$bd_sha" <<'PY'
import hashlib, json, os, sys
m = json.load(open(sys.argv[1], encoding="utf-8"))
expected = {"city": sys.argv[2], "rig": sys.argv[3], "pack_source": sys.argv[4], "toolchain_pair": sys.argv[5], "delivery_mode": sys.argv[6], "bead_prefix": sys.argv[7], "bead_database": sys.argv[8]}
if m.get("schema_version") != 1 or any(m.get(k) != v for k, v in expected.items()):
    raise SystemExit("existing managed city identity differs from request")
if m.get("configuration_digest") != sys.argv[9]:
    raise SystemExit("existing managed city configuration differs from current request or source bytes")
def sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for b in iter(lambda: f.read(1024 * 1024), b""): h.update(b)
    return h.hexdigest()
# Re-materializing the toolchain can move it or change its bytes. The existing
# marker's fast path must not silently succeed while pointing at a stale or
# deleted toolchain: compare the marker's recorded gc/bd paths and digests to
# this invocation's toolchain, and confirm both binaries still exist and match.
gc_now, gc_now_sha, bd_now, bd_now_sha = sys.argv[10:14]
tc = m.get("toolchain", {})
recorded = {"gc": (tc.get("gc", {}).get("path"), tc.get("gc", {}).get("sha256")),
            "bd": (tc.get("bd", {}).get("path"), tc.get("bd", {}).get("sha256"))}
requested = {"gc": (gc_now, gc_now_sha), "bd": (bd_now, bd_now_sha)}
if recorded != requested:
    raise SystemExit("existing managed city points at a different toolchain (paths or digests changed); re-materializing moves it — re-bootstrap a fresh --city, or restore the toolchain at the marker's recorded path")
for name, (path, expected_sha) in recorded.items():
    if not path or not os.path.exists(path):
        raise SystemExit(f"managed {name} binary is missing at {path}; re-bootstrap a fresh --city")
    if sha(path) != expected_sha:
        raise SystemExit(f"managed {name} binary changed on disk; re-bootstrap a fresh --city")
def tree_sha(root):
    h=hashlib.sha256()
    for parent,dirs,files in os.walk(root):
        dirs.sort()
        for name in sorted(files):
            path=os.path.join(parent,name); rel=os.path.relpath(path,root)
            h.update(rel.encode()+b"\0"+open(path,"rb").read()+b"\0")
    return h.hexdigest()
if tree_sha(os.path.dirname(m["pack_snapshot"])) != m.get("pack_sha256"):
    raise SystemExit("installed pack snapshot differs from bootstrap marker")
with open(os.path.join(m["city"], "city.toml"), "rb") as handle:
    if hashlib.sha256(handle.read()).hexdigest() != m.get("city_config_sha256"):
        raise SystemExit("managed city config differs from bootstrap marker")
PY
  if [ "$start" -eq 1 ]; then
    GC_HOME="$city/.gc-home"; PATH="$gc_bin_dir:$PATH"; export GC_HOME PATH
    "$gc_bin" --city "$city" start --no-auto-restart >/dev/null
    "$gc_bin" --city "$city" rig resume "$rig_name" --json >/dev/null
  fi
  printf '%s\n' "$marker"; exit 0
fi
[ ! -e "$city" ] || [ -z "$(find "$city" -mindepth 1 -maxdepth 1 -print -quit)" ] || die "refusing nonempty unmanaged city: $city"

"$gc_bin" lint "$pack" --json >/dev/null
"$gc_bin" lint "$executor" --json >/dev/null
rendered=""
bootstrap_committed=0
cleanup() {
  [ -z "$rendered" ] || rm -f "$rendered"
  if [ "$bootstrap_committed" -eq 0 ] && [ -d "$city" ]; then
    "$gc_bin" --city "$city" stop --force >/dev/null 2>&1 || true
    python3 - "$city" <<'PY'
import os, shutil, sys
target = os.path.realpath(sys.argv[1])
if target in {"", "/"} or not os.path.isdir(target): raise SystemExit(1)
shutil.rmtree(target)
PY
  fi
}
trap cleanup EXIT
mkdir -p "$city/.gc-home"
port="$(python3 - <<'PY'
import socket
s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()
PY
)"
printf '[supervisor]\nport = %s\nbind = "127.0.0.1"\n' "$port" >"$city/.gc-home/supervisor.toml"
GC_HOME="$city/.gc-home"; PATH="$gc_bin_dir:$PATH"; export GC_HOME PATH
registry_entry="$("$gc_bin" pack registry show main:gascity --refresh --json)" || die "cannot refresh the built-in Gas City pack registry"
python3 - "$registry_lock" "$registry_entry" <<'PY'
import json, sys
locked = json.load(open(sys.argv[1], encoding="utf-8"))
entry = json.loads(sys.argv[2])
workflow = locked.get("maintainer_workflow", {})
release = next((item for item in entry.get("releases", []) if item.get("version") == workflow.get("version")), None)
if locked.get("schema_version") != 1 or entry.get("registry") != "main":
    raise SystemExit("unexpected Gas City registry identity")
if entry.get("name") != workflow.get("name") or entry.get("source") != workflow.get("source"):
    raise SystemExit("maintainer workflow source differs from pack registry lock")
if not isinstance(release, dict):
    raise SystemExit("maintainer workflow version is absent from the main registry")
for field in ("commit", "hash"):
    if release.get(field) != workflow.get(field):
        raise SystemExit(f"maintainer workflow {field} differs from pack registry lock")
PY
rendered="$(mktemp "${TMPDIR:-/tmp}/agentops-city.XXXXXX.toml")"
python3 - "$script_dir/city.toml" "$rendered" "$gc_bin" "$PATH" "$rig" "$worktree_root" "$city" "$base_ref" "$delivery_mode" "$rig_name" "$metrics_url" "$logs_url" "$max_active_sessions" "$otel_disabled" <<'PY'
import hashlib, json, os, sys
source, target, gc_bin, path, rig, workers, city, base, mode, rig_name, metrics, logs, maximum, disabled = sys.argv[1:]
text = open(source, encoding="utf-8").read()
values = {
"__GC_BIN__":gc_bin,"__GC_PATH__":path,"__GC_RIG_ROOT__":rig,"__GC_WORKTREE_ROOT__":workers,
"__GC_WORKTREE_SCRIPT__":os.path.join(city,".gc/scripts/agentops-worktree"),"__GC_REFINER_SCRIPT__":os.path.join(city,".gc/scripts/agentops-refine"),
"__GC_BASE_REF__":base,"__GC_DELIVERY_MODE__":mode,"__GC_PLAN_TARGET__":f"{rig_name}/agentops.plan-reviewer",
"__GC_TERRA_TARGET__":f"{rig_name}/agentops.implementer","__GC_OPUS_TARGET__":f"{rig_name}/agentops.implementer-claude",
"__GC_VALIDATE_TARGET__":f"{rig_name}/agentops.validator","__GC_REFINER_TARGET__":f"{rig_name}/agentops.refiner",
"__GC_OTEL_METRICS_URL__":metrics,"__GC_OTEL_LOGS_URL__":logs,"__GC_OTEL_DISABLED__":disabled,"__GC_TMUX_SOCKET__":"agentops-"+hashlib.sha256(city.encode()).hexdigest()[:20]}
for key,value in values.items(): text=text.replace(key, json.dumps(value)[1:-1])
text=text.replace("__GC_MAX_ACTIVE_SESSIONS__", maximum)
open(target,"w",encoding="utf-8").write(text)
PY
"$gc_bin" init --file "$rendered" --name "$(basename "$city")" --no-start --skip-provider-readiness "$city" >/dev/null

managed_port="$(tr -d '[:space:]' <"$city/.beads/dolt-server.port")"
[[ "$managed_port" =~ ^[1-9][0-9]{0,4}$ ]] && [ "$managed_port" -le 65535 ] || die "fresh city did not publish a valid managed Dolt port"
bd_city_env=(env BD_NON_INTERACTIVE=1 BD_EXPORT_AUTO=false BD_BACKUP_ENABLED=false BEADS_DOLT_AUTO_START=0 BEADS_DOLT_SERVER_MODE=1 BEADS_DOLT_SERVER_HOST=127.0.0.1 BEADS_DOLT_SERVER_PORT="$managed_port" BEADS_DOLT_SERVER_DATABASE="$bead_database")
bootstrap_plan="$("${bd_city_env[@]}" "$bd_bin" -C "$rig" bootstrap --dry-run --json)" || die "cannot resolve the official Beads bootstrap plan"
python3 - "$bootstrap_plan" <<'PY'
import json, sys
plan = json.loads(sys.argv[1])
if plan.get("schema_version") != 1 or plan.get("action") != "sync" or plan.get("has_existing") is not False or not plan.get("sync_remote"):
    raise SystemExit("fresh city requires a durable source bead on the configured Beads sync remote")
PY
"${bd_city_env[@]}" "$bd_bin" -C "$rig" bootstrap --yes >/dev/null
source_count="$("${bd_city_env[@]}" "$bd_bin" -C "$rig" list --limit 1 --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')" || die "cannot read the bootstrapped Beads source"
[ "$source_count" -gt 0 ] || die "bootstrapped Beads database contains no source bead"

snapshot="$city/.gc/agentops-packs"
mkdir -p "$snapshot"
cp -R "$pack" "$snapshot/agentops-factory"
cp -R "$executor" "$snapshot/agentops-executor"
mkdir -p "$city/.gc/scripts"
install -m 0755 "$script_dir/worktree.sh" "$city/.gc/scripts/agentops-worktree"
install -m 0755 "$script_dir/refine.sh" "$city/.gc/scripts/agentops-refine"
(cd "$city" && "$gc_bin" import add "$snapshot/agentops-factory" --name agentops >/dev/null)
add_args=(--city "$city" rig add "$rig" --name "$rig_name" --prefix "$bead_prefix" --start-suspended --adopt)
"$gc_bin" "${add_args[@]}" >/dev/null
python3 - "$city/city.toml" "$snapshot/agentops-factory" <<'PY'
import json, sys
path, source = sys.argv[1:]
text = open(path, encoding="utf-8").read()
header = "[rigs.imports.agentops]"
if header not in text:
    if text and not text.endswith("\n"): text += "\n"
    text += f"\n{header}\nsource = {json.dumps(source)}\n"
    open(path, "w", encoding="utf-8").write(text)
PY
"$gc_bin" --city "$city" import install >/dev/null
"$gc_bin" --city "$city" config show >/dev/null
"$gc_bin" --city "$city" import status --json >/dev/null

python3 - "$marker" "$city" "$rig" "$rig_name" "$pack" "$snapshot/agentops-factory" "$pair_id" "$gc_commit" "$bd_commit" "$gc_bin" "$bd_bin" "$gc_sha" "$bd_sha" "$delivery_mode" "$base_ref" "$worktree_root" "$telemetry_mode" "$telemetry_status" "$metrics_url" "$logs_url" "$otel_disabled" "$port" "$script_dir/city.toml" "$registry_lock" "$request_digest" "$max_active_sessions" "$bead_prefix" "$bead_database" <<'PY'
import hashlib, json, os, sys, tempfile
(path,city,rig,rig_name,pack,pack_snapshot,pair,gc_commit,bd_commit,gc_bin,bd_bin,gc_sha,bd_sha,mode,base,workers,tmode,tstatus,metrics,logs,disabled,port,template,registry,configuration,maximum,prefix,database)=sys.argv[1:]
def tree_sha(root):
    h=hashlib.sha256()
    for parent,dirs,files in os.walk(root):
        dirs.sort()
        for name in sorted(files):
            p=os.path.join(parent,name); rel=os.path.relpath(p,root)
            h.update(rel.encode()+b"\0"+open(p,"rb").read()+b"\0")
    return h.hexdigest()
pack_sha=tree_sha(os.path.dirname(pack_snapshot))
policy_sha=hashlib.sha256(open(template,"rb").read()).hexdigest()
city_config_sha=hashlib.sha256(open(os.path.join(city,"city.toml"),"rb").read()).hexdigest()
m={"schema_version":1,"state":"ready","city":city,"rig":rig,"rig_name":rig_name,"bead_prefix":prefix,"bead_database":database,"pack_source":pack,"pack_snapshot":pack_snapshot,"pack_sha256":pack_sha,"pack_registry_lock_sha256":hashlib.sha256(open(registry,"rb").read()).hexdigest(),"policy_sha256":policy_sha,"city_config_sha256":city_config_sha,"configuration_digest":configuration,"toolchain_pair":pair,"toolchain":{"gc":{"path":gc_bin,"sha256":gc_sha,"commit":gc_commit},"bd":{"path":bd_bin,"sha256":bd_sha,"commit":bd_commit}},"delivery_mode":mode,"base_ref":base,"worktree_root":workers,"max_active_sessions":int(maximum),"telemetry":{"mode":tmode,"status":tstatus,"metrics_url":metrics,"logs_url":logs,"sdk_disabled":disabled == "true"},"supervisor_port":int(port)}
os.makedirs(os.path.dirname(path),exist_ok=True)
fd,tmp=tempfile.mkstemp(prefix=".bootstrap.",dir=os.path.dirname(path)); os.fchmod(fd,0o600)
with os.fdopen(fd,"w",encoding="utf-8") as f: json.dump(m,f,indent=2,sort_keys=True); f.write("\n"); f.flush(); os.fsync(f.fileno())
os.replace(tmp,path)
PY
bootstrap_committed=1

if [ "$start" -eq 1 ]; then
  "$gc_bin" --city "$city" start --no-auto-restart >/dev/null
  "$gc_bin" --city "$city" rig resume "$rig_name" --json >/dev/null
  "$gc_bin" --city "$city" doctor --json >/dev/null
fi
printf '%s\n' "$marker"
