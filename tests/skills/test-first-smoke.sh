#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
IMPLEMENT="$ROOT/skills/implement/SKILL.md"
SCHEMA="$ROOT/schemas/candidate-packet.v1.schema.json"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

[[ -f "$IMPLEMENT" ]] || fail "Implement contract is missing"
[[ -f "$SCHEMA" ]] || fail "CandidatePacket schema is missing"

grep -Fq 'Execute exactly one bounded experiment' "$IMPLEMENT" || fail "Implement is not bounded to one experiment"
grep -Fq 'fails for the expected missing' "$IMPLEMENT" || fail "Implement does not require a RED behavior-change baseline"
grep -Fq 'Refactor only while those checks stay green' "$IMPLEMENT" || fail "Implement does not preserve GREEN while refactoring"
grep -Fq 'Refactoring does not change the' "$IMPLEMENT" || fail "Implement does not preserve acceptance tests"
grep -Fq 'Return the CandidatePacket and stop' "$IMPLEMENT" || fail "Implement does not stop after one candidate"
grep -Fq 'Do not commit, push, claim, close, release, land, reserve, retry' "$IMPLEMENT" || fail "Implement retains lifecycle authority"

python3 - "$SCHEMA" <<'PY'
import json
import sys

schema = json.load(open(sys.argv[1], encoding="utf-8"))
required = set(schema["required"])
expected = {
    "plan_packet_digest",
    "acceptance_digest",
    "author_context_id",
    "subject_locator",
    "subject_manifest",
    "actual_changed_paths",
    "changed_path_coverage_complete",
    "factual_evidence",
    "acceptance_check_results",
}
missing = sorted(expected - required)
if missing:
    raise SystemExit(f"CandidatePacket missing required fields: {missing}")
PY

echo 'test-first contract: PASS'
