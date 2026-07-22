#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  INVOKE="$REPO_ROOT/deploy/gc/invoke.sh"
  README="$REPO_ROOT/deploy/gc/README.md"
  HELP="$REPO_ROOT/packs/agentops-factory/commands/program-start/help.md"
  TMP="$(mktemp -d "${TMPDIR:-/tmp}/gc-agentops-invoke.XXXXXX")"
  CITY="$TMP/city"
  BIN="$TMP/bin"
  LOG="$TMP/gc.log"
  mkdir -p "$CITY/.gc" "$CITY/.gc-home" "$BIN"

  FAKE_GC="$BIN/gc"
  cat >"$FAKE_GC" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
requested_city=""
if [ "${1-}" = "--city" ] && [ -n "${2-}" ]; then
  requested_city="$(cd "$2" && pwd -P)"
fi
current_dir="$(pwd -P)"
if [ "$current_dir" != "$requested_city" ]; then
  for argument in "$@"; do
    if [ "$argument" = "--source-bead" ]; then
      printf 'gc: unknown flag: --source-bead\n' >&2
      exit 1
    fi
  done
fi
printf 'CWD <%s>\n' "$current_dir" >>"$root/gc.log"
printf 'ENV GC_HOME=<%s> GC_ISOLATED=<%s> OTEL_SDK_DISABLED=<%s> OTEL_EXPORTER_OTLP_ENDPOINT=<%s> GC_OTEL_METRICS_URL=<%s> GC_OTEL_LOGS_URL=<%s> BD_OTEL_METRICS_URL=<%s> BD_OTEL_LOGS_URL=<%s>\n' \
  "${GC_HOME-}" "${GC_ISOLATED-}" "${OTEL_SDK_DISABLED-}" \
  "${OTEL_EXPORTER_OTLP_ENDPOINT-}" "${GC_OTEL_METRICS_URL-}" \
  "${GC_OTEL_LOGS_URL-}" "${BD_OTEL_METRICS_URL-}" \
  "${BD_OTEL_LOGS_URL-}" >>"$root/gc.log"
printf 'ARGS' >>"$root/gc.log"
printf ' <%s>' "$@" >>"$root/gc.log"
printf '\n' >>"$root/gc.log"
EOF
  chmod +x "$FAKE_GC"

  python3 - "$CITY/.gc/agentops-bootstrap.json" "$CITY" "$FAKE_GC" <<'PY'
import hashlib
import json
import os
import sys

path, city, gc_bin = sys.argv[1:]
with open(gc_bin, "rb") as handle:
    gc_digest = hashlib.sha256(handle.read()).hexdigest()
with open(path, "w", encoding="utf-8") as handle:
    json.dump({
        "schema_version": 5,
        "state": "ready",
        "city": os.path.realpath(city),
        "telemetry": {
            "mode": "required",
            "status": "enabled",
            "requested_metrics_url": "http://127.0.0.1:8428/opentelemetry/api/v1/push",
            "requested_logs_url": "http://127.0.0.1:9428/insert/opentelemetry/v1/logs",
            "effective_metrics_url": "http://127.0.0.1:8428/opentelemetry/api/v1/push",
            "effective_logs_url": "http://127.0.0.1:9428/insert/opentelemetry/v1/logs",
        },
        "toolchain": {
            "gc": {
                "path": os.path.realpath(gc_bin),
                "sha256": gc_digest,
            },
        },
    }, handle)
    handle.write("\n")
PY
}

teardown() {
  rm -rf "$TMP"
}

@test "invoke binds the managed binary, city, telemetry, and direct pack arguments" {
  run env \
    OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 \
    OTEL_SDK_DISABLED=true \
    GC_OTEL_METRICS_URL=http://ambient.invalid/metrics \
    GC_OTEL_LOGS_URL=http://ambient.invalid/logs \
    BD_OTEL_METRICS_URL=http://ambient.invalid/bd-metrics \
    BD_OTEL_LOGS_URL=http://ambient.invalid/bd-logs \
    "$INVOKE" --city "$CITY" -- \
      agentops program start --source-bead ag-blj --max-parallel 2

  [ "$status" -eq 0 ]
  canonical_city="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$CITY")"
  grep -Fq "CWD <$canonical_city>" "$LOG"
  grep -Fq "ENV GC_HOME=<$canonical_city/.gc-home> GC_ISOLATED=<1> OTEL_SDK_DISABLED=<false> OTEL_EXPORTER_OTLP_ENDPOINT=<> GC_OTEL_METRICS_URL=<http://127.0.0.1:8428/opentelemetry/api/v1/push> GC_OTEL_LOGS_URL=<http://127.0.0.1:9428/insert/opentelemetry/v1/logs> BD_OTEL_METRICS_URL=<http://127.0.0.1:8428/opentelemetry/api/v1/push> BD_OTEL_LOGS_URL=<http://127.0.0.1:9428/insert/opentelemetry/v1/logs>" "$LOG"
  grep -Fq "ARGS <--city> <$canonical_city> <agentops> <program> <start> <--source-bead> <ag-blj> <--max-parallel> <2>" "$LOG"
  ! grep -Fq ' <--> ' "$LOG"
}

@test "invoke disables telemetry for an explicitly off managed city" {
  python3 - "$CITY/.gc/agentops-bootstrap.json" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as handle:
    value = json.load(handle)
value["telemetry"] = {
    "mode": "off",
    "status": "off",
    "requested_metrics_url": "",
    "requested_logs_url": "",
    "effective_metrics_url": "",
    "effective_logs_url": "",
}
with open(path, "w", encoding="utf-8") as handle:
    json.dump(value, handle)
    handle.write("\n")
PY

  run env OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 "$INVOKE" --city "$CITY" -- status --json

  [ "$status" -eq 0 ]
  grep -Fq 'OTEL_SDK_DISABLED=<true> OTEL_EXPORTER_OTLP_ENDPOINT=<> GC_OTEL_METRICS_URL=<> GC_OTEL_LOGS_URL=<> BD_OTEL_METRICS_URL=<> BD_OTEL_LOGS_URL=<>' "$LOG"
}

@test "invoke refuses changed managed binary bytes and an ambiguous command boundary" {
  printf '%s\n' '# replaced' >>"$FAKE_GC"

  run "$INVOKE" --city "$CITY" -- status
  [ "$status" -ne 0 ]
  [[ "$output" == *"managed gc binary digest mismatch"* ]]

  run "$INVOKE" --city "$CITY" status
  [ "$status" -ne 0 ]
  [[ "$output" == *"expected -- before the Gas City command"* ]]
}

@test "operator docs use the managed invocation boundary and direct discovered-command flags" {
  run rg -n 'program start -- --source-bead' "$README" "$HELP"
  [ "$status" -eq 1 ]

  rg -Fq 'deploy/gc/invoke.sh --city /path/to/city --' "$README"
  rg -Fq 'agentops program start --source-bead age-example --max-parallel 2' "$README"
  rg -Fq 'enters the exact managed city before executing GC' "$README"
  rg -Fq 'gc agentops program start --source-bead ID' "$HELP"
}
