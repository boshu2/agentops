---
description: Execute a specific issue, validate, commit, close
---

# /implement (L3)

Execute a beads issue by ID. Marks in_progress, makes changes, validates, commits, closes.

## Usage

```
/implement <issue-id>
/implement agentops-abc
```

## What's Different from L2

At L3, implementation is issue-driven:
- Specify which issue to work on
- Issue marked `in_progress` automatically
- Issue closed after successful commit
- Dependencies enforced (blocked issues can't start)

## Steps

1. Claude marks issue `in_progress`
2. Claude reads issue details and makes changes
3. Validation runs (tests, lint)
4. Changes committed with issue reference
5. Claude closes issue with `BEADS_DIR="$(ao beads dir)" br close`

## Output

```
Implementing: agentops-abc "Add theme context provider"

[Read, Edit, Test...]

$ git commit -m "feat: add theme context provider

Closes: agentops-abc"

$ BEADS_DIR="$(ao beads dir)" br close agentops-abc --reason "Theme context implemented"

✓ agentops-abc closed
Next ready: agentops-def (was blocked by abc)
```

## Example

```
You: /implement agentops-abc

Claude: Working on agentops-abc: "Add theme context provider"

$ BEADS_DIR="$(ao beads dir)" br update agentops-abc --claim --json

[Reads requirements, creates src/theme/context.tsx...]

$ npm test
4 passed

$ git commit -m "feat: add theme context provider"
$ BEADS_DIR="$(ao beads dir)" br close agentops-abc --reason "Theme context implemented"

✓ Done. Run `BEADS_DIR="$(ao beads dir)" br ready --json` for next issue.
```

## Session Close Protocol

Before ending, always run:
```
BEADS_DIR="$(ao beads dir)" br sync --flush-only
git -C "$(ao beads dir)" push   # sync the private br ledger (git-JSONL)
git push
```

## Next

- `BEADS_DIR="$(ao beads dir)" br ready --json` - See newly unblocked issues
- `/implement <next-id>` - Continue through the plan
