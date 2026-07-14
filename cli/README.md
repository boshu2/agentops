# ao — AgentOps CLI

`ao` is the deterministic transaction kernel and evidence recorder beneath the
AgentOps operating loop. Agents own intent, implementation, and semantic
judgment. Repositories own Git delivery.

## Install

```bash
go install github.com/boshu2/agentops/cli/cmd/ao@latest
```

## Current executable truth

The command tree is mid-cut. Inspect the binary you actually have rather than
assuming a profile or narrative list:

```bash
ao capabilities
ao robot-docs
ao --help
```

The generated [command reference](docs/COMMANDS.md) follows executable source
and is not hand-maintained in an authority-doc change.

## Final boundary

The direct-cut program converges on one profile-free command tree. Its lifecycle
transactions are:

- pull one ready work leaf;
- freeze exact candidate identity;
- run or reuse exact-input deterministic evidence;
- record a PASS or FAIL supplied by an external fresh-context validator;
- record one Learn receipt;
- record repository-owned delivery and remote identity; and
- close the report and tracker leaf after verification.

These transactions do not ask a model to judge work and do not push, merge,
queue, or select CI policy. Local and cloud agents use the same ports.

K5, K7, and K9 own deletion of the old verdict-driving, delivery, and retired
gate implementations. Exact `CLI.<source>` leaves own the remaining command
dispositions. F4 removes alternate build profiles. D2 regenerates this command
surface only after executable ownership is final.

## Development

During the cut, use the focused test named by the owning leaf. Do not add new
profile membership, `init()` registration, package-global command ownership,
compatibility aliases, or dormant scaffolds.

## References

- [Operating loop](../docs/architecture/operating-loop.md)
- [Go CLI architecture guide](../docs/architecture/go-cli-architecture-guide.md)
- [Direct-cut ADR](../docs/adr/ADR-0012-focus-surface-on-membrane-bookkeeper-archive-satellites.md)
