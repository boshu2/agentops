#!/usr/bin/env bash
set -euo pipefail

# Prove the hookless RPI phase path without relying on runtime hooks or live LLM
# access. The fake runtime records prompt sizes and writes the minimal artifacts
# the phased engine expects, so this stays deterministic and CI-safe.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

AO_BIN=""
KEEP_TEMP="false"
OUTPUT_PATH=""
TMP_ROOT=""
BASELINE_TOKENS="10350000"

usage() {
  cat <<'EOF'
test-hookless-rpi-phased.sh

Options:
  --ao-bin <path>     Use an existing ao binary instead of building ./cli/cmd/ao
  --output <path>     Write a JSON proof artifact to this path
  --baseline <tokens> Baseline token count for comparison (default: 10350000)
  --keep-temp         Preserve the temp repo for inspection
  --help              Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ao-bin)
      AO_BIN="${2:-}"
      shift 2
      ;;
    --output)
      OUTPUT_PATH="${2:-}"
      shift 2
      ;;
    --baseline)
      BASELINE_TOKENS="${2:-}"
      shift 2
      ;;
    --keep-temp)
      KEEP_TEMP="true"
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown arg: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

info() {
  echo "INFO: $*" >&2
}

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "Required command not found: $1"
}

cleanup() {
  if [[ "$KEEP_TEMP" != "true" && -n "${TMP_ROOT:-}" && -d "$TMP_ROOT" ]]; then
    rm -rf "$TMP_ROOT"
  fi
}
trap cleanup EXIT

require_cmd bash
require_cmd git
require_cmd go
require_cmd jq
require_cmd wc

case "$BASELINE_TOKENS" in
  ''|*[!0-9]*)
    fail "--baseline must be a positive integer token count"
    ;;
esac

if [[ -n "$OUTPUT_PATH" ]]; then
  mkdir -p "$(dirname "$OUTPUT_PATH")"
fi

TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/hookless-rpi-phased.XXXXXX")"
HOME_ROOT="$TMP_ROOT/home"
RPI_REPO_DIR="$TMP_ROOT/repo"
BROKEN_BD_BIN="$TMP_ROOT/bd-broken.sh"
FAKE_RUNTIME_BIN="$TMP_ROOT/fake-runtime.sh"
GOAL="Hookless RPI phase proof"

mkdir -p "$HOME_ROOT" "$RPI_REPO_DIR"
git init -q "$RPI_REPO_DIR"

if [[ -z "$AO_BIN" ]]; then
  AO_BIN="$TMP_ROOT/ao"
  info "Building ao from current worktree"
  (
    cd "$REPO_ROOT/cli"
    go build -o "$AO_BIN" ./cmd/ao
  )
fi
[[ -x "$AO_BIN" ]] || fail "ao binary is not executable: $AO_BIN"

cat > "$BROKEN_BD_BIN" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'simulated bd unavailable for hookless RPI proof\n' >&2
exit 1
EOF
chmod +x "$BROKEN_BD_BIN"

cat > "$FAKE_RUNTIME_BIN" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

PROMPT=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -p)
      PROMPT="${2:-}"
      shift 2
      ;;
    exec)
      shift
      ;;
    *)
      shift
      ;;
  esac
done

mkdir -p .agents/council .agents/plans .agents/rpi

if grep -q "phase 1 of 3" <<<"$PROMPT"; then
  PHASE=1
elif grep -q "phase 2 of 3" <<<"$PROMPT"; then
  PHASE=2
elif grep -q "phase 3 of 3" <<<"$PROMPT"; then
  PHASE=3
else
  printf 'unrecognized prompt shape\n' >&2
  exit 1
fi

PROMPT_PATH=".agents/rpi/fake-runtime-phase-${PHASE}-prompt.md"
printf '%s' "$PROMPT" > "$PROMPT_PATH"
BYTES="$(wc -c < "$PROMPT_PATH" | tr -d ' ')"
TOKENS="$(( (BYTES + 3) / 4 ))"
if [[ "${AGENTOPS_HOOKS_DISABLED:-}" != "1" ]]; then
  printf 'AGENTOPS_HOOKS_DISABLED was not set for phase %s\n' "$PHASE" >&2
  exit 1
