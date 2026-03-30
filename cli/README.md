# ao - Agent Operations CLI

CLI for the CASS (Contextual Agent Session Search) knowledge flywheel. Automates knowledge capture, retrieval, and reinforcement learning across Claude Code sessions.

## Install

```bash
go install github.com/boshu2/agentops/cli/cmd/ao@latest
```

## Quick Start

```bash
# From your repo root: create `.agents/` + register hooks
ao init --hooks

# Verify installation
ao hooks test
```

That's it. Knowledge now flows automatically between sessions.

## What It Does

**SessionStart**: Injects relevant prior knowledge weighted by freshness and utility.

**SessionEnd**: Extracts learnings and updates the feedback loop.

## Core Commands

| Command | Purpose |
|---------|---------|
| `ao inject` | Inject knowledge into current session |
| `ao forge transcript` | Extract learnings from session transcripts |
| `ao feedback-loop` | Update utility scores based on outcomes |
| `ao metrics report` | View flywheel health and escape velocity |

## Ratchet Workflow

Track progress through the RPI (Research → Plan → Implement) workflow:

```bash
ao ratchet gate plan        # Check if ready for planning
ao ratchet record research  # Lock research completion
ao ratchet status           # View current progress
```

## Hooks Management

```bash
ao init --hooks        # Recommended: repo setup + hooks (SessionStart + Stop)
ao init --hooks --full # Optional: all 8 lifecycle events

# Lower-level (hooks only; does not create `.agents/`)
ao hooks init          # Generate hooks configuration
ao hooks install       # Install to Claude Code
ao hooks show      # View current hooks
ao hooks test      # Verify hooks work
```

## Knowledge Commands

```bash
ao lookup --query "kubernetes"  # Look up knowledge about k8s
ao search "error handling"      # Search knowledge base
```

## Task Integration

Sync Claude Code tasks with CASS maturity system:

```bash
ao task-sync              # Sync current tasks
ao task-sync --promote    # Promote completed tasks
ao task-status            # View maturity distribution
```

## The Science

The flywheel equation:

```
dK/dt = I(t) - δ·K + σ·ρ·K
```

- **δ = 0.17/week** - Knowledge decay rate (Darr et al.)
- **σρ > δ/100** - Operational escape velocity for compounding knowledge

See [docs/HOOKS.md](docs/HOOKS.md) for details.

For a complete command reference, see [docs/COMMANDS.md](docs/COMMANDS.md).

## License

MIT
