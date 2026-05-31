#!/usr/bin/env bash
# validate-provenance-ledger.sh — Validate provenance ledger events against
# schemas/agentops-sdlc-provenance.v1.schema.json (ag-x31t.2).
#
# The committed, per-record hash-chained provenance ledger
# (docs/provenance/ledger.jsonl) is the AUDIT authority for the SDLC
# provenance/intent graph (council: .agents/council/2026-05-30-debate-provenance-substrate.md).
# This validator is the schema's structural acceptance surface: each ledger
# event (one JSON object per line) must carry a typed provenance edge plus the
# export-time hash-chain fields, so a malformed edge — e.g. one missing a
# trust_tier — FAILS validation, not merely a code review.
#
# Usage:
#   scripts/validate-provenance-ledger.sh <event.json> [<event.json>...]
#       Validate each single-event JSON file. Exit 0 only if EVERY file is
#       schema-valid.
#   scripts/validate-provenance-ledger.sh --jsonl <ledger.jsonl> [...]
#       Validate every non-blank line of each JSONL ledger file as one event.
#   scripts/validate-provenance-ledger.sh --selftest
#       Validate the tracked fixtures with expected pass/fail and exit 0 only if
#       all match expectations.
#
# Gating validator: python `jsonschema` (Draft7), mirroring
# scripts/validate-outcomes-rubric.sh. If python3/jsonschema is unavailable the
# script SKIPs (exit 0 with a notice) so it never blocks an environment without
# the tool — the bats suite's structural-JSON checks are the always-on guard.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCHEMA="$REPO_ROOT/schemas/agentops-sdlc-provenance.v1.schema.json"
FIXTURES="$REPO_ROOT/tests/fixtures/provenance"

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

errors = sorted(Draft7Validator(schema).iter_errors(doc), key=lambda e: list(e.path))
if errors:
    for e in errors:
        loc = "/".join(str(p) for p in e.path) or "(root)"
        print(f"  invalid at {loc}: {e.message}", file=sys.stderr)
    sys.exit(1)
sys.exit(0)
PY
}

# validate_jsonl <file> -> exit 0 if EVERY non-blank line validates.
validate_jsonl() {
    python3 - "$SCHEMA" "$1" <<'PY'
import json, sys
from jsonschema import Draft7Validator

schema_path, data_path = sys.argv[1], sys.argv[2]
with open(schema_path) as f:
    schema = json.load(f)
validator = Draft7Validator(schema)
bad = 0
with open(data_path) as f:
    for n, line in enumerate(f, 1):
        line = line.strip()
        if not line:
            continue
        try:
            doc = json.loads(line)
        except json.JSONDecodeError as e:
            print(f"  line {n}: not valid JSON: {e}", file=sys.stderr)
            bad += 1
            continue
        errors = sorted(validator.iter_errors(doc), key=lambda e: list(e.path))
        if errors:
            for e in errors:
                loc = "/".join(str(p) for p in e.path) or "(root)"
                print(f"  line {n} invalid at {loc}: {e.message}", file=sys.stderr)
            bad += 1
sys.exit(1 if bad else 0)
PY
}

if [[ "${1:-}" == "--selftest" ]]; then
    rc=0
    declare -A EXPECT=(
        [valid-decision-produces-artifact.json]=0
        [invalid-missing-trust-tier.json]=1
    )
    for name in valid-decision-produces-artifact.json invalid-missing-trust-tier.json; do
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

if [[ "${1:-}" == "--jsonl" ]]; then
    shift
    if [[ $# -lt 1 ]]; then
        echo "usage: $(basename "$0") --jsonl <ledger.jsonl> [...]" >&2
        exit 2
    fi
    overall=0
    for f in "$@"; do
        if validate_jsonl "$f"; then
            echo "PASS: $f"
        else
            echo "FAIL: $f (one or more events schema-invalid)" >&2
            overall=1
        fi
    done
    exit "$overall"
fi

if [[ $# -lt 1 ]]; then
    echo "usage: $(basename "$0") <event.json> [...] | --jsonl <ledger.jsonl> [...] | --selftest" >&2
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
