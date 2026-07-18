# ao — AgentOps CLI

`ao` supplies deterministic repository utilities and evidence inspection. The
AgentOps semantic loop lives in the skills:

```text
RPI → Plan → Implement → fresh Validate → durable verdict → report and stop
```

The CLI does not own retries, queues, work claims, Git delivery, release,
closure, or semantic validation. Consumer repositories choose their own Git and
CI policy.

## Install

```bash
go install github.com/boshu2/agentops/cli/cmd/ao@latest
```

## Current executable truth

```bash
ao capabilities
ao robot-docs
ao --help
```

The generated [command reference](docs/COMMANDS.md) follows the published Cobra
tree. Removed lifecycle commands are not registered at all: invoking one fails
as an unknown command with a pointer to its replacement, and no build tag or
compatibility profile restores their implementation.

## Development

```bash
make build
make test
```

Add deterministic utilities only when they do not become lifecycle or delivery
authorities. Keep semantic judgment in the Validate skill and external delivery
in the consumer repository.

## References

- [Operating loop](../docs/architecture/operating-loop.md)
- [CLI architecture](../docs/architecture/go-cli.md)
- [Migration map](../docs/MIGRATION.md)
