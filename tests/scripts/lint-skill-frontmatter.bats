#!/usr/bin/env bats
# Regression tests for scripts/lint-skill-frontmatter.sh (soc-e8pj).
#
# Each test builds an isolated fixture repo with a small skills/ tree and
# runs the script with HOME-relative repo overrides. The script resolves
# the skills dir via `git rev-parse --show-toplevel`, so cd'ing into the
# fixture repo is sufficient.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/lint-skill-frontmatter.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"

  git init --quiet --initial-branch=main "$TMP/repo"
  cd "$TMP/repo"
  git config user.email t@t.test
  git config user.name tester
  git commit --quiet --allow-empty -m "initial"
  mkdir -p skills
  cd "$ORIG_DIR"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

# Helper: write a skill SKILL.md with given frontmatter body.
write_skill() {
  local name="$1" fm="$2"
  mkdir -p "$TMP/repo/skills/$name"
  {
    echo "---"
    printf '%s\n' "$fm"
    echo "---"
    echo
    echo "# $name"
    echo
    echo "skill body."
  } > "$TMP/repo/skills/$name/SKILL.md"
}

# Helper: invoke the script from inside the fixture repo.
run_lint() {
  cd "$TMP/repo"
  run "$SCRIPT" "$@"
}

@test "passes a clean skill with all keys present" {
  write_skill clean "name: clean
description: a clean skill
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  run_lint
  [ "$status" -eq 0 ]
  [[ "$output" == *"1 skill(s) clean"* ]] || [[ "$output" == *"OK"* ]]
}

@test "fails when consumes is missing" {
  write_skill nocons "name: nocons
description: no consumes key
hexagonal_role: domain
produces: []
context_rel: []"
  run_lint
  [ "$status" -eq 1 ]
  [[ "$output" == *"nocons|missing|consumes"* ]]
}

@test "fails when produces is missing" {
  write_skill noprod "name: noprod
description: no produces key
hexagonal_role: domain
consumes: []
context_rel: []"
  run_lint
  [ "$status" -eq 1 ]
  [[ "$output" == *"noprod|missing|produces"* ]]
}

@test "fails when context_rel is missing" {
  write_skill noctx "name: noctx
description: no context_rel key
hexagonal_role: domain
consumes: []
produces: []"
  run_lint
  [ "$status" -eq 1 ]
  [[ "$output" == *"noctx|missing|context_rel"* ]]
}

@test "fails when hexagonal_role is missing" {
  write_skill norole "name: norole
description: no role
consumes: []
produces: []
context_rel: []"
  run_lint
  [ "$status" -eq 1 ]
  [[ "$output" == *"norole|missing|hexagonal_role"* ]]
}

@test "fails when hexagonal_role has an invalid value" {
  write_skill badrole "name: badrole
description: bad role
hexagonal_role: weasel
consumes: []
produces: []
context_rel: []"
  run_lint
  [ "$status" -eq 1 ]
  [[ "$output" == *"badrole|invalid|hexagonal_role"* ]]
  [[ "$output" == *"weasel"* ]]
}

@test "fails when name is empty" {
  write_skill noname "name:
description: x
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  run_lint
  [ "$status" -eq 1 ]
  [[ "$output" == *"noname|empty|name"* ]]
}

@test "accepts all 5 documented hexagonal_role enum values" {
  for role in domain driving-adapter driven-adapter supporting generic; do
    write_skill "ok-$role" "name: ok-$role
description: testing $role
hexagonal_role: $role
consumes: []
produces: []
context_rel: []"
  done
  run_lint
  [ "$status" -eq 0 ]
}

@test "reports a skill missing frontmatter entirely" {
  mkdir -p "$TMP/repo/skills/empty"
  echo "no frontmatter here" > "$TMP/repo/skills/empty/SKILL.md"
  run_lint
  [ "$status" -eq 1 ]
  [[ "$output" == *"empty|missing|no-frontmatter"* ]]
}

@test "--skill <name> scopes to one skill only" {
  write_skill bad "name: bad
description: missing keys"
  write_skill good "name: good
description: ok
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  run_lint --skill good
  [ "$status" -eq 0 ]
  [[ "$output" != *"bad|"* ]]
}

@test "--skill rejects unknown skill name with exit 3" {
  run_lint --skill does-not-exist
  [ "$status" -eq 3 ]
  [[ "$output" == *"not found"* ]]
}

@test "--list mode prints findings + summary line" {
  write_skill miss "name: miss
description: x
hexagonal_role: domain
consumes: []"
  run_lint --list
  [ "$status" -eq 1 ]
  [[ "$output" == *"miss|missing|produces"* ]]
  [[ "$output" == *"miss|missing|context_rel"* ]]
  [[ "$output" == *"1 skill(s);"* ]] || [[ "$output" == *"clean,"* ]]
}

@test "--json produces a parseable summary" {
  write_skill miss "name: miss
description: x
hexagonal_role: domain
consumes: []"
  run_lint --json
  [ "$status" -eq 1 ]
  echo "$output" | jq -e '.total == 1 and .violations == 1' >/dev/null
  echo "$output" | jq -e '.findings | length >= 2' >/dev/null
}

@test "no skills present = exit 0 with informational message" {
  run_lint
  [ "$status" -eq 0 ]
  [[ "$output" == *"no SKILL.md files"* ]]
}

@test "rejects unknown flag with usage error" {
  run_lint --weasel
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown"* ]]
}
