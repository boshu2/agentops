#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/check-competitive-freshness.sh"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

make_repo() {
  local path="$1"
  mkdir -p "$path/docs/comparisons"
  git -C "$path" init -q
  git -C "$path" config user.email test@example.com
  git -C "$path" config user.name "Competitive Freshness Test"
}

write_doc() {
  local path="$1"
  local title="$2"
  cat > "$path" <<EOF
# $title

Reviewed for competitive freshness.
EOF
}

test_passes_with_required_radar() {
  local fixture="$TMP_DIR/fresh"
  make_repo "$fixture"
  write_doc "$fixture/docs/comparisons/vs-example.md" "Example"
  write_doc "$fixture/docs/comparisons/competitive-radar.md" "Radar"
  git -C "$fixture" add docs/comparisons
  git -C "$fixture" commit -q -m "docs: add fresh comparisons"

  if COMPETITIVE_FRESHNESS_ROOT="$fixture" bash "$SCRIPT" >/dev/null; then
    pass "passes when comparison docs and radar are fresh"
  else
    fail "should pass with fresh docs and radar"
  fi
}

test_fails_without_required_radar() {
  local fixture="$TMP_DIR/missing-radar"
  make_repo "$fixture"
  write_doc "$fixture/docs/comparisons/vs-example.md" "Example"
  git -C "$fixture" add docs/comparisons
  git -C "$fixture" commit -q -m "docs: add comparison"

  if COMPETITIVE_FRESHNESS_ROOT="$fixture" bash "$SCRIPT" >/dev/null 2>&1; then
    fail "should fail when competitive-radar.md is missing"
  else
    pass "fails when competitive-radar.md is missing"
  fi
}

test_uncommitted_radar_counts_as_current() {
  local fixture="$TMP_DIR/uncommitted-radar"
  make_repo "$fixture"
  write_doc "$fixture/docs/comparisons/vs-example.md" "Example"
  git -C "$fixture" add docs/comparisons
  git -C "$fixture" commit -q -m "docs: add comparison"
  write_doc "$fixture/docs/comparisons/competitive-radar.md" "Radar"

  if COMPETITIVE_FRESHNESS_ROOT="$fixture" bash "$SCRIPT" >/dev/null; then
    pass "passes for newly added working-tree radar before commit"
  else
    fail "should use working-tree timestamp for new radar docs"
  fi
}

test_fails_stale_committed_docs() {
  local fixture="$TMP_DIR/stale"
  make_repo "$fixture"
  write_doc "$fixture/docs/comparisons/vs-example.md" "Example"
  write_doc "$fixture/docs/comparisons/competitive-radar.md" "Radar"
  git -C "$fixture" add docs/comparisons
  GIT_AUTHOR_DATE="2000-01-01T00:00:00Z" \
    GIT_COMMITTER_DATE="2000-01-01T00:00:00Z" \
    git -C "$fixture" commit -q -m "docs: add stale comparisons"

  if COMPETITIVE_FRESHNESS_ROOT="$fixture" bash "$SCRIPT" >/dev/null 2>&1; then
    fail "should fail stale committed docs"
  else
    pass "fails stale committed docs"
  fi
}

echo "== test-competitive-freshness =="
test_passes_with_required_radar
test_fails_without_required_radar
test_uncommitted_radar_counts_as_current
test_fails_stale_committed_docs

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
