# CLI subtree — operator pointer

This file is **not** the issue-tracker or workflow source of truth. Read the repo root contracts first:

- [`../AGENTS.md`](../AGENTS.md) — canonical operator contract
- [`../docs/architecture/go-cli.md`](../docs/architecture/go-cli.md) — CLI composition, gate system, evidence boundary

## Issue tracker (br only)

```bash
BEADS_DIR="$(ao beads dir)" br ready              # Find available work
BEADS_DIR="$(ao beads dir)" br show <id>          # View issue details
BEADS_DIR="$(ao beads dir)" br update <id> --claim  # Claim work
BEADS_DIR="$(ao beads dir)" br close <id> -r "Done" # Complete work
```

**Two-store truth:** `br` is this repo's tracker; `bd`/Dolt is the gascity substrate store (a different layer, not this repo's tracker). Do not run `bd` for this repo's tracking here. Sync the private ledger with `git -C "$(ao beads dir)" push`; never stage that ledger from the public repo.

## CLI development

```bash
cd cli && make build   # Build ao binary
cd cli && make test    # Run tests
cd cli && make lint    # Run linter
```
