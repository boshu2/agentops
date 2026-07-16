#!/usr/bin/env bash
# skill-usage-report.sh — count skill invocations from local Claude Code
# transcripts (age-skills-audit-fable-l6ic.5; automates the 2026-06-10 hand count).
#
# Counts two persisted shapes per transcript line:
#   Skill-tool calls:   "skill":"<name>"  (compact) / "skill": "<name>"
#   slash commands:     <command-name>/<name></command-name>
#
# Segments library / read-not-invoked skills (metadata.internal or the known
# JIT set) so a zero on them is not misread as death. STATED CAVEATS: this
# undercounts — Codex-side sessions, other hosts, and skills consumed by being
# READ as references never appear here. A high count proves life; a zero is
# weak evidence, only meaningful for user-invocable skills.
#
# Usage: scripts/skill-usage-report.sh [--since <days>] [--dir <transcripts-dir>] [--json]
#   --since  lookback in days (default 30)
#   --dir    transcripts dir (default ~/.claude/projects, recursive)
set -euo pipefail

usage() { sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'; }

SINCE_DAYS=30
TDIR="${HOME}/.claude/projects"
AS_JSON=0
while [ $# -gt 0 ]; do
  case "$1" in
    --since) [ $# -ge 2 ] || { echo "skill-usage: --since needs a value" >&2; exit 2; }
             SINCE_DAYS="$2"; shift 2 ;;
    --dir)   [ $# -ge 2 ] || { echo "skill-usage: --dir needs a value" >&2; exit 2; }
             TDIR="$2"; shift 2 ;;
    --json)  AS_JSON=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "skill-usage: unknown arg $1 (see --help)" >&2; exit 2 ;;
  esac
done
[ -d "$TDIR" ] || { echo "skill-usage: FAIL — no transcripts dir at $TDIR (remedy: pass --dir)" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "skill-usage: FAIL — jq required" >&2; exit 1; }

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

# Library / read-not-invoked set: metadata.internal skills plus the known JIT
# libraries — invocation counts do not apply to them.
lib_set() {
  local d
  for d in "$REPO_ROOT"/skills/*/; do
    [ -f "$d/SKILL.md" ] || continue
    if /usr/bin/grep -q '^  internal: true' "$d/SKILL.md" 2>/dev/null \
       || /usr/bin/grep -q '^internal: true' "$d/SKILL.md" 2>/dev/null; then
      basename "$d"
    fi
  done
  printf '%s\n' standards shared domain
}

# Count both persisted shapes across transcripts newer than the window.
counts() {
  # find -mtime is day-granular; matches the hand-count method.
  /usr/bin/find "$TDIR" -name '*.jsonl' -mtime "-${SINCE_DAYS}" -size +0c 2>/dev/null \
    | while IFS= read -r f; do
        /usr/bin/grep -ho '"skill": *"[a-zA-Z0-9:_-]*"' "$f" 2>/dev/null \
          | sed -E 's/.*"skill": *"([^"]*)".*/\1/'
        /usr/bin/grep -ho '<command-name>/[a-zA-Z0-9:_-]*</command-name>' "$f" 2>/dev/null \
          | sed -E 's#<command-name>/([^<]*)</command-name>#\1#'
      done | sed 's#^.*:##' | sort | uniq -c | sort -rn
}

raw="$(counts)"
libs="$(lib_set | sort -u)"

if [ "$AS_JSON" -eq 1 ]; then
  printf '%s\n' "$raw" | awk 'NF==2 {printf "{\"skill\":\"%s\",\"count\":%s}\n", $2, $1}' \
    | jq -s --arg since "$SINCE_DAYS" --arg dir "$TDIR" --argjson libs "$(printf '%s\n' "$libs" | jq -R . | jq -s .)" '
      { since_days: ($since|tonumber), transcripts_dir: $dir,
        caveats: ["undercounts codex-side sessions","undercounts other hosts","read-as-reference consumption invisible","zero is weak evidence; high count is conclusive"],
        library_read_not_invoked: $libs,
        invocations: (map(select(.skill as $s | $libs | index($s) | not))),
        library_hits: (map(select(.skill as $s | $libs | index($s)))) }'
  exit 0
fi

echo "Skill usage — last ${SINCE_DAYS} days ($TDIR)"
echo
echo "CAVEATS: undercounts Codex-side, other hosts, and read-as-reference use."
echo "A high count is conclusive life; a zero is weak evidence (library skills segmented below)."
echo
echo "user-invocable invocations:"
printf '%s\n' "$raw" | while read -r c s; do
  [ -n "${s:-}" ] || continue
  if printf '%s\n' "$libs" | /usr/bin/grep -qx "$s"; then continue; fi
  printf '  %6s  %s\n' "$c" "$s"
done
echo
echo "library / read-not-invoked (counts not meaningful):"
printf '%s\n' "$raw" | while read -r c s; do
  [ -n "${s:-}" ] || continue
  if printf '%s\n' "$libs" | /usr/bin/grep -qx "$s"; then printf '  %6s  %s\n' "$c" "$s"; fi
done
echo
echo "usage is advisory evidence; skills/*/SKILL.md metadata remains the inventory source."
