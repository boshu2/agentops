#!/usr/bin/env bash
# check-corpus-secret-scan.sh — git-safety CI backstop (ag-y2xy).
#
# Scrub-on-write (corpus_fs.Writer.Capture + `ao redact` in compile) is the
# PRIMARY defense. This gate is the BACKSTOP for the corpus content that is
# actually committed: repo-root .agents/ is local-only by policy
# (scripts/check-no-tracked-agents.sh keeps it untracked except a narrow
# allowlist), so a blanket scan would see almost nothing. We therefore scan
# ONLY git-tracked .agents markdown/JSONL + any committed canon projection.
# If a secret pattern reaches a committed corpus file, fail with file:line.
set -euo pipefail

# High-signal credential patterns (mirror of the canonical llm.Redact set —
# the subset that is unambiguous enough for a zero-false-positive CI gate).
PATTERNS='sk-ant-[A-Za-z0-9_-]{40,}|sk-[A-Za-z0-9]{32,}|AKIA[A-Z0-9]{16}|gh[pousr]_[A-Za-z0-9]{36,}|glpat-[A-Za-z0-9_-]{20,}|xox[baprs]-[0-9A-Za-z-]{20,}|ya29\.[A-Za-z0-9_-]+|AIza[A-Za-z0-9_-]{35}|-----BEGIN [A-Z ]*PRIVATE KEY-----'

# Only tracked (= committed/allowlisted) corpus files are in scope.
mapfile -t files < <(git ls-files -- '.agents/**/*.md' '.agents/**/*.jsonl' '.agents/canon/**/*.md' 2>/dev/null | sort -u)

if [ "${#files[@]}" -eq 0 ]; then
  echo "check-corpus-secret-scan: ok (no tracked corpus files in scope)"
  exit 0
fi

fail=0
for f in "${files[@]}"; do
  [ -f "$f" ] || continue
  if grep -nEH "$PATTERNS" -- "$f"; then
    fail=1
  fi
done

if [ "$fail" -ne 0 ]; then
  echo "check-corpus-secret-scan: FAIL — secret pattern in committed .agents corpus." >&2
  echo "  fix: scrub before commit (corpus writes go through llm.Redact / 'ao redact'); never commit raw credentials." >&2
  exit 1
fi

echo "check-corpus-secret-scan: ok (${#files[@]} tracked corpus file(s) scanned, clean)"
