#!/usr/bin/env bash
# Behavioral contract: authorized conversion succeeds atomically; caller-owned,
# escaped, symlinked, and oversized surfaces fail before mutation.
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONVERT="$SKILL_DIR/scripts/convert.sh"
FAIL=0
pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1" >&2; FAIL=$((FAIL + 1)); }

WORK="$(mktemp -d "${TMPDIR:-/tmp}/converter-selftest.XXXXXX")"
trap 'rm -rf -- "$WORK"' EXIT
FIX="$WORK/fixture-skill"
mkdir -p "$FIX/references" "$FIX/scripts"
printf -- '---\nname: fixture-skill\ndescription: converter self-test fixture\n---\n# Body\n' > "$FIX/SKILL.md"
printf 'reference payload\n' > "$FIX/references/note.md"
printf 'echo fixture\n' > "$FIX/scripts/tool.sh"

dir_digest() {
  ( cd "$1" && find . -type f -exec shasum -a 256 {} + | LC_ALL=C sort ) \
    | shasum -a 256 | awk '{print $1}'
}
SRC_DIGEST="$(dir_digest "$FIX")"

assert_refused_unchanged() {
  local desc="$1" output="$2" before after rc
  before="$(dir_digest "$FIX")"
  set +e
  bash "$CONVERT" --output-root "$WORK" "$FIX" codex "$output" >"$WORK/command.log" 2>&1
  rc=$?
  set -e
  after="$(dir_digest "$FIX")"
  if [[ "$rc" -ne 0 && "$before" == "$after" ]]; then
    pass "$desc"
  else
    sed -n '1,120p' "$WORK/command.log" >&2
    fail "$desc"
  fi
}

if bash "$CONVERT" --output-root "$WORK" "$FIX" codex out \
  && [[ -f "$WORK/out/SKILL.md" && -f "$WORK/out/prompt.md" \
     && -f "$WORK/out/references/note.md" && -f "$WORK/out/scripts/tool.sh" \
     && -f "$WORK/out/.agentops-converter-owned-v1" \
     && "$(dir_digest "$FIX")" == "$SRC_DIGEST" ]]; then
  pass "authorized output writes converted and passthrough files without source mutation"
else
  fail "authorized output writes converted and passthrough files without source mutation"
fi

# A marked converter output may be atomically refreshed.
printf 'stale\n' > "$WORK/out/stale.txt"
if bash "$CONVERT" --output-root "$WORK" "$FIX" codex out \
  && [[ ! -e "$WORK/out/stale.txt" && -f "$WORK/out/SKILL.md" ]]; then
  pass "converter-owned output refreshes atomically"
else
  fail "converter-owned output refreshes atomically"
fi

# Baseline defect: clean-write deleted arbitrary caller output. Candidate keeps
# a near-identical unmarked directory byte-for-byte unchanged.
mkdir "$WORK/caller-output"
printf 'do not delete\n' > "$WORK/caller-output/sentinel"
victim_before="$(dir_digest "$WORK/caller-output")"
assert_refused_unchanged "caller-owned output is refused" caller-output
[[ "$(dir_digest "$WORK/caller-output")" == "$victim_before" ]] \
  && pass "caller-owned sentinel survives refusal" \
  || fail "caller-owned sentinel survives refusal"

assert_refused_unchanged "output equal to source is refused" fixture-skill
assert_refused_unchanged "out-of-root traversal is refused" ../escape
assert_refused_unchanged "absolute output is refused" "$WORK/absolute"

# Source links were previously dereferenced by rsync --copy-links.
printf 'external secret\n' > "$WORK/external-secret"
ln -s "$WORK/external-secret" "$FIX/references/linked-secret"
assert_refused_unchanged "source symlink is refused before copying" linked-out
[[ ! -e "$WORK/linked-out" ]] \
  && pass "symlink target was not copied" \
  || fail "symlink target was not copied"
rm "$FIX/references/linked-secret"

# A symlink output itself must never be followed or replaced.
mkdir "$WORK/external-dir"
printf 'outside\n' > "$WORK/external-dir/sentinel"
ln -s "$WORK/external-dir" "$WORK/output-link"
assert_refused_unchanged "symlink output is refused" output-link
[[ "$(<"$WORK/external-dir/sentinel")" == "outside" ]] \
  && pass "symlink destination remains unchanged" \
  || fail "symlink destination remains unchanged"

# Finite source bounds fail closed without creating the output.
truncate -s $((16 * 1024 * 1024 + 1)) "$FIX/oversized.bin"
assert_refused_unchanged "oversized source file is refused" oversized-out
[[ ! -e "$WORK/oversized-out" ]] \
  && pass "oversized source produced no output" \
  || fail "oversized source produced no output"

if [[ "$FAIL" -eq 0 ]]; then
  echo "converter behavioral matrix: PASS"
  exit 0
fi
echo "converter behavioral matrix: FAIL ($FAIL failed)" >&2
exit 1
