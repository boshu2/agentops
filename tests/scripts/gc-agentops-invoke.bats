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
  mkdir -p "$CITY/.gc/agentops" "$CITY/.gc-home" "$BIN" \
    "$TMP/rig/.beads" "$TMP/rig/.gc/agentops/factory/evidence/delivery" \
    "$TMP/worktrees"

  FAKE_GC="$BIN/gc"
  cat >"$FAKE_GC" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
requested_city="$root/city"
current_dir="$(pwd -P)"
if [ "$current_dir" != "$requested_city" ]; then
  for argument in "$@"; do
    if [ "$argument" = "--source-bead" ]; then
      printf 'gc: unknown flag: --source-bead\n' >&2
      exit 1
    fi
  done
fi
for argument in "$@"; do
  if [ "$argument" = "--city" ] || [ "$argument" = "--" ]; then
    printf 'program_start.py: error: unrecognized forwarded root argument: %s\n' "$argument" >&2
    exit 2
  fi
done
printf 'CWD <%s>\n' "$current_dir" >>"$root/gc.log"
printf 'ENV GC_HOME=<%s> GC_ISOLATED=<%s> OTEL_SDK_DISABLED=<%s> OTEL_EXPORTER_OTLP_ENDPOINT=<%s> GC_OTEL_METRICS_URL=<%s> GC_OTEL_LOGS_URL=<%s> BD_OTEL_METRICS_URL=<%s> BD_OTEL_LOGS_URL=<%s> NATIVE=<%s> NATIVE_DIGEST=<%s> DELIVERY_ROOT=<%s> DELIVERY_MODE=<%s> DELIVERY_DEADLINE=<%s> BD=<%s> GIT=<%s> GH=<%s> BASH=<%s> DELIVERY=<%s> RIG=<%s> REPOSITORY=<%s> REMOTE=<%s> AO=<%s>\n' \
  "${GC_HOME-}" "${GC_ISOLATED-}" "${OTEL_SDK_DISABLED-}" \
  "${OTEL_EXPORTER_OTLP_ENDPOINT-}" "${GC_OTEL_METRICS_URL-}" \
  "${GC_OTEL_LOGS_URL-}" "${BD_OTEL_METRICS_URL-}" \
  "${BD_OTEL_LOGS_URL-}" "${AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT-}" \
  "${AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT_DIGEST-}" \
  "${AGENTOPS_GC_DELIVERY_ROOT-}" "${AGENTOPS_GC_DELIVERY_MODE-}" \
  "${AGENTOPS_GC_DELIVERY_DEADLINE_SECONDS-}" "${AGENTOPS_GC_BEADS_BIN-}" \
  "${AGENTOPS_GC_GIT_BIN-}" "${AGENTOPS_GC_GH_BIN-}" \
  "${AGENTOPS_GC_BASH_BIN-}" "${AGENTOPS_GC_DELIVERY_BIN-}" \
  "${AGENTOPS_GC_DELIVERY_RIG-}" "${AGENTOPS_GC_DELIVERY_REPOSITORY-}" \
  "${AGENTOPS_GC_DELIVERY_REMOTE-}" "${AO_BIN-}" >>"$root/gc.log"
printf 'ARGS' >>"$root/gc.log"
printf ' <%s>' "$@" >>"$root/gc.log"
printf '\n' >>"$root/gc.log"
EOF
  chmod +x "$FAKE_GC"

  python3 - "$CITY/.gc/agentops-bootstrap.json" "$CITY" "$FAKE_GC" "$TMP" <<'PY'
import hashlib
import json
import os
import sys

path, city, gc_bin, root = sys.argv[1:]

def file_digest(selected):
    with open(selected, "rb") as handle:
        return hashlib.sha256(handle.read()).hexdigest()

