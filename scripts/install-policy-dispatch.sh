#!/usr/bin/env bash
# install-policy-dispatch.sh — opt-in installer for the PreToolUse policy
# dispatcher + policies-as-data registry (age-bhsz, epic age-4qw1).
#
# AgentOps is hookless by default — the engine ships INERT. Run this script
# explicitly to activate it on a host. It copies the dispatcher and the policy
# registry into ~/.claude/hooks/aop/ and wires PreToolUse matchers for Bash and
# Edit|Write into a Claude settings.json. Idempotent: re-running refreshes the
# installed copies and is a settings no-op once wired.
#
# Usage:
#   scripts/install-policy-dispatch.sh            # user settings (~/.claude/settings.json)
#   scripts/install-policy-dispatch.sh --project  # project settings (.claude/settings.json)
#   SETTINGS=/path/to/settings.json scripts/install-policy-dispatch.sh
set -euo pipefail
umask 022

# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
repo_root="$(resolve_repo_root)"
src_dispatch="${repo_root}/skills/cc-hooks/hooks/policy-dispatch.sh"
src_policies="${repo_root}/skills/cc-hooks/policies/policies.json"
lint="${repo_root}/skills/cc-hooks/scripts/lint-policies.sh"
[[ -f "$src_dispatch" ]] || { echo "ERROR: dispatcher missing: ${src_dispatch}" >&2; exit 1; }
[[ -f "$src_policies" ]] || { echo "ERROR: registry missing: ${src_policies}" >&2; exit 1; }
command -v jq >/dev/null || { echo "ERROR: jq required" >&2; exit 1; }

# Never install a registry that fails its own contract.
bash "$lint" "$src_policies"

settings="${SETTINGS:-}"
if [[ -z "$settings" ]]; then
  case "${1:-}" in
    --project) settings=".claude/settings.json" ;;
    *)         settings="${HOME}/.claude/settings.json" ;;
  esac
fi

hooks_dir="${HOME}/.claude/hooks/aop"
mkdir -p "$hooks_dir"
install -m 0755 "$src_dispatch" "${hooks_dir}/policy-dispatch.sh"
install -m 0644 "$src_policies" "${hooks_dir}/policies.json"
dst="${hooks_dir}/policy-dispatch.sh"
echo "✓ installed ${dst} (+ policies.json beside it)"

mkdir -p "$(dirname "$settings")"
[[ -f "$settings" ]] || echo '{}' > "$settings"

if [[ -s "$settings" ]]; then
  backup="${settings}.bak.$(date +%Y%m%d%H%M%S)"
  cp -p "$settings" "$backup"
  echo "✓ backed up settings → ${backup}"
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
jq --arg cmd "$dst" '
  .hooks //= {} |
  .hooks.PreToolUse //= [] |
  reduce ("Bash", "Edit|Write") as $m (.;
    if any(.hooks.PreToolUse[]?; .matcher == $m and any((.hooks // [])[]?; .command == $cmd))
    then .
    else .hooks.PreToolUse += [{
      "matcher": $m,
      "hooks": [ { "type": "command", "command": $cmd } ]
    }]
    end
  )
' "$settings" > "$tmp" && mv "$tmp" "$settings"
trap - EXIT

if grep -qF "$dst" "$settings"; then
  echo "✓ wired PreToolUse (Bash, Edit|Write) policy dispatcher into ${settings}"
else
  echo "ERROR: failed to wire dispatcher into ${settings}" >&2
  exit 1
fi

echo ""
echo "Policy dispatcher active for this Claude scope. SILENT on every clean call;"
echo "deny policies block with a one-line route to the correct tool; every fire"
echo "lands one hashed telemetry line in \${AGENTOPS_HOME:-~/.agents/ao}/guardrail-telemetry.jsonl."
echo "Waive once:  AOP_WAIVE=<policy-id> <your command>"
echo "Uninstall:   remove the two PreToolUse matchers for ${dst} from ${settings}, then rm -rf ${hooks_dir}"
