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
if not isinstance(gc, dict):
    raise SystemExit("managed-city marker has no gc toolchain identity")
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

values = (
    os.path.realpath(os.path.expanduser(gc_bin)),
    gc_sha256,
    mode,
    status,
    metrics,
    logs,
)
if any("\x1f" in value or "\n" in value or "\r" in value for value in values):
    raise SystemExit("managed-city marker contains unsafe control characters")
print("\x1f".join(values))
PY
)" || die "invalid managed-city marker: $marker"
IFS=$'\x1f' read -r gc_bin gc_sha256 telemetry_mode telemetry_status metrics_url logs_url <<<"$marker_identity"
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
exec "$gc_bin" --city "$city" "$@"