gc_digest = file_digest(gc_bin)
native_path = os.path.join(city, ".gc", "agentops", "delivery-native-context.v1.json")
evidence_root = os.path.join(root, "rig", ".gc", "agentops", "factory", "evidence", "delivery")
executables = {
    name: {"path": os.path.realpath(gc_bin), "digest": gc_digest}
    for name in ("gc", "bd", "git", "gh", "bash", "agentops-gc-delivery")
}
native = {
    "schema_version": "gc-delivery-native-context.v1",
    "rig_id": "agentops",
    "repository": "boshu2/agentops",
    "repository_dir": os.path.realpath(os.path.join(root, "rig")),
    "worktree_root": os.path.realpath(os.path.join(root, "worktrees")),
    "beads_dir": os.path.realpath(os.path.join(root, "rig", ".beads")),
    "remote": "origin",
    "base_ref": "main",
    "successor_capability_digest": "a" * 64,
    "toolchain_lock_digest": "b" * 64,
    "toolchain_receipt_path": os.path.realpath(gc_bin),
    "toolchain_receipt_digest": gc_digest,
    "beads_representation": "B-successor-delivery-bead",
    "executables": executables,
    "check_only_gate_argv": [[os.path.realpath(gc_bin), "scripts/check-gc-executor.sh"]],
}
native_raw = (json.dumps(native, ensure_ascii=True, separators=(",", ":"), sort_keys=True) + "\n").encode()
with open(native_path, "wb") as handle:
    handle.write(native_raw)
native_digest = hashlib.sha256(native_raw).hexdigest()
with open(path, "w", encoding="utf-8") as handle:
    json.dump({
        "schema_version": 5,
        "state": "ready",
        "city": os.path.realpath(city),
        "rig": native["repository_dir"],
        "rig_name": "agentops",
        "repository": "boshu2/agentops",
        "delivery_mode": "auto",
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
            "bd": {
                "path": os.path.realpath(gc_bin),
                "sha256": gc_digest,
            },
        },
        "ao_reducer": {
            "path": os.path.realpath(gc_bin),
            "binary_sha256": gc_digest,
            "delivery_path": os.path.realpath(gc_bin),
            "delivery_binary_sha256": gc_digest,
        },
        "delivery": {
            "native_context_path": os.path.realpath(native_path),
            "native_context_digest": native_digest,
            "evidence_root": os.path.realpath(evidence_root),
            "deadline_seconds": 86400,
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
  native="$canonical_city/.gc/agentops/delivery-native-context.v1.json"
  native_digest="$(shasum -a 256 "$native" | awk '{print $1}')"
  canonical_tmp="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$TMP")"
  fake_gc="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$FAKE_GC")"
  grep -Fq "NATIVE=<$native> NATIVE_DIGEST=<$native_digest> DELIVERY_ROOT=<$canonical_tmp/rig/.gc/agentops/factory/evidence/delivery> DELIVERY_MODE=<auto> DELIVERY_DEADLINE=<86400> BD=<$fake_gc> GIT=<$fake_gc> GH=<$fake_gc> BASH=<$fake_gc> DELIVERY=<$fake_gc> RIG=<agentops> REPOSITORY=<boshu2/agentops> REMOTE=<origin> AO=<$fake_gc>" "$LOG"
  grep -Fq "ARGS <agentops> <program> <start> <--source-bead> <ag-blj> <--max-parallel> <2>" "$LOG"
  ! grep -Fq ' <--city> ' "$LOG"
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
  [[ "$output" == *"native context gc bytes differ from its digest"* ]]

  run "$INVOKE" --city "$CITY" status
  [ "$status" -ne 0 ]
  [[ "$output" == *"expected -- before the Gas City command"* ]]
}

@test "invoke refuses native delivery context bytes outside the ready marker identity" {
  printf ' ' >>"$CITY/.gc/agentops/delivery-native-context.v1.json"

  run "$INVOKE" --city "$CITY" -- status
  [ "$status" -ne 0 ]
  [[ "$output" == *"native context bytes differ from the marker digest"* ]]
}

@test "operator docs use the managed invocation boundary and direct discovered-command flags" {
  run rg -n 'program start -- --source-bead' "$README" "$HELP"
  [ "$status" -eq 1 ]

  rg -Fq 'deploy/gc/invoke.sh --city /path/to/city --' "$README"
  rg -Fq 'agentops program start --source-bead age-example --max-parallel 2' "$README"
  rg -Fq 'enters the exact managed city before executing GC' "$README"
  rg -Fq 'omits the redundant root `--city`' "$README"
  rg -Fq 'gc agentops program start --source-bead ID' "$HELP"
}
