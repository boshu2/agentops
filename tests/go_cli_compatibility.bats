#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  CHECKER="$REPO_ROOT/scripts/check-go-cli-compatibility.sh"
  BASELINE="$REPO_ROOT/cli/testdata/compatibility-baseline"
  TMP="$(mktemp -d)"
}

teardown() {
  rm -rf "$TMP"
}

hash_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

seed_family_fixture() {
  local root="$1" family="${2:-demo}" dir
  dir="$root/families/$family"
  mkdir -p "$dir"
  cat >"$dir/case.json" <<JSON
{
  "schema_version": 1,
  "family": "$family",
  "checks": [{"id":"exact-behavior","command":"true"}],
  "dimensions": {
    "path":{"status":"evidence","evidence":["exact-behavior"]},
    "aliases":{"status":"not_applicable","reason":"no aliases"},
    "accepted_args":{"status":"evidence","evidence":["exact-behavior"]},
    "rejected_args":{"status":"evidence","evidence":["exact-behavior"]},
    "stdout":{"status":"evidence","evidence":["exact-behavior"]},
    "stderr":{"status":"evidence","evidence":["exact-behavior"]},
    "exit_classes":{"status":"evidence","evidence":["exact-behavior"]},
    "tracker_selection":{"status":"not_applicable","reason":"family does not resolve trackers"},
    "ordered_effects":{"status":"not_applicable","reason":"family is pure"}
  }
}
JSON
  cat >"$dir/ownership.json" <<JSON
{
  "schema_version":1,
  "family":"$family",
  "profiles":{"default":"absent","flywheel":"present","legacy":"absent","combined":"present"},
  "legacy_symbols":["old${family}Cmd"],
  "live_owner":"cli/internal/$family",
  "allowed_paths":["cli/internal/commands/$family/**"]
}
JSON
}

@test "the captured four-profile baseline is internally immutable" {
  run "$CHECKER" --verify-baseline-integrity
  [ "$status" -eq 0 ]
}

@test "family case ownership and lineage schemas are valid Draft 2020-12" {
  run python3 - "$BASELINE" <<'PY'
import json
import pathlib
import sys
from jsonschema import Draft202012Validator

root = pathlib.Path(sys.argv[1])
for name in ("family-case.schema.json", "ownership.schema.json", "lineage.schema.json"):
    schema = json.loads((root / name).read_text())
    Draft202012Validator.check_schema(schema)
PY
  [ "$status" -eq 0 ]
}

@test "fresh binaries match default, flywheel, legacy, and combined manifests" {
  run "$CHECKER"
  [ "$status" -eq 0 ]
  [[ "$output" == *"default,flywheel,legacy,combined"* ]]
}

@test "capture refuses to regenerate an existing baseline" {
  run "$CHECKER" --capture --execution-base "$(git -C "$REPO_ROOT" rev-parse HEAD)"
  [ "$status" -ne 0 ]
  [[ "$output" == *"baseline already exists"* ]]
}

@test "unclassified timestamps and absolute paths fail raw capture" {
  printf '{"generated_at":"2026-07-11T00:00:00Z"}\n' >"$TMP/time.json"
  run "$CHECKER" --validate-raw "$TMP/time.json"
  [ "$status" -ne 0 ]
  [[ "$output" == *"volatile time field"* ]]

  printf '{"path":"/tmp/ambient"}\n' >"$TMP/path.json"
  run "$CHECKER" --validate-raw "$TMP/path.json"
  [ "$status" -ne 0 ]
  [[ "$output" == *"absolute path"* ]]
}

@test "missing or tampered profile files fail integrity" {
  cp -R "$BASELINE" "$TMP/base"
  rm "$TMP/base/profiles/legacy.json"
  run env AO_CLI_COMPAT_BASELINE_DIR="$TMP/base" "$CHECKER" --verify-baseline-integrity
  [ "$status" -ne 0 ]
  [[ "$output" == *"missing profile baseline: legacy"* ]]

  cp "$BASELINE/profiles/legacy.json" "$TMP/base/profiles/legacy.json"
  jq '.commands[0].short = "blessed drift"' "$TMP/base/profiles/default.json" >"$TMP/default.json"
  mv "$TMP/default.json" "$TMP/base/profiles/default.json"
  run env AO_CLI_COMPAT_BASELINE_DIR="$TMP/base" "$CHECKER" --verify-baseline-integrity
  [ "$status" -ne 0 ]
  [[ "$output" == *"baseline hash mismatch: default"* ]]
}

