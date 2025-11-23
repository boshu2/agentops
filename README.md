# AgentOps - Personal Claude Code Marketplace

> Personal marketplace demonstrating Claude Code plugin system with references to production marketplaces

A minimal example marketplace for Claude Code that showcases:
- ✅ Official `.claude-plugin/marketplace.json` format
- ✅ Links to external production marketplaces
- ✅ Example plugin structure
- ✅ AgentOps and 12-factor patterns

## Quick Start

### Add This Marketplace

```bash
/plugin marketplace add boshu2/agentops
```

### Install Example Plugin

```bash
/plugin install example-plugin@agentops
```

### Access Referenced Marketplaces

This marketplace references two production-ready marketplaces:

#### 1. Agents Marketplace (Comprehensive)
```bash
/plugin marketplace add wshobson/agents
```

**63 plugins | 85 agents | 47 skills**
- Full-stack development tools
- Multi-agent orchestration
- Language-specific plugins (Python, JS/TS, Rust, Go)
- Infrastructure (Kubernetes, Cloud, CI/CD)
- Security, quality, and performance tools

[→ View agents marketplace](https://github.com/wshobson/agents)

#### 2. AgentOps Showcase (Production Patterns)
```bash
/plugin marketplace add boshu2/agentops-showcase
```

**8 plugin profiles | 92 agents**
- Production-tested from real GitOps operations
- Platform engineering workflows
- DevOps automation
- SRE operations
- Meta-operations (agents for managing agents)

[→ View agentops-showcase](https://github.com/boshu2/agentops-showcase)

## Repository Structure

```
agentops/
├── .claude-plugin/
│   └── marketplace.json          # Marketplace metadata + external references
├── plugins/
│   └── example-plugin/
│       ├── .claude-plugin/
│       │   └── plugin.json       # Plugin metadata
│       └── agents/
│           └── example-agent.md  # Example agent
├── README.md                     # This file
└── LICENSE                       # Apache 2.0
```

## Creating Your Own Plugins

### 1. Create Plugin Directory

```bash
mkdir -p plugins/my-plugin/{.claude-plugin,agents}
```

### 2. Create Plugin Metadata

**`plugins/my-plugin/.claude-plugin/plugin.json`:**

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "My custom plugin",
  "author": "Your Name",
  "license": "Apache-2.0",
  "keywords": ["custom", "example"],
  "components": {
    "agents": ["agents/my-agent.md"],
    "commands": [],
    "hooks": [],
    "mcp": []
  }
}
```

### 3. Create Agent

**`plugins/my-plugin/agents/my-agent.md`:**

```markdown
---
name: my-agent
description: My custom agent
model: sonnet
tools: [Read, Write, Bash]
---

# My Agent

Agent content here following the example-agent structure.
```

### 4. Register in Marketplace

Add to `.claude-plugin/marketplace.json`:

```json
{
  "plugins": [
    {
      "name": "my-plugin",
      "source": "./my-plugin",
      "description": "My custom plugin",
      "version": "1.0.0"
    }
  ]
}
```

## Marketplace Features

### Plugin Sourcing

Supports multiple source types:

```json
{
  "plugins": [
    {
      "name": "local-plugin",
      "source": "./local-plugin"
    },
    {
      "name": "github-plugin",
      "source": {
        "type": "github",
        "owner": "username",
        "repo": "plugin-repo"
      }
    },
    {
      "name": "git-url",
      "source": {
        "type": "url",
        "url": "https://gitlab.com/user/plugin.git"
      }
    }
  ]
}
```

### External Marketplace References

Link to other marketplaces:

```json
{
  "externalMarketplaces": [
    {
      "name": "agents",
      "source": {
        "type": "github",
        "owner": "wshobson",
        "repo": "agents"
      },
      "description": "Comprehensive marketplace",
      "recommended": true
    }
  ]
}
```

## AgentOps Principles

This marketplace demonstrates:

1. **Single Responsibility** - Each plugin does one thing well
2. **Clear Contracts** - Explicit metadata and expectations
3. **Composability** - Plugins work together
4. **Discoverability** - Good descriptions and keywords
5. **Versioning** - Semantic versioning for stability
6. **Documentation** - Clear usage instructions
7. **Examples** - Working demonstration code
8. **Standards** - Following Claude Code plugin specification

## Use Cases

### Personal Development

Perfect for:
- Learning the plugin system
- Testing plugin structures
- Building custom tools
- Experimenting with agents

### Team Distribution

Can be extended for:
- Team-specific plugins
- Internal tool distribution
- Custom workflow automation
- Organization-wide standards

### Integration Testing

Use to test:
- Marketplace integration
- Plugin installation
- Multi-marketplace setup
- External references

## Resources

### Official Documentation

- [Claude Code Plugins](https://docs.claude.com/en/docs/claude-code/plugins)
- [Plugin Marketplaces](https://docs.claude.com/en/docs/claude-code/plugin-marketplaces)
- [Subagents Guide](https://docs.claude.com/en/docs/claude-code/sub-agents)

### Referenced Marketplaces

- [wshobson/agents](https://github.com/wshobson/agents) - 63 plugins, comprehensive
- [boshu2/agentops-showcase](https://github.com/boshu2/agentops-showcase) - Production patterns

### AgentOps Framework

- [12-Factor AgentOps](https://github.com/boshu2/12-factor-agentops) - Operational framework
- [AgentOps Showcase](https://agentops-showcase.com) - Examples and tutorials

## Custom Agent (Claude Agent SDK)

This repository includes a **custom agent implementation** built with the Claude Agent SDK that programmatically implements the 12-Factor AgentOps principles.

### Installation

```bash
# Install dependencies
pip install -r agent/requirements.txt

# Install Claude Code globally
npm install -g @anthropic-ai/claude-code

# Set API key
export ANTHROPIC_API_KEY=your_key_here
```

### Usage

```bash
# Interactive mode
python -m agent.main -i

# Single task
python -m agent.main "Add user authentication with JWT"

# With specific working directory
python -m agent.main --cwd /path/to/project "Your task"
```

### Agent Structure

```
agent/
├── __init__.py      # Package exports
├── main.py          # Entry point and orchestration
├── config.py        # System prompts and configuration
├── tools.py         # Workflow tools (research, plan, implement, learn)
├── skills.py        # JIT skill loading
├── hooks.py         # Constitutional guardrails (Five Laws)
└── requirements.txt # Python dependencies
```

### Features

- **4-Phase Workflow**: Research → Plan → Implement → Learn
- **Constitutional Guardrails**: Enforces the Five Laws through hooks
- **JIT Skill Loading**: Load domain knowledge on-demand
- **Validation Gates**: Blocks dangerous operations, requires testing
- **Metrics Tracking**: Captures learnings and patterns

### Workflow Commands

| Command | Description |
|---------|-------------|
| `/research [topic]` | Start research phase |
| `/plan [summary]` | Create implementation plan |
| `/implement [plan]` | Execute with validation |
| `/learn` | Extract patterns |
| `/workflow [topic]` | Complete 4-phase cycle |

## License

Apache 2.0 - See [LICENSE](LICENSE) file

---

**Created by**: Boden Fuller
**Purpose**: Personal marketplace example + production marketplace references + custom agent
**Status**: Example/Educational
