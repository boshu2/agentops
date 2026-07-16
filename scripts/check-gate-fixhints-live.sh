#!/usr/bin/env bash
# check-gate-fixhints-live.sh — a gate whose repair instruction names a DEAD
# command is a self-inflicted wedge (age-verification-economics-ebec.12).
#
# On 2026-07-07 the corpus-freshness gate BLOCKED a land with 'fix: ao corpus
# snapshot' — but `ao corpus` had been ARCHIVED, so the gate failed closed with no
# working way out; the operator burned time discovering the hint was a lie (the
# audit's "embedded fix-hints are CLAIMS, not orders", now as a gate defect).
#
# This scans the gate-backing scripts for fix/remedy/repair directives that name
# an `ao <subcommand>` and asserts the subcommand resolves in the LIVE ao command
# tree. A dead one is reported UNLESS the same line carries a removal/historical
# marker (removed|retired|historical|was|formerly) — so a line that documents
# what a command used to be is correctly not flagged.
#
# Posture: WARN-ONLY by default (GOALS row, tags: warn-only) — prints offenders,
# exits 0. --strict (or AGENTOPS_GATE_FIXHINTS_STRICT=1) exits nonzero on any dead
# hint, for the flip to blocking after a clean baseline.
#
# Exit: 0 clean (or warn-only with offenders), 1 dead hint under --strict, 2 env.

# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

cd "$REPO_ROOT" || exit 2

strict=0
[[ "${AGENTOPS_GATE_FIXHINTS_STRICT:-0}" == "1" ]] && strict=1
[[ "${1:-}" == "--strict" ]] && strict=1

# Live top-level ao commands (column 1 of the completion listing). If ao is
# unavailable, skip cleanly — this is a hygiene gate, never a hard dependency.
ao_bin="$(command -v ao 2>/dev/null || true)"
if [[ -z "$ao_bin" ]]; then
  echo "check-gate-fixhints-live: SKIP — ao not on PATH (hygiene gate, not a hard dep)"
  exit 0
fi
live_cmds="$("$ao_bin" __complete "" 2>/dev/null | awk 'NF{print $1}' | grep -E '^[a-z]' || true)"
if [[ -z "$live_cmds" ]]; then
  echo "check-gate-fixhints-live: SKIP — could not enumerate live ao commands"
  exit 0
fi

removal_re='restore|removed|retired|archived|historical|legacy|formerly|\bwas\b|deprecated|superseded'
offenders=0

# Fix-directive lines that reference `ao <subcommand>`, across the gate-backing scripts.
while IFS= read -r hit; do
  file="${hit%%:*}"; rest="${hit#*:}"; lineno="${rest%%:*}"; text="${rest#*:}"
  # skip shell comments — the actionable fix-hints live in echo'd output, not in
  # the code's own documentation (a comment may name a dead command as an example,
  # like this very script's docstring; that is not a directive the operator runs).
  if [[ "$(printf '%s' "$text" | sed 's/^[[:space:]]*//')" == \#* ]]; then continue; fi
  # skip lines documenting a removed command (historical reference, not a live directive)
  if grep -qiE "$removal_re" <<<"$text"; then continue; fi
  # extract each `ao <subcommand>` token on the line
  while read -r sub; do
    [[ -n "$sub" ]] || continue
    if ! grep -qx "$sub" <<<"$live_cmds"; then
      echo "DEAD-FIXHINT: $file:$lineno names \`ao $sub\` which is not a live ao command"
      echo "   line: $(printf '%s' "$text" | sed 's/^[[:space:]]*//' | cut -c1-120)"
      offenders=$((offenders + 1))
    fi
  done < <(grep -oE '\bao [a-z][a-z-]*' <<<"$text" | awk '{print $2}' | sort -u)
done < <(grep -rnE '([Ff]ix|[Rr]emedy|[Rr]epair|[Rr]un):.*\bao [a-z]' scripts/*.sh 2>/dev/null || true)

if [[ "$offenders" -eq 0 ]]; then
  echo "check-gate-fixhints-live: PASS — every gate fix-hint names a live ao command."
  exit 0
fi
echo "check-gate-fixhints-live: found $offenders dead fix-hint(s) — a gate that blocks with a dead repair command is a self-inflicted wedge. Point the hint at a live command or add a removal marker." >&2
[[ "$strict" -eq 1 ]] && exit 1
echo "(warn-only; set --strict / AGENTOPS_GATE_FIXHINTS_STRICT=1 to block)"
exit 0
