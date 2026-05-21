# AGENTS-CODEX.md — Codex skill parity, CLI skill-map maintenance, audit scripts

> Sibling of [`AGENTS.md`](AGENTS.md), [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md), [`AGENTS-CI.md`](AGENTS-CI.md), [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md). Split out of the monolithic AGENTS.md per soc-vuu6.3.

### CLI Skill-Map Refresh

After changing `ao` command usage in any of these locations, refresh [`docs/cli-skills-map.md`](docs/cli-skills-map.md):

- `skills/*/SKILL.md`
- `skills-codex/*/SKILL.md`
- `hooks/*.sh`
- `hooks/hooks.json`

Process:
1. Update this map from current sources.
2. Run `bash scripts/validate-hooks-doc-parity.sh`.
3. Run `bash tests/docs/validate-doc-release.sh` and `bash tests/docs/validate-skill-count.sh` before pushing.

### Codex Skill Maintenance

Codex is a first-class runtime in this repo.

- `skills/<name>/SKILL.md` is the canonical behavior contract.
- `skills-codex-overrides/<name>/` is the Codex-specific tailoring layer.
- `skills-codex-overrides/catalog.json` is the machine-readable treatment map for the full catalog.
- `skills-codex/<name>/` is the checked-in Codex runtime artifact. It is manually maintained, while the legacy manifest/marker files remain part of the validation contract.

When a skill change affects Codex behavior, phrasing, orchestration, or UX:

1. Update the source skill under `skills/` when the shared contract changes.
2. Update `skills-codex/<name>/SKILL.md` directly when the Codex runtime copy needs to change, or update `skills-codex-overrides/<name>/` when the Codex experience should differ from Claude.
   - Prompt/operator-layer changes belong in `skills-codex-overrides/<name>/prompt.md`.
   - Durable Codex-only body rewrites belong in `skills-codex-overrides/<name>/SKILL.md`.
3. Run the semantic audit if the checked-in Codex body looks suspicious:
   ```bash
   bash scripts/audit-codex-parity.sh
   # or target one skill
   bash scripts/audit-codex-parity.sh --skill <name>
   ```
4. Validate the checked-in Codex artifacts:
   ```bash
   bash scripts/audit-codex-parity.sh
   bash scripts/validate-codex-override-coverage.sh
   bash scripts/validate-codex-generated-artifacts.sh --scope worktree
   bash scripts/validate-codex-backbone-prompts.sh
   bash scripts/validate-codex-rpi-contract.sh
   bash scripts/validate-codex-lifecycle-guards.sh
   bash scripts/validate-headless-runtime-skills.sh
   ```

Think of `skills/` as the shared contract, `skills-codex-overrides/` as the durable Codex-only tailoring layer, and `skills-codex/` as the checked-in Codex artifact shipped to users.

