#!/usr/bin/env bats
# Tests for scripts/regen-claim-registry.sh — additive regen from AOP-CLAIM markers.

setup() {
  export REPO_ROOT
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/regen-claim-registry.sh"
  TMPDIR="$(mktemp -d)"
  export TMPDIR
}

teardown() {
  rm -rf "$TMPDIR"
}

@test "regen-claim-registry.sh exists and is executable" {
  [ -x "$SCRIPT" ]
}

@test "--check passes when registry matches markers" {
  # Seed a minimal PRODUCT.md with one marker
  mkdir -p "$TMPDIR/docs/contracts" "$TMPDIR/schemas"
  cp "$REPO_ROOT/schemas/claim-registry.v1.schema.json" "$TMPDIR/schemas/"
  cat > "$TMPDIR/PRODUCT.md" <<'EOF'
Some product text.
<!-- agentops:claim:AOP-CLAIM-TEST-ONE -->
More text.
EOF
  cat > "$TMPDIR/docs/contracts/claim-registry.yaml" <<'EOF'
version: 1
claims:
  AOP-CLAIM-TEST-ONE:
    tier: UNPROVEN
    summary: "Test claim"
    surfaces:
      - PRODUCT.md
    marker: "agentops:claim:AOP-CLAIM-TEST-ONE"
    eval_binding: ""
    evidence: []
    owner: ""
EOF
  run bash "$SCRIPT" --check --root "$TMPDIR"
  [ "$status" -eq 0 ]
}

@test "--check fails when marker missing from registry" {
  mkdir -p "$TMPDIR/docs/contracts" "$TMPDIR/schemas"
  cp "$REPO_ROOT/schemas/claim-registry.v1.schema.json" "$TMPDIR/schemas/"
  cat > "$TMPDIR/PRODUCT.md" <<'EOF'
<!-- agentops:claim:AOP-CLAIM-TEST-ONE -->
<!-- agentops:claim:AOP-CLAIM-TEST-TWO -->
EOF
  cat > "$TMPDIR/docs/contracts/claim-registry.yaml" <<'EOF'
version: 1
claims:
  AOP-CLAIM-TEST-ONE:
    tier: UNPROVEN
    summary: "Test claim"
    surfaces:
      - PRODUCT.md
    marker: "agentops:claim:AOP-CLAIM-TEST-ONE"
    eval_binding: ""
    evidence: []
    owner: ""
EOF
  run bash "$SCRIPT" --check --root "$TMPDIR"
  [ "$status" -ne 0 ]
  [[ "$output" == *"AOP-CLAIM-TEST-TWO"* ]]
}

@test "additive regen creates stub for missing marker" {
  mkdir -p "$TMPDIR/docs/contracts" "$TMPDIR/schemas"
  cp "$REPO_ROOT/schemas/claim-registry.v1.schema.json" "$TMPDIR/schemas/"
  cat > "$TMPDIR/PRODUCT.md" <<'EOF'
<!-- agentops:claim:AOP-CLAIM-TEST-NEW -->
EOF
  cat > "$TMPDIR/docs/contracts/claim-registry.yaml" <<'EOF'
version: 1
claims: {}
EOF
  run bash "$SCRIPT" --root "$TMPDIR"
  [ "$status" -eq 0 ]
  # Verify the stub was added
  grep -q "AOP-CLAIM-TEST-NEW" "$TMPDIR/docs/contracts/claim-registry.yaml"
  grep -q "tier: UNPROVEN" "$TMPDIR/docs/contracts/claim-registry.yaml"
}

@test "additive regen does NOT overwrite curated tier" {
  mkdir -p "$TMPDIR/docs/contracts" "$TMPDIR/schemas"
  cp "$REPO_ROOT/schemas/claim-registry.v1.schema.json" "$TMPDIR/schemas/"
  cat > "$TMPDIR/GOALS.md" <<'EOF'
<!-- agentops:claim:AOP-CLAIM-TEST-CURATED -->
EOF
  cat > "$TMPDIR/docs/contracts/claim-registry.yaml" <<'EOF'
version: 1
claims:
  AOP-CLAIM-TEST-CURATED:
    tier: PILOT
    summary: "Curated claim"
    surfaces:
      - GOALS.md
    marker: "agentops:claim:AOP-CLAIM-TEST-CURATED"
    eval_binding: "age-abc"
    evidence:
      - docs/evidence/abc.md
    owner: "bo"
EOF
  run bash "$SCRIPT" --root "$TMPDIR"
  [ "$status" -eq 0 ]
  # Curated tier must survive
  grep -q "tier: PILOT" "$TMPDIR/docs/contracts/claim-registry.yaml"
  grep -q "eval_binding: \"age-abc\"" "$TMPDIR/docs/contracts/claim-registry.yaml" || \
    grep -q "eval_binding: age-abc" "$TMPDIR/docs/contracts/claim-registry.yaml"
}
