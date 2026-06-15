# Task: generated inventory edit guard

AgentOps has generated inventories. The sources are edited, then regeneration
updates the generated files. A PR that touches only the generated inventory is a
hand-edit smell.

Write an executable script `check-generated-edits.sh` in the current directory.

Contract: `check-generated-edits.sh <changed-files.txt>`
- exit **1** if a generated inventory path is changed without any matching
  source path;
- generated paths include `registry.json`, `docs/SKILLS.md`,
  `cli/docs/COMMANDS.md`, `docs/cli-surface.json`, and `docs/cli-surface.md`;
- source paths include `skills/`, `skills-codex/`, and `cli/cmd/ao/`;
- exit **0** otherwise.

Just write the script. Do not explain.
