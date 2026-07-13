# Project Memory

This is a small, fallible projection of lessons that still change current work.
Executable behavior and declared contracts outrank it.

- **Earn autonomy with evidence.** Increase unattended scope only after the
  relevant lane repeatedly satisfies its acceptance and escape-rate threshold.
  Source: [autonomy ladder](docs/architecture/autonomy-ladder.md).
- **No verdict means not done.** Deterministic checks prove facts; a fresh-context
  reviewer binds the semantic verdict to the exact candidate. Source:
  [pawl contract](docs/contracts/pawls.md).
- **Default to one writer.** Parallel lanes require independent outputs and
  disjoint write scopes; shared paths serialize. Source:
  [agent workflow reference](docs/agent-workflow-reference.md).
- **Edit source owners, not projections.** Generated skill and runtime artifacts
  are regenerated through their declared owners. Source:
  [Codex skill API](docs/contracts/codex-skill-api.md).
- **Local memory is not authority.** `.agents/` is workspace-local runtime state;
  promote only learning that changes a tracked plan, skill, test, contract, or
  gate. Source: [agent write surfaces](docs/contracts/agents-write-surfaces.md).
