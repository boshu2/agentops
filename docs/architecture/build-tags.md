# Build tags: archiving the satellites (ADR-0012)

> How AgentOps keeps the unproven corpus/flywheel and RPI/factory satellites
> **buildable but omitted by default**. Decision: [ADR-0012](../adr/ADR-0012-focus-surface-on-membrane-bookkeeper-archive-satellites.md).
> The satellites are **archived, not deleted**, because the ADR-0004/0009/0011
> revival conditions require the code to stay compilable.

## The two tags

| Tag | Archives | Bead |
|-----|----------|------|
| `flywheel` | corpus/flywheel commands + packages (forge, compile, wiki, harvest, mine, pool, maturity, refinery, flywheel, dedup, defrag, curate, ratchet, …) | `…m1wg.13` |
| `legacy` | RPI/factory commands (orchestrate, codex lifecycle, tick, autodev, evolve, loop, turn, harness, operator) | `…m1wg.14` |

The **default** `go build ./...` (and the shipped `ao`) compile **neither**. They are the spine.

## How to build each variant

```bash
make build                    # spine — archived sets omitted (default)
make build-flywheel           # restore BOTH (-tags "flywheel legacy")
AGENTOPS_LEGACY=1 make build  # restore the legacy (RPI/factory) set only
cd cli && go build -tags flywheel ./...          # flywheel only
cd cli && go build -tags "flywheel legacy" ./... # both
```

Introspect a binary: `ao buildtags` prints `spine` (default) or the tags it was built with.

## How to archive a command behind a tag

Build tags are **file-level** in Go. To archive a command:

1. Put the command's definition + its `init()` registration (the `rootCmd.AddCommand(...)`
   call) in a file carrying the tag constraint as the **first line**, followed by a blank line:

   ```go
   //go:build flywheel

   package main
   // … the cobra command var + func init(){ rootCmd.AddCommand(fooCmd) } …
   ```

   With the tag absent, the file is not compiled, so the command is neither built
   nor registered — it simply isn't in the spine binary.

2. **Self-containment is the rule.** Any symbol the archived command references
   (helper funcs, vars, packages) must also be tag-gated, or the spine build
   breaks with "undefined". Move shared helpers that the spine still needs into an
   untagged file; move helpers only the archived command uses into the tagged file.
   If a spine file references an archived symbol, provide a `//go:build !flywheel`
   stub.

3. Whole packages (e.g. `internal/wiki`, `internal/pool`, `internal/ratchet`) are
   archived by tagging every file in the package, or by ensuring the only importers
   are themselves tag-gated.

4. Retag the disposition ledger and regenerate: `make regen-all`, then `make regen-check`.

## Verifying the mechanism

`make verify-buildtags` (→ `scripts/verify-buildtags.sh`) compiles the spine,
`flywheel`, `legacy`, and combined variants and asserts `ao buildtags` reports
each correctly. The default build must omit the tagged sets; the tags must restore
them. The `cmd/ao/buildtags*.go` files are the mechanism's anchor and self-test.
