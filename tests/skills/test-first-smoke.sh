#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
IMPLEMENT="$ROOT/skills/implement/SKILL.md"
MANIFEST_SCHEMA="$ROOT/schemas/subject-manifest.v1.schema.json"
ACTIVE_CONTRACTS=(
  "$ROOT/docs/ARCHITECTURE.md"
  "$ROOT/docs/getting-started/index.md"
  "$ROOT/docs/GLOSSARY.md"
)

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

[[ -f "$IMPLEMENT" ]] || fail "Implement contract is missing"
[[ -f "$MANIFEST_SCHEMA" ]] || fail "subject-manifest.v1 schema is missing"

grep -Fq 'Execute exactly one bounded experiment' "$IMPLEMENT" || fail "Implement is not bounded to one experiment"
grep -Fq 'fails for the expected missing' "$IMPLEMENT" || fail "Implement does not require a RED behavior-change baseline"
grep -Fq 'Refactor only while those checks stay green' "$IMPLEMENT" || fail "Implement does not preserve GREEN while refactoring"
grep -Fq 'Refactoring does not change the' "$IMPLEMENT" || fail "Implement does not preserve acceptance tests"
grep -Fq 'runtime derive actual changed paths' "$IMPLEMENT" || fail "Implement does not derive changed paths from the subject"
grep -Fq 'Return the manifest digest, author context ID, and exact check receipts' "$IMPLEMENT" || fail "Implement does not return derived identity and check facts"
grep -Fq 'Do not commit, push, claim, close, release, land, reserve, retry' "$IMPLEMENT" || fail "Implement retains lifecycle authority"
if grep -Eq 'CandidatePacket|candidate-packet' "$IMPLEMENT"; then
  fail "Implement advertises the removed CandidatePacket contract"
fi

for path in "${ACTIVE_CONTRACTS[@]}"; do
  [[ -f "$path" ]] || fail "active contract is missing: $path"
  if grep -Eq 'PlanPacket|CandidatePacket|RevisionPacket|plan-packet|candidate-packet|revision-packet' "$path"; then
    fail "active contract advertises a removed packet: $path"
  fi
done

python3 - "$MANIFEST_SCHEMA" <<'PY'
import json
import sys

schema = json.load(open(sys.argv[1], encoding="utf-8"))
required = set(schema["required"])
expected = {
    "schema_version",
    "declared_roots",
    "exclusions",
    "entries",
    "canonical_manifest_digest",
}
missing = sorted(expected - required)
if missing:
    raise SystemExit(f"subject-manifest.v1 missing required fields: {missing}")

entry_required = set(schema["properties"]["entries"]["items"]["required"])
entry_missing = sorted({"path", "kind", "executable"} - entry_required)
if entry_missing:
    raise SystemExit(f"subject-manifest.v1 entries missing required fields: {entry_missing}")
PY

echo 'test-first contract: PASS'
