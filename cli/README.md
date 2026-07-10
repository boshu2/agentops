# ao — AgentOps CLI

`ao` is the local verification membrane and durable bookkeeper for agent work.
It turns a committed change into an independently checked verdict and records
that verdict in a hash-chained provenance ledger: no verdict means not done.

The default binary intentionally exposes the membrane/bookkeeper spine from
[ADR-0012](../docs/adr/ADR-0012-focus-surface-on-membrane-bookkeeper-archive-satellites.md).
Experimental corpus/flywheel commands are restorable with the `flywheel` build
tag; retired factory/orchestration commands are restorable with `legacy`.

## Install

```bash
go install github.com/boshu2/agentops/cli/cmd/ao@latest
```

## One live path

From a repository root:

```bash
ao quick-start
ao session bootstrap
ao beads tracker
ao beads ready
git add .
git commit -m "fix: first validated change"
ao verify my-first-change
ao gate check --fast --scope head
```

`ao quick-start` creates the local readiness seed. `ao session bootstrap`
orients the agent. `ao beads tracker` reports the selected BR/BD backend, and
all tracker consumers use that same selection. `ao verify` obtains and records
the first independent verdict. The gate is the final pre-push windshield.

## Default surfaces

| Surface | Purpose |
|---|---|
| `ao capabilities` | Recursive, versioned machine contract for the live command tree |
| `ao quick-start` | Seed a repository and print the one live first-verdict path |
| `ao beads` | Resolve and operate through the selected tracker |
| `ao pawl` / `ao verify` | Independent review and commit-bound verdict |
| `ao gate` / `ao validate` | Deterministic release checks and verdict-as-exit-code validation |
| `ao provenance` | Append, inspect, export, and verify the hash-chained ledger |
| `ao session` | Session bootstrap and closeout bookkeeping |
| `ao goals` / `ao claim` | Intent, fitness, ownership, and evidence bindings |
| `ao skills` | Inspect the checked-in skill contracts |

Machine consumers should begin with `ao capabilities`. Global output formats
are closed to `table`, `json`, and `yaml`; a leaf-local flag with the same name
retains its local meaning.

## Build variants

```bash
go build ./cmd/ao
go build -tags flywheel ./cmd/ao
go build -tags legacy ./cmd/ao
go build -tags 'flywheel legacy' ./cmd/ao
```

Run `../scripts/verify-buildtags.sh` from this directory's parent to prove all
variants compile and that the default executable membership remains focused.

## Reference

- [Generated CLI reference](docs/COMMANDS.md)
- [ADR-0012](../docs/adr/ADR-0012-focus-surface-on-membrane-bookkeeper-archive-satellites.md)
- [Operating loop](../docs/architecture/operating-loop.md)
- [Pawl contract](../docs/contracts/pawls.md)
