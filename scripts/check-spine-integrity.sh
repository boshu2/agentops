#!/usr/bin/env bash
#
# check-spine-integrity.sh — behavioral durability guard for the 15-skill
# membrane + bookkeeper spine (age-focus-membrane-bookkeeper-m1wg.23).
#
# This is a REAL behavioral probe, not a doc/content check (the repo's
# "doc-instruction-is-inert" lesson): it EXECUTES each spine skill's own
# validate.sh + the schema contract, and reads the ACTUAL router order in
# docs/SKILLS.md. Fail-closed — exits non-zero if any spine skill is missing,
# unflagged, invalid, or no longer leads the router, so the factory can't
# silently re-bury the partition.
#
# Exit 0 iff ALL of (a) EXISTS, (b) FLAGGED, (c) VALIDATES, (d) LEADS ROUTER
# hold. macOS bash 3.2 compatible (no mapfile/readarray).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

# The 13 canonical spine slugs — the anti-re-bury anchor. This pinned list is
# what forces a failure if the factory ever drops or renames a spine skill.
# 2026-07-07 retire wave (age-skills-audit-fable-l6ic.12, audit
# docs/audits/skills-audit-2026-07-06.md): review merged into validate
# (--mode=pr) and red-team retired (validate --debate absorbs) — 15 -> 13.
SPINE=(
  # membrane (7)
  validate council pre-mortem converge security reality-check pawl-review
  # bookkeeper (6)
  beads-br status handoff discovery plan implement
)

ROUTER="docs/SKILLS.md"

fail() { echo "FAIL(spine-integrity): $*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# (a) EXISTS — every pinned spine skill has a SKILL.md.
# ---------------------------------------------------------------------------
for s in "${SPINE[@]}"; do
  [ -f "skills/$s/SKILL.md" ] || fail "spine skill missing: skills/$s/SKILL.md"
done

# ---------------------------------------------------------------------------
# (b) FLAGGED — each carries top-level `spine: true`; exactly 15 repo-wide;
#     the flagged set equals the pinned set (no missing, no stray).
# ---------------------------------------------------------------------------
for s in "${SPINE[@]}"; do
  grep -q '^spine: true$' "skills/$s/SKILL.md" \
    || fail "skills/$s/SKILL.md lacks top-level 'spine: true' frontmatter"
done

flagged_count="$(grep -l '^spine: true$' skills/*/SKILL.md 2>/dev/null | wc -l | tr -d ' ' || true)"
[ "$flagged_count" = "13" ] \
  || fail "expected exactly 13 skills flagged 'spine: true' repo-wide, found ${flagged_count:-0}"

flagged_sorted="$(grep -l '^spine: true$' skills/*/SKILL.md 2>/dev/null \
  | sed 's#^skills/##; s#/SKILL\.md$##' | sort || true)"
pinned_sorted="$(printf '%s\n' "${SPINE[@]}" | sort)"

stray="$(comm -23 <(printf '%s\n' "$flagged_sorted") <(printf '%s\n' "$pinned_sorted") || true)"
[ -z "$stray" ] || fail "skills flagged 'spine: true' but NOT in the pinned spine set: $(echo $stray)"

missing="$(comm -13 <(printf '%s\n' "$flagged_sorted") <(printf '%s\n' "$pinned_sorted") || true)"
[ -z "$missing" ] || fail "pinned spine skills NOT flagged 'spine: true': $(echo $missing)"

# ---------------------------------------------------------------------------
# (c) VALIDATES — run each spine skill's own validate.sh where present (only
#     ~8 of 15 ship one; run-where-present, never require the file), and run
#     the schema contract once. All must exit 0.
# ---------------------------------------------------------------------------
for s in "${SPINE[@]}"; do
  v="skills/$s/scripts/validate.sh"
  if [ -f "$v" ]; then
    bash "$v" >/dev/null 2>&1 || fail "skills/$s/scripts/validate.sh returned non-zero"
  fi
done

bash scripts/validate-skill-schema.sh >/dev/null 2>&1 \
  || fail "scripts/validate-skill-schema.sh failed — spine frontmatter breaks the schema contract"

# ---------------------------------------------------------------------------
# (d) LEADS ROUTER — the 15 slugs lead docs/SKILLS.md, inside explicit
#     BEGIN/END spine markers, ahead of every other skill-listing section.
# ---------------------------------------------------------------------------
[ -f "$ROUTER" ] || fail "router missing: $ROUTER"

begin_line="$(grep -n '<!-- BEGIN:spine -->' "$ROUTER" | head -1 | cut -d: -f1)"
end_line="$(grep -n '<!-- END:spine -->' "$ROUTER" | head -1 | cut -d: -f1)"
[ -n "$begin_line" ] || fail "$ROUTER missing '<!-- BEGIN:spine -->' marker"
[ -n "$end_line" ]   || fail "$ROUTER missing '<!-- END:spine -->' marker"
[ "$begin_line" -lt "$end_line" ] \
  || fail "BEGIN:spine (line $begin_line) is not before END:spine (line $end_line)"

region="$(sed -n "${begin_line},${end_line}p" "$ROUTER")"
for s in "${SPINE[@]}"; do
  printf '%s\n' "$region" | grep -qF "/$s" \
    || fail "spine skill '/$s' not referenced inside the SKILLS.md spine region"
done

# The spine block must LEAD: both markers precede the first OTHER ('## ')
# skill-listing section. '## Flow Skills' is the first non-spine one; if the
# factory reorders the router so a non-spine section comes first, this fails.
flow_line="$(grep -n '^## Flow Skills' "$ROUTER" | head -1 | cut -d: -f1)"
[ -n "$flow_line" ] || fail "$ROUTER missing '## Flow Skills' section (router shape changed)"
[ "$begin_line" -lt "$flow_line" ] \
  || fail "spine region does not lead the router (BEGIN at $begin_line, Flow Skills at $flow_line)"
[ "$end_line" -lt "$flow_line" ] \
  || fail "spine region overruns the rest of the router (END at $end_line, Flow Skills at $flow_line)"

echo "PASS(spine-integrity): 13 spine skills exist, flagged spine:true, validate, and lead $ROUTER"
