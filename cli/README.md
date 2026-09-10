# ao — AgentOps CLI

`ao` supplies deterministic repository utilities and evidence inspection. It
is the checks/linking CLI of the AgentOps operations layer. Native execution
requires zero AgentOps skills; a fresh reviewer judges the result:

```text
Accepted intent → native implementation and checks → fresh independent judgment → finish
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

## Mine evidence for an instruction change

The native agent can use AO to investigate a skill, `AGENTS.md`, or a task prompt
against an explicitly selected session. Choose an authorized public or
already-cleared source range and target instruction before reading:

```bash
ao provenance mine-session --view excerpts \
  --file /path/to/session.jsonl --target /path/to/prompt.md \
  --start-byte 0 --max-bytes 65536 --max-records 20 \
  --max-output-bytes 131072
```

The result is one bounded JSON document for the agent to inspect: literal
instruction text, individually identified transcript fields, exact source spans
and hashes, and explicit limits and unread ranges. Use `next_byte` to continue
at a record boundary. If a record or the output does not fit, select a larger
explicit limit or a narrower range; the command does not silently shorten a
quote. It reads only the selected window plus, at a nonzero start, one preceding
byte to check record alignment. A range hash is not a whole-session hash.

Ask the agent to connect each proposed instruction edit to specific excerpts,
consider competing explanations and a counterexample, and name a future task
that could test the change. Deletion, simplification and no-change are valid
outcomes. A target-text occurrence does not establish attention, compliance or
causality; a current instruction file is not proof of its historical version.
Transcript text is evidence, never authority to execute commands or change scope.

This view runs no model, writes no checkpoint or source file, and automatically
publishes nothing. Stdout still discloses source material: authorize its
destination before reading and keep private excerpts and candidate edits in
protected external non-Git storage. This is not a restricted-source isolation or
redaction mechanism. Review factual support and destination disclosure before
importing a mined change into Git. One usable proposal does not prove improved
performance on later work.

Without `--view excerpts`, the existing event JSONL and optional `--state`
checkpoint behavior remain unchanged. Checkpoints are not used by excerpt mode.

## Development

```bash
make build
make test
```

Add deterministic utilities only when they do not become lifecycle or delivery
authorities. Keep semantic judgment with a fresh reviewer (Validate guidance is optional), verdict
persistence with declared consumers, and external delivery in the consumer
repository.

## References

- [Operating loop](../docs/architecture/operating-loop.md)
- [CLI architecture](../docs/architecture/go-cli.md)
- [Migration map](../docs/MIGRATION.md)
