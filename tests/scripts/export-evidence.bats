#!/usr/bin/env bats
# Regression tests for scripts/export-evidence.sh (soc-m6v5.8).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/export-evidence.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"

  git init --quiet --initial-branch=main "$TMP/repo"
  cd "$TMP/repo"
  git config user.email t@t.test
  git config user.name tester
  git commit --quiet --allow-empty -m "initial"
  mkdir -p .agents/research
  cd "$ORIG_DIR"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

run_export() {
  cd "$TMP/repo"
  run "$SCRIPT" "$@"
}

@test "promotes a .agents/ artifact to docs/evidence/<bead-id>/ with provenance footer" {
  echo "# Hypothesis" > "$TMP/repo/.agents/research/sample.md"
  run_export soc-xxx.1 .agents/research/sample.md
  [ "$status" -eq 0 ]
  [ -f "$TMP/repo/docs/evidence/soc-xxx.1/sample.md" ]
  grep -q '^# Hypothesis$' "$TMP/repo/docs/evidence/soc-xxx.1/sample.md"
  grep -q '^  source-path: .agents/research/sample.md$' "$TMP/repo/docs/evidence/soc-xxx.1/sample.md"
  grep -qE '^  source-sha256: [0-9a-f]{64}$' "$TMP/repo/docs/evidence/soc-xxx.1/sample.md"
  grep -qE '^  promoted-at: [0-9-]{10}T[0-9:]+Z$' "$TMP/repo/docs/evidence/soc-xxx.1/sample.md"
  grep -q '^  bead-id: soc-xxx.1$' "$TMP/repo/docs/evidence/soc-xxx.1/sample.md"
}

@test "custom dest-name (3rd arg) controls the output filename" {
  echo "x" > "$TMP/repo/.agents/research/sample.md"
  run_export soc-xxx.1 .agents/research/sample.md custom-name.md
  [ "$status" -eq 0 ]
  [ -f "$TMP/repo/docs/evidence/soc-xxx.1/custom-name.md" ]
  [ ! -f "$TMP/repo/docs/evidence/soc-xxx.1/sample.md" ]
}

@test "re-running with identical source = no-op (exit 0, idempotent)" {
  echo "stable" > "$TMP/repo/.agents/research/sample.md"
  run_export soc-xxx.1 .agents/research/sample.md
  [ "$status" -eq 0 ]
  # First run should have written the file. Second run = idempotent.
  first_promoted_at="$(grep -oE 'promoted-at: [^[:space:]]+' "$TMP/repo/docs/evidence/soc-xxx.1/sample.md" | awk '{print $2}')"
  run_export soc-xxx.1 .agents/research/sample.md
  [ "$status" -eq 0 ]
  [[ "$output" == *"already up-to-date"* ]]
  # Footer's promoted-at must be unchanged (we didn't rewrite).
  second_promoted_at="$(grep -oE 'promoted-at: [^[:space:]]+' "$TMP/repo/docs/evidence/soc-xxx.1/sample.md" | awk '{print $2}')"
  [ "$first_promoted_at" = "$second_promoted_at" ]
}

@test "DRIFT detection: re-run with mutated source refuses to overwrite (exit 1)" {
  echo "v1" > "$TMP/repo/.agents/research/sample.md"
  run_export soc-xxx.1 .agents/research/sample.md
  [ "$status" -eq 0 ]
  echo "v2-changed" > "$TMP/repo/.agents/research/sample.md"
  run_export soc-xxx.1 .agents/research/sample.md
  [ "$status" -eq 1 ]
  [[ "$output" == *"DRIFT"* ]]
  # Original file still has v1 content (we refused the overwrite).
  grep -q '^v1$' "$TMP/repo/docs/evidence/soc-xxx.1/sample.md"
}

@test "rejects invalid bead-id shape with usage error" {
  echo "x" > "$TMP/repo/.agents/research/sample.md"
  run_export "Not-A-Bead-Id" .agents/research/sample.md
  [ "$status" -eq 2 ]
  [[ "$output" == *"does not match"* ]]
}

@test "rejects missing source with exit 3" {
  run_export soc-xxx.1 .agents/research/does-not-exist.md
  [ "$status" -eq 3 ]
  [[ "$output" == *"not readable"* ]]
}

@test "missing required args (less than 2) exits 2" {
  run_export soc-xxx.1
  [ "$status" -eq 2 ]
}

@test "too many args (4+) exits 2" {
  echo "x" > "$TMP/repo/.agents/research/sample.md"
  run_export soc-xxx.1 .agents/research/sample.md custom.md extra-arg
  [ "$status" -eq 2 ]
}

@test "provenance source-sha256 matches the actual file content hash" {
  echo "deterministic content" > "$TMP/repo/.agents/research/sample.md"
  run_export soc-xxx.1 .agents/research/sample.md
  [ "$status" -eq 0 ]
  expected="$(sha256sum "$TMP/repo/.agents/research/sample.md" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$TMP/repo/.agents/research/sample.md" | awk '{print $1}')"
  recorded="$(grep -oE 'source-sha256: [0-9a-f]{64}' "$TMP/repo/docs/evidence/soc-xxx.1/sample.md" | awk '{print $2}')"
  [ "$expected" = "$recorded" ]
}

@test "nested bead-id with dots is accepted (e.g. soc-m6v5.9.4.1)" {
  echo "x" > "$TMP/repo/.agents/research/sample.md"
  run_export soc-m6v5.9.4.1 .agents/research/sample.md
  [ "$status" -eq 0 ]
  [ -d "$TMP/repo/docs/evidence/soc-m6v5.9.4.1" ]
}
