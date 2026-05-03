#!/usr/bin/env bash
# nightly-knowledge-cycle.sh — Shared substrate for the nightly knowledge-cycle
# workstream (compile + dream-cycle + Athena, deduped per soc-2xmg).
#
# Purpose:
#   1. Provide a single corpus-empty precondition check used by every nightly
#      knowledge-cycle gate (compile health, dream-cycle proof, Athena follow-up).
#      Implements f-2026-04-30-002: when the upstream corpus is empty, return
#      SKIP with reason "corpus-empty"; when it has artifacts but zero citation
#      activity in the measurement window, return SKIP with reason
#      "corpus-dormant" rather than FAIL. This stops three separate jobs from
#      going spuriously red on the same unavailable-corpus condition.
#
#   2. Emit a single consolidated triage artifact (nightly-knowledge-cycle/
#      triage.json + triage.md) so on-call reads one log per nightly, not three.
#
# Shape:
#   ao metrics report --json
#       .total_artifacts
#       .citations_this_period       (citations within the report's period)
#
# Subcommands:
#   precondition    Decide RUN | SKIP for the cycle. Emits triage.json/md and
#                   exits 0 in either case. Reads $AO and writes to
#                   $NIGHTLY_KNOWLEDGE_CYCLE_DIR/triage.{json,md}.
#                   Exit 1 only on hard input errors (e.g. ao not runnable).
#
#   record-stage    Append a stage result to the consolidated triage report.
#                   Usage: record-stage <stage> <status> [<notes>]
#                          status ∈ {ok, warn, skip, fail}
#
# Environment:
#   AO                          Path to ao binary (required for `precondition`).
#   NIGHTLY_KNOWLEDGE_CYCLE_DIR Output dir for triage.{json,md} (required).
#   NIGHTLY_KNOWLEDGE_CYCLE_FORCE  Set to 1 to force RUN regardless of corpus
#                               state (escape hatch for diagnostic runs).
#
# Time-deferred acceptance criterion:
#   The 5-consecutive-nightlies green-or-skip gate from soc-2xmg cannot be
#   verified in a single session. This script implements the dedupe + skip
#   substrate; the acceptance signal closes once 5 nightly runs land.

set -euo pipefail

usage() {
  cat <<'EOF'
nightly-knowledge-cycle.sh <subcommand> [args]

Subcommands:
  precondition                       Decide RUN | SKIP. Exit 0 always (unless
                                     hard error reading inputs).
  record-stage <name> <status> [note]  Append stage to consolidated triage.
                                     status ∈ {ok, warn, skip, fail}
  read-decision                      Print "RUN" or "SKIP" from triage.json.
                                     Exit 0 on RUN, 78 on SKIP, 2 on missing.

Environment:
  AO                          Path to ao binary
  NIGHTLY_KNOWLEDGE_CYCLE_DIR Output directory for the consolidated triage
  NIGHTLY_KNOWLEDGE_CYCLE_FORCE=1   Override SKIP (diagnostic runs)
EOF
}

die() {
  echo "nightly-knowledge-cycle: $*" >&2
  exit 1
}

require_dir() {
  [[ -n "${NIGHTLY_KNOWLEDGE_CYCLE_DIR:-}" ]] \
    || die "NIGHTLY_KNOWLEDGE_CYCLE_DIR is required"
  mkdir -p "$NIGHTLY_KNOWLEDGE_CYCLE_DIR"
}

require_jq() {
  command -v jq >/dev/null 2>&1 || die "jq not found on PATH"
}

triage_json_path() {
  echo "$NIGHTLY_KNOWLEDGE_CYCLE_DIR/triage.json"
}

triage_md_path() {
  echo "$NIGHTLY_KNOWLEDGE_CYCLE_DIR/triage.md"
}

