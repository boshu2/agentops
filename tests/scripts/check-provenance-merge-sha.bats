#!/usr/bin/env bats
# age-0tn: merge_sha mesh join must resolve on origin/main.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/check-provenance-merge-sha.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  cd "$TMP"
  export GIT_TEMPLATE_DIR=""
  git init -q
  git config user.email t@t.t
  git config user.name t
  mkdir -p docs/provenance
  git commit --allow-empty -qm "genesis"
  git branch -M main
  git update-ref refs/remotes/origin/main "$(git rev-parse main)"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

@test "PASS when merge_sha is on origin/main" {
  sha="$(git rev-parse main)"
  printf '{"merge_sha":"%s","bead_id":"ag-test"}\n' "$sha" > docs/provenance/ledger.jsonl
  run bash "$SCRIPT" --trunk origin/main
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}

@test "WARN when merge_sha is off trunk (default)" {
  printf '{"merge_sha":"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef","bead_id":"ag-test"}\n' > docs/provenance/ledger.jsonl
  run bash "$SCRIPT" --trunk origin/main
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]]
}

@test "FAIL strict when merge_sha is off trunk" {
  printf '{"merge_sha":"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef","bead_id":"ag-test"}\n' > docs/provenance/ledger.jsonl
  run bash "$SCRIPT" --trunk origin/main --strict
  [ "$status" -eq 1 ]
}
