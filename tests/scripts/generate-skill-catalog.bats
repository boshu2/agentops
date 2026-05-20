#!/usr/bin/env bats
# Regression tests for scripts/generate-skill-catalog.sh (soc-vuu6.4).
#
# Builds isolated fixture repos with small skills/ trees, then runs the
# generator and asserts structure + drift detection. We avoid touching the
# real repo's catalog by always running inside $TMP.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/generate-skill-catalog.sh"
  DRIFT_SCRIPT="$REPO_ROOT/scripts/check-skill-catalog-drift.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"

  git init --quiet --initial-branch=main "$TMP/repo"
  cd "$TMP/repo"
  git config user.email t@t.test
  git config user.name tester
  git commit --quiet --allow-empty -m "initial"
  mkdir -p skills schemas
  # The drift check script needs to find the generator at the same repo path
  # so we install both into the fixture's `scripts/` dir.
  mkdir -p scripts
  cp "$SCRIPT" scripts/generate-skill-catalog.sh
  cp "$DRIFT_SCRIPT" scripts/check-skill-catalog-drift.sh
  chmod +x scripts/generate-skill-catalog.sh scripts/check-skill-catalog-drift.sh
  cd "$ORIG_DIR"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

# Write a skill SKILL.md with a given frontmatter body.
write_skill() {
  local name="$1" fm="$2"
  mkdir -p "$TMP/repo/skills/$name"
  {
    echo "---"
    printf '%s\n' "$fm"
    echo "---"
    echo
    echo "# $name"
  } > "$TMP/repo/skills/$name/SKILL.md"
}

run_gen() {
  cd "$TMP/repo"
  run scripts/generate-skill-catalog.sh "$@"
}

run_drift() {
  cd "$TMP/repo"
  run scripts/check-skill-catalog-drift.sh "$@"
}

@test "generator emits valid JSON with required envelope fields" {
  write_skill alpha "name: alpha
description: first skill
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  run_gen --stdout
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.schema_version == "1"' >/dev/null
  echo "$output" | jq -e '.skill_count == 1' >/dev/null
  echo "$output" | jq -e '.skills | length == 1' >/dev/null
  echo "$output" | jq -e '.generated_at | startswith("20")' >/dev/null
}

@test "extracts name/description/hexagonal_role from frontmatter" {
  write_skill alpha "name: alpha
description: hello world
hexagonal_role: supporting
consumes: []
produces: []
context_rel: []"
  run_gen --stdout
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.skills[0].name == "alpha"' >/dev/null
  echo "$output" | jq -e '.skills[0].description == "hello world"' >/dev/null
  echo "$output" | jq -e '.skills[0].hexagonal_role == "supporting"' >/dev/null
}

@test "consumes/produces/practices lists round-trip into arrays" {
  write_skill beta "name: beta
description: lists test
hexagonal_role: domain
practices:
- tdd
- bdd-gherkin
consumes:
- standards
- domain
produces:
- result.json
- verdict.json
context_rel: []"
  run_gen --stdout
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.skills[0].consumes == ["standards","domain"]' >/dev/null
  echo "$output" | jq -e '.skills[0].produces == ["result.json","verdict.json"]' >/dev/null
  echo "$output" | jq -e '.skills[0].practices == ["tdd","bdd-gherkin"]' >/dev/null
}

@test "context_rel captures all entries including the last one (the parser-bug regression)" {
  write_skill gamma "name: gamma
description: context-rel test
hexagonal_role: domain
consumes: []
produces: []
context_rel:
- kind: customer-of
  with: alpha
- kind: customer-of
  with: beta
- kind: customer-of
  with: delta
skill_api_version: 1"
  run_gen --stdout
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.skills[0].context_rel | length == 3' >/dev/null
  echo "$output" | jq -e '.skills[0].context_rel[2].with == "delta"' >/dev/null
}

@test "user_invocable is true only when explicitly set" {
  write_skill u1 "name: u1
description: invocable
hexagonal_role: domain
user-invocable: true
consumes: []
produces: []
context_rel: []"
  write_skill u2 "name: u2
description: not invocable
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  run_gen --stdout
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.skills[] | select(.name=="u1") | .user_invocable == true' >/dev/null
  echo "$output" | jq -e '.skills[] | select(.name=="u2") | .user_invocable == false' >/dev/null
}

@test "references_count reflects skills/<name>/references/*.md count" {
  write_skill withrefs "name: withrefs
description: x
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  mkdir -p "$TMP/repo/skills/withrefs/references"
  touch "$TMP/repo/skills/withrefs/references/a.md" \
        "$TMP/repo/skills/withrefs/references/b.md"
  run_gen --stdout
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.skills[] | select(.name=="withrefs") | .references_count == 2' >/dev/null
}

@test "codex_override_present is true when skills-codex/<name>/ exists" {
  write_skill swithcodex "name: swithcodex
description: x
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  mkdir -p "$TMP/repo/skills-codex/swithcodex"
  touch "$TMP/repo/skills-codex/swithcodex/SKILL.md"
  run_gen --stdout
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.skills[] | select(.name=="swithcodex") | .codex_override_present == true' >/dev/null
}

@test "default --out writes skills/catalog.json and prints summary" {
  write_skill alpha "name: alpha
description: x
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  run_gen
  [ "$status" -eq 0 ]
  [ -f "$TMP/repo/skills/catalog.json" ]
  [[ "$output" == *"wrote"* ]]
}

@test "--check passes when committed catalog matches regeneration" {
  write_skill alpha "name: alpha
description: x
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  run_gen
  [ "$status" -eq 0 ]
  run_gen --check
  [ "$status" -eq 0 ]
}

@test "--check fails (exit 1) when committed catalog drifts from source" {
  write_skill alpha "name: alpha
description: x
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  run_gen
  [ "$status" -eq 0 ]
  # Mutate a SKILL.md without regenerating the catalog.
  write_skill alpha "name: alpha
description: x CHANGED
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  run_gen --check
  [ "$status" -eq 1 ]
  [[ "$output" == *"DRIFT"* ]]
}

@test "drift wrapper exits 0 when in sync" {
  write_skill alpha "name: alpha
description: x
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  run_gen
  run_drift
  [ "$status" -eq 0 ]
}

@test "drift wrapper exits 1 when out of sync" {
  write_skill alpha "name: alpha
description: x
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  run_gen
  write_skill alpha "name: alpha
description: drifted
hexagonal_role: domain
consumes: []
produces: []
context_rel: []"
  run_drift
  [ "$status" -eq 1 ]
}

@test "empty skills/ produces a valid empty catalog" {
  run_gen --stdout
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.skill_count == 0' >/dev/null
  echo "$output" | jq -e '.skills | length == 0' >/dev/null
}

@test "unknown flag exits 2" {
  run_gen --weasel
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown"* ]]
}
