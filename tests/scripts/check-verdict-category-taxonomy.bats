#!/usr/bin/env bats
# ag-twl8: verdict.v1 carries the RubricRefine silent-contract-violation finding
# taxonomy. A finding MAY declare a `category` drawn from the four
# registry-conditioned contract-check buckets (tool-choice, output-contract,
# call-signature, data-provenance). The field is OPTIONAL so existing validators
# that emit uncategorized findings stay valid; an UNKNOWN category is rejected so
# the taxonomy cannot silently drift. Behavioral validation uses python
# jsonschema against the real schema (no Go binds verdict.v1 — it is a contract).
# Source: RubricRefine (Anduril, arxiv 2605.09730v3); epic ag-5elx.

setup() {
  SCHEMA="$BATS_TEST_DIRNAME/../../schemas/verdict.v1.schema.json"
  [ -f "$SCHEMA" ] || skip "schema not found: $SCHEMA"
  command -v python3 >/dev/null || skip "python3 required"
  python3 -c 'import jsonschema' 2>/dev/null || skip "python jsonschema required"
}

# Emit a minimal schema-valid verdict whose single finding has the given
# category injected verbatim ($1). Pass the literal "OMIT" to emit no category.
_verdict_with_category() {
  local cat="$1" catfield=""
  if [ "$cat" != "OMIT" ]; then
    catfield=", \"category\": \"$cat\""
  fi
  cat <<JSON
{
  "verdict_id": "01ABCDEFGHJKMNPQRSTVWXYZ00",
  "bead_id": "01ABCDEFGHJKMNPQRSTVWXYZ01",
  "verdict": "FAIL",
  "confidence": "HIGH",
  "briefing_learnings": [],
  "findings": [
    {"severity": "critical", "description": "step 3 consumes an artifact no prior step produced", "location": "plan:step3"$catfield}
  ],
  "not_checked": [],
  "validated_at": "2026-06-05T00:00:00Z",
  "validator_session": "sess-judge-1",
  "schema_version": 1
}
JSON
}

# Validate stdin instance against the schema; exit non-zero iff invalid.
# Program is passed via -c so stdin stays free for the piped instance.
_validate() {
  python3 -c 'import json,sys,jsonschema; jsonschema.validate(json.load(sys.stdin), json.load(open(sys.argv[1])))' "$SCHEMA"
}

@test "finding with a valid contract-check category validates" {
  _verdict_with_category output-contract | _validate
}

@test "all four RubricRefine categories are accepted" {
  for c in tool-choice output-contract call-signature data-provenance; do
    _verdict_with_category "$c" | _validate || { echo "category rejected: $c"; return 1; }
  done
}

@test "finding without a category still validates (backward compatible)" {
  _verdict_with_category OMIT | _validate
}

@test "an unknown category is rejected" {
  local v; v="$(_verdict_with_category bogus-category)"
  run _validate <<<"$v"
  [ "$status" -ne 0 ]
}

@test "schema declares exactly the four taxonomy values" {
  run jq -e '.properties.findings.items.properties.category.enum
             | sort == ["call-signature","data-provenance","output-contract","tool-choice"]' "$SCHEMA"
  [ "$status" -eq 0 ]
}
