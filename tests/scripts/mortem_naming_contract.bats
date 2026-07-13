#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  DISPOSITIONS="$REPO_ROOT/docs/contracts/skill-dispositions.yaml"
  OVERRIDES="$REPO_ROOT/skills-codex-overrides/catalog.json"
  REGISTRY="$REPO_ROOT/registry.json"
  TIERS="$REPO_ROOT/skills/SKILL-TIERS.md"
  COMPAT_FIXTURES="$REPO_ROOT/tests/fixtures/mortem-compatibility"
  DRIFT_FIXTURES="$REPO_ROOT/tests/fixtures/four-umbrella-wave-drift"
}

@test "four-umbrella examples conform to their schemas and inventory" {
  run bash "$REPO_ROOT/scripts/check-four-umbrella-examples.sh"
  [ "$status" -eq 0 ]
}

@test "mortem naming migration rejects stale executable spellings" {
  run bash "$REPO_ROOT/scripts/check-mortem-name-migration.sh"
  [ "$status" -eq 0 ]
}

copy_mortem_compatibility_fixtures() {
  CORRUPT_FIXTURES="$BATS_TEST_TMPDIR/mortem-compatibility"
  rm -rf "$CORRUPT_FIXTURES"
  cp -R "$COMPAT_FIXTURES" "$CORRUPT_FIXTURES"
}

run_checker_with_fixture_override() {
  run env MORTEM_COMPAT_FIXTURES_DIR="$CORRUPT_FIXTURES" \
    "$REPO_ROOT/scripts/check-mortem-compatibility.sh" --writer=legacy-v2
}

assert_historical_redirect() {
  local source_slug="$1"
  local target_slug="$2"

  awk -v source_slug="$source_slug" -v target_slug="$target_slug" '
    /^historical:/ { in_historical = 1; next }
    in_historical && /^[^[:space:]]/ { exit 1 }
    in_historical && $0 ~ "^  " source_slug ":$" { in_row = 1; next }
    in_row && /^  [^[:space:]][^:]*:$/ { exit 1 }
    in_row && /^[[:space:]]+state:[[:space:]]+merged-into([[:space:]]|$)/ { merged = 1 }
    in_row && $0 ~ "^[[:space:]]+merged-into:[[:space:]]+" target_slug "([[:space:]]|$)" { target = 1 }
    END { exit !(in_row && merged && target) }
  ' "$DISPOSITIONS"
}

assert_paths_exist() {
  local relative
  for relative in "$@"; do
    if [[ ! -e "$REPO_ROOT/$relative" ]]; then
      echo "missing required contract path: $relative" >&2
      return 1
    fi
  done
}

assert_runtime_pointer() {
  local relative="$1"
  local legacy_slug="$2"
  local target_slug="$3"
  local invocation="$4"
  local pointer="$REPO_ROOT/$relative"

  [[ -f "$pointer" ]]
  [[ "$(awk '/^name:/{print $2; exit}' "$pointer")" == "$legacy_slug" ]]
  [[ "$(awk '/^redirect_to:/{print $2; exit}' "$pointer")" == "$target_slug" ]]
  grep -Eq '^implementation:[[:space:]]+false$' "$pointer"
  [[ "$(grep -Foc "$invocation" "$pointer")" -eq 1 ]]
  [[ "$(wc -l <"$pointer" | tr -d ' ')" -le 16 ]]
  [[ ! -d "$(dirname "$pointer")/references" ]]
}

