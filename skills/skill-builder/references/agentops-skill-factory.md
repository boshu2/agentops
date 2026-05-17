# AgentOps Skill Factory Productization

This reference captures the local Codex `agentops-skill-factory` prototype as a
repo workflow. The goal is not to ship the local prototype verbatim; the goal is
to fold its proven behavior into the existing `skill-builder` and
`skill-auditor` pair.

## Clean-room Inputs

Use only AgentOps-owned artifacts:

- `docs/reference/jsm-clean-room-extraction-policy.md`
- `docs/reference/jsm-skill-standards.md`
- `docs/reference/skill-quality-rubric.md`
- `docs/reference/jsm-cli-capability-map.md`
- `docs/reference/jsm-agentops-gap-audit.md`
- `docs/reference/jsm-pilot-upgrade-backlog.md`
- `skills/standards/references/skill-structure.md`
- `skills/standards/references/jsm-attribution.md`

Do not copy protected third-party skill prose, prompts, scripts, names, or
examples into AgentOps skills. Extract reusable structure and quality signals
only.

## Factory Loop

1. Start with the built-in Codex skill-creator shape: a short `SKILL.md` kernel,
   progressive disclosure through `references/`, reusable `scripts/`, optional
   `assets/`, and validation evidence.
2. Score the target skill:

   ```bash
   python3 skills/skill-auditor/scripts/score_agentops_skill.py skills/<name> --markdown
   ```

3. Pick the smallest score-improving patch, usually one of:
   - add or link `SELF-TEST.md`;
   - move bulky context into `references/`;
   - add a focused validation script;
   - add an output contract or explicit quality rubric;
   - tighten trigger language in frontmatter and body.
4. Re-run `skill-auditor`, `heal-skill`, and any target-specific validation.
5. Mirror behavior into `skills-codex/<name>/` or
   `skills-codex-overrides/<name>/` when the Codex runtime needs different
   phrasing or execution instructions.

## Productization Rule

Local prototype skills may guide the workflow, but PRs should land durable
AgentOps artifacts:

- source skill changes under `skills/`;
- Codex runtime changes under `skills-codex/` or `skills-codex-overrides/`;
- reusable scoring/audit scripts under `skills/skill-auditor/scripts/`;
- clean-room standards under `docs/reference/` and `skills/standards/`.

Avoid adding a duplicate top-level skill when an existing AgentOps skill already
owns the domain. Extend `skill-builder`, `skill-auditor`, `rpi`, or `evolve`
instead.
