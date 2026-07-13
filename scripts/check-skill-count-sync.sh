#!/usr/bin/env bash
# check-skill-count-sync.sh — the skill COUNT is gated, not vibes
# (age-skills-audit-fable-l6ic.6; audit finding: declared counts drifted 63/66/77
# across surfaces with nothing comparing them).
#
# Compares, and FAILS on mismatch:
#   A. skills/ dirs carrying a SKILL.md (minus _fixtures)
#   B. ACTIVE rows in docs/contracts/skill-dispositions.yaml `dispositions:` with kind: skill
#   C. the total implied by skills/SKILL-TIERS.md's "User-Facing Skills (N)" + "Internal Skills (M)"
#
# Warn-only wiring (GOALS.md Gates row, tags: warn-only): treat a FAIL as a count
# desync to fix in the same change that moved a count.
#
# Exit codes: 0 = all three surfaces agree, 1 = counts disagree.

# Strict mode + hijack-proof REPO_ROOT come from the shared preamble
# (scripts/lib/preamble.sh). `CDPATH=` is an intentional env-prefix, not an
# assignment — hence the SC1007 disable.
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

cd "$REPO_ROOT" || exit 2

a=0
for d in skills/*/; do
  case "$(basename "$d")" in _fixtures|pre-mortem|post-mortem|pre_mortem|post_mortem) continue ;; esac
  [ -f "$d/SKILL.md" ] && a=$((a + 1))
done

b="$(python3 - <<'PY'
import re, pathlib
t = pathlib.Path("docs/contracts/skill-dispositions.yaml").read_text()
t = t[t.index("\ndispositions:"):]
rows = re.findall(r"^  - skill:\s+(\S+)(.*?)(?=^  - skill:|\Z)", t, re.M | re.S)
n = 0
for name, body in rows:
    if re.search(r"^\s+kind:\s+skill\s*$", body, re.M):
        n += 1
print(n)
PY
)"

uf="$(grep -oE 'User-Facing Skills \(([0-9]+)\)' skills/SKILL-TIERS.md | grep -oE '[0-9]+' | head -1 || echo 0)"
intl="$(grep -oE 'Internal Skills \(([0-9]+)\)' skills/SKILL-TIERS.md | grep -oE '[0-9]+' | head -1 || echo 0)"
c=$((uf + intl))

echo "skill-count-sync: dirs=$a dispositions-active=$b tiers-declared=$c (user-facing $uf + internal $intl)"
if [ "$a" -eq "$b" ] && [ "$a" -eq "$c" ]; then
  echo "PASS: all three surfaces agree at $a"
  exit 0
fi
echo "FAIL: counts disagree — fix the surface that moved without its siblings (dirs=$a ledger=$b tiers=$c)" >&2
exit 1
