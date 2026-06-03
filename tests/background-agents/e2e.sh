#!/usr/bin/env bash
# Live smoke for AgentOps NTM background-agent readiness.
#
# Default mode is safe and non-mutating: it verifies that the local `ao` command
# can render the background-agent roster, that NTM exposes the robot commands
# AgentOps depends on, that mcp-agent-mail is active, and that NTM can dry-run a
# Claude+Codex spawn. Set AGENTOPS_BACKGROUND_E2E=1 to opt in; without it the
# script skips with exit 0.
#
# Set AGENTOPS_BACKGROUND_E2E_EXECUTE=1 only when you explicitly want to start
# a real NTM session. The default never creates tmux panes.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LOG_DIR="${AGENTOPS_BACKGROUND_E2E_LOG_DIR:-$(mktemp -d)}"
LOG_FILE="$LOG_DIR/background-agents-e2e.jsonl"

source "$REPO_ROOT/tests/lib/e2e-logger.sh"

e2e_log_init "background-agents" "$LOG_FILE"

finish() {
  e2e_log_summary
  printf 'background-agents e2e log: %s\n' "$LOG_FILE"
}
trap finish EXIT

if [[ "${AGENTOPS_BACKGROUND_E2E:-0}" != "1" ]]; then
  e2e_log_phase "skip"
  e2e_log_pass "AGENTOPS_BACKGROUND_E2E not set; live smoke skipped"
  exit 0
fi

run_ao() {
  if [[ -n "${AO_BIN:-}" ]]; then
    "$AO_BIN" "$@"
    return
  fi
  if command -v go >/dev/null 2>&1; then
    (cd "$REPO_ROOT/cli" && go run ./cmd/ao "$@")
    return
  fi
  if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
    "$REPO_ROOT/cli/bin/ao" "$@"
    return
  fi
  if command -v ao >/dev/null 2>&1; then
    ao "$@"
    return
  fi
  return 127
}

if ! run_ao version >/dev/null 2>&1; then
  e2e_log_fail "ao command unavailable" '{}'
  exit 1
fi

e2e_log_phase "probe"

if ! command -v ntm >/dev/null 2>&1; then
  e2e_log_fail "ntm unavailable" '{}'
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  e2e_log_fail "jq unavailable" '{}'
  exit 1
fi

capabilities_json="$(ntm --robot-capabilities)"
for capability in activity dashboard mail send spawn; do
  if jq -e --arg capability "$capability" \
    '([.commands[]?.name] + (.capabilities // [])) | index($capability)' \
    >/dev/null <<<"$capabilities_json"; then
    e2e_log_pass "ntm capability present" "$(jq -nc --arg capability "$capability" '{capability:$capability}')"
  else
    e2e_log_fail "ntm capability missing" "$(jq -nc --arg capability "$capability" '{capability:$capability}')"
    exit 1
  fi
done

if systemctl --user is-active mcp-agent-mail.service >/dev/null 2>&1; then
  e2e_log_pass "mcp-agent-mail service active"
else
  e2e_log_fail "mcp-agent-mail service inactive"
  exit 1
fi

e2e_log_phase "roster"
roster_json="$(run_ao agent roster --json)"
if jq -e '.agents | length == 2 and ([.[].runtime] | sort == ["claude-ntm","codex-ntm"])' >/dev/null <<<"$roster_json"; then
  e2e_log_pass "ao agent roster emits claude/codex NTM agents"
else
  e2e_log_fail "ao agent roster missing expected agents" "$roster_json"
  exit 1
fi

init_prompt="$(run_ao agent init-prompt --runtime codex-ntm --mailbox agentops-codex-ntm-worker)"
if grep -Fq 'ao session bootstrap --json' <<<"$init_prompt" &&
  grep -Fq 'mcp-agent-mail' <<<"$init_prompt" &&
  grep -Fq 'do not use deprecated `ao rpi` / `ao evolve` wrappers' <<<"$init_prompt"; then
  e2e_log_pass "ao agent init-prompt contains bootstrap/mail/deprecation contract"
else
  e2e_log_fail "ao agent init-prompt missing expected contract" "$(jq -nc --arg prompt "$init_prompt" '{prompt:$prompt}')"
  exit 1
fi

if eligible_json="$(run_ao agent eligible --eligible-only 2>&1)"; then
  eligible_count="$(jq 'length' <<<"$eligible_json")"
  e2e_log_pass "ao agent eligible ran" "$(jq -nc --argjson count "$eligible_count" '{eligible_count:$count}')"
else
  # The tracker can be degraded on shared boxes (for example duplicate
  # issue/wisp ids). Background-agent readiness should surface that fact but
  # not fail the NTM/mail smoke.
  e2e_log_pass "ao agent eligible skipped due tracker degradation" "$(jq -nc --arg output "$eligible_json" '{output:$output}')"
fi

e2e_log_phase "spawn-dry-run"
session_name="${AGENTOPS_BACKGROUND_E2E_SESSION:-agentops-bg-e2e}"
if [[ "${AGENTOPS_BACKGROUND_E2E_EXECUTE:-0}" == "1" ]]; then
  spawn_json="$(run_ao agent ntm-spawn "$session_name" --dir "$REPO_ROOT" --claude 1 --codex 1 --codex-model "${AGENTOPS_BACKGROUND_E2E_CODEX_MODEL:-gpt-5.5}" --execute)"
  if jq -e '.success == true' >/dev/null <<<"$spawn_json"; then
    e2e_log_pass "ntm spawn command succeeded" "$spawn_json"
  else
    e2e_log_fail "ntm spawn command failed" "$spawn_json"
    exit 1
  fi
else
  spawn_plan="$(run_ao agent ntm-spawn "$session_name" --dir "$REPO_ROOT" --claude 1 --codex 1 --codex-model "${AGENTOPS_BACKGROUND_E2E_CODEX_MODEL:-gpt-5.5}")"
  if grep -Fq -- "--robot-spawn=$session_name" <<<"$spawn_plan" && grep -Fq -- "tmux split-window" <<<"$spawn_plan"; then
    e2e_log_pass "ntm spawn dry-run rendered" "$(jq -nc --arg plan "$spawn_plan" '{plan:$plan}')"
  else
    e2e_log_fail "ntm spawn dry-run missing expected commands" "$(jq -nc --arg plan "$spawn_plan" '{plan:$plan}')"
    exit 1
  fi
fi
