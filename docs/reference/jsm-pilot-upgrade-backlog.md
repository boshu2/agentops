# JSM-Informed Pilot Upgrade Backlog

This backlog selects the first AgentOps skills to upgrade using the JSM-informed quality rubric. It does not implement the upgrades.

## Selection Criteria

Pilot skills should be:

- high-traffic in normal AgentOps workflows,
- user-facing or behavior-defining,
- already backed by references and scripts,
- small enough to upgrade without creating a mega-skill,
- useful as examples for the rest of the catalog.

## Pilot 1: `standards`

Status: started. `skills/standards/SELF-TEST.md` now exists as the first minimal pilot improvement. Remaining work should stay focused: tighten trigger wording only if it does not disturb library-skill behavior, then decide whether `standards` should remain repo-runtime only.

Files likely touched:

- `skills/standards/SKILL.md`
- `skills/standards/SELF-TEST.md`
- `skills/standards/references/skill-structure.md`
- `skills/standards/references/jsm-attribution.md`
- `skills-codex/standards/SKILL.md` if runtime body changes

Expected changes:

- Add self-test cases for skill structure, attribution, and export-profile triggers.
- Link the skill quality rubric from the standards body or reference map.
- Preserve repo-runtime versus JSM-export separation.

Validation:

```bash
bash skills/heal-skill/scripts/heal.sh --strict
bash scripts/validate-skill-frontmatter.sh --strict
scripts/check-jsm-export.sh --json skills/standards
```

## Pilot 2: `research`

Files likely touched:

- `skills/research/SKILL.md`
- `skills/research/SELF-TEST.md`
- selected `skills/research/references/*`
- `skills-codex/research/SKILL.md` if runtime body changes

Expected changes:

- Add self-test for prior-art lookup, applicability decisions, and research output contract.
- Add an asset only if a reusable report skeleton is needed.

Validation:

```bash
bash skills/heal-skill/scripts/heal.sh --strict
scripts/check-jsm-export.sh --json skills/research
```

## Pilot 3: `plan`

Files likely touched:

- `skills/plan/SKILL.md`
- `skills/plan/SELF-TEST.md`
- `skills/plan/references/plan-document-template.md`
- `skills-codex/plan/SKILL.md` if runtime body changes

Expected changes:

- Add self-test for baseline audit, issue decomposition, file ownership, and mechanical acceptance checks.
- Ensure worker latitude language is included for small mechanical files needed to satisfy acceptance criteria.

Validation:

```bash
bash skills/heal-skill/scripts/heal.sh --strict
scripts/check-jsm-export.sh --json skills/plan
```

## Pilot 4: `validation`

Files likely touched:

- `skills/validation/SKILL.md`
- `skills/validation/SELF-TEST.md`
- `skills/validation/references/four-surface-closure.md`
- `skills-codex/validation/SKILL.md` if runtime body changes

Expected changes:

- Add self-test for four-surface closure, lifecycle checks, and phase summary output.
- Confirm cheap/default validation lanes remain separate from explicit release gates.

Validation:

```bash
bash skills/heal-skill/scripts/heal.sh --strict
scripts/check-jsm-export.sh --json skills/validation
```

## Pilot 5: `reverse-engineer-rpi`

Files likely touched:

- `skills/reverse-engineer-rpi/SKILL.md`
- `skills/reverse-engineer-rpi/SELF-TEST.md`
- `skills/reverse-engineer-rpi/assets/` if a reusable report template is warranted
- `skills-codex/reverse-engineer-rpi/SKILL.md` if runtime body changes

Expected changes:

- Add self-test for product-spec extraction, evidence boundaries, and no-copy source handling.
- Consider an AgentOps-owned report skeleton in `assets/`.

Validation:

```bash
bash skills/heal-skill/scripts/heal.sh --strict
scripts/check-jsm-export.sh --json skills/reverse-engineer-rpi
```

## Codex Artifact Rule

If any pilot changes skill behavior, phrasing, orchestration, or UX:

```bash
bash scripts/refresh-codex-artifacts.sh --scope worktree
bash scripts/validate-codex-generated-artifacts.sh --scope worktree
```

Keep pilot implementation separate from the standards extraction work.

---

**Source:** Pattern-only comparison of AgentOps skill structure with the user-local JSM corpus. No proprietary source text copied.
