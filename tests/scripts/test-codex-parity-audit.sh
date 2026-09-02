#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
AUDIT_SCRIPT="$ROOT/scripts/audit-codex-parity.sh"
AUDIT_IMPL="$ROOT/scripts/audit-codex-parity.py"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

setup_repo() {
  local repo="$1"

  mkdir -p \
    "$repo/scripts" \
    "$repo/skills-codex/example/references" \
    "$repo/skills-codex-overrides/example"

  cp "$AUDIT_SCRIPT" "$repo/scripts/audit-codex-parity.sh"
  cp "$AUDIT_IMPL" "$repo/scripts/audit-codex-parity.py"
  chmod +x "$repo/scripts/audit-codex-parity.sh" "$repo/scripts/audit-codex-parity.py"

  cat > "$repo/skills-codex/example/SKILL.md" <<'EOF'
---
name: example
description: fixture
---

# Example

Clean skill body.
EOF

  cat > "$repo/skills-codex/example/references/guide.md" <<'EOF'
# Guide

Clean reference.
EOF

  cat > "$repo/skills-codex-overrides/example/SKILL.md" <<'EOF'
---
name: example
description: fixture override
---

# Override

Clean override.
EOF

  cat > "$repo/skills-codex-overrides/catalog.json" <<'EOF'
{
  "version": 1,
  "waves": [
    {"id": "fixture", "description": "fixture"}
  ],
  "skills": [
    {"name": "example", "treatment": "bespoke", "wave": "fixture", "reason": "fixture"}
  ]
}
EOF
}

test_passes_on_clean_fixture() {
  local repo="$TMP_DIR/clean"
  setup_repo "$repo"

  if (cd "$repo" && bash scripts/audit-codex-parity.sh >/dev/null); then
    pass "passes on clean skills, refs, and overrides"
  else
    fail "should pass on clean fixture"
  fi
}

test_fails_on_reference_drift() {
  local repo="$TMP_DIR/reference-drift"
  setup_repo "$repo"
  cat > "$repo/skills-codex/example/references/guide.md" <<'EOF'
# Guide

Use spawn_agents_on_csv for this workflow.
EOF

  if (cd "$repo" && bash scripts/audit-codex-parity.sh >/dev/null 2>&1); then
    fail "should fail on stale syntax inside references"
  else
    pass "fails on stale syntax inside references"
  fi
}

test_fails_on_override_drift() {
  local repo="$TMP_DIR/override-drift"
  setup_repo "$repo"
  cat > "$repo/skills-codex-overrides/example/SKILL.md" <<'EOF'
---
name: example
description: fixture override
---

# Override

wait(timeout_seconds=300)
EOF

  if (cd "$repo" && bash scripts/audit-codex-parity.sh >/dev/null 2>&1); then
    fail "should fail on stale syntax inside overrides"
  else
    pass "fails on stale syntax inside overrides"
  fi
}

test_allows_negative_examples() {
  local repo="$TMP_DIR/negative-example"
  setup_repo "$repo"
  cat > "$repo/skills-codex/example/references/guide.md" <<'EOF'
# Guide

Do not use spawn_agents_on_csv in Codex.
EOF

  if (cd "$repo" && bash scripts/audit-codex-parity.sh >/dev/null); then
    pass "allows explicitly negative examples"
  else
    fail "should allow explicitly negative examples"
  fi
}

echo "== test-codex-parity-audit =="
test_passes_on_clean_fixture
test_fails_on_reference_drift
test_fails_on_override_drift
test_allows_negative_examples

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
