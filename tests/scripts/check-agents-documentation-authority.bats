#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  CHECKER="$REPO_ROOT/scripts/check-agents-documentation-authority.sh"
  LIVE_MANIFEST="$REPO_ROOT/docs/contracts/agents-documentation-authority.yaml"
  FIXTURES="$REPO_ROOT/tests/fixtures/agents-documentation-authority"
  TEST_REPO="$BATS_TEST_TMPDIR/repo"

  mkdir -p "$TEST_REPO/docs/contracts"
  printf 'contract\n' >"$TEST_REPO/A.md"
  printf 'owner\n' >"$TEST_REPO/B.md"
  printf 'A.md\n' >"$TEST_REPO/consumer.txt"
  git -C "$TEST_REPO" init -q
  git -C "$TEST_REPO" config user.email test@example.com
  git -C "$TEST_REPO" config user.name test
}

install_manifest() {
  cp "$FIXTURES/$1" "$TEST_REPO/docs/contracts/authority.yaml"
  git -C "$TEST_REPO" add .
  git -C "$TEST_REPO" commit -qm fixture
}

require_checker_or_skip() {
  [[ -x "$CHECKER" ]] || skip "documentation-authority checker is not implemented yet"
}

@test "live inventory covers every root Markdown document and literal consumer" {
  [[ -f "$LIVE_MANIFEST" ]]
  [[ -x "$CHECKER" ]]

  run "$CHECKER" \
    --root="$REPO_ROOT" \
    --manifest=docs/contracts/agents-documentation-authority.yaml \
    --phase=inventory

  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS phase=inventory root_markdown=10 declared=10"* ]]
}

@test "inventory accepts exact root and literal consumer coverage" {
  require_checker_or_skip
  install_manifest valid.yaml

  run "$CHECKER" --root="$TEST_REPO" --manifest=docs/contracts/authority.yaml --phase=inventory

  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS phase=inventory root_markdown=2 declared=2"* ]]
}

@test "inventory rejects a root Markdown path absent from the manifest" {
  require_checker_or_skip
  install_manifest missing-root.yaml

  run "$CHECKER" --root="$TEST_REPO" --manifest=docs/contracts/authority.yaml --phase=inventory

  [ "$status" -eq 1 ]
  [[ "$output" == *"root Markdown missing from manifest: B.md"* ]]
}

@test "inventory rejects an owner path that does not exist" {
  require_checker_or_skip
  install_manifest missing-owner.yaml

  run "$CHECKER" --root="$TEST_REPO" --manifest=docs/contracts/authority.yaml --phase=inventory

  [ "$status" -eq 1 ]
  [[ "$output" == *"owner path does not exist: missing-owner.md"* ]]
}

@test "inventory permits an explicitly declared optional generated root file to be absent" {
  require_checker_or_skip
  install_manifest optional-generated.yaml

  run "$CHECKER" --root="$TEST_REPO" --manifest=docs/contracts/authority.yaml --phase=inventory

  [ "$status" -eq 0 ]
  [[ "$output" == *"root_markdown=2 declared=3"* ]]
}

@test "inventory rejects an undeclared literal consumer" {
  require_checker_or_skip
  install_manifest valid.yaml
  printf 'A.md\n' >"$TEST_REPO/second-consumer.txt"
  git -C "$TEST_REPO" add second-consumer.txt
  git -C "$TEST_REPO" commit -qm consumer

  run "$CHECKER" --root="$TEST_REPO" --manifest=docs/contracts/authority.yaml --phase=inventory

  [ "$status" -eq 1 ]
  [[ "$output" == *"undeclared live consumers: second-consumer.txt"* ]]
}

@test "manifest rejects duplicate document records" {
  require_checker_or_skip
  install_manifest duplicate-path.yaml

  run "$CHECKER" --root="$TEST_REPO" --manifest=docs/contracts/authority.yaml --phase=inventory

  [ "$status" -eq 1 ]
  [[ "$output" == *"duplicate document path: A.md"* ]]
}

@test "manifest rejects a misspelled references key instead of disabling checks" {
  require_checker_or_skip
  install_manifest misspelled-references.yaml

  run "$CHECKER" --root="$TEST_REPO" --manifest=docs/contracts/authority.yaml --phase=inventory

  [ "$status" -eq 1 ]
  [[ "$output" == *"documents[0]: unknown key: referneces"* ]]
}

@test "manifest rejects duplicate YAML mapping keys before loading values" {
  require_checker_or_skip
  install_manifest duplicate-key.yaml

  run "$CHECKER" --root="$TEST_REPO" --manifest=docs/contracts/authority.yaml --phase=inventory

  [ "$status" -eq 2 ]
  [[ "$output" == *"duplicate mapping key: status"* ]]
}

@test "cutover requires zero literal consumers when policy says zero" {
  require_checker_or_skip
  install_manifest valid.yaml

  run "$CHECKER" --root="$TEST_REPO" --manifest=docs/contracts/authority.yaml --phase=cutover

  [ "$status" -eq 1 ]
  [[ "$output" == *"cutover requires zero live consumers: consumer.txt"* ]]
}