@test "canonical mortem skills have tiny non-implementing runtime pointers for old explicit requests" {
  assert_paths_exist \
    skills/premortem/SKILL.md \
    skills/postmortem/SKILL.md \
    skills-codex/premortem/SKILL.md \
    skills-codex/postmortem/SKILL.md

  # The disposition ledger is not a runtime alias mechanism. Claude and Codex
  # discover skills by directory, so the installed bundles need redirect-only
  # pointer skills. Each pointer invokes the canonical implementation once and
  # carries no implementation/reference tree of its own.
  assert_runtime_pointer skills/pre-mortem/SKILL.md pre-mortem premortem /premortem
  assert_runtime_pointer skills/post-mortem/SKILL.md post-mortem postmortem /postmortem
  assert_runtime_pointer skills/pre_mortem/SKILL.md pre_mortem premortem /premortem
  assert_runtime_pointer skills/post_mortem/SKILL.md post_mortem postmortem /postmortem
  assert_runtime_pointer skills-codex/pre-mortem/SKILL.md pre-mortem premortem '$premortem'
  assert_runtime_pointer skills-codex/post-mortem/SKILL.md post-mortem postmortem '$postmortem'
  assert_runtime_pointer skills-codex/pre_mortem/SKILL.md pre_mortem premortem '$premortem'
  assert_runtime_pointer skills-codex/post_mortem/SKILL.md post_mortem postmortem '$postmortem'
  [[ -f "$REPO_ROOT/skills-codex/pre-mortem/prompt.md" ]]
  [[ -f "$REPO_ROOT/skills-codex/post-mortem/prompt.md" ]]
  [[ -f "$REPO_ROOT/skills-codex/pre_mortem/prompt.md" ]]
  [[ -f "$REPO_ROOT/skills-codex/post_mortem/prompt.md" ]]

  grep -Eq '^  - skill:[[:space:]]+premortem$' "$DISPOSITIONS"
  grep -Eq '^  - skill:[[:space:]]+postmortem$' "$DISPOSITIONS"
  grep -Eq '^[[:space:]]+path:[[:space:]]+skills/premortem/SKILL\.md$' "$DISPOSITIONS"
  grep -Eq '^[[:space:]]+path:[[:space:]]+skills/postmortem/SKILL\.md$' "$DISPOSITIONS"

  jq -e '([.skills[].name] | index("premortem")) != null' "$OVERRIDES"
  jq -e '([.skills[].name] | index("postmortem")) != null' "$OVERRIDES"
  jq -e '([.skills[].name] | index("pre-mortem")) == null' "$OVERRIDES"
  jq -e '([.skills[].name] | index("post-mortem")) == null' "$OVERRIDES"
  ! grep -Eq 'skills(-codex)?/(pre-mortem|post-mortem)/SKILL\.md' "$OVERRIDES"
}

@test "legacy skill requests permanently resolve once to canonical live skills" {
  assert_historical_redirect pre-mortem premortem
  assert_historical_redirect post-mortem postmortem
  assert_historical_redirect pre_mortem premortem
  assert_historical_redirect post_mortem postmortem

  run bash -c 'source "$1"; resolve_skill_path "$2"' _ \
    "$REPO_ROOT/scripts/lib/resolve-skill-path.sh" \
    "skills/pre-mortem/SKILL.md"
  [[ "$status" -eq 0 ]]
  [[ "$output" == "skills/premortem/SKILL.md" ]]

  run bash -c 'source "$1"; resolve_skill_path "$2"' _ \
    "$REPO_ROOT/scripts/lib/resolve-skill-path.sh" \
    "skills-codex/post-mortem/SKILL.md"
  [[ "$status" -eq 0 ]]
  [[ "$output" == "skills-codex/postmortem/SKILL.md" ]]

  run bash -c 'source "$1"; resolve_skill_path "$2"' _ \
    "$REPO_ROOT/scripts/lib/resolve-skill-path.sh" \
    "skills/pre_mortem/SKILL.md"
  [[ "$status" -eq 0 ]]
  [[ "$output" == "skills/premortem/SKILL.md" ]]

  run bash -c 'source "$1"; resolve_skill_path "$2"' _ \
    "$REPO_ROOT/scripts/lib/resolve-skill-path.sh" \
    "skills-codex/post_mortem/SKILL.md"
  [[ "$status" -eq 0 ]]
  [[ "$output" == "skills-codex/postmortem/SKILL.md" ]]

  run bash "$REPO_ROOT/scripts/check-skill-redirects.sh"
  [[ "$status" -eq 0 ]]
}

@test "mortem compatibility checker owns every accepted conflict and legacy-readback fixture" {
  local checker="$REPO_ROOT/scripts/check-mortem-compatibility.sh"
  [[ -x "$checker" ]] || {
    echo "missing executable compatibility checker: scripts/check-mortem-compatibility.sh" >&2
    return 1
  }

  assert_paths_exist \
    tests/fixtures/mortem-compatibility/v1-old-only.json \
    tests/fixtures/mortem-compatibility/v2-old-only.json \
    tests/fixtures/mortem-compatibility/v3-new-only.json \
    tests/fixtures/mortem-compatibility/both-equal.json \
    tests/fixtures/mortem-compatibility/both-conflicting.json \
    tests/fixtures/mortem-compatibility/neither-optional.json \
    tests/fixtures/mortem-compatibility/neither-required.json \
    tests/fixtures/mortem-compatibility/v1-new-only-invalid.json \
    tests/fixtures/mortem-compatibility/v2-new-only-invalid.json \
    tests/fixtures/mortem-compatibility/v3-old-only-invalid.json \
    tests/fixtures/mortem-compatibility/unknown-version.json \
    tests/fixtures/mortem-compatibility/legacy-directory/pre-mortem-check.json \
    tests/fixtures/mortem-compatibility/directory-conflict/pre-mortem-check.json \
    tests/fixtures/mortem-compatibility/directory-conflict/premortem-check.json \
    tests/fixtures/mortem-compatibility/explicit-skill-redirect.yaml \
    tests/fixtures/mortem-compatibility/legacy-readback/v1-old-only.json \
    tests/fixtures/mortem-compatibility/legacy-readback/v2-old-only.json \
    tests/fixtures/mortem-compatibility/writer-legacy-v2.json

  run "$checker" --writer=legacy-v2
  [[ "$status" -eq 0 ]]
}

