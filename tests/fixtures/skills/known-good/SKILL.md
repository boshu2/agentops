---
name: known-good
description: |
  Test fixture skill that satisfies all 15 auditor checks (Pass 1 + Pass 2).

  **Use when:**
  - The auditor integration tests assert PASS verdict on a canonical input

  **Triggers:** "known-good fixture", "auditor PASS smoke", "skill-auditor regression"

  **Not ideal for:**
  - Real-world use (this is a test fixture)
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: meta
  dependencies: []
  stability: experimental
output_contract: "stdout: known-good marker"
---

# /known-good — Auditor PASS test fixture

A minimal but complete SKILL.md that satisfies all 15 auditor checks. Used by `tests/integration/test_skill_auditor.bats` to confirm the auditor returns PASS or WARN on conforming input.

## ⚠️ Critical Constraints

- **This is a test fixture, not a runtime skill.** Do not invoke. **Why:** auditor tests pin the exact content; runtime use would surprise contributors.
- **Frontmatter shape is canonical.** YAML `description: |` block scalar with explicit Triggers and Use-when markers. **Why:** exercises `description-has-triggers` form (a) AND form (b) simultaneously.
- **Constraints are front-loaded.** This `## ⚠️ Critical Constraints` section appears before any other H2. **Why:** tests `constraints-frontloaded` heuristic.
- **Every constraint has a Why.** Each bullet ends with `**Why:** <reason>`. **Why:** tests `rationale-present` heuristic.

## Workflow

### Phase 1: Setup

Read the fixture, parse frontmatter.

**Checkpoint:** frontmatter has `name`, `description`, `metadata.tier`.

### Phase 2: Validate

Run all 8 Pass-2 checks.

**Checkpoint:** all 8 checks return `pass` or `n/a`.

### Phase 3: Emit verdict

Aggregate via max-severity rule.

**Checkpoint:** verdict is `PASS` (no warns, no fails).

## Output Specification

**Format:** stdout marker `known-good` confirming the fixture loaded cleanly.
**Filename:** none — runtime fixture only.
**Exit code:** 0.

## Quality Rubric

- [ ] Frontmatter passes Pass 1 (heal-skill structural)
- [ ] Constraints section is front-loaded (first H2 after title)
- [ ] Every constraint has a Why: rationale
- [ ] Each Phase has a Checkpoint
- [ ] Output Specification is explicit
- [ ] Quality Rubric exists with checkboxes
- [ ] SKILL.md is under 250 lines

## Examples

```bash
# Run auditor against this fixture; expect exit 0 and verdict PASS or WARN
bash skills/skill-auditor/scripts/audit.sh tests/fixtures/skills/known-good
```

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Auditor returns FAIL | Fixture drifted from canonical shape | Restore from git; do not edit without updating tests |
