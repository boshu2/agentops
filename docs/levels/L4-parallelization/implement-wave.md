---
description: Execute all unblocked issues in parallel using sub-agents
---

# /implement-wave

Runs all ready issues in parallel. Each issue gets a sub-agent. Results batched into single commit.

## Usage

```
/implement-wave
```

## What's Different from L3

At L4, parallelization speeds execution:
- Multiple issues run simultaneously
- Sub-agents handle each issue independently
- Single commit captures all wave changes
- Dramatically faster for independent work

## How It Works

1. Claude runs `BEADS_DIR=$PWD/_beads br ready` to find unblocked issues
2. Spawns sub-agent for each issue (max 3 per wave)
3. Sub-agents work in parallel via Task tool
4. Results merged and validated
5. Single commit closes all wave issues

**Why max 3?** Each subagent returns results that accumulate in context. Capping at 3 prevents context overflow on complex issues while still providing meaningful parallelism.

## Output

```
Wave 1: 3 issues ready

Launching sub-agents:
  → agentops-abc: Add login form
  → agentops-def: Add logout button
  → agentops-ghi: Add session display

[Sub-agents complete...]

All 3 issues completed successfully.

$ git commit -m "feat: add auth UI components

- Login form (agentops-abc)
- Logout button (agentops-def)
- Session display (agentops-ghi)

Closes: agentops-abc, agentops-def, agentops-ghi"

$ BEADS_DIR=$PWD/_beads br close agentops-abc agentops-def agentops-ghi

Wave 1 complete. Run `br ready` for Wave 2.
```

## Example

```
You: /implement-wave

Claude: Checking ready issues...

$ BEADS_DIR=$PWD/_beads br ready
1. [P1] agentops-xyz: Create user model
2. [P1] agentops-abc: Create order model
3. [P2] agentops-def: Add database migrations

Launching 3 sub-agents...

[3 parallel agents work...]

✓ All complete. Tests passing.

$ git commit -m "feat: add data models and migrations"
$ BEADS_DIR=$PWD/_beads br close agentops-xyz agentops-abc agentops-def

Done. Next wave has 2 issues ready.
```

## When to Use

- Multiple independent issues ready
- Issues don't share file dependencies
- Want maximum velocity

## Next

- `BEADS_DIR=$PWD/_beads br ready` - See next wave
- `/implement-wave` - Run next wave
- `/retro` - Extract learnings after completing plan
