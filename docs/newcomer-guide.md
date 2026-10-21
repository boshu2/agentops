# Newcomer guide

AgentOps adds optional skills, `ao` tools and evidence around your native
coding agent. Start with one small behavior; native execution needs no AgentOps
skill. Select RPI only when you want the full workflow.

## What to expect

- Plan makes acceptance and scope explicit when they are missing.
- Implement performs one bounded experiment and runs the repository's checks.
- For an ordinary change those checks and CI are the gate. One fresh
  author-distinct read happens only when you ask, a mistake cannot be cheaply
  undone after it lands, or no deterministic check covers the changed behavior.
- The result records checked and unchecked claims; durable storage is optional.
- The invocation stops.

The [agent workflow reference](agent-workflow-reference.md) states these steps.

The one thing you do need is a coding agent to run the skills in: AgentOps
skills execute inside your agent runtime (Claude Code, Codex, Cursor, …), and
`/rpi` is typed in that agent's chat. You do not need a tracker, Git
repository, multi-agent substrate, or `ao` binary for the semantic loop. If
your project has those systems, keep using their native workflow after
AgentOps reports its result.

Use [premortem](https://github.com/boshu2/agentops/blob/main/skills/premortem/SKILL.md)
when a plan deserves an advisory challenge,
[council](https://github.com/boshu2/agentops/blob/main/skills/council/SKILL.md)
when multiple independent views justify the cost, and
[postmortem](https://github.com/boshu2/agentops/blob/main/skills/postmortem/SKILL.md)
only for an explicit causal question.
