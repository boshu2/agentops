#!/usr/bin/env bash
# validate-outcomes-rubric.sh — Validate Outcomes-rubric projection payloads
# against schemas/outcomes-rubric.v1.schema.json (ag-hguuf).
#
# The Outcomes rubric is the holdout-safe projection of a locked eval Task's
# grading criteria (cli/internal/evalsubstrate/rubric.go). This validator is the
# bead's "standalone validator" acceptance surface: it enforces the committed
# JSON Schema structurally, so a malformed compiler emission that smuggles a
# leak field (target / ground_truth / expected_output) FAILS validation, not
# merely a code check. Managed Agents are not ZDR — a leak is permanent.
#
# Usage:
#   scripts/validate-outcomes-rubric.sh <payload.json> [<payload.json>...]
#       Validate each file. Exit 0 only if EVERY file is schema-valid.
#   scripts/validate-outcomes-rubric.sh --selftest
#       Validate the three tracked fixtures with expected pass/pass/fail and
#       exit 0 only if all three match expectations.
#
# Gating validator: python `jsonschema` (Draft7), mirroring the posture in
# scripts/validate-next-work.sh. If python3/jsonschema is unavailable the script
# SKIPs (exit 0 with a notice) so it never blocks an environment without the
# tool — the bats suite and the Go schema↔struct drift test are the always-on
# guards; CI provides python3+jsonschema for the gating structural pass.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCHEMA="$REPO_ROOT/schemas/outcomes-rubric.v1.schema.json"
FIXTURES="$REPO_ROOT/tests/fixtures/outcomes-rubric"

if [[ ! -f "$SCHEMA" ]]; then
    echo "FAIL: schema not found: $SCHEMA" >&2
    exit 2
fi

if ! command -v python3 >/dev/null 2>&1 || ! python3 -c 'import jsonschema' >/dev/null 2>&1; then
    echo "SKIP: python3 + jsonschema not available; structural validation skipped" >&2
    exit 0
fi

# validate_one <file> -> exit 0 if schema-valid, 1 if invalid, 2 on read error.
validate_one() {
    python3 - "$SCHEMA" "$1" <<'PY'
import json, sys
from jsonschema import Draft7Validator

schema_path, data_path = sys.argv[1], sys.argv[2]
try:
    with open(schema_path) as f:
        schema = json.load(f)
    with open(data_path) as f:
        doc = json.load(f)
except Exception as e:
    print(f"read error: {e}", file=sys.stderr)
    sys.exit(2)

errors = sorted(Draft7Validator(schema).iter_errors(doc), key=lambda e: e.path)
if errors:
    for e in errors:
        loc = "/".join(str(p) for p in e.path) or "(root)"
        print(f"  invalid at {loc}: {e.message}", file=sys.stderr)
    sys.exit(1)
sys.exit(0)
PY
}

if [[ "${1:-}" == "--selftest" ]]; then
    rc=0
    declare -A EXPECT=(
        [valid-dev.json]=0
        [valid-holdout-criteria-only.json]=0
        [invalid-contains-target.json]=1
    )
    for name in valid-dev.json valid-holdout-criteria-only.json invalid-contains-target.json; do
        want="${EXPECT[$name]}"
        if validate_one "$FIXTURES/$name" >/dev/null 2>&1; then got=0; else got=1; fi
        if [[ "$got" == "$want" ]]; then
            echo "PASS: $name (expected $([[ $want == 0 ]] && echo valid || echo invalid))"
        else
            echo "FAIL: $name (expected $([[ $want == 0 ]] && echo valid || echo invalid), got $([[ $got == 0 ]] && echo valid || echo invalid))" >&2
            rc=1
        fi
    done
    exit "$rc"
fi

if [[ $# -lt 1 ]]; then
    echo "usage: $(basename "$0") <payload.json> [<payload.json>...] | --selftest" >&2
    exit 2
fi

overall=0
for f in "$@"; do
    if validate_one "$f"; then
        echo "PASS: $f"
    else
        echo "FAIL: $f (schema-invalid)" >&2
        overall=1
    fi
done
exit "$overall"
