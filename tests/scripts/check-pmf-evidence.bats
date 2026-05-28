#!/usr/bin/env bats
# Regression tests for scripts/check-pmf-evidence.sh (soc-m6v5.8).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/check-pmf-evidence.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"

  git init --quiet --initial-branch=main "$TMP/repo"
  cd "$TMP/repo"
  git config user.email t@t.test
  git config user.name tester
  git commit --quiet --allow-empty -m "initial"
  cd "$ORIG_DIR"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

run_check() {
  cd "$TMP/repo"
  run "$SCRIPT" "$@"
}

@test "exits 0 on a file with no .agents/ citations" {
  echo "PMF claim with no internal-path citations." > "$TMP/repo/PUBLIC.md"
  run_check PUBLIC.md
  [ "$status" -eq 0 ]
  [[ "$output" == *"clean"* ]]
}

@test "exits 1 when public doc cites .agents/ but no docs/evidence/" {
  cat > "$TMP/repo/PUBLIC.md" <<'EOF'
PMF wedge measured at 40% in .agents/research/ablation.md.
EOF
  run_check PUBLIC.md
  [ "$status" -eq 1 ]
  [[ "$output" == *"FAIL"* ]]
  [[ "$output" == *".agents/research/ablation.md"* ]]
}

@test "exits 0 when public doc cites .agents/ AND also references a docs/evidence/ promotion" {
  cat > "$TMP/repo/PUBLIC.md" <<'EOF'
PMF wedge measured at 40% in .agents/research/ablation.md.
Promoted artifact: docs/evidence/soc-xxx/ablation.md
EOF
  run_check PUBLIC.md
  [ "$status" -eq 0 ]
  [[ "$output" == *"clean"* ]]
}

@test "internal-comment opt-out skips lines marked <!-- internal -->" {
  cat > "$TMP/repo/PUBLIC.md" <<'EOF'
The rule is: never cite .agents/foo.md directly. <!-- internal -->
PMF claims must use promoted paths.
EOF
  run_check PUBLIC.md
  [ "$status" -eq 0 ]
}

@test "violation line number reported correctly" {
  cat > "$TMP/repo/PUBLIC.md" <<'EOF'
Header.

Claim with .agents/path/foo.md only here.
EOF
  run_check PUBLIC.md
  [ "$status" -eq 1 ]
  [[ "$output" == *"PUBLIC.md:3"* ]]
}

@test "multiple violations all reported" {
  cat > "$TMP/repo/PUBLIC.md" <<'EOF'
First cite: .agents/research/a.md
Second cite: .agents/research/b.md
EOF
  run_check PUBLIC.md
  [ "$status" -eq 1 ]
  [[ "$output" == *"2 .agents/-only citation"* ]]
}

@test "--json output is parseable" {
  cat > "$TMP/repo/PUBLIC.md" <<'EOF'
.agents/research/x.md
EOF
  run_check --json PUBLIC.md
  [ "$status" -eq 1 ]
  echo "$output" | jq -e '.count >= 1' >/dev/null
}

@test "exits 3 on missing target file (when single explicit target)" {
  run_check /no/such/file.md
  [ "$status" -eq 3 ]
}

@test "unknown flag exits 2" {
  run_check --weasel
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown"* ]]
}

@test "default scan picks up PRODUCT.md when no target given" {
  cat > "$TMP/repo/PRODUCT.md" <<'EOF'
Bad claim cites .agents/research/foo.md only.
EOF
  run_check
  [ "$status" -eq 1 ]
  [[ "$output" == *"PRODUCT.md"* ]]
}

@test "default scan tolerates missing PRODUCT.md and README.md" {
  # Empty repo (no PRODUCT.md, no README.md, no docs/launch/). Default
  # scan should complete cleanly without erroring on the missing targets.
  run_check
  [ "$status" -eq 0 ]
}
