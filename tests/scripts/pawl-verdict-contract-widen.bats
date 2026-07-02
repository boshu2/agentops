#!/usr/bin/env bats
# age-rk3r.16: the verdict-JSON contract is DOUBLY fail-closed — a shell top-level
# key allowlist + disposition-enum jq fallback in scripts/pawl-verdict.sh, and a
# JSON-schema (additionalProperties:false + enum) in schemas/pawl-verdict.v1.schema.json.
# This bead widens BOTH sites ONCE, atomically, ahead of siblings .2 (degraded flag),
# .9 (REBOUND disposition + lineage) and .13 (multi-family record), so their verdict
# content is not REJECTED (which would fail-close every one of their lands).
#
# The matrix pins the four acceptance cases across BOTH validator paths (the real
# JSON-schema validator AND the jq fallback used on a minimal/unattended host):
#   1. degraded=true            -> accepted (a full CONFIRMED verdict still authorizes)
#   2. disposition REBOUND       -> accepted by schema validation AND the jq fallback
#   3. an unknown top-level key   -> STILL rejected (the allowlist stays strict)
#   4. existing fixtures          -> unchanged (behavior lock)

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/pawl-verdict.sh"
  SCHEMA="$REPO_ROOT/schemas/pawl-verdict.v1.schema.json"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"
  mkdir -p "$TMP/bin" "$TMP/verdicts"
  SHA="cafef00dbabe1234cafef00dbabe1234cafef00d"
  printf 'fresh-context review evidence\n' > "$TMP/evidence.txt"
  # A python3 shim that makes `import jsonschema` fail, to force schema_validate's
  # strict jq fallback (as on a minimal Codex / unattended host). Enabled per-test
  # via mask_jsonschema. check-jsonschema being absent on such hosts is the norm;
  # if it is present here, the jq-fallback tests still pass (they assert the same
  # contract), they just also exercise the real validator — so mask python3 only.
  REAL_PYTHON="$(command -v python3 || true)"
  if [[ -n "$REAL_PYTHON" ]]; then
    cat >"$TMP/bin/python3" <<EOF
#!/usr/bin/env bash
if [[ "\$1" == "-c" && "\$2" == *jsonschema* ]]; then exit 1; fi
exec "$REAL_PYTHON" "\$@"
EOF
    chmod +x "$TMP/bin/python3"
  fi
}

teardown() {
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

# Force schema_validate down its jq fallback by masking `import jsonschema`.
mask_jsonschema() { export PATH="$TMP/bin:$PATH"; }

# schema_validate_available: true when a REAL JSON-schema validator is present, so a
# test that must exercise the schema file directly can skip on a thin host.
schema_validate_available() {
  command -v check-jsonschema >/dev/null 2>&1 && return 0
  command -v python3 >/dev/null 2>&1 && python3 -c 'import jsonschema' >/dev/null 2>&1
}

# validate_against_schema <file>: run the file through whichever real validator exists.
validate_against_schema() {
  local f="$1"
  if command -v check-jsonschema >/dev/null 2>&1; then
    check-jsonschema --schemafile "$SCHEMA" "$f"
  else
    python3 -c "import json,jsonschema,sys; jsonschema.validate(json.load(open(sys.argv[1])), json.load(open(sys.argv[2])))" "$f" "$SCHEMA"
  fi
}

# emit_verdict <bead> <disposition> [extra-json]: write a schema-shaped verdict.
# extra-json is a raw fragment inserted after author_context_id (must start with a
# comma when non-empty), e.g. ',"degraded":true'.
emit_verdict() {
  local bead="$1" disp="$2" extra="${3:-}"
  cat > "$TMP/verdicts/$bead.json" <<EOF
{"schema_version":"pawl-verdict.v1","bead_id":"$bead","pr":0,"head_sha":"$SHA","disposition":"$disp","generated_at":"2026-01-01T00:00:00Z","author_context_id":"author-ctx"$extra,"refuters":[{"family":"claude","verdict":"CONFIRMED","context_id":"fresh-reviewer-ctx","evidence":"$TMP/evidence.txt"}]}
EOF
}

# --- (1) degraded=true -------------------------------------------------------
@test "degraded=true: a full CONFIRMED verdict still AUTHORIZES the merge (schema path)" {
  emit_verdict deg-schema CONFIRMED ',"degraded":true'
  run bash "$SCRIPT" check deg-schema 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 0 ]
  [[ "$output" == *"merge authorized"* ]]
}

