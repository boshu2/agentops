# AgentOps plugin for Claude Code

This folder is the AgentOps Claude Code plugin: the skills, agents, the policy
hook dispatcher, and the Workflow tool scripts that the plugin loads.

Install it from the AgentOps marketplace:

```bash
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace
```

Documentation, the `ao` CLI, and other install paths live in the repository
README: https://github.com/boshu2/agentops#readme

The component folders here are generated from the repository's canonical
`skills/`, `hooks/`, `agents/`, and `workflows/` by
`scripts/regen-plugin-tree.sh`. Edit the canonical folders, not this copy.
