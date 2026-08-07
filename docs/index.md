# AgentOps

AgentOps is the operations layer for agentic engineering: portable skills and
evidence contracts connecting intent, coding agents, software factories,
context sources, and independent judgment — without replacing the systems
that own work, execution, or delivery. It turns one intent into one
evidence-bound engineering judgment:

```text
RPI -> Plan -> Implement -> fresh Validate -> report and stop
```

The product supplies behavior-first planning, one bounded implementation
experiment, deterministic content identity, fresh independent judgment, and a
standalone content-addressed verdict. It does not own retries, work queues,
trackers, Git, CI, release, or delivery.

## Start here

- [First value path](first-value-path.md)
- [How it works](how-it-works.md)
- [Architecture](ARCHITECTURE.md)
- [Skill router](SKILL-ROUTER.md)
- [CLI reference](cli/commands.md)
- [Migration](MIGRATION.md)

The `ao` CLI is a supporting tool for deterministic repository checks and
evidence inspection. The semantic loop can run in a non-Git directory without
`ao`.