@test "degraded=true: still AUTHORIZES when jsonschema validators are absent (jq fallback)" {
  emit_verdict deg-jq CONFIRMED ',"degraded":true'
  mask_jsonschema
  run bash "$SCRIPT" check deg-jq 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 0 ]
  [[ "$output" == *"merge authorized"* ]]
}

@test "schema layer TYPES degraded as boolean: a string value is rejected" {
  # The SCHEMA types degraded:boolean; the jq fallback only allowlists it (same
  # policy as attempt/council_artifact). This pins the schema-layer type guarantee.
  emit_verdict deg-type CONFIRMED ',"degraded":"not-a-bool"'
  schema_validate_available || skip "no JSON-schema validator available for the type-enforcement check"
  run validate_against_schema "$TMP/verdicts/deg-type.json"
  [ "$status" -ne 0 ]
}

# --- (2) disposition REBOUND + lineage ---------------------------------------
@test "REBOUND: a verdict with REBOUND + lineage fields validates against the JSON schema" {
  emit_verdict reb-schema REBOUND ',"rebound_from_verdict":".agents/pawl-verdicts/prior.json","rebound_from_sha":"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef","patch_id_proof":"patch-abc123"'
  schema_validate_available || skip "no JSON-schema validator available (jq-fallback test covers the shell enum)"
  run validate_against_schema "$TMP/verdicts/reb-schema.json"
  [ "$status" -eq 0 ]
}

@test "REBOUND: passes the jq fallback (schema-VALID exit 1, not fail-closed exit 2) when validators are absent" {
  emit_verdict reb-jq REBOUND ',"rebound_from_sha":"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef","patch_id_proof":"patch-abc123"'
  mask_jsonschema
  run bash "$SCRIPT" check reb-jq 0 --dir "$TMP/verdicts" --head "$SHA"
  # exit 1 = schema-VALID but disposition != CONFIRMED (admitted, non-authorizing).
  # exit 2 would mean schema-INVALID (fail-closed) — the pre-widen rejection.
  [ "$status" -eq 1 ]
  [[ "$output" == *"disposition=REBOUND"* ]]
}

@test "REBOUND is ADMITTED to the contract but does NOT authorize a merge (only CONFIRMED does)" {
  emit_verdict reb-lock REBOUND ',"rebound_from_sha":"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef","patch_id_proof":"patch-abc123"'
  run bash "$SCRIPT" check reb-lock 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -ne 0 ]
  [[ "$output" != *"merge authorized"* ]]
}

# --- (3) unknown top-level key still rejected (allowlist stays strict) --------
@test "unknown top-level key: STILL rejected fail-closed (schema path, additionalProperties:false)" {
  emit_verdict unk-schema CONFIRMED ',"bogus_key":"nope"'
  run bash "$SCRIPT" check unk-schema 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 2 ]
}

@test "unknown top-level key: STILL rejected when validators are absent (jq allowlist stays strict)" {
  emit_verdict unk-jq CONFIRMED ',"bogus_key":"nope"'
  mask_jsonschema
  run bash "$SCRIPT" check unk-jq 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 2 ]
}

@test "control: an off-enum disposition (MAYBE) is STILL rejected on BOTH paths (widening did not over-widen)" {
  emit_verdict maybe-ctl MAYBE ',"degraded":false'
  run bash "$SCRIPT" check maybe-ctl 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 2 ]
  mask_jsonschema
  run bash "$SCRIPT" check maybe-ctl 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 2 ]
}

# --- (4) existing fixtures unchanged (behavior lock) -------------------------
@test "behavior lock: the checked-in real-sample fixture still validates against the widened schema" {
  local fx="$REPO_ROOT/tests/fixtures/provenance/pawl-verdict-real-sample.json"
  [ -f "$fx" ]
  schema_validate_available || skip "no JSON-schema validator available"
  run validate_against_schema "$fx"
  [ "$status" -eq 0 ]
}
