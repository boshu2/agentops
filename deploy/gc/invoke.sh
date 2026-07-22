#!/usr/bin/env bash
# preamble-exempt: standalone managed-city invoker; intentionally runs outside a repo checkout.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  invoke.sh --city PATH -- GC_COMMAND [ARG...]

Required:
  --city PATH  City previously created by deploy/gc/bootstrap.sh
  --           Explicit boundary before the Gas City command and its arguments

This executes the exact gc binary recorded by the managed-city marker. It binds
the city's private supervisor home and effective telemetry policy and removes an
ambient generic OTLP fallback. It executes from the managed city directory so
GC can register imported pack commands before parsing their leaf flags.
EOF
}

die() {
  printf 'gc-agentops invoke: %s\n' "$*" >&2
  exit 1
}

city=""
boundary_seen=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --city)
      [ "$#" -ge 2 ] || die "--city requires a path"
      city="$2"
      shift 2
      ;;
    --)
      boundary_seen=1
      shift
      break
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "expected -- before the Gas City command; found: $1"
      ;;
  esac
done

[ -n "$city" ] || die "--city is required"
[ "$boundary_seen" -eq 1 ] || die "expected -- before the Gas City command"
[ "$#" -gt 0 ] || die "a Gas City command is required after --"
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
[ -f "$marker" ] && [ ! -L "$marker" ] || \
  die "refusing unmanaged city without a regular bootstrap marker: $city"

marker_identity="$(python3 - "$marker" "$city" <<'PY'
import json
import hashlib
import os
import sys
from urllib.parse import urlparse

marker_path, expected_city = sys.argv[1:]
with open(marker_path, encoding="utf-8") as handle:
    marker = json.load(handle)

if marker.get("schema_version") != 5:
    raise SystemExit("managed-city marker schema_version must be 5")
if marker.get("state") != "ready":
    raise SystemExit("managed-city marker state must be ready")
actual_city = os.path.realpath(os.path.expanduser(str(marker.get("city", ""))))
if actual_city != expected_city:
    raise SystemExit(f"managed-city marker city mismatch: {actual_city!r} != {expected_city!r}")

toolchain = marker.get("toolchain")
gc = toolchain.get("gc") if isinstance(toolchain, dict) else None
bd = toolchain.get("bd") if isinstance(toolchain, dict) else None
if not isinstance(gc, dict):
    raise SystemExit("managed-city marker has no gc toolchain identity")
if not isinstance(bd, dict):
    raise SystemExit("managed-city marker has no bd toolchain identity")
gc_bin = gc.get("path")
gc_sha256 = gc.get("sha256")
if not isinstance(gc_bin, str) or not gc_bin.strip():
    raise SystemExit("managed-city marker has no gc path")
if (
    not isinstance(gc_sha256, str)
    or len(gc_sha256) != 64
    or any(char not in "0123456789abcdef" for char in gc_sha256)
):
    raise SystemExit("managed-city marker has invalid gc sha256")

def digest_file(path, label):
    if not isinstance(path, str) or not path or not os.path.isabs(path):
        raise SystemExit(f"managed-city marker has invalid {label} path")
    if os.path.islink(path) or not os.path.isfile(path) or not os.access(path, os.X_OK):
        raise SystemExit(f"managed-city marker {label} is not an executable regular file")
    canonical = os.path.realpath(os.path.expanduser(path))
    if canonical != path:
        raise SystemExit(f"managed-city marker {label} path is not canonical")
    value = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            value.update(chunk)
    return value.hexdigest()

def hex_digest(value, label):
    if (
        not isinstance(value, str)
        or len(value) != 64
        or any(char not in "0123456789abcdef" for char in value)
    ):
        raise SystemExit(f"managed-city marker has invalid {label} digest")
    return value

telemetry = marker.get("telemetry")
if not isinstance(telemetry, dict):
    raise SystemExit("managed-city marker has no telemetry policy")
mode = telemetry.get("mode")
status = telemetry.get("status")
metrics = telemetry.get("effective_metrics_url")
logs = telemetry.get("effective_logs_url")
if mode not in {"auto", "required", "off"}:
    raise SystemExit("managed-city marker has invalid telemetry mode")
if status not in {"enabled", "degraded", "off"}:
    raise SystemExit("managed-city marker has invalid telemetry status")
if (mode, status) not in {
    ("auto", "enabled"),
    ("auto", "degraded"),
    ("required", "enabled"),
    ("off", "off"),
}:
    raise SystemExit("managed-city marker has inconsistent telemetry mode and status")
