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

# Shape assertion, not a name pin: the old `skills[0].name == "compile"` broke the
# moment compile retired (2026-07-07 wave) — pin the CONTRACT instead: the embed
# exists, is non-empty, and carries no retired slug (l6ic.12 prune stays enforced).
if jq -e '.codex_override_catalog.skills | length > 0' "$MANIFEST" >/dev/null; then
  pass "artifact manifest embeds a non-empty codex override catalog"
else
  fail "artifact manifest should embed a non-empty codex override catalog"
fi

if jq -e '[.codex_override_catalog.skills[].name] | any(. == "compile" or . == "curate" or . == "review" or . == "recover" or . == "red-team" or . == "eval-outcomes" or . == "perf" or . == "flywheel") | not' "$MANIFEST" >/dev/null; then
  pass "embedded catalog carries no retired skill rows"
else
  fail "embedded catalog still carries retired skill rows (2026-07-07 wave prune regressed)"
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
