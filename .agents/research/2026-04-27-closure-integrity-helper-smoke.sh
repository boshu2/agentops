#!/usr/bin/env bash
set -uo pipefail

SCRIPT="skills/post-mortem/scripts/closure-integrity-audit.sh"

DEPS="$(awk '
  /^json_array_from_stream\(\)/ { capture=1 }
  /^run_git_clean\(\)/ { capture=1 }
  /^regex_escape_extended\(\)/ { capture=1 }
  capture { print }
  capture && /^}/ { capture=0 }
' "$SCRIPT")"

HELPERS="$(awk '
  /^bd_show_json\(\)/ { capture=1 }
  /^extract_description_from_show_text\(\)/ { capture=1 }
  /^extract_close_reason_from_show_text\(\)/ { capture=1 }
  /^issue_audit_text\(\)/ { capture=1 }
  capture { print }
  capture && /^}/ { capture=0 }
' "$SCRIPT")"

eval "$DEPS"
eval "$HELPERS"

beads=(ag-0af ag-x4g ag-8v8)
hits=0
fail=0
for b in "${beads[@]}"; do
  text="$(issue_audit_text "$b" 2>/dev/null)"
  if [[ -z "$text" ]]; then
    echo "FAIL: $b — issue_audit_text returned empty"
    fail=1
    continue
  fi
  close_reason="$(bd show "$b" --json 2>/dev/null | jq -r '.[0].close_reason // ""')"
  if [[ -n "$close_reason" ]]; then
    snippet="${close_reason:0:40}"
    if grep -qF "$snippet" <<<"$text"; then
      echo "PASS: $b — close_reason content surfaced via issue_audit_text"
      echo "  snippet: ${snippet}..."
      hits=$((hits + 1))
    else
      echo "FAIL: $b — has close_reason but it's NOT in audit text"
      echo "  close_reason: ${close_reason:0:80}"
      echo "  audit_text[0:200]: ${text:0:200}"
      fail=1
    fi
  else
    echo "INFO: $b — no close_reason"
  fi
done

echo
echo "Summary: $hits of ${#beads[@]} beads surfaced close_reason"
if [[ "$hits" -ge 1 && "$fail" -eq 0 ]]; then
  echo "ACCEPTANCE: PASS (>= 1 close_reason surfaced, no failures)"
  exit 0
else
  echo "ACCEPTANCE: FAIL"
  exit 1
fi
