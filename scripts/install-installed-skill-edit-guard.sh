#!/usr/bin/env bash
# install-installed-skill-edit-guard.sh — opt-in installer for the
# installed-skill-edit PreToolUse guard (age-workflow-guardrail-hooks-j39.1).
#
# AgentOps is hookless by default — this guard ships INERT. Run this script
# explicitly to activate it. It copies the guard into ~/.claude/hooks/ and adds a
# PreToolUse Edit|Write matcher to a Claude settings.json. Idempotent: re-running
# is a no-op once wired. Nothing here runs at build/install-of-skills time.
#
# Usage:
#   scripts/install-installed-skill-edit-guard.sh            # user settings (~/.claude/settings.json)
#   scripts/install-installed-skill-edit-guard.sh --project  # project settings (.claude/settings.json)
#   SETTINGS=/path/to/settings.json scripts/install-installed-skill-edit-guard.sh
set -euo pipefail
shopt -s lastpipe 2>/dev/null || true
umask 022

# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
repo_root="$(resolve_repo_root)"
src="${repo_root}/skills/cc-hooks/hooks/installed-skill-edit-guard.sh"
[[ -f "$src" ]] || { echo "ERROR: guard script missing: ${src}" >&2; exit 1; }
command -v jq >/dev/null || { echo "ERROR: jq required" >&2; exit 1; }

# Resolve target settings file.
settings="${SETTINGS:-}"
if [[ -z "$settings" ]]; then
  case "${1:-}" in
    --project) settings=".claude/settings.json" ;;
    *)         settings="${HOME}/.claude/settings.json" ;;
  esac
fi

# Install the guard into ~/.claude/hooks/ (referenced by absolute path).
hooks_dir="${HOME}/.claude/hooks"
mkdir -p "$hooks_dir"
dst="${hooks_dir}/installed-skill-edit-guard.sh"
install -m 0755 "$src" "$dst"
echo "✓ installed ${dst}"

# Merge the PreToolUse Edit|Write matcher into settings.json (idempotent).
mkdir -p "$(dirname "$settings")"
[[ -f "$settings" ]] || echo '{}' > "$settings"

# Timestamped backup before mutating settings (installer-workmanship).
if [[ -f "$settings" && -s "$settings" ]]; then
  backup="${settings}.bak.$(date +%Y%m%d%H%M%S)"
  cp -p "$settings" "$backup"
  echo "✓ backed up settings → ${backup}"
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
jq --arg cmd "$dst" '
  .hooks //= {} |
  .hooks.PreToolUse //= [] |
  if any(.hooks.PreToolUse[]?; (.hooks // [])[]?.command == $cmd)
  then .
  else .hooks.PreToolUse += [{
    "matcher": "Edit|Write",
    "hooks": [ { "type": "command", "command": $cmd } ]
  }]
  end
' "$settings" > "$tmp" && mv "$tmp" "$settings"
trap - EXIT

if grep -qF "$dst" "$settings"; then
  echo "✓ wired Edit|Write PreToolUse guard into ${settings}"
else
  echo "ERROR: failed to wire guard into ${settings}" >&2
  exit 1
fi

echo ""
echo "Installed-skill-edit guard active for this Claude scope."
echo "It is SILENT on every non-installed-skill edit; fires once per session on a"
echo "*/.claude/skills/** (or .codex/.gemini) Edit/Write, routing you to repo skills/."
echo "Uninstall: remove the PreToolUse matcher for ${dst} from ${settings}, then rm -f ${dst}"
