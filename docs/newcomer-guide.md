# Newcomer guide

AgentOps is a skill-based evidence loop for agent-created work. Start with one
small behavior and use RPI when you want the full loop.

## What to expect

- Plan makes acceptance and scope explicit.
- Implement performs one bounded experiment.
- A fresh validator judges exact content.
- A durable verdict records checked and unchecked claims.
- The invocation stops.

You do not need a tracker, Git repository, multi-agent substrate, or `ao` binary
for the semantic loop. If your project has those systems, keep using their native
workflow after AgentOps reports its verdict.

Use [premortem](https://github.com/boshu2/agentops/blob/main/skills/premortem/SKILL.md)
when a plan deserves an advisory challenge,
[council](https://github.com/boshu2/agentops/blob/main/skills/council/SKILL.md)
when multiple independent views justify the cost, and
[postmortem](https://github.com/boshu2/agentops/blob/main/skills/postmortem/SKILL.md)
only for an explicit causal question.
