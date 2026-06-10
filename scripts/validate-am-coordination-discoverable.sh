#!/usr/bin/env bash
# Validator for cp-jgcl: a fresh agent must be able to DISCOVER how to send an
# Agent Mail message from the skills, and the documented CLI verb must exist.
# Guards against regressing the agent-mail / using-atm discoverability fix.
#
# Exit 0 = pass, 1 = fail. Pure read-only checks; safe to run anywhere `am` is on PATH.
set -uo pipefail
fail=0
note() { printf '%s %s\n' "$1" "$2"; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AM_SKILL="$ROOT/skills/agent-mail/SKILL.md"
ATM_SKILL="$ROOT/skills/using-atm/SKILL.md"

# 1) The working send verb must actually exist in the am CLI.
if am mail send --help >/dev/null 2>&1; then
  note "PASS" "'am mail send' verb exists in the am CLI"
else
  note "FAIL" "'am mail send --help' did not succeed (send verb missing/moved)"; fail=1
fi

# 2) Both coordination skills must document the CLI send verb (not only the MCP tool).
for f in "$AM_SKILL" "$ATM_SKILL"; do
  if grep -q "am mail send" "$f" 2>/dev/null; then
    note "PASS" "$(basename "$(dirname "$f")")/SKILL.md documents 'am mail send'"
  else
    note "FAIL" "$(basename "$(dirname "$f")")/SKILL.md does NOT document 'am mail send'"; fail=1
  fi
done

# 3) The agent-mail skill must warn that 'am send' (flat) is NOT the verb (the trap).
if grep -q "am send" "$AM_SKILL" 2>/dev/null && grep -qi "does not exist\|not exist\|trap" "$AM_SKILL" 2>/dev/null; then
  note "PASS" "agent-mail skill flags the 'am send' discoverability trap"
else
  note "WARN" "agent-mail skill does not explicitly flag the 'am send' trap"
fi

# 4) Informational: has the deeper am-binary fix landed (a flat 'am send' alias)?
if am send --help >/dev/null 2>&1; then
  note "INFO" "'am send' flat alias now works — binary discoverability fix landed; skill trap note can be relaxed"
else
  note "INFO" "'am send' flat alias still absent (expected until the am-binary fix in cp-jgcl follow-up)"
fi

exit $fail