fi

jq -cn \
  --arg phase "$PHASE" \
  --arg bytes "$BYTES" \
  --arg tokens "$TOKENS" \
  --arg prompt_path "$PROMPT_PATH" \
  --arg hooks_disabled "${AGENTOPS_HOOKS_DISABLED:-}" \
  --arg structured_handoffs "$(grep -c 'structured handoffs from prior phases' "$PROMPT_PATH" || true)" \
  '{phase:($phase|tonumber), prompt_bytes:($bytes|tonumber), approx_tokens:($tokens|tonumber), prompt_path:$prompt_path, hooks_disabled:$hooks_disabled, structured_handoff_mentions:($structured_handoffs|tonumber)}' \
  >> .agents/rpi/hookless-rpi-prompt-metrics.jsonl

case "$PHASE" in
  1)
    cat > .agents/plans/2026-05-16-hookless-rpi-proof-plan.md <<'PLAN'
# Hookless RPI Proof Plan

- Goal: prove the phased RPI engine can pass context by execution packet and phase handoffs.
- Tracker: intentionally degraded to tasklist mode.
- Hooks: disabled by AGENTOPS_HOOKS_DISABLED=1.
PLAN
    cat > .agents/council/2026-05-16-hookless-rpi-pre-mortem.md <<'REPORT'
# Hookless RPI Pre-Mortem

## Council Verdict: PASS

The deterministic proof is bounded and exercises the phase handoff path.
REPORT
    cat > .agents/rpi/phase-1-summary.md <<'SUMMARY'
# Phase 1 Summary

Discovery produced a tasklist plan and pre-mortem PASS while hooks were disabled.
SUMMARY
    ;;
  2)
    cat > .agents/rpi/phase-2-summary.md <<'SUMMARY'
# Phase 2 Summary

Implementation consumed .agents/rpi/execution-packet.json in tasklist mode and produced no source edits.
SUMMARY
    ;;
  3)
    cat > .agents/council/2026-05-16-hookless-rpi-vibe.md <<'REPORT'
# Hookless RPI Vibe

## Council Verdict: PASS

Validation confirmed the deterministic hookless phase path.
REPORT
    cat > .agents/council/2026-05-16-hookless-rpi-post-mortem.md <<'REPORT'
# Hookless RPI Post-Mortem

## Council Verdict: PASS

Closeout completed through explicit artifacts.
REPORT
    cat > .agents/rpi/phase-3-summary.md <<'SUMMARY'
# Phase 3 Summary

Validation and closeout completed using explicit artifacts.
SUMMARY
    ;;
esac

printf '<promise>DONE</promise>\n'
EOF
chmod +x "$FAKE_RUNTIME_BIN"

info "Running ao rpi phased with AGENTOPS_HOOKS_DISABLED=1"
(
  cd "$RPI_REPO_DIR"
  HOME="$HOME_ROOT" \
    AGENTOPS_HOOKS_DISABLED=1 \
    AGENTOPS_RPI_BD_COMMAND="$BROKEN_BD_BIN" \
    "$AO_BIN" rpi phased "$GOAL" \
      --runtime direct \
      --runtime-cmd "$FAKE_RUNTIME_BIN" \
      --no-worktree \
      --no-budget \
      --no-dashboard >/dev/null
)

PACKET_PATH="$RPI_REPO_DIR/.agents/rpi/execution-packet.json"
STATE_PATH="$RPI_REPO_DIR/.agents/rpi/phased-state.json"
METRICS_PATH="$RPI_REPO_DIR/.agents/rpi/hookless-rpi-prompt-metrics.jsonl"

[[ -f "$PACKET_PATH" ]] || fail "RPI execution packet missing"
[[ -f "$STATE_PATH" ]] || fail "RPI phased state missing"
[[ -f "$METRICS_PATH" ]] || fail "prompt metrics missing"

