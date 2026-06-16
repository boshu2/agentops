# CLI subtree — operator pointer

This file is **not** the issue-tracker or workflow source of truth. Read the repo root contracts first:

- [`../AGENTS.md`](../AGENTS.md) — canonical operator contract
- [`../docs/architecture/codebase-overview.md`](../docs/architecture/codebase-overview.md) — map, footguns, active waist

## Issue tracker (br only)

```bash
BEADS_DIR=$PWD/_beads br ready              # Find available work
BEADS_DIR=$PWD/_beads br show <id>          # View issue details
BEADS_DIR=$PWD/_beads br update <id> --claim  # Claim work
BEADS_DIR=$PWD/_beads br close <id> -r "Done" # Complete work
```

**bd/Dolt is retired legacy (2026-06-11).** Do not run `bd` here. Sync the private ledger with `git -C _beads push` — never `git add _beads`.

## CLI development

```bash
cd cli && make build   # Build ao binary
cd cli && make test    # Run tests
cd cli && make lint    # Run linter
```
