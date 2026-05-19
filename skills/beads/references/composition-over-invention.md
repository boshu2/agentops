# Composition Over Invention (bd Primitives)

When writing docs, scripts, or workflow rules that reference `bd`
subcommands, **run `bd <subcmd> --help` first.** `bd` has *real*
primitives that compose into most coordination semantics. Inventing a
plausible-sounding command name (`bd lease`, `bd lock`, `bd reserve`)
produces an unimplementable spec that erodes trust the moment someone
tries to run it.

## The Trigger

You're about to write a doc, rule, or script that uses a `bd` command,
and the command name sounds plausible — *especially* if the rule needs
"locking," "leasing," "claiming," or "reserving" semantics. Pause and
verify the command exists.

## The Verifiable Surface

`bd --help` lists the actual subcommand set:

```
bd hooks ready list show create update close
   worktree merge-slot gate set-state dolt
   remember memories recall forget prime context
```

**No `lease`. No `lock`. No `reserve`.**

The composition primitives that produce those semantics:

| Intent | Real bd command |
|---|---|
| Intake / take ownership of an issue | `bd update <id> --claim` |
| Create an isolated workspace for the work | `bd worktree create --branch <type>/<bead-id>-<slug>` |
| Serialize merges across concurrent agents | `bd merge-slot` |
| Mark an issue blocked / waiting | `bd update <id> --status=blocked` |
| Record an insight for future cycles | `bd remember "<insight>"` |
| Look up prior insights | `bd memories <keyword>` |
| Pull/push the dolt-backed history | `bd dolt pull` / `bd dolt push` |

## Evidence (anchored)

> "In the SDLC-shape duel's Round 1, Opus 4.7's proposal included
> `bd lease` as a soft-signal command for multi-agent coordination.
> Codex gpt-5.5 caught it: `bd help lease` returns 'unknown topic.'
> bd has *real* primitives — `bd update --claim` (intake),
> `bd worktree create --branch` (workspace), `bd merge-slot` (merge
> serialization) — which compose into the same semantics without
> inventing a new command. Opus conceded in Round 3."
— `.agents/learnings/2026-05-17-bd-real-primitives.md`

The fix is never "build the missing command." The fix is "find the
composition of existing primitives that achieves the same intent."

## How To Apply

1. **Before writing any `bd <subcmd>` in a doc, run it.**
   ```bash
   bd <subcmd> --help
   ```
   If you see "unknown topic" or "unknown command," the subcommand
   doesn't exist.

2. **Identify the actual intent.** Is it intake? Workspace creation?
   Merge serialization? Insight capture? Map intent to the primitive
   table above.

3. **Compose, don't invent.** Express the rule with the existing
   primitives. If two or three primitives together achieve the goal,
   that's the spec.

4. **Cross-vendor catches this reliably.** Codex bias toward
   "composition over invention" reliably catches invented commands
   under `/council --mode=debate`. For tooling specs, lean on the duel.

## Failure Mode

A spec that references a nonexistent `bd <subcmd>` is unimplementable
until someone runs it and gets "unknown topic." If that happens during
a long autonomous loop (`/evolve`, `/loop`), the cycle stalls and the
agent re-derives the failure each fire until someone notices. Pre-flighting the `--help` check costs ~1 second; the
recovery from a phantom command can cost a whole cycle.

## When This Rule Applies Beyond bd

The same principle applies to any tool whose surface is
operator-facing: `gh`, `ao`, `ntm`, `dolt`. **Before referencing
`<tool> <subcmd>` in a doc, run `<tool> <subcmd> --help`.** If the
output says "unknown," compose existing primitives. The bd case is the
canonical example because the duel-caught fabrication is durable
evidence — but the rule generalizes.

## Falsified By

- bd's command surface changes (e.g., a future `bd lease` actually
  ships). Re-verify on each major bd upgrade.
- A different tracker replaces bd. The rule transfers; the surface
  inventory needs re-derivation.

## See Also

- `references/CLI_REFERENCE.md` — the bd command catalog
- `references/BR_REFERENCE.md` — the `br` (Rust port) catalog
- `references/ANTI_PATTERNS.md` — common bd misuse patterns
- `skills/council/` — the duel forum where this rule was empirically
  validated (Round 1-3 of the 2026-05-17 SDLC-shape council)
