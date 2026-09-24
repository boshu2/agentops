# CLI subtree

Inherit [the root operating contract](../AGENTS.md), including its BD identity,
private-history preservation, fresh-judgment, and delivery-authority rules.

- **CLI architecture:** before changing command composition, gates, or evidence
  boundaries, read [the Go CLI architecture](../docs/architecture/go-cli.md).
- **Tracker work:** follow the root's repository-tracker section. The root
  `.beads/redirect` also applies here. Git and Dolt synchronization follow that
  store's configured policy; discover the actual remote before any authorized
  push. Preserve the pre-migration `.beads` estate as well as `_beads` history.
  A BV recommendation authorizes neither a claim, parallel write, nor closure.
- **Go changes:** follow the root's focused-check, build, vet, test, and lint
  requirements. Commands in [Makefile](Makefile) run from `cli/`; from any
  directory, use `make -C /absolute/path/to/agentops/cli <target>`.

Finish only with evidence for the changed behavior and all required checks;
report unchecked acceptance through the root's `NOT_PROVEN` rule.
