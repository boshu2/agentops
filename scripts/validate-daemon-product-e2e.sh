#!/usr/bin/env bash
set -uo pipefail

fixture=true
json=false
list_only=false
verbose=false
requested_sections=()

usage() {
  cat <<'USAGE'
Usage: scripts/validate-daemon-product-e2e.sh [--fixture] [--json] [--verbose] [--section <name>]

Runs the daemon product end-to-end fixture gate:
  fake GasCity worker flow, daemon RPI/Dream/wiki smoke paths, ledger replay,
  projection rebuild, state-machine invariants, boundary failpoints, OpenClaw,
  and product runtime doctor checks.

Options:
  --fixture          Force fake/local fixture mode. This is the default.
  --json             Print a JSON summary instead of the text report.
  --verbose          Print go test output for passing sections too.
  --section <name>   Run one named section. May be repeated.
  --list             List section names and exit.
  -h, --help         Show this help.

Environment:
  AGENTOPS_DAEMON_E2E_RUNNER  Optional command used instead of go test.
                              Tests use this to validate orchestration.
USAGE
}

all_sections=(
  fake-gascity
  daemon-product-smoke
  dream-wiki-smoke
  ledger-replay
  projection-rebuild
  state-machine-invariants
  boundary-failpoints
  openclaw-consumer
  doctor-runtime
)

section_description() {
  case "$1" in
    fake-gascity) echo "fake GasCity client, terminal classifier, AgentWorker, and wiki worker fixtures" ;;
    daemon-product-smoke) echo "daemon-backed RPI and Dream CLI smoke fixtures" ;;
    dream-wiki-smoke) echo "Dream job schema plus daemon wiki/forge runner smoke fixtures" ;;
    ledger-replay) echo "authoritative ledger append, idempotence, replay, and quarantine invariants" ;;
    projection-rebuild) echo "RPI, Dream, wiki, and OpenClaw projection rebuild fixtures" ;;
    state-machine-invariants) echo "daemon job transition and Dream stage-manifest invariants" ;;
    boundary-failpoints) echo "mutation, ack, projection, and trigger failpoint boundaries" ;;
    openclaw-consumer) echo "read-only OpenClaw API, trigger policy, and external consumer fixtures" ;;
    doctor-runtime) echo "ao doctor daemon, GasCity bridge, and OpenClaw runtime checks" ;;
    *) echo "" ;;
  esac
}

section_args() {
  case "$1" in
    fake-gascity)
      printf '%s\0' ./internal/gascity ./internal/agentworker ./internal/wikiworker \
        -run 'Test(ClientMutationHeadersAndRequestID|TerminalStateClassifier|GasCityAgentWorker|WikiWorker)'
      ;;
    daemon-product-smoke)
      printf '%s\0' ./cmd/ao \
        -run 'Test(RPIDaemonL3SmokeFakeGasCity|OvernightDaemonReadySubmitsRunJob|OvernightDaemonUnreadyRefusesWithoutFallback|OvernightDaemonFallbackPreservesOneShotPath|DreamDaemonRestartRecoveryIntegration|RunPostLoopTier1Forge_QueuesDaemonWikiForgeWhenReady)'
      ;;
    dream-wiki-smoke)
      printf '%s\0' ./internal/daemon \
        -run 'Test(DreamStageManifestJSONFixtureValidates|DreamJobSpecsValidateAndRoundTrip|WikiForgeJobSpecRoundTrip|WikiForgeRunnerCompletesJobWithAgentWorkerSessionRefs)'
      ;;
    ledger-replay)
      printf '%s\0' ./internal/daemon \
        -run 'Test(LedgerAppendRead|LedgerAppendRejectsInvalidWithoutPartialWrite|LedgerIdempotentAppend|ReplayLedgerDeduplicatesDuplicateEventID|CorruptLedgerRecordsAreQuarantined)'
      ;;
    projection-rebuild)
      printf '%s\0' ./internal/daemon ./internal/openclaw \
        -run 'Test(ProjectionRebuildsRpiDreamWikiAndOpenClawFromLedger|ProjectionReplayFromStoreCarriesRequestIDsAndDegradesOnCorruptLedger|SnapshotProjectionRebuildsFromInput|ExternalStyleOpenClawClientReadsSnapshot)'
      ;;
    state-machine-invariants)
      printf '%s\0' ./internal/daemon \
        -run 'Test(JobStatusTransitionMatrix|DreamStageManifestRejectsInvalidStageModeAndOrder)'
      ;;
    boundary-failpoints)
      printf '%s\0' ./internal/daemon \
        -run 'Test(MutationAckFailpointBeforeAppendNoSideEffect|MutationAckFailpointAfterAppendBeforeAckRecoverable|MutationProjectionFailpointStillAcknowledgesAcceptedJob|AckFailpointAfterAppendBeforeAckIsRecoverableByIdempotency|FailpointBeforeAppendDoesNotAcceptQueueMutation|OpenClawTriggerRequiresAuthAndHasNoSideEffect|OpenClawTriggerAcceptsAllowlistedJobAfterLedgerAck)'
      ;;
    openclaw-consumer)
      printf '%s\0' ./internal/daemon ./internal/openclaw \
        -run 'Test(OpenClawReadOnlyEndpoints|OpenClawReadOnlyEndpointsRejectMutationMethods|OpenClawTriggerRequiresReadyDaemon|OpenClawTriggerRejectsNonAllowlistedJobType|ExternalStyleOpenClawClientReadsSnapshot)'
      ;;
    doctor-runtime)
      printf '%s\0' ./cmd/ao \
        -run 'TestDoctor(DaemonRuntimeCheckPassesWithReadyServer|OpenClawConsumerCheckPassesWithReadyServer|GasCityBridgeCheckUsesDiagnostics)'
      ;;
    *)
      return 1
      ;;
  esac
}

