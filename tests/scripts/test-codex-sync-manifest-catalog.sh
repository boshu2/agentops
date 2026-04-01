#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MANIFEST="$ROOT/skills-codex/.agentops-manifest.json"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

[[ -f "$MANIFEST" ]] || {
  echo "FAIL: missing manifest: $MANIFEST" >&2
  exit 1
}

if jq -e '.codex_override_catalog.skills[0].name == "athena"' "$MANIFEST" >/dev/null; then
  pass "artifact manifest embeds codex override catalog"
else
  fail "artifact manifest should embed codex override catalog"
fi

if jq -e '.codex_override_catalog_hash | strings | length > 0' "$MANIFEST" >/dev/null; then
  pass "artifact manifest includes catalog hash"
else
  fail "artifact manifest should include catalog hash"
fi

echo
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
