#!/usr/bin/env bats
# ag-j4l1: the /evolve skill must bake in this session's fix-and-repush discipline
# as durable references — new-skill-landing.md (the six derived surfaces) and the
# gate-hygiene.md additions (pre-push diff-scope check + pre-existing-vs-mine red
# triage). Every references/*.md must be linked from SKILL.md (heal --strict rule);
# this locks the two specific links so they cannot silently drop.

SKILL="$BATS_TEST_DIRNAME/../../skills/evolve/SKILL.md"
REF_DIR="$BATS_TEST_DIRNAME/../../skills/evolve/references"

@test "new-skill-landing.md reference exists and enumerates the six surfaces" {
  [ -f "$REF_DIR/new-skill-landing.md" ]
  run grep -c "registry.json" "$REF_DIR/new-skill-landing.md"
  [ "$status" -eq 0 ]
  # The six numbered surfaces + the regen-all.sh shortcut must be present.
  grep -q "regen-all.sh" "$REF_DIR/new-skill-landing.md"
  grep -q "register-new-codex-skill.sh" "$REF_DIR/new-skill-landing.md"
  grep -q "SKILL-TIERS.md" "$REF_DIR/new-skill-landing.md"
}

@test "SKILL.md links new-skill-landing.md (heal --strict reference-linkage rule)" {
  run grep -E 'references/new-skill-landing\.md' "$SKILL"
  [ "$status" -eq 0 ]
}

@test "gate-hygiene.md gained the pre-push diff-scope + red-triage subsections" {
  run grep -E '## Pre-push diff-scope check' "$REF_DIR/gate-hygiene.md"
  [ "$status" -eq 0 ]
  run grep -E '## Triage red precisely' "$REF_DIR/gate-hygiene.md"
  [ "$status" -eq 0 ]
}

@test "gate-hygiene.md is linked from SKILL.md" {
  run grep -E 'references/gate-hygiene\.md' "$SKILL"
  [ "$status" -eq 0 ]
}