contains_section() {
  local needle="$1"
  local item
  for item in "${all_sections[@]}"; do
    [[ "$item" == "$needle" ]] && return 0
  done
  return 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --fixture)
      fixture=true
      shift
      ;;
    --json)
      json=true
      shift
      ;;
    --verbose)
      verbose=true
      shift
      ;;
    --section)
      if [[ $# -lt 2 ]]; then
        usage >&2
        exit 2
      fi
      requested_sections+=("$2")
      shift 2
      ;;
    --list)
      list_only=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "${#requested_sections[@]}" -gt 0 ]]; then
  for section in "${requested_sections[@]}"; do
    if ! contains_section "$section"; then
      echo "Unknown section: $section" >&2
      echo "Use --list to see valid sections." >&2
      exit 2
    fi
  done
  sections=("${requested_sections[@]}")
else
  sections=("${all_sections[@]}")
fi

if [[ "$list_only" == "true" ]]; then
  for section in "${all_sections[@]}"; do
    printf '%s\t%s\n' "$section" "$(section_description "$section")"
  done
  exit 0
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd "$script_dir/.." && pwd -P)"
cli_root="$repo_root/cli"

if [[ ! -d "$cli_root" ]]; then
  echo "Missing cli module at $cli_root" >&2
  exit 1
fi

if [[ -z "${AGENTOPS_DAEMON_E2E_RUNNER:-}" ]] && ! command -v go >/dev/null 2>&1; then
  echo "Missing go binary and AGENTOPS_DAEMON_E2E_RUNNER is not set" >&2
  exit 1
fi

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/daemon-product-e2e.XXXXXX")"
report_tsv="$tmpdir/report.tsv"
trap 'rm -rf "$tmpdir"' EXIT

export AGENTOPS_DAEMON_E2E_FIXTURE=1
export AGENTOPS_LIVE_GASCITY=0

run_go_test() {
  if [[ -n "${AGENTOPS_DAEMON_E2E_RUNNER:-}" ]]; then
    "$AGENTOPS_DAEMON_E2E_RUNNER" "$@"
  else
    go test "$@"
  fi
}

run_section() {
  local section="$1"
  local output_file="$tmpdir/$section.out"
  local start end duration status exit_code
  local args=()

  while IFS= read -r -d '' arg; do
    args+=("$arg")
  done < <(section_args "$section")

  start="$(date +%s)"
  (
    cd "$cli_root" || exit 1
    export AGENTOPS_DAEMON_E2E_SECTION="$section"
    run_go_test "${args[@]}"
  ) >"$output_file" 2>&1
  exit_code=$?
  end="$(date +%s)"
  duration=$((end - start))

  if [[ "$exit_code" -eq 0 ]]; then
    status="PASS"
  else
    status="FAIL"
  fi

  printf '%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$section" "$status" "$duration" "$exit_code" "${args[*]}" "$output_file" >> "$report_tsv"

  if [[ "$json" != "true" ]]; then
    printf '== %s ==\n' "$section"
    printf '%s\n' "$(section_description "$section")"
    printf 'command: go test %s\n' "${args[*]}"
    printf 'result: %s (%ss)\n' "$status" "$duration"
    if [[ "$status" != "PASS" || "$verbose" == "true" ]]; then
      sed 's/^/  /' "$output_file"
    fi
    printf '\n'
  fi

  return "$exit_code"
}

overall=0
for section in "${sections[@]}"; do
  if ! run_section "$section"; then
    overall=1
  fi
done

if [[ "$json" == "true" ]]; then
  export DAEMON_E2E_REPORT="$report_tsv"
  export DAEMON_E2E_ROOT="$repo_root"
  export DAEMON_E2E_FIXTURE="$fixture"
  daemon_e2e_result="$([[ "$overall" -eq 0 ]] && echo PASS || echo FAIL)"
  export DAEMON_E2E_RESULT="$daemon_e2e_result"
  python3 - <<'PY'
import json
import os
from pathlib import Path

report_path = Path(os.environ["DAEMON_E2E_REPORT"])
sections = []
if report_path.exists():
    for line in report_path.read_text().splitlines():
        section, status, duration, exit_code, command, output_path = line.split("\t", 5)
        output = Path(output_path).read_text(errors="replace")
        tail = "\n".join(output.splitlines()[-40:])
        sections.append({
            "name": section,
            "status": status,
            "duration_seconds": int(duration),
            "exit_code": int(exit_code),
            "command": f"go test {command}",
            "output_tail": tail if status != "PASS" else "",
        })

print(json.dumps({
    "schema_version": 1,
    "result": os.environ["DAEMON_E2E_RESULT"],
    "fixture": os.environ["DAEMON_E2E_FIXTURE"] == "true",
    "root": os.environ["DAEMON_E2E_ROOT"],
    "sections": sections,
}, indent=2, sort_keys=True))
PY
else
  pass_count="$(awk -F '\t' '$2 == "PASS" { count++ } END { print count + 0 }' "$report_tsv")"
  fail_count="$(awk -F '\t' '$2 == "FAIL" { count++ } END { print count + 0 }' "$report_tsv")"
  printf 'daemon product e2e fixture gate: %s (%s pass, %s fail)\n' \
    "$([[ "$overall" -eq 0 ]] && echo PASS || echo FAIL)" "$pass_count" "$fail_count"
fi

exit "$overall"
