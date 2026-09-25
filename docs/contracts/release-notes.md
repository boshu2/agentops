# Release Notes Contract

Curated release notes live at
`docs/releases/YYYY-MM-DD-vX.Y.Z-notes.md`. They are repository release
artifacts, not skill output and not semantic verdicts.

Each file contains:

- `## Highlights`
- `## Upgrade Notes`
- `## Breaking Changes` for a major release
- `## At a Glance`
- `## Product Areas`
- `## Known Issues`
- a `Full changelog` link

Product-area headings under `## Product Areas` use this taxonomy:

- Install, Upgrade, and Distribution
- CLI and Operator Commands
- Daemon, Scheduling, and Factory
- Skills and Workflows
- Codex and Runtime Integrations
- Hooks and Lifecycle
- Eval, Validation, and Release Gates
- Docs and Onboarding
- Security, Privacy, and Supply Chain
- Contributor/Internal Refactors

Top-level bullets sit under a `### <product area>` heading.

`scripts/validate-release-notes.sh` is the executable owner of section, taxonomy,
bullet-placement, and changed-path coverage checks. `scripts/scaffold-release-notes.sh`
may create a draft; it never publishes, tags, or declares a release valid.