jq -e '.tracker_mode == "tasklist"' "$PACKET_PATH" >/dev/null \
  || fail "execution packet did not record tracker_mode=tasklist"
jq -e '.tracker_mode == "tasklist" and .terminal_status == "completed" and .terminal_reason == "all phases completed"' "$STATE_PATH" >/dev/null \
  || fail "phased state did not record completed tasklist run"
jq -e '([.verdicts[]] | all(. == "PASS" or . == "WARN"))' "$STATE_PATH" >/dev/null \
  || fail "phased state contains a non-PASS/WARN verdict"

for phase in 1 2 3; do
  [[ -f "$RPI_REPO_DIR/.agents/rpi/phase-${phase}-handoff.json" ]] \
    || fail "phase ${phase} structured handoff missing"
done

metrics_count="$(jq -s 'length' "$METRICS_PATH")"
[[ "$metrics_count" == "3" ]] || fail "expected prompt metrics for 3 phases, got $metrics_count"
jq -s -e 'all(.[]; .hooks_disabled == "1")' "$METRICS_PATH" >/dev/null \
  || fail "one or more phases did not run with hooks disabled"

total_prompt_bytes="$(jq -s 'map(.prompt_bytes) | add' "$METRICS_PATH")"
total_prompt_tokens="$(jq -s 'map(.approx_tokens) | add' "$METRICS_PATH")"
phase2_structured="$(jq -s 'map(select(.phase == 2))[0].structured_handoff_mentions' "$METRICS_PATH")"
phase3_structured="$(jq -s 'map(select(.phase == 3))[0].structured_handoff_mentions' "$METRICS_PATH")"

if [[ "$phase2_structured" -lt 1 || "$phase3_structured" -lt 1 ]]; then
  fail "phase 2/3 prompts did not include structured handoff context"
fi

reduction_ppm="$(
  jq -n \
    --argjson baseline "$BASELINE_TOKENS" \
    --argjson measured "$total_prompt_tokens" \
    'if $baseline == 0 then 0 else (((($baseline - $measured) * 1000000) / $baseline) | floor) end'
)"

proof_json="$(
  jq -n \
    --arg status "PASS" \
    --arg goal "$GOAL" \
    --arg temp_root "$TMP_ROOT" \
    --arg repo_dir "$RPI_REPO_DIR" \
    --arg packet_path "$PACKET_PATH" \
    --arg state_path "$STATE_PATH" \
    --arg metrics_path "$METRICS_PATH" \
    --argjson baseline_tokens "$BASELINE_TOKENS" \
    --argjson total_prompt_bytes "$total_prompt_bytes" \
    --argjson total_prompt_tokens "$total_prompt_tokens" \
    --argjson reduction_ppm "$reduction_ppm" \
    --slurpfile metrics "$METRICS_PATH" \
    '{
      status: $status,
      goal: $goal,
      hooks_disabled: true,
      runtime: "deterministic fake runtime",
      baseline_tokens: $baseline_tokens,
      measured_surface: "rendered phase prompt payloads, bytes/4 token estimate",
      measured_prompt_bytes: $total_prompt_bytes,
      measured_prompt_tokens: $total_prompt_tokens,
      reduction_vs_baseline_ppm: $reduction_ppm,
      reduction_vs_baseline_percent: (($reduction_ppm / 10000) | tostring + "%"),
      caveat: "This is deterministic prompt-surface telemetry, not provider /usage cache-read telemetry.",
      artifacts: {
        temp_root: $temp_root,
        repo_dir: $repo_dir,
        execution_packet: $packet_path,
        phased_state: $state_path,
        prompt_metrics: $metrics_path
      },
      phase_metrics: $metrics
    }'
)"

if [[ -n "$OUTPUT_PATH" ]]; then
  printf '%s\n' "$proof_json" > "$OUTPUT_PATH"
fi

printf '%s\n' "$proof_json"

if [[ "$KEEP_TEMP" == "true" ]]; then
  info "Temp root preserved: $TMP_ROOT"
fi