if not isinstance(metrics, str) or not isinstance(logs, str):
    raise SystemExit("managed-city marker telemetry endpoints must be strings")
if status == "enabled":
    if mode == "off" or not metrics or not logs:
        raise SystemExit("enabled telemetry requires two effective endpoints")
    for label, value in (("metrics", metrics), ("logs", logs)):
        parsed = urlparse(value)
        if parsed.scheme not in {"http", "https"} or not parsed.netloc:
            raise SystemExit(f"managed-city marker has invalid {label} endpoint")
elif metrics or logs:
    raise SystemExit("disabled or degraded telemetry cannot have effective endpoints")
if mode == "required" and status != "enabled":
    raise SystemExit("required telemetry must be enabled")
if mode == "off" and status != "off":
    raise SystemExit("off telemetry must have off status")

rig = marker.get("rig")
rig_name = marker.get("rig_name")
repository = marker.get("repository")
delivery_mode = marker.get("delivery_mode")
if not isinstance(rig, str) or os.path.realpath(os.path.expanduser(rig)) != rig or not os.path.isdir(rig):
    raise SystemExit("managed-city marker has invalid rig path")
if not isinstance(rig_name, str) or not rig_name:
    raise SystemExit("managed-city marker has invalid rig name")
if not isinstance(repository, str) or not repository:
    raise SystemExit("managed-city marker has invalid repository")
if delivery_mode not in {"auto", "manual"}:
    raise SystemExit("managed-city marker has invalid delivery mode")

delivery = marker.get("delivery")
if not isinstance(delivery, dict):
    raise SystemExit("managed-city marker has no ready delivery binding")
native_path = delivery.get("native_context_path")
native_digest = hex_digest(delivery.get("native_context_digest"), "native context")
expected_native_path = os.path.join(expected_city, ".gc", "agentops", "delivery-native-context.v1.json")
if native_path != expected_native_path or os.path.islink(native_path) or not os.path.isfile(native_path):
    raise SystemExit("managed-city marker native context path differs from the managed city")
with open(native_path, "rb") as handle:
    native_raw = handle.read()
if hashlib.sha256(native_raw).hexdigest() != native_digest:
    raise SystemExit("managed-city native context bytes differ from the marker digest")
try:
    native = json.loads(native_raw)
except json.JSONDecodeError as exc:
    raise SystemExit("managed-city native context is invalid JSON") from exc
canonical_native = (json.dumps(native, ensure_ascii=True, separators=(",", ":"), sort_keys=True) + "\n").encode()
if not isinstance(native, dict) or canonical_native != native_raw:
    raise SystemExit("managed-city native context is not canonical JSON")
if native.get("schema_version") != "gc-delivery-native-context.v1":
    raise SystemExit("managed-city native context has an invalid schema version")
expected_native = {
    "rig_id": rig_name,
    "repository": repository,
    "repository_dir": rig,
    "beads_dir": os.path.join(rig, ".beads"),
    "remote": "origin",
}
if any(native.get(key) != value for key, value in expected_native.items()):
    raise SystemExit("managed-city native context differs from the marker identity")
if not isinstance(native.get("base_ref"), str) or not native["base_ref"]:
    raise SystemExit("managed-city native context has no base ref")

evidence_root = delivery.get("evidence_root")
expected_evidence_root = os.path.join(rig, ".gc", "agentops", "factory", "evidence", "delivery")
if evidence_root != expected_evidence_root or not os.path.isdir(evidence_root):
    raise SystemExit("managed-city marker has an invalid delivery evidence root")
deadline_seconds = delivery.get("deadline_seconds")
if not isinstance(deadline_seconds, int) or not 300 <= deadline_seconds <= 604800:
    raise SystemExit("managed-city marker has an invalid delivery deadline")

executables = native.get("executables")
if not isinstance(executables, dict):
    raise SystemExit("managed-city native context has no executable bindings")
resolved = {}
for name in ("gc", "bd", "git", "gh", "bash", "agentops-gc-delivery"):
    binding = executables.get(name)
    if not isinstance(binding, dict):
        raise SystemExit(f"managed-city native context has no {name} binding")
    selected_path = binding.get("path")
    selected_digest = hex_digest(binding.get("digest"), name)
    if digest_file(selected_path, name) != selected_digest:
        raise SystemExit(f"managed-city native context {name} bytes differ from its digest")
    resolved[name] = (selected_path, selected_digest)
if resolved["gc"] != (gc_bin, gc_sha256):
    raise SystemExit("managed-city native context gc binding differs from the marker")