@test "mortem compatibility checker executes production readers and writers" {
  local checker="$REPO_ROOT/scripts/check-mortem-compatibility.sh"

  ! grep -q '^def normalize' "$checker"
  ! grep -q 'writer-v1\.json' "$checker"
  grep -q 'go test ./internal/domain/packet' "$checker"
  grep -q 'go test ./cmd/ao' "$checker"
  grep -q 'TestExecutionPacketDecodeJSON_MortemSchemaOwnership' "$checker"
  grep -q 'TestExecutionPacketPublishedSchema_MortemOwnership' "$checker"
  grep -q 'TestPremortemDirectoryReader_' "$checker"
  grep -q -- '--writer=legacy-v2' "$checker"
  grep -q -- '--writer=canonical-v3' "$checker"
  grep -q -- '--legacy-readback' "$checker"
}

@test "mortem compatibility checker rejects corrupt packet fixture bytes" {
  copy_mortem_compatibility_fixtures
  printf '{invalid packet json\n' >"$CORRUPT_FIXTURES/v2-old-only.json"

  run_checker_with_fixture_override
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"v2-old-only.json"* ]]
  [[ "$output" == *"JSON"* || "$output" == *"json"* ]]
}

@test "mortem compatibility checker rejects corrupt legacy-readback fixture bytes" {
  copy_mortem_compatibility_fixtures
  printf '{invalid legacy readback json\n' >"$CORRUPT_FIXTURES/legacy-readback/v2-old-only.json"

  run_checker_with_fixture_override
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"legacy-readback/v2-old-only.json"* ]]
  [[ "$output" == *"JSON"* || "$output" == *"json"* ]]
}

@test "mortem compatibility checker rejects corrupt legacy-directory fixture bytes through production reader" {
  copy_mortem_compatibility_fixtures
  printf '{"id":"check-legacy","rule":"corrupted content"}\n' \
    >"$CORRUPT_FIXTURES/legacy-directory/pre-mortem-check.json"

  run_checker_with_fixture_override
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"legacy-directory/pre-mortem-check.json"* ]]
}

@test "mortem compatibility checker rejects corrupt legacy-v2 writer contract bytes" {
  copy_mortem_compatibility_fixtures
  printf '%s\n' '{"schema_version":3,"packet_fields":{"premortem_verdict":"PASS"},"runtime_paths":[".agents/premortem-checks/current.md"]}' \
    >"$CORRUPT_FIXTURES/writer-legacy-v2.json"

  run_checker_with_fixture_override
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"writer-legacy-v2.json"* ]]
  [[ "$output" == *"schema_version"* || "$output" == *"legacy-v2"* ]]
}

@test "mortem compatibility checker rejects a directory-conflict fixture made equal" {
  copy_mortem_compatibility_fixtures
  cp "$CORRUPT_FIXTURES/directory-conflict/premortem-check.json" \
    "$CORRUPT_FIXTURES/directory-conflict/pre-mortem-check.json"

  run_checker_with_fixture_override
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"directory-conflict/pre-mortem-check.json"* ]]
  [[ "$output" == *"directory-conflict/premortem-check.json"* ]]
}

@test "mortem compatibility checker rejects corrupt redirect fixture bytes" {
  copy_mortem_compatibility_fixtures
  sed 's/merged-into: premortem/merged-into: missing-premortem/' \
    "$CORRUPT_FIXTURES/explicit-skill-redirect.yaml" \
    >"$CORRUPT_FIXTURES/explicit-skill-redirect.yaml.tmp"
  mv "$CORRUPT_FIXTURES/explicit-skill-redirect.yaml.tmp" \
    "$CORRUPT_FIXTURES/explicit-skill-redirect.yaml"

  run_checker_with_fixture_override
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"explicit-skill-redirect.yaml"* ]]
  [[ "$output" == *"missing-premortem"* ]]
}

