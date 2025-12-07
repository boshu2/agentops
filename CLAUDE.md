# AgentOps Marketplace — Repository Kernel

## Purpose

Claude Code plugin marketplace implementing the Vibe Coding ecosystem by Gene Kim & Steve Yegge. Educational repository with production-ready patterns for AI-assisted development.

---

## Repository Structure

```
agentops/
├── .claude-plugin/
│   └── marketplace.json     # Marketplace definition
├── plugins/
│   ├── core-workflow/       # Base workflow (research → plan → implement → learn)
│   ├── vibe-coding/         # Vibe Coding framework (5 metrics, 6 levels)
│   ├── devops-operations/   # Kubernetes, Helm, CI/CD patterns
│   └── software-development/ # Python, JS, Go development
└── README.md
```

---

## Opus 4.5 Behavioral Standards

<default_to_action>
By default, implement changes rather than only suggesting them. If the user's intent is unclear, infer the most useful likely action and proceed, using tools to discover any missing details instead of guessing.
</default_to_action>

<use_parallel_tool_calls>
When performing multiple independent operations (reading multiple files, running multiple checks), execute them in parallel rather than sequentially. Only sequence operations when one depends on another's output.
</use_parallel_tool_calls>

<investigate_before_answering>
Before proposing code changes, read and understand the relevant files. Do not speculate about code you have not opened. Give grounded, hallucination-free answers based on actual code inspection.
</investigate_before_answering>

<avoid_overengineering>
Only make changes that are directly requested or clearly necessary. Keep solutions simple and focused. Do not add features, refactor code, or make "improvements" beyond what was asked. Do not create helpers or abstractions for one-time operations.
</avoid_overengineering>

<communication_style>
After completing tasks involving tool use, provide a brief summary of work done. When making significant changes, explain what was changed and why. Keep summaries concise but informative.
</communication_style>

---

## Plugins Overview

| Plugin | Description | Depends On |
|--------|-------------|------------|
| `core-workflow` | Research → Plan → Implement → Learn | None (base) |
| `vibe-coding` | 5 metrics, 6 levels, tracer tests | core-workflow |
| `devops-operations` | Kubernetes, Helm, ArgoCD patterns | core-workflow |
| `software-development` | Python, JS, Go development | core-workflow |

---

## Plugin Development

### Creating a New Plugin

1. Create directory under `plugins/your-plugin/`
2. Add `.claude-plugin/plugin.json` manifest
3. Add components (agents/, commands/, skills/)
4. Register in `.claude-plugin/marketplace.json`

### Plugin Structure

```
plugins/your-plugin/
├── .claude-plugin/
│   └── plugin.json          # Required: manifest
├── agents/                  # Optional: AI specialists
├── commands/                # Optional: slash commands
├── skills/                  # Optional: knowledge modules
└── README.md                # Recommended: documentation
```

### Plugin Manifest (plugin.json)

```json
{
  "name": "your-plugin",
  "version": "1.0.0",
  "description": "What your plugin does",
  "author": "Your Name",
  "license": "Apache-2.0",
  "components": {
    "agents": ["agents/your-agent.md"],
    "commands": ["commands/your-command.md"],
    "skills": ["skills/your-skill"]
  }
}
```

---

## Commands

```bash
# Test plugins locally
/plugin install core-workflow --source ./plugins/core-workflow

# Validate marketplace structure
cat .claude-plugin/marketplace.json | jq .

# Check plugin dependencies
grep -r "dependencies" plugins/*/. claude-plugin/plugin.json
```

---

## Key Files

| File | Purpose |
|------|---------|
| `.claude-plugin/marketplace.json` | Marketplace definition and plugin registry |
| `plugins/*/README.md` | Plugin documentation |
| `plugins/*/.claude-plugin/plugin.json` | Plugin manifests |

---

## Vibe Coding Ecosystem

Based on [Vibe Coding](https://itrevolution.com/product/vibe-coding-book/) by Gene Kim & Steve Yegge.

### Trust Calibration (L0-L5)

| Level | Trust | Use For |
|-------|-------|---------|
| L5 | 95% | Formatting, linting |
| L4 | 80% | Boilerplate, config |
| L3 | 60% | Standard features |
| L2 | 40% | New features, integrations |
| L1 | 20% | Architecture, security |
| L0 | 0% | Novel research |

### The 40% Rule

- **Below 40% context** → 98% success rate
- **Above 60% context** → 24% success rate

### Three Feedback Loops

| Loop | Timeframe | Focus |
|------|-----------|-------|
| Inner | Seconds | Prompts/responses |
| Middle | Hours | Sessions/features |
| Outer | Days-weeks | Architecture |

### FAAFO Promise

**F**ast (10-16x) · **A**mbitious (solo feasible) · **A**utonomous (team output) · **F**un (50% more flow) · **O**ptionality (120x options)

### Failure Patterns

Watch for: Tests Passing Lie, Fix Spiral, Eldritch Horror, Silent Deletion, Confident Hallucination

---

## Conventions

- All plugins inherit from `core-workflow`
- Use Apache-2.0 license for all plugins
- Follow existing patterns in `plugins/core-workflow/` as reference
- Keep READMEs concise with quick start instructions

---

## External Marketplaces (Production Ready)

For real work, use these comprehensive catalogs:

| Marketplace | Size | Command |
|-------------|------|---------|
| AITMPL | 63+ plugins, 85+ agents | `/plugin marketplace add https://www.aitmpl.com/agents` |
| Claude Code Templates | 100+ templates | `/plugin marketplace add davila7/claude-code-templates` |
| wshobson/agents | 63 plugins, 85 agents | `/plugin marketplace add wshobson/agents` |

---

## Resources

- [Vibe Coding Book](https://itrevolution.com/product/vibe-coding-book/) - Gene Kim & Steve Yegge
- [Vibe Ecosystem](https://www.bodenfuller.com/builds/vibe-ecosystem) - Implementation details
- [vibe-check](https://www.npmjs.com/package/@boshu2/vibe-check) - Metrics tool
- [Claude Code Plugins Docs](https://docs.anthropic.com/en/docs/claude-code/plugins)
- [Plugin Marketplaces Docs](https://docs.anthropic.com/en/docs/claude-code/plugin-marketplaces)

---

## Last Updated

December 7, 2025
