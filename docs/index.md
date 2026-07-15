# AgentOps

AgentOps helps coding agents turn one intent into one evidence-bound engineering
judgment:

```text
RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop
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
