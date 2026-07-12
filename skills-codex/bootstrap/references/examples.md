# Bootstrap Examples

## Bare Repository

**User says:** `/bootstrap`

The workflow detects no AgentOps artifacts, runs `/goals init`, `/product`, and `/doc --mode=readme`, creates the `.agents/` structure, leaves hooks optional, and reports all core artifact statuses.

## Partial Repository

**User says:** `/bootstrap`

When `GOALS.md` and `.agents/` already exist, the workflow preserves them, runs only the missing product and documentation steps, and reports created and skipped artifacts separately.

## Dry Run

**User says:** `/bootstrap --dry-run`

The workflow detects repository state and reports the create/skip plan without writing files.
