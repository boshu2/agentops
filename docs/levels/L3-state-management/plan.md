---
description: Decompose a goal into tracked issues with dependencies
---

# /plan

Breaks down a goal into beads issues. Use when work spans multiple sessions or has dependencies.

## Usage

```
/plan "your goal"
```

## What's Different from L2

At L3, planning creates trackable issues:
- Issues stored in the `_beads/` ledger (br, beads_rust)
- Dependencies tracked (what blocks what)
- Progress visible with `BEADS_DIR=$PWD/_beads br ready`, `br list`

## Steps

1. Claude researches what's needed
2. Claude creates issues with `BEADS_DIR=$PWD/_beads br create`
3. Claude sets dependencies with `br dep add`
4. Plan saved to `.agents/plans/`

## Output

```
Created 5 issues for "Add user authentication":
  [P1] agentops-abc: Set up auth middleware
  [P1] agentops-def: Create login endpoint
  [P2] agentops-ghi: Add session storage (depends on abc)
  [P2] agentops-jkl: Create logout endpoint (depends on def)
  [P3] agentops-mno: Add tests (depends on ghi, jkl)

Ready to work: agentops-abc, agentops-def
```

## Example

```
You: /plan "add dark mode support"

Claude: I'll research and plan dark mode implementation.

[Research existing theme handling...]

Creating issues:

$ BEADS_DIR=$PWD/_beads br create "Add theme context provider" -p 1
Created: agentops-xyz

$ br create "Create dark mode toggle" -p 2
Created: agentops-abc

$ br dep add agentops-abc agentops-xyz

Plan saved to: .agents/plans/2025-01-15-dark-mode.md

Next: `br ready` to see what's unblocked, then `/implement`
```

## Next

- `BEADS_DIR=$PWD/_beads br ready` - See unblocked issues
- `/implement agentops-xyz` - Work on specific issue
