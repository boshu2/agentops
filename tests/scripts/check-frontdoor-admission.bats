#!/usr/bin/env bats
# check-frontdoor-admission.bats — acceptance for the M5 governance front-door
# admission guard (age-d16-self-hosting-route-nkr.6). The contract: a newly-ADDED
# skill/workflow/loop CANNOT merge unless front-door evidence shows
# bounded-context FOUND + role ASSIGNED + acceptance RUN. Enforcement is
# fail-closed (exit 1); a change with no new unit passes; the guard never
# retroactively judges existing units (added-only).
#
# Fully offline: fixtures (a skills root, a domain-map table, a workflows ledger)
# are built in a temp dir and the added set is injected via --added (no git).

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/check-frontdoor-admission.sh"
  FIX="$(mktemp -d)"
  SKILLS="$FIX/skills"
  MAP="$FIX/domain-map.md"
  LEDGER="$FIX/dispositions.yaml"
  mkdir -p "$SKILLS"

  # Domain map: a GOOD skill placed in a BC; a BAD skill absent from the table.
  cat > "$MAP" <<'EOF'
| Skill | Domain | Role | State |
|---|---|---|---|
| `good-skill` | BC4 Factory | supporting | keep |
| `role-only-skill` | BC4 Factory | supporting | keep |
EOF

  # GOOD skill: role in frontmatter + a references/<name>.feature acceptance.
  mkdir -p "$SKILLS/good-skill/references"
  cat > "$SKILLS/good-skill/SKILL.md" <<'EOF'
---
name: good-skill
hexagonal_role: supporting
---
# good-skill
EOF
  cat > "$SKILLS/good-skill/references/good-skill.feature" <<'EOF'
Feature: good-skill
  Scenario: it works
    Given a thing, When acted on, Then the result holds.
EOF

  # empty-acceptance skill: in the BC map + role, but a ZERO-BYTE .feature (a
  # present-but-not-runnable acceptance) -> must still block on acceptance.
  mkdir -p "$SKILLS/empty-feature-skill/references"
  cat > "$SKILLS/empty-feature-skill/SKILL.md" <<'EOF'
---
name: empty-feature-skill
hexagonal_role: supporting
---
# empty-feature-skill
EOF
  : > "$SKILLS/empty-feature-skill/references/empty-feature-skill.feature"
  printf '| `empty-feature-skill` | BC4 Factory | supporting | keep |\n' >> "$MAP"

  # role-only skill: in the BC map + has a role, but NO acceptance (no feature,
  # no ## Scenarios) -> must be blocked on acceptance alone.
  mkdir -p "$SKILLS/role-only-skill"
  cat > "$SKILLS/role-only-skill/SKILL.md" <<'EOF'
---
name: role-only-skill
hexagonal_role: supporting
---
# role-only-skill
EOF

  # no-evidence skill: dir + SKILL.md with NO role, absent from the BC map, no
  # acceptance -> blocked on all three.
  mkdir -p "$SKILLS/bad-skill"
  cat > "$SKILLS/bad-skill/SKILL.md" <<'EOF'
---
name: bad-skill
---
# bad-skill
EOF

  # redirect-only skill: a loadable compatibility alias, not an independent
  # implementation entering through the front door.
  mkdir -p "$SKILLS/legacy-skill"
  cat > "$SKILLS/legacy-skill/SKILL.md" <<'EOF'
---
name: legacy-skill
implementation: false
---
Use `$good-skill` instead.
EOF

  # acceptance-via-scenarios skill: role + BC + a `## Scenarios` block (the
  # alternative acceptance form) but no .feature file.
  mkdir -p "$SKILLS/scenario-skill"
  cat > "$SKILLS/scenario-skill/SKILL.md" <<'EOF'
---
name: scenario-skill
hexagonal_role: domain
---
# scenario-skill
## Scenarios
- Given x, When y, Then z.
EOF
  # add it to the BC map
  printf '| `scenario-skill` | BC1 Corpus | domain | keep |\n' >> "$MAP"

  # Workflows ledger: a registered workflow (BC+role+kind) and an under-specified
  # one (kind only, no domain/role).
  cat > "$LEDGER" <<'EOF'
skills:
  something:
    state: keep
workflows:
  good-workflow:
    kind:            workflow
    domain:          "BC3 Loop"
    hexagonal_role:  driving-adapter
  half-workflow:
    kind:            workflow
  bogus-bc-workflow:
    kind:            workflow
    domain:          "not-a-bc"
    hexagonal_role:  driving-adapter
  comment-leak-bc-workflow:
    kind:            workflow
    domain:          "not-a-bc"   # this is really BC2 someday
    hexagonal_role:  driving-adapter
  comment-leak-role-workflow:
    kind:            workflow
    domain:          "BC2 Planning"
    hexagonal_role:    # TODO fill in later
EOF

  COMMON=(--skills-root "$SKILLS" --domain-map "$MAP" --dispositions "$LEDGER")
}

teardown() { rm -rf "$FIX"; }

# --- CORE: a fully-evidenced new skill is admitted ---------------------------
@test "admit: new skill with BC + role + .feature acceptance -> exit 0" {
  run "$SCRIPT" "${COMMON[@]}" --added skills/good-skill/SKILL.md
  [ "$status" -eq 0 ]
  [[ "$output" == *'"admitted":1'* ]]
  [[ "$output" == *'"blocked":0'* ]]
}

@test "admit: acceptance via a ## Scenarios block (no .feature) -> exit 0" {
  run "$SCRIPT" "${COMMON[@]}" --added skills/scenario-skill/SKILL.md
  [ "$status" -eq 0 ]
  [[ "$output" == *'"admitted":1'* ]]
}

