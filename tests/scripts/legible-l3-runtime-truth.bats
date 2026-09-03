#!/usr/bin/env bats
# L3 (legible membrane plan, docs/plans/2026-09-02-legible-membrane-plan.md):
# README.md and docs/install-day2-ops.md must state the true runtime
# requirements: which skills execute `ao` or a `.py` script and cannot
# complete without it, which do so only on some branch (conditional), and
# which merely mention one and complete without it (optional). Both docs
# must carry the identical table, every row must be grounded in the target
# skill's own SKILL.md rather than a passing mention, an unconditional
# ("HARD") row must additionally show a real invocation line, and neither
# doc may claim "No other runtime is required" (F4).

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  README="$REPO_ROOT/README.md"
  INSTALL_DOC="$REPO_ROOT/docs/install-day2-ops.md"
  SKILLS_DIR="$REPO_ROOT/skills"
}

# Data rows only: "| `skill` | ... " (excludes the header and separator
# rows, which do not start with a backtick-quoted skill name).
extract_table_rows() {
  grep -E '^\| `[a-z-]+` \|' "$1"
}

# skill<TAB>needs<TAB>why for every data row, parsed from the table itself
# (never hardcoded) so the test tracks whatever the docs actually claim.
parse_rows() {
  python3 - "$1" <<'PY'
import re
import sys

path = sys.argv[1]
with open(path) as f:
    text = f.read()

for line in text.splitlines():
    if not re.match(r"^\| `[a-z-]+` \|", line):
        continue
    cells = [c.strip() for c in line.strip().strip("|").split("|")]
    if len(cells) < 3:
        continue
    skill = cells[0].strip("`")
    needs = cells[1]
    why = cells[2]
    print(f"{skill}\t{needs}\t{why}")
PY
}

# A row's skill is delegation-only when its behavior is entirely "apply
# <other skill> exactly as written". Grounding checks then run against the
# delegate's SKILL.md, not the alias's own (near-empty) file. No alias row
# exists today (the last one, goals -> fitness, was deleted 2026-09-03);
# add a case arm here if one returns.
ground_skill_for() {
  case "$1" in
    *) echo "$1" ;;
  esac
}

# Unconditional dependency: Needs is exactly "`python3`" or "`ao`", with no
# ", conditional" / ", optional" qualifier: this skill's procedure cannot
# complete without it on every path.
is_hard() {
  case "$1" in
    '`python3`'|'`ao`') return 0 ;;
    *) return 1 ;;
  esac
}

@test "README.md and docs/install-day2-ops.md carry the identical runtime-requirements table" {
  readme_rows="$(extract_table_rows "$README")"
  install_rows="$(extract_table_rows "$INSTALL_DOC")"
  [ -n "$readme_rows" ]
  [ -n "$install_rows" ]
  diff <(echo "$readme_rows") <(echo "$install_rows")
}

@test "every table row is grounded in the target skill's own SKILL.md, not a mention elsewhere" {
  while IFS=$'\t' read -r skill needs why; do
    target="$(ground_skill_for "$skill")"
    file="$SKILLS_DIR/$target/SKILL.md"
    [ -f "$file" ] || {
      echo "missing skill file for row '$skill' (target '$target'): $file" >&2
      return 1
    }

    # Weak grounding for every row: at least one backtick-quoted span in the
    # Why column must appear verbatim in the target's own SKILL.md.
    found=0
    while IFS= read -r span; do
      [ -z "$span" ] && continue
      if grep -Fq -- "$span" "$file"; then
        found=1
        break
      fi
    done < <(printf '%s' "$why" | grep -oE '`[^`]+`' | tr -d '`')
    if [ "$found" -ne 1 ]; then
      echo "row '$skill' (target '$target'): no backtick span from Why ('$why') found in $file" >&2
      return 1
    fi

    # Strong grounding for HARD rows only: a real invocation line, not prose
    # that merely names the command or script.
    if is_hard "$needs"; then
      if ! grep -qE '^[[:space:]]*(\$ )?(python3 |ao [a-z]+|bash .*\.py|.*scripts/[a-z_]+\.py)' "$file"; then
        echo "HARD row '$skill' (target '$target'): no invocation line matched in $file" >&2
        return 1
      fi
    fi
  done < <(parse_rows "$README")
}

@test "'No other runtime is required' appears nowhere under README.md or docs/ (dated plans excluded: they record the finding)" {
  run grep -rn --exclude-dir=plans -- "No other runtime is required" "$README" "$REPO_ROOT/docs"
  [ "$status" -ne 0 ]
  [ -z "$output" ]
}
