# CLI subtree — operator pointer

This file is **not** the issue-tracker or workflow source of truth. Read the repo root contracts first:

- [`../AGENTS.md`](../AGENTS.md) — canonical operator contract
- [`../docs/architecture/go-cli.md`](../docs/architecture/go-cli.md) — CLI composition, gate system, evidence boundary

## Issue tracker (BD)

Use the latest stable `bd`, the same native repository tracker used from the
repo root. The root `.beads/redirect` resolves its private BD/Dolt store from
this subtree too. Check the resolved identity before work:

```bash
bd context --json
bd ready --json
bd show <id> --json
bd update <id> --claim
bd close <id> --reason "Acceptance and validation evidence"
```

`br` is a separate implementation and is not used for this repository. Never
substitute it, run the removed `ao beads dir`, or initialize an empty store to
bypass missing routing. Preserve `_beads` and the pre-migration `.beads` estate
as history. Git and Dolt synchronization follow the configured native store's
policy; do not infer a remote or push private data from these examples.

BV may rank a fresh explicit BD export. Recheck selected IDs in live BD; a
viewer recommendation cannot authorize a claim, parallel write or closure.

## CLI development

```bash
cd cli && make build   # Build ao binary
cd cli && make test    # Run tests
cd cli && make lint    # Run linter
```