# --- CORE: a new skill missing ALL evidence is blocked -----------------------
@test "block: new skill missing BC + role + acceptance -> exit 1, names all three" {
  run "$SCRIPT" "${COMMON[@]}" --added skills/bad-skill/SKILL.md
  [ "$status" -eq 1 ]
  [[ "$output" == *'"blocked":1'* ]]
  [[ "$output" == *"bounded-context"* ]]
  [[ "$output" == *"role"* ]]
  [[ "$output" == *"acceptance"* ]]
}

# --- each fact is independently load-bearing ---------------------------------
@test "block: new skill with BC + role but NO acceptance -> exit 1 (acceptance is required)" {
  run "$SCRIPT" "${COMMON[@]}" --added skills/role-only-skill/SKILL.md
  [ "$status" -eq 1 ]
  [[ "$output" == *"acceptance"* ]]
  # NOT blocked on the facts it has.
  [[ "$output" != *"missing: bounded-context role acceptance"* ]]
}

# --- workflows are governed too ----------------------------------------------
@test "admit: new workflow registered with BC + role + kind -> exit 0" {
  run "$SCRIPT" "${COMMON[@]}" --added .claude/workflows/good-workflow.js
  [ "$status" -eq 0 ]
  [[ "$output" == *'"admitted":1'* ]]
}

@test "block: new workflow missing domain + role in the ledger -> exit 1" {
  run "$SCRIPT" "${COMMON[@]}" --added .claude/workflows/half-workflow.js
  [ "$status" -eq 1 ]
  [[ "$output" == *"bounded-context"* ]]
  [[ "$output" == *"role"* ]]
}

@test "block: brand-new workflow absent from the ledger entirely -> exit 1" {
  run "$SCRIPT" "${COMMON[@]}" --added .claude/workflows/ghost-workflow.js
  [ "$status" -eq 1 ]
  [[ "$output" == *'"blocked":1'* ]]
}

# --- added-only: a change with NO new unit passes (legacy not re-judged) ------
@test "pass: a change touching no skill/workflow -> exit 0, nothing to admit" {
  run "$SCRIPT" "${COMMON[@]}" --added scripts/foo.sh --added docs/bar.md
  [ "$status" -eq 0 ]
  [[ "$output" == *'"new_units":0'* ]]
}

@test "pass: a redirect-only skill is not treated as a new implementation" {
  run "$SCRIPT" "${COMMON[@]}" --added skills/legacy-skill/SKILL.md
  [ "$status" -eq 0 ]
  [[ "$output" == *'"new_units":0'* ]]
  [[ "$output" == *'"blocked":0'* ]]
}

# --- mixed batch: one good + one bad -> BLOCKED (fail-closed on any) ----------
@test "block: a batch with one evidenced + one unevidenced unit -> exit 1" {
  run "$SCRIPT" "${COMMON[@]}" \
    --added skills/good-skill/SKILL.md \
    --added skills/bad-skill/SKILL.md
  [ "$status" -eq 1 ]
  [[ "$output" == *'"admitted":1'* ]]
  [[ "$output" == *'"blocked":1'* ]]
}

# --- REGRESSION (refuter): runnable acceptance, not just a present file ------
@test "block: new skill with an EMPTY .feature -> exit 1 (present != runnable)" {
  run "$SCRIPT" "${COMMON[@]}" --added skills/empty-feature-skill/SKILL.md
  [ "$status" -eq 1 ]
  [[ "$output" == *"acceptance"* ]]
}

# --- REGRESSION (refuter): workflow BC must be a real BC1..6, not any string --
@test "block: new workflow with a non-BC domain ('not-a-bc') -> exit 1 on bounded-context" {
  run "$SCRIPT" "${COMMON[@]}" --added .claude/workflows/bogus-bc-workflow.js
  [ "$status" -eq 1 ]
  [[ "$output" == *"bounded-context"* ]]
}

# --- REGRESSION (refuter): an inline `# comment` must NOT leak into the ledger
# value. A bogus domain ("not-a-bc") with a trailing comment mentioning a BC
# token would otherwise false-satisfy the BC[1-6] check (a fail-open). The
# value, stripped of its comment, is what's judged.
@test "block: workflow with bogus domain + a BC token in a trailing comment -> exit 1" {
  run "$SCRIPT" "${COMMON[@]}" --added .claude/workflows/comment-leak-bc-workflow.js
  [ "$status" -eq 1 ]
  [[ "$output" == *"bounded-context"* ]]
}

# --- REGRESSION (refuter): an EMPTY role value with a trailing comment must
# still block on role — the comment text must not make the field "non-empty".
@test "block: workflow with empty role value masked by a trailing comment -> exit 1" {
  run "$SCRIPT" "${COMMON[@]}" --added .claude/workflows/comment-leak-role-workflow.js
  [ "$status" -eq 1 ]
  [[ "$output" == *"role"* ]]
}

# --- REGRESSION (refuter): unresolvable base FAILS-CLOSED, never admit-on-doubt
# When the added set is NOT injected and the base ref cannot be resolved, the
# guard must BLOCK (exit 1) rather than silently degrade to a narrower base and
# report "nothing to admit". (No --added here, so the git base path is taken.)
@test "block: unresolvable --base -> fail-closed exit 1 (no silent degrade)" {
  run "$SCRIPT" "${COMMON[@]}" --base zzz-nonexistent-ref-deadbeef
  [ "$status" -eq 1 ]
  [[ "$output" == *"unresolvable-base"* ]]
}

# --- usage errors are loud (exit 2) ------------------------------------------
@test "usage: unknown argument exits 2" {
  run "$SCRIPT" --bogus
  [ "$status" -eq 2 ]
}
