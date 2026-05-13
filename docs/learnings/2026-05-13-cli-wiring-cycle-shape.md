---
title: CLI wiring is a repeatable cycle-shape — 3 production adapters exposed in 3 cycles
date: 2026-05-13
tags: [hexagonal-architecture, ddd-bounded-context, cli, cobra, wire-up, cycle-shape]
source: cycles 144-146 of the 2026-05-12 → 2026-05-13 /evolve session
companion: 2026-05-13-bc-ports-wire-up-arc.md
---

# CLI-wiring cycle-shape

Cycles 144-146 shipped three `ao` subcommands that expose three
production adapters from the 14-port wire-up arc. Each cycle had the
same shape and the same cost. This learning captures the template
so future cycles can replicate it for any production adapter.

## The 3 cycles in numbers

| Cycle | Subcommand | Adapter | Time | LOC | Tests |
|---|---|---|---|---|---|
| 144 | `ao loop history` | `productionLoopReader` (cycle 108) | ~10 min | 335 | 6 |
| 145 | `ao ci latest/recent` | `productionCIStatus` (cycle 117) | ~8 min | 258 | 4 |
| 146 | `ao corpus inject` | `productionCorpusReader` (cycle 112) | ~8 min | 252 | 5 |

8-10 minutes wall-clock. ~250-335 LOC including tests + docs. The
adapter-side complexity dominates the variation — Loop took longer
because it needed JSON slicing logic; CI was simplest because the
adapter already had a clean stub-injectable shape.

## The template

```go
// cli/cmd/ao/<noun>.go
var <noun>Cmd = &cobra.Command{
    Use:   "<noun>",
    Short: "BC<n> <surface> operations",
}

var <noun><Verb>Cmd = &cobra.Command{
    Use:   "<verb> [flags]",
    Short: "Short imperative description",
    Long:  `Long description with Examples block.`,
    RunE:  run<Noun><Verb>,
}

type <noun><Verb>Options struct {
    // flag-derived fields
    writer io.Writer
    // injectFn lets tests substitute the port without real I/O
    injectFn func(ctx context.Context, opts <noun><Verb>Options) ([]ports.X, error)
}

func init() {
    <noun>Cmd.GroupID = "core"
    rootCmd.AddCommand(<noun>Cmd)
    // flag registrations
    <noun>Cmd.AddCommand(<noun><Verb>Cmd)
}

func run<Noun><Verb>(cmd *cobra.Command, _ []string) error {
    // pull flag values, build options, delegate
    return <noun><Verb>Run(cmd.Context(), opts)
}

func <noun><Verb>Run(ctx context.Context, opts <noun><Verb>Options) error {
    if opts.writer == nil { opts.writer = os.Stdout }
    fn := opts.injectFn
    if fn == nil { fn = <noun><Verb>ViaPort }
    items, err := fn(ctx, opts)
    if err != nil { return fmt.Errorf("<noun> <verb>: %w", err) }
    enc := json.NewEncoder(opts.writer)
    for _, item := range items {
        if err := enc.Encode(item); err != nil {
            return fmt.Errorf("<noun> <verb> encode: %w", err)
        }
    }
    return nil
}

func <noun><Verb>ViaPort(ctx context.Context, opts <noun><Verb>Options) ([]ports.X, error) {
    adapter := newProduction<X>(/* construction args */)
    return adapter.<Method>(ctx, /* args */)
}
```

```go
// cli/cmd/ao/<noun>_test.go
// 4-6 tests covering:
//   - stub returns N items → N lines emitted
//   - stub returns empty → 0 bytes emitted
//   - stub error → wrapped error
//   - live root (filesystem fixture) → walks correctly
//   - flag combinations (limit, range, etc.) honored
```

After the .go + _test.go files: `scripts/generate-cli-reference.sh`
regenerates `cli/docs/COMMANDS.md`.

## Why this shape works

1. **Parent noun + verb subcommands.** `ao loop history` reads
   better than `ao loop-history`. The parent groups future
   subcommands (`ao loop write`, `ao loop tail`) under one verb-
   space. cobra handles this natively.

2. **Injectable function field on Options.** Production runs use
   the default port wrapper; tests substitute a stub. This is the
   same pattern cycle 117's `productionCIStatus.runGH` proved —
   refined here from a struct field to an option-bag function.
   Faster than fake-file-tree harnesses and platform-neutral.

3. **Line-delimited JSON output.** One record per line means the
   output composes with `jq -c`, `head`, `grep`, `awk`. Operators
   don't need to remember the schema; they pipe and `jq '.field'`.

4. **Error wrapping with the command name.** `"<noun> <verb>:
   underlying error"` makes debugging easy when the cobra layer
   surfaces an error to stderr.

5. **Validate by live smoke after build.** Each cycle ran `make
   build` then `./bin/ao <noun> <verb> <args>` against the real
   data. Cycle 146's smoke ranked the wire-up-arc learning as the
   top match for "wire-up" — proves end-to-end semantic
   correctness, not just compilation.

## Anti-patterns observed during the 3 cycles

- **Name collisions in cli/cmd/ao.** Cycle 144's first helper was
  named `loadCycleHistory` — collided with an existing function in
  `metrics_health.go`. `go vet` caught it; renamed to
  `loadCycleHistoryViaPort`. **Always grep before naming helpers
  in cli/cmd/ao** (the package is ~150 files).
- **Don't shadow Go builtins.** Cycle 117 used `cap := limit`; same
  rule applies to CLI helpers.
- **Don't import `internal/ports` into test files needlessly.**
  Cycle 144's first test file had a dead import; go vet caught it.

## What this unblocks

Three cycles (144-146) cleared the cycle-122 wire-up arc's named
gap: "production adapters are latent until cross-package consumer
exists." Future:

- Any production adapter can be exposed in ~10 min with this template.
- The remaining BC ports lacking CLI exposure (Citation, Operator,
  Harness, ClaimEvidenceBinder, GateRunner, FactoryAdmission,
  ClaimEvidence, etc.) are now low-cost surfaces to add when needed.
- skills/evolve/SKILL.md Step 0/1.5 prior-failure-injection and
  healing-classifier callers (soc-y5vh.1, .2) can shell out to
  these subcommands instead of reaching for inline awk/jq/gh.

## See also

- `2026-05-13-bc-ports-wire-up-arc.md` (cycle 122) — the arc that
  built the 12 production adapters this template wraps.
- `2026-05-13-substring-sed-rename-overreach.md` (cycle 127) —
  prevention rule that applies to grep-before-name in this template.
- `cli/cmd/ao/loop.go`, `cli/cmd/ao/ci.go`, `cli/cmd/ao/corpus_inject.go`
  — the three reference implementations.
- soc-y5vh.5 (closed cycle 146) — the bd that tracked these 3 slices.