bd_path = bd.get("path")
bd_sha256 = hex_digest(bd.get("sha256"), "bd")
if resolved["bd"] != (bd_path, bd_sha256):
    raise SystemExit("managed-city native context bd binding differs from the marker")

ao_reducer = marker.get("ao_reducer")
if not isinstance(ao_reducer, dict):
    raise SystemExit("managed-city marker has no ao reducer identity")
ao_bin = ao_reducer.get("path")
ao_digest = hex_digest(ao_reducer.get("binary_sha256"), "ao")
if digest_file(ao_bin, "ao") != ao_digest:
    raise SystemExit("managed-city ao bytes differ from the marker digest")
delivery_bin = ao_reducer.get("delivery_path")
delivery_digest = hex_digest(ao_reducer.get("delivery_binary_sha256"), "delivery reducer")
if resolved["agentops-gc-delivery"] != (delivery_bin, delivery_digest):
    raise SystemExit("managed-city native context delivery reducer differs from the marker")

values = (
    os.path.realpath(os.path.expanduser(gc_bin)),
    gc_sha256,
    mode,
    status,
    metrics,
    logs,
    native_path,
    native_digest,
    evidence_root,
    str(deadline_seconds),
    resolved["bd"][0],
    resolved["git"][0],
    resolved["gh"][0],
    resolved["bash"][0],
    resolved["agentops-gc-delivery"][0],
    rig_name,
    repository,
    native["remote"],
    delivery_mode,
    ao_bin,
)
if any("\x1f" in value or "\n" in value or "\r" in value for value in values):
    raise SystemExit("managed-city marker contains unsafe control characters")
print("\x1f".join(values))
PY
)" || die "invalid managed-city marker: $marker"
IFS=$'\x1f' read -r gc_bin gc_sha256 telemetry_mode telemetry_status metrics_url logs_url \
  delivery_native_context delivery_native_context_digest delivery_root \
  delivery_deadline_seconds bd_bin git_bin gh_bin bash_bin delivery_bin \
  delivery_rig delivery_repository delivery_remote delivery_mode ao_bin <<<"$marker_identity"
[ "$telemetry_mode" != "required" ] || [ "$telemetry_status" = "enabled" ] || \
  die "required telemetry is not enabled"

[ -x "$gc_bin" ] || die "managed gc binary is not executable: $gc_bin"
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
[ "$actual_gc_sha256" = "$gc_sha256" ] || \
  die "managed gc binary digest mismatch: $actual_gc_sha256 != $gc_sha256"

export GC_HOME="$city/.gc-home"
export GC_ISOLATED=1
export GC_BIN="$gc_bin"
export AO_BIN="$ao_bin"
export AGENTOPS_GC_DELIVERY_BIN="$delivery_bin"
export AGENTOPS_GC_BEADS_BIN="$bd_bin"
export AGENTOPS_GC_GIT_BIN="$git_bin"
export AGENTOPS_GC_GH_BIN="$gh_bin"
export AGENTOPS_GC_BASH_BIN="$bash_bin"
export AGENTOPS_GC_DELIVERY_ROOT="$delivery_root"
export AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT="$delivery_native_context"
export AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT_DIGEST="$delivery_native_context_digest"
export AGENTOPS_GC_DELIVERY_RIG="$delivery_rig"
export AGENTOPS_GC_DELIVERY_REPOSITORY="$delivery_repository"
export AGENTOPS_GC_DELIVERY_REMOTE="$delivery_remote"
export AGENTOPS_GC_DELIVERY_MODE="$delivery_mode"
export AGENTOPS_GC_DELIVERY_DEADLINE_SECONDS="$delivery_deadline_seconds"
export OTEL_EXPORTER_OTLP_ENDPOINT=""
unset OTEL_EXPORTER_OTLP_METRICS_ENDPOINT OTEL_EXPORTER_OTLP_LOGS_ENDPOINT || true

if [ "$telemetry_status" = "enabled" ]; then
  export OTEL_SDK_DISABLED=false
  export GC_OTEL_METRICS_URL="$metrics_url"
  export GC_OTEL_LOGS_URL="$logs_url"
  export BD_OTEL_METRICS_URL="$metrics_url"
  export BD_OTEL_LOGS_URL="$logs_url"
else
  export OTEL_SDK_DISABLED=true
  export GC_OTEL_METRICS_URL=""
  export GC_OTEL_LOGS_URL=""
  export BD_OTEL_METRICS_URL=""
  export BD_OTEL_LOGS_URL=""
fi

cd -- "$city" || die "cannot enter managed city directory: $city"
exec "$gc_bin" "$@"
