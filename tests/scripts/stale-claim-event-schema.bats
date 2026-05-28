#!/usr/bin/env bats
# Regression tests for schemas/stale-claim-event.v1.schema.json (soc-vuu6.27 slice 1).
#
# ajv is not in the test environment, so we exercise the schema structurally
# via jq: check the schema itself is valid JSON with the expected required
# fields, and validate fixtures by deriving required-field presence from the
# schema. This catches "the schema and the fixtures disagree" — exactly the
# class of bug schemas exist to prevent.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCHEMA="$REPO_ROOT/schemas/stale-claim-event.v1.schema.json"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

@test "schema file is valid JSON" {
  run jq . "$SCHEMA"
  [ "$status" -eq 0 ]
}

@test "schema declares the right \$id and draft" {
  jq -e '.["$schema"] == "https://json-schema.org/draft/2020-12/schema"' "$SCHEMA" >/dev/null
  jq -e '.["$id"] | endswith("stale-claim-event.v1.schema.json")' "$SCHEMA" >/dev/null
}

@test "schema requires the six core fields" {
  for field in schema_version event_type bead_id detected_at original_claimant evidence; do
    jq -e --arg f "$field" '.required | index($f) != null' "$SCHEMA" >/dev/null
  done
}

@test "schema constrains schema_version to const 1" {
  jq -e '.properties.schema_version.const == 1' "$SCHEMA" >/dev/null
}

@test "event_type enum has exactly stale_detected + claim_transferred" {
  jq -e '.properties.event_type.enum == ["stale_detected", "claim_transferred"]' "$SCHEMA" >/dev/null
}

@test "bead_id pattern matches soc-* shape" {
  pattern="$(jq -r '.properties.bead_id.pattern' "$SCHEMA")"
  [ "$pattern" = '^[a-z]{2,6}-[0-9a-z.]+$' ]
  # Spot-check on real-shaped bead ids
  echo "soc-vuu6.27" | grep -qE "$pattern"
  echo "soc-m6v5.9.4.1" | grep -qE "$pattern"
  # Counter-spot
  ! echo "Soc-VUU6.27" | grep -qE "$pattern"
}

@test "evidence allows any single staleness signal (anyOf clause)" {
  jq -e '.properties.evidence.anyOf | length == 3' "$SCHEMA" >/dev/null
  jq -e '.properties.evidence.anyOf[0].required[0] == "last_touch_ts"' "$SCHEMA" >/dev/null
  jq -e '.properties.evidence.anyOf[1].required[0] == "worktree_quiet_since"' "$SCHEMA" >/dev/null
  jq -e '.properties.evidence.anyOf[2].required[0] == "heartbeat_expired_at"' "$SCHEMA" >/dev/null
}

@test "claim_transferred event_type requires new_claimant + transfer (allOf if/then)" {
  # The allOf if/then enforces conditional required fields.
  jq -e '.allOf[0].then.required | contains(["new_claimant", "transfer"])' "$SCHEMA" >/dev/null
}

@test "Agent \$def requires only id" {
  jq -e '.["$defs"].Agent.required == ["id"]' "$SCHEMA" >/dev/null
  jq -e '.["$defs"].Agent.properties.id.minLength == 1' "$SCHEMA" >/dev/null
}

@test "Agent runtime enum covers the documented runtimes" {
  for rt in claude-code codex openclaw agentopsd unknown; do
    jq -e --arg r "$rt" '.["$defs"].Agent.properties.runtime.enum | index($r) != null' "$SCHEMA" >/dev/null
  done
}

@test "fixture: minimal stale_detected record satisfies the required-field gate" {
  cat > "$TMP/sample.json" <<'EOF'
{
  "schema_version": 1,
  "event_type": "stale_detected",
  "bead_id": "soc-test.1",
  "detected_at": "2026-05-20T23:00:00Z",
  "original_claimant": { "id": "claude:opus-4-7:abc123" },
  "evidence": {
    "last_touch_ts": "2026-05-20T18:00:00Z"
  }
}
EOF
  jq -e . "$TMP/sample.json" >/dev/null
  # Each top-level required field must be present in the fixture.
  for field in schema_version event_type bead_id detected_at original_claimant evidence; do
    jq -e --arg f "$field" 'has($f)' "$TMP/sample.json" >/dev/null
  done
}

@test "fixture: minimal claim_transferred record carries new_claimant + transfer" {
  cat > "$TMP/sample.json" <<'EOF'
{
  "schema_version": 1,
  "event_type": "claim_transferred",
  "bead_id": "soc-test.2",
  "detected_at": "2026-05-20T23:00:00Z",
  "original_claimant": { "id": "claude:opus-4-7:abc" },
  "new_claimant": { "id": "codex:gpt-5.5:def" },
  "evidence": {
    "last_touch_ts": "2026-05-20T18:00:00Z",
    "claim_age_hours": 5.0
  },
  "transfer": {
    "prior_revision": "rev-001",
    "new_revision": "rev-002",
    "notes_appended": true
  }
}
EOF
  jq -e 'has("new_claimant") and has("transfer")' "$TMP/sample.json" >/dev/null
  jq -e '.transfer | has("prior_revision") and has("new_revision")' "$TMP/sample.json" >/dev/null
}

@test "additionalProperties: false at root, evidence, transfer, and Agent" {
  jq -e '.additionalProperties == false' "$SCHEMA" >/dev/null
  jq -e '.properties.evidence.additionalProperties == false' "$SCHEMA" >/dev/null
  jq -e '.properties.transfer.additionalProperties == false' "$SCHEMA" >/dev/null
  jq -e '.["$defs"].Agent.additionalProperties == false' "$SCHEMA" >/dev/null
}
