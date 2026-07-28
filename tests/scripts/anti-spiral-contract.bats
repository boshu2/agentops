#!/usr/bin/env bats
# Anti-spiral guards (2026-07-28 incident: three days of planning/validation
# artifacts, zero implementation commits). Deterministic surface:
#   1. the rpi contract carries admission, phase-lock, spiral-breaker, and
#      subject-first-reporting language, and its validator fails when any of
#      those lines is removed;
#   2. the validate contract requires a nonempty implementation candidate,
#      and store-verdict mechanically refuses an empty subject manifest.
# Scenario guards that are runtime-behavioral (an orchestrator dispatching an
# unsolicited planning lane) reduce to this contract text plus the validators
# that pin it; they cannot be replayed in repo CI and are not faked here.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

# Removing a load-bearing contract line must make the skill validator fail.
stripped_validator_must_fail() {
  local slug="$1" phrase="$2"
  local copy="$BATS_TEST_TMPDIR/$slug-strip"
  mkdir -p "$copy"
  cp -R "$REPO_ROOT/skills/$slug/." "$copy/"
  run bash "$copy/scripts/validate.sh"
  [ "$status" -eq 0 ]
  grep -Fv "$phrase" "$copy/SKILL.md" > "$copy/SKILL.md.tmp"
  mv "$copy/SKILL.md.tmp" "$copy/SKILL.md"
  run bash "$copy/scripts/validate.sh"
  [ "$status" -ne 0 ]
}

@test "rpi validator pins the phase lock" {
  stripped_validator_must_fail rpi 'Plan is closed for that intent'
}

@test "rpi validator pins the spiral breaker" {
  stripped_validator_must_fail rpi 'spiral breaker'
}

@test "rpi validator pins subject-first reporting" {
  stripped_validator_must_fail rpi 'A rising artifact count over an unchanged subject is a stop'
}

@test "validate validator pins the nonempty-candidate precondition" {
  # validate's validator resolves ../../schemas, so the copy needs a mini
  # repo root, not a bare skill dir.
  local root="$BATS_TEST_TMPDIR/validate-strip"
  local copy="$root/skills/validate"
  mkdir -p "$copy" "$root/schemas"
  cp -R "$REPO_ROOT/skills/validate/." "$copy/"
  cp "$REPO_ROOT"/schemas/subject-manifest.v1.schema.json \
     "$REPO_ROOT"/schemas/verdict.v2.schema.json "$root/schemas/"
  run bash "$copy/scripts/validate.sh"
  [ "$status" -eq 0 ]
  grep -Fv 'nonempty implementation candidate' "$copy/SKILL.md" > "$copy/SKILL.md.tmp"
  mv "$copy/SKILL.md.tmp" "$copy/SKILL.md"
  run bash "$copy/scripts/validate.sh"
  [ "$status" -ne 0 ]
}

@test "store-verdict refuses an empty subject manifest" {
  local ws="$BATS_TEST_TMPDIR/ws"
  mkdir -p "$ws"
  printf 'intent bytes\n' > "$ws/intent.txt"
  printf '{}\n' > "$ws/draft.json"
  printf '{"schema_version":"subject-manifest.v1","entries":[]}\n' > "$ws/empty-manifest.json"
  run python3 "$REPO_ROOT/skills/validate/scripts/validate.py" store-verdict \
    --workspace "$ws" \
    --draft "$ws/draft.json" \
    --intent-source "$ws/intent.txt" \
    --subject-manifest "$ws/empty-manifest.json" \
    --author-context-id author-1 \
    --validator-context-id validator-1 \
    --freshness-source runtime \
    --freshness-attester-id attester-1 \
    --scope-result PASS
  [ "$status" -ne 0 ]
  [[ "$output" == *"no entries"* ]]
}

@test "AGENTS.md carries the constraint floor and spiral stop" {
  grep -Fq 'A synthesis frozen without an active constraint is invalid' "$REPO_ROOT/AGENTS.md"
  grep -Fq 'check-skill-python-ratchet.sh' "$REPO_ROOT/AGENTS.md"
  grep -Fq 'Plan is closed' "$REPO_ROOT/AGENTS.md"
  grep -Fq 'no new implementation evidence end the run' "$REPO_ROOT/AGENTS.md"
}
