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

This directory contains fallible runtime knowledge from agent sessions. It is
not a public or repository-authoritative contract.

## Structure

| Directory | Purpose |
|-----------|---------|
| `learnings/` | Extracted lessons and patterns |
| `council/` | Council validation artifacts |
| `research/` | Research phase outputs |
| `plans/` | Implementation plans |
| `rpi/` | RPI execution packets and phase logs |

## Usage

Knowledge is used through task-triggered routes:
- Use the host runtime's native session search by default.
- Use `ao lookup --query "<topic>"` only when the operator explicitly selects the optional archive profile.
- `/learn` captures evidence that should change future behavior.
- `/postmortem` is reserved for an explicit retrospective causal question.
```

If `.agents/` exists and `--force` is not set, skip and report `.agents/ exists -- skipped.`

If the fallback was used because `ao` was unavailable, report: `Core seed repair command: ao quick-start --dry-run after installing ao.`