cmd_precondition() {
  require_dir
  require_jq

  local ao="${AO:-${AO_BIN:-}}"
  [[ -n "$ao" ]] || die "AO env var must point to the ao binary"
  [[ -x "$ao" || "$ao" != */* ]] || die "ao binary not executable: $ao"

  local triage_json triage_md
  triage_json="$(triage_json_path)"
  triage_md="$(triage_md_path)"

  local generated_at
  generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

  # Capture metrics report. Tolerate non-zero exit (treat as RUN with warning)
  # because a metrics-report failure is itself signal worth running the cycle on.
  local metrics_json metrics_status
  if ! metrics_json="$("$ao" metrics report --json 2>/dev/null)"; then
    metrics_json='{}'
    metrics_status="unavailable"
  else
    metrics_status="ok"
  fi

  local total_artifacts citations_in_window
  total_artifacts="$(echo "$metrics_json" | jq -r '.total_artifacts // 0')"
  citations_in_window="$(echo "$metrics_json" | jq -r '.citations_this_period // 0')"

  # Coerce to integer or fall back to 0 if jq returned non-numeric.
  [[ "$total_artifacts" =~ ^[0-9]+$ ]] || total_artifacts=0
  [[ "$citations_in_window" =~ ^[0-9]+$ ]] || citations_in_window=0

  local decision reason
  if [[ "${NIGHTLY_KNOWLEDGE_CYCLE_FORCE:-0}" == "1" ]]; then
    decision="RUN"
    reason="forced via NIGHTLY_KNOWLEDGE_CYCLE_FORCE=1"
  elif [[ "$metrics_status" != "ok" ]]; then
    decision="RUN"
    reason="metrics-report-unavailable: ao metrics report failed; running nightly knowledge-cycle for diagnostic signal"
  elif (( total_artifacts == 0 )); then
    # f-2026-04-30-002 corpus-empty precondition.
    decision="SKIP"
    reason="corpus-empty: total_artifacts=0; no checked-in corpus is available for nightly knowledge-cycle work"
  elif (( citations_in_window == 0 )); then
    # f-2026-04-30-002 corpus-dormant precondition.
    decision="SKIP"
    reason="corpus-dormant: total_citations_in_window=0 with total_artifacts=${total_artifacts}; gate cannot be moved by single-session work"
  else
    decision="RUN"
    reason="corpus-active: citations_in_window=${citations_in_window}, total_artifacts=${total_artifacts}"
  fi

  jq -n \
    --arg generated_at "$generated_at" \
    --arg decision "$decision" \
    --arg reason "$reason" \
    --arg metrics_status "$metrics_status" \
    --argjson total_artifacts "$total_artifacts" \
    --argjson citations_in_window "$citations_in_window" \
    '{
      generated_at: $generated_at,
      decision: $decision,
      reason: $reason,
      precondition: {
        total_artifacts: $total_artifacts,
        citations_in_window: $citations_in_window,
        metrics_status: $metrics_status,
        finding: "f-2026-04-30-002"
      },
      stages: []
    }' >"$triage_json"

  {
    echo "## Nightly Knowledge Cycle — Triage"
    echo ""
    echo "- Generated: ${generated_at}"
    echo "- Decision: **${decision}**"
    echo "- Reason: ${reason}"
    echo "- total_artifacts: ${total_artifacts}"
    echo "- citations_in_window: ${citations_in_window}"
    echo "- metrics_status: ${metrics_status}"
    echo "- Finding: \`f-2026-04-30-002\` (corpus availability precondition)"
    echo ""
    echo "### Stages"
    echo ""
    echo "_(populated as compile/dream-cycle stages report in)_"
    echo ""
  } >"$triage_md"

  echo "decision=${decision}"
  echo "reason=${reason}"
  echo "total_artifacts=${total_artifacts}"
  echo "citations_in_window=${citations_in_window}"
}

cmd_record_stage() {
  require_dir
  require_jq

  local stage="${1:-}"
  local status="${2:-}"
  local notes="${3:-}"

  [[ -n "$stage" && -n "$status" ]] || die "record-stage requires <stage> <status>"

  case "$status" in
    ok|warn|skip|fail) ;;
    *) die "status must be one of: ok, warn, skip, fail (got: $status)" ;;
  esac

  local triage_json triage_md
  triage_json="$(triage_json_path)"
  triage_md="$(triage_md_path)"

  if [[ ! -f "$triage_json" ]]; then
    # Bootstrap a minimal triage.json so record-stage works even when called
    # before precondition (defensive — should not happen in practice).
    jq -n \
      --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      '{generated_at: $generated_at, decision: "UNKNOWN", reason: "no precondition recorded", stages: []}' \
      >"$triage_json"
    : >"$triage_md"
  fi

  local now
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

  local tmp
  tmp="$(mktemp)"
  jq \
    --arg name "$stage" \
    --arg status "$status" \
    --arg notes "$notes" \
    --arg recorded_at "$now" \
    '.stages += [{name: $name, status: $status, notes: $notes, recorded_at: $recorded_at}]' \
    "$triage_json" >"$tmp"
  mv "$tmp" "$triage_json"

  printf -- "- [%s] **%s**: %s — %s\n" "$now" "$stage" "$status" "$notes" >>"$triage_md"
}

cmd_read_decision() {
  require_dir
  require_jq
  local triage_json
  triage_json="$(triage_json_path)"
  if [[ ! -f "$triage_json" ]]; then
    echo "ERROR: $triage_json not found — run 'precondition' first" >&2
    exit 2
  fi
  local decision
  decision="$(jq -r '.decision // "UNKNOWN"' "$triage_json")"
  echo "$decision"
  case "$decision" in
    RUN) exit 0 ;;
    SKIP) exit 78 ;;
    *) exit 2 ;;
  esac
}

main() {
  if [[ $# -eq 0 ]]; then
    usage >&2
    exit 1
  fi
  local sub="$1"
  shift
  case "$sub" in
    precondition) cmd_precondition "$@" ;;
    record-stage) cmd_record_stage "$@" ;;
    read-decision) cmd_read_decision "$@" ;;
    -h|--help|help) usage ;;
    *)
      echo "unknown subcommand: $sub" >&2
      usage >&2
      exit 1
      ;;
  esac
}

main "$@"
