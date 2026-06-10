#!/usr/bin/env bash
# test-skill-count-ssot.sh — red-green acceptance for the skill-count SSOT (cp-9wvq).
#
# Proves the load-bearing property: the skill count derives from ONE source (the
# set of skill directories), so adding a skill and running the sync keeps the
# doc-release skill-count gate GREEN with ZERO manual doc edits — and a
# hand-typed wrong count is still CAUGHT.
#
# Mechanism under test:
#   scripts/ensure-skill-tiers-rows.sh  — auto-renders a SKILL-TIERS.md row for
#                                         any skill dir missing one (the manual
#                                         step that used to block releases).
#   scripts/sync-skill-counts.sh        — derives the count from disk and patches
#                                         every doc surface (incl. same-width
#                                         ASCII-diagram counts).
#   tests/docs/validate-skill-count.sh  — the gate that verifies docs == derived.
#
# This test mutates the working tree under a fixture skill dir + tracked docs,
# then restores everything via git. It refuses to run on a dirty relevant tree
# so it can never lose real edits.
set -euo pipefail
export LC_ALL=C

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$REPO_ROOT"

FIXTURE_NAME="zzz-skill-count-ssot-fixture"
FIXTURE_DIR="skills/$FIXTURE_NAME"

# Docs the sync patches — snapshot/restore set.
DOCS=(
  skills/SKILL-TIERS.md
  docs/SKILLS.md
  docs/ARCHITECTURE.md
  PRODUCT.md
  docs/index.md
  docs/documentation-index.md
  docs/GLOSSARY.md
  docs/agentops-system-map.md
  docs/agentops-brief.md
)

fail() { echo "FAIL: $*" >&2; exit 1; }

# --- Guard: refuse to run if the surfaces we mutate already have local edits ---
if [[ -e "$FIXTURE_DIR" ]]; then
  fail "fixture dir already exists: $FIXTURE_DIR (leftover from a prior run?)"
fi
for d in "${DOCS[@]}"; do
  if ! git diff --quiet -- "$d" 2>/dev/null; then
    fail "refusing to run: $d has uncommitted changes (would be clobbered by restore)"
  fi
done

SNAP_DIR="$(mktemp -d)"
restore() {
  # Restore every patched doc from the snapshot, remove the fixture.
  for d in "${DOCS[@]}"; do
    if [[ -f "$SNAP_DIR/$(basename "$d")" ]]; then
      cp "$SNAP_DIR/$(basename "$d")" "$d"
    fi
  done
  if [[ -d "$FIXTURE_DIR" ]]; then
    rm -f "$FIXTURE_DIR/SKILL.md"
    rmdir "$FIXTURE_DIR" 2>/dev/null || true
  fi
  rm -rf "$SNAP_DIR"
}
trap restore EXIT

for d in "${DOCS[@]}"; do
  cp "$d" "$SNAP_DIR/$(basename "$d")"
done

echo "=== Baseline: doc-release skill-count gate is GREEN ==="
bash tests/docs/validate-skill-count.sh >/dev/null \
  || fail "baseline skill-count gate is already red — fix the tree before testing"
bash scripts/sync-skill-counts.sh --check >/dev/null \
  || fail "baseline sync --check is already red — fix the tree before testing"
echo "PASS: baseline green"
echo ""

echo "=== RED: add a skill dir, run gate with ZERO doc edits (must FAIL) ==="
mkdir -p "$FIXTURE_DIR"
cat > "$FIXTURE_DIR/SKILL.md" <<'EOF'
---
name: zzz-skill-count-ssot-fixture
description: Fixture skill proving the count derives from disk (cp-9wvq red-green test).
---
# fixture
EOF
if bash tests/docs/validate-skill-count.sh >/dev/null 2>&1; then
  fail "expected the gate to FAIL after adding a skill with no doc edits, but it passed"
fi
echo "PASS: gate correctly fails before sync (proves the count is enforced)"
echo ""

echo "=== GREEN: run sync (auto-row + patch), gate must PASS with ZERO hand-edits ==="
bash scripts/sync-skill-counts.sh >/dev/null || fail "sync-skill-counts.sh errored"
bash tests/docs/validate-skill-count.sh >/dev/null \
  || fail "skill-count gate still red after sync (SSOT did not propagate)"
bash scripts/sync-skill-counts.sh --check >/dev/null \
  || fail "sync --check red after sync (a doc surface did not get patched)"
# Confirm the auto-row landed and zero human edited SKILL-TIERS by hand.
grep -q "^| \*\*${FIXTURE_NAME}\*\*" skills/SKILL-TIERS.md \
  || fail "auto-row for the fixture skill was not rendered into SKILL-TIERS.md"
echo "PASS: gate green after sync, zero manual doc edits, auto-row present"
echo ""

echo "=== NEGATIVE: a hand-typed WRONG count must still be CAUGHT ==="
# Corrupt one surface to a bogus value and confirm the gate catches it.
perl -0pi -e 's/(Skills system — )\d+( skills,)/${1}99999${2}/' PRODUCT.md
if bash tests/docs/validate-skill-count.sh >/dev/null 2>&1; then
  fail "gate passed with a deliberately wrong count (99999) — SSOT not enforced"
fi
echo "PASS: wrong hand-typed count is caught"
echo ""

echo "PASS: skill-count SSOT red-green acceptance complete"
