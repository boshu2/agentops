# Explicit Skill Request Structure

This suite checks explicit `/agentops:<name>` fixture addresses against current
canonical `skills/<name>/SKILL.md` and projected `skills-codex/<name>/SKILL.md`
artifacts, including matching frontmatter names and manifest validation. Every
current canonical skill has a fixture. Removed names are not runtime aliases and
have been replaced by fixtures for the current inventory.

This is Tier S structural proof under
[the runtime charter](../../docs/contracts/multi-runtime-tier-charter.md).
It launches no Claude print worker and proves neither natural-language selection,
live invocation, first-tool ordering nor release C1/C8 live qualification.
The maintained Claude registration smoke is
`tests/skills/test-runtime-claude-code-smoke.sh`; live host qualification remains
separate and unproven by these checks.

```bash
bash tests/explicit-skill-requests/run-all.sh
bash tests/explicit-skill-requests/run-test.sh research
bats tests/scripts/explicit-skill-requests.bats
```

The optional repository-root argument exists for negative test fixtures. Missing
targets, mismatched frontmatter names, wrong explicit addresses, missing current
fixtures and invalid manifests must fail. The regression suite tests those
boundaries without fabricated runtime transcripts.
