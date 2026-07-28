#!/usr/bin/env bash
# Behavioral self-test for the converter. Replaces the prior vacuous check
# ("SKILL.md exists" only, which passed while the pipeline could delete its own
# source) with real proofs (CV-2):
#   1. a happy-path conversion writes the expected target + passthrough files;
#   2. the destructive clean-write path is CLOSED — an output dir equal to, or
#      an ancestor of, the source package is refused and the source survives
#      (CV-1: a source-dir output previously went 4 files -> 2 at exit 0).
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONVERT="$SKILL_DIR/scripts/convert.sh"
FAIL=0
pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1"; FAIL=$((FAIL + 1)); }

if [[ -f "$SKILL_DIR/SKILL.md" ]]; then pass "SKILL.md exists"; else fail "SKILL.md exists"; fi
if [[ -f "$CONVERT" ]]; then pass "convert.sh exists"; else fail "convert.sh exists"; fi

# Disposable fixture, left in place on exit (no rm -rf): the guard under test
# must never be handed a destructive cleanup to imitate.
WORK="$(mktemp -d "${TMPDIR:-/tmp}/converter-selftest.XXXXXX")"
FIX="$WORK/fixture-skill"
mkdir -p "$FIX/references"
printf -- '---\nname: fixture-skill\ndescription: converter self-test fixture\n---\n# Body\n' > "$FIX/SKILL.md"
printf 'reference payload\n' > "$FIX/references/note.md"
before_count="$(find "$FIX" -type f | wc -l | tr -d ' ')"

# 1. Happy path: a distinct output dir converts and writes the target files.
out="$WORK/out"
if bash "$CONVERT" "$FIX" codex "$out" >/dev/null 2>&1 \
  && [[ -f "$out/SKILL.md" && -f "$out/prompt.md" && -f "$out/references/note.md" ]]; then
  pass "happy-path conversion writes target + passthrough files"
else
  fail "happy-path conversion writes target + passthrough files"
fi

# 2. Destructive path closed: output == source must be refused (nonzero) AND
# leave every source file intact.
if bash "$CONVERT" "$FIX" codex "$FIX" >/dev/null 2>&1; then
  fail "clean-write onto the source package is refused"
else
  after_count="$(find "$FIX" -type f | wc -l | tr -d ' ')"
  if [[ "$after_count" == "$before_count" ]]; then
    pass "clean-write onto the source package is refused; source intact ($after_count files)"
  else
    fail "source package damaged by clean-write onto itself ($before_count -> $after_count files)"
  fi
fi

# 3. Ancestor output dir (contains the source) must also be refused.
if bash "$CONVERT" "$FIX" codex "$WORK" >/dev/null 2>&1; then
  fail "clean-write onto an ancestor of the source is refused"
else
  pass "clean-write onto an ancestor of the source is refused"
fi

echo ""
if [[ "$FAIL" -eq 0 ]]; then
  echo "converter self-test: PASS"
  exit 0
fi
echo "converter self-test: FAIL ($FAIL failed)" >&2
exit 1
