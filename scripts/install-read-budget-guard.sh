#!/usr/bin/env bash
# install-read-budget-guard.sh — opt-in installer for the read-budget
# PreToolUse guard (policy core.context:unbounded-read).
#
# AgentOps is hookless by default — this guard ships INERT. Run this script
# explicitly to activate it. It copies the guard into ~/.claude/hooks/ and adds a
# PreToolUse Read|Bash matcher to a Claude settings.json. Idempotent: re-running
# is a no-op once wired. Nothing here runs at build/install-of-skills time, and
# hooks/hooks.json is never touched.
#
# Usage:
#   scripts/install-read-budget-guard.sh            # user settings (~/.claude/settings.json)
#   scripts/install-read-budget-guard.sh --project  # project settings (.claude/settings.json)
#   SETTINGS=/path/to/settings.json scripts/install-read-budget-guard.sh

# The preamble sets strict mode and exports a CWD-hijack-proof REPO_ROOT.
# `CDPATH=` is an intentional env-prefix (clears CDPATH for that one cd), not a
# botched assignment — hence the SC1007 disable, matching scripts/lib/preamble.sh.
# SC1091: the preamble is a sourced library resolved at runtime, never a lint
# input (same reason as scripts/gc-maintainer-ops.sh).
# shellcheck disable=SC1091
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"
shopt -s lastpipe 2>/dev/null || true
umask 022

src="$REPO_ROOT/skills/cc-hooks/hooks/read-budget-guard.sh"
[[ -f "$src" ]] || { echo "ERROR: guard script missing: ${src}" >&2; exit 1; }
require_cmd jq

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
dst="${hooks_dir}/read-budget-guard.sh"
install -m 0755 "$src" "$dst"
echo "✓ installed ${dst}"

# Merge the PreToolUse Read|Bash matcher into settings.json (idempotent).
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
    "matcher": "Read|Bash",
    "hooks": [ { "type": "command", "command": $cmd } ]
  }]
  end
' "$settings" > "$tmp" && mv "$tmp" "$settings"
trap - EXIT

if grep -qF "$dst" "$settings"; then
  echo "✓ wired Read|Bash PreToolUse guard into ${settings}"
else
  echo "ERROR: failed to wire guard into ${settings}" >&2
  exit 1
fi

echo ""
echo "Read-budget guard active for this Claude scope."
echo "It is SILENT on every bounded or at-budget read; it blocks (exit 2) an unbounded"
echo "Read / cat / head / tail over AOP_READ_BUDGET_LINES (default 350) lines, routing"
echo "you to a slice (offset+limit / sed -n) or a bulk-reader delegation."
echo "Uninstall: remove the PreToolUse matcher for ${dst} from ${settings}, then rm -f ${dst}"
