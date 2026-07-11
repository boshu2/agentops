# Core Seed Procedure

Use this procedure for bootstrap Step 4 only.

If `.agents/` is absent or `--force` is set, prefer the CLI golden path:

```bash
ao quick-start --no-beads
```

If `ao` is unavailable, create the minimal directory structure and report the repair command:

```bash
mkdir -p .agents/learnings .agents/council .agents/research .agents/plans .agents/rpi .agents/patterns .agents/retro .agents/handoff
```

Create `.agents/AGENTS.md` only when it does not exist, with this content:

```markdown
# Agent Knowledge Store

This directory contains accumulated knowledge from agent sessions.

## Structure

| Directory | Purpose |
|-----------|---------|
| `learnings/` | Extracted lessons and patterns |
| `council/` | Council validation artifacts |
| `research/` | Research phase outputs |
| `plans/` | Implementation plans |
| `rpi/` | RPI execution packets and phase logs |

## Usage

Knowledge is managed by explicit AgentOps commands:
- `ao lookup` surfaces relevant prior knowledge on demand.
- `/post-mortem` extracts and processes new learnings.
- `/compile` runs maintenance when that skill is installed.
```

If `.agents/` exists and `--force` is not set, skip and report `.agents/ exists -- skipped.`

If the fallback was used because `ao` was unavailable, report: `Core seed repair command: ao quick-start --dry-run after installing ao.`