@test "coherent baseline and metadata tampering fails the git freeze" {
  cp -R "$BASELINE" "$TMP/base"
  jq '.commands[0].short = "coherently blessed drift"' \
    "$TMP/base/profiles/default.json" >"$TMP/default.json"
  mv "$TMP/default.json" "$TMP/base/profiles/default.json"
  mutated_hash="$(hash_file "$TMP/base/profiles/default.json")"
  jq --arg sha "$mutated_hash" '.profiles.default.sha256 = $sha' \
    "$TMP/base/metadata.json" >"$TMP/metadata.json"
  mv "$TMP/metadata.json" "$TMP/base/metadata.json"

  run env AO_CLI_COMPAT_BASELINE_DIR="$TMP/base" \
    "$CHECKER" --verify-baseline-integrity
  [ "$status" -ne 0 ]
  [[ "$output" == *"frozen baseline drift"* ]]
}

@test "path alias profile Args output effects and exits are comparison dimensions" {
  local mutation actual
  for mutation in path aliases profile args output effects exits; do
    actual="$TMP/$mutation.json"
    case "$mutation" in
      path) jq '.commands[0].path = "ao drift"' "$BASELINE/profiles/default.json" >"$actual" ;;
      aliases) jq '.commands[0].aliases = ["drift"]' "$BASELINE/profiles/default.json" >"$actual" ;;
      profile) jq 'del(.commands[0])' "$BASELINE/profiles/default.json" >"$actual" ;;
      args) jq '.commands[0].args = "arbitrary"' "$BASELINE/profiles/default.json" >"$actual" ;;
      output) jq '.commands[0].output = "yaml"' "$BASELINE/profiles/default.json" >"$actual" ;;
      effects) jq '.commands[0].effects = "network"' "$BASELINE/profiles/default.json" >"$actual" ;;
      exits) jq '.commands[0].exit_codes = {"0":"drift"}' "$BASELINE/profiles/default.json" >"$actual" ;;
    esac
    run "$CHECKER" --verify-profile-file default "$actual"
    [ "$status" -ne 0 ]
    [[ "$output" == *"compatibility drift"* ]]
  done
}

@test "every family compatibility dimension needs evidence or a reason" {
  seed_family_fixture "$TMP/base"
  run env AO_CLI_COMPAT_BASELINE_DIR="$TMP/base" "$CHECKER" --validate-family-fixture demo
  [ "$status" -eq 0 ]

  jq 'del(.dimensions.tracker_selection)' "$TMP/base/families/demo/case.json" >"$TMP/case.json"
  mv "$TMP/case.json" "$TMP/base/families/demo/case.json"
  run env AO_CLI_COMPAT_BASELINE_DIR="$TMP/base" "$CHECKER" --validate-family-fixture demo
  [ "$status" -ne 0 ]
  [[ "$output" == *"tracker_selection"* ]]
}

@test "family evidence commands execute and fail closed" {
  seed_family_fixture "$TMP/base"
  jq '.checks[0].command = "false"' "$TMP/base/families/demo/case.json" >"$TMP/case.json"
  mv "$TMP/case.json" "$TMP/base/families/demo/case.json"
  run env AO_CLI_COMPAT_BASELINE_DIR="$TMP/base" "$CHECKER" --validate-family-fixture demo
  [ "$status" -ne 0 ]
  [[ "$output" == *"family evidence check failed"* ]]
}

@test "frozen family lineage passes before mutation and rejects mutation" {
  repo="$TMP/repo"
  git init -q "$repo"
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name Test
  printf 'base\n' >"$repo/README.md"
  git -C "$repo" add README.md
  git -C "$repo" commit -q -m base
  old="$(git -C "$repo" rev-parse HEAD)"
  fixture_root="$repo/cli/testdata/compatibility-baseline"
  seed_family_fixture "$fixture_root"
  git -C "$repo" add cli/testdata
  git -C "$repo" commit -q -m freeze
  freeze="$(git -C "$repo" rev-parse HEAD)"
  case_sha="$(hash_file "$fixture_root/families/demo/case.json")"
  owner_sha="$(hash_file "$fixture_root/families/demo/ownership.json")"
  cat >"$fixture_root/families/demo/lineage.json" <<JSON
{"schema_version":1,"family":"demo","old_implementation_sha":"$old","freeze_sha":"$freeze","fixture_sha256":"$case_sha","ownership_sha256":"$owner_sha"}
JSON
  git -C "$repo" add cli/testdata
  git -C "$repo" commit -q -m lineage

  run env AO_CLI_COMPAT_REPO_ROOT="$repo" AO_CLI_COMPAT_BASELINE_DIR="$fixture_root" "$CHECKER" --validate-family-fixture demo --verify-frozen
  [ "$status" -eq 0 ]

  jq '.checks[0].command = "printf mutated"' "$fixture_root/families/demo/case.json" >"$TMP/case.json"
  mv "$TMP/case.json" "$fixture_root/families/demo/case.json"
  run env AO_CLI_COMPAT_REPO_ROOT="$repo" AO_CLI_COMPAT_BASELINE_DIR="$fixture_root" "$CHECKER" --validate-family-fixture demo --verify-frozen
  [ "$status" -ne 0 ]
  [[ "$output" == *"fixture digest drift"* ]]
}