@test "generated registry and tier surfaces contain only canonical live mortem identities" {
  jq -e '
    ([.surfaces.skills[] | select(.name == "premortem" and .path == "skills/premortem/" and .has_skill_md == true)] | length) == 1 and
    ([.surfaces.skills[] | select(.name == "postmortem" and .path == "skills/postmortem/" and .has_skill_md == true)] | length) == 1 and
    ([.surfaces.skills[] | select(
      .name == "pre-mortem" or .name == "post-mortem" or
      .name == "pre_mortem" or .name == "post_mortem" or
      .path == "skills/pre-mortem/" or .path == "skills/post-mortem/" or
      .path == "skills/pre_mortem/" or .path == "skills/post_mortem/"
    )] | length) == 0
  ' "$REGISTRY"

  grep -q '| \*\*premortem\*\* |' "$TIERS"
  grep -q '| \*\*postmortem\*\* |' "$TIERS"
  ! grep -q '| \*\*pre-mortem\*\* |' "$TIERS"
  ! grep -q '| \*\*post-mortem\*\* |' "$TIERS"
}

@test "runtime pointer skills are excluded from the canonical registry and active disposition list" {
  jq -e '
    ([.surfaces.skills[] | select(.name == "premortem" and .has_skill_md == true)] | length) == 1 and
    ([.surfaces.skills[] | select(.name == "postmortem" and .has_skill_md == true)] | length) == 1 and
    ([.surfaces.skills[] | select(.name == "pre-mortem" or .name == "post-mortem" or .name == "pre_mortem" or .name == "post_mortem")] | length) == 0 and
    ([.capabilities[] | select(
      .sku == "skill:pre-mortem" or .sku == "skill:post-mortem" or
      .sku == "skill:pre_mortem" or .sku == "skill:post_mortem" or
      .name == "pre-mortem" or .name == "post-mortem" or
      .name == "pre_mortem" or .name == "post_mortem" or
      .path == "skills/pre-mortem/" or .path == "skills/post-mortem/" or
      .path == "skills/pre_mortem/" or .path == "skills/post_mortem/"
    )] | length) == 0 and
    .capability_summary.skills == 63 and
    .capability_summary.skills == (.surfaces.skills | length) and
    .capability_summary.total == (.capabilities | length) and
    .capability_summary.total == (
      .capability_summary.skills +
      .capability_summary.cli_commands +
      .capability_summary.gates +
      .capability_summary.reference_impls
    )
  ' "$REGISTRY"

  [[ "$(grep -Ec '^[[:space:]]+- skill:[[:space:]]+(pre-mortem|post-mortem|pre_mortem|post_mortem)$' "$DISPOSITIONS")" -eq 0 ]]
}

@test "S1 drift guard rejects a committed out-of-manifest path" {
  local sandbox="$BATS_TEST_TMPDIR/committed-range"
  local remote="$BATS_TEST_TMPDIR/committed-range.git"
  mkdir -p "$sandbox/scripts" "$sandbox/docs/contracts" "$sandbox/tests/fixtures"
  git init --bare "$remote" >/dev/null
  git -C "$sandbox" init -b main >/dev/null
  git -C "$sandbox" config user.name "S1 drift test"
  git -C "$sandbox" config user.email "s1-drift@example.invalid"

  cp "$REPO_ROOT/scripts/check-four-umbrella-wave-drift.sh" "$sandbox/scripts/"
  cp "$REPO_ROOT/scripts/check-file-manifest-overlap.sh" "$sandbox/scripts/"
  cp -R "$DRIFT_FIXTURES" "$sandbox/tests/fixtures/four-umbrella-wave-drift"
  cat >"$sandbox/docs/contracts/four-umbrella-write-manifests.json" <<'JSON'
{
  "schema_version": 1,
  "s1_frozen_base_sha": "0000000000000000000000000000000000000000",
  "slices": {"S1": {"paths": ["skills/premortem/**"]}}
}
JSON
  printf '.agents/\n' >"$sandbox/.gitignore"
  git -C "$sandbox" add .
  git -C "$sandbox" commit -m "base" >/dev/null
  git -C "$sandbox" remote add origin "$remote"
  git -C "$sandbox" push -u origin main >/dev/null

  local base digest
  base="$(git -C "$sandbox" rev-parse HEAD)"
  digest="$(sha256sum "$sandbox/docs/contracts/four-umbrella-write-manifests.json" | awk '{print $1}')"
  mkdir -p "$sandbox/.agents/evidence/four-umbrella"
  printf '{"schema_version":1,"slice":"S1","base_sha":"%s","manifest_sha256":"%s"}\n' \
    "$base" "$digest" >"$sandbox/.agents/evidence/four-umbrella/s1-base.json"

  git -C "$sandbox" switch -c feature >/dev/null
  printf 'committed but outside S1\n' >"$sandbox/README.md"
  git -C "$sandbox" add README.md
  git -C "$sandbox" commit -m "out of manifest" >/dev/null

  run bash -c 'cd "$1" && bash scripts/check-four-umbrella-wave-drift.sh --phase=verify S1' _ "$sandbox"
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"out-of-manifest committed changes: README.md"* ]]
}
