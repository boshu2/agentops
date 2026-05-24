---
name: dream
description: 'Run overnight compounding sessions.'
---
# Dream Skill - Retired (out-of-session compounding moved to GC)

**Status: RETIRED (soc-2rtm0).** The in-tree overnight/dream engine and its CLI
surface (the former ao overnight, ao schedule, ao watch, ao plans commands, and
the dream/schedule wiring inside the agentopsd daemon) were removed. The daemon
itself was carved out in wave 5. AgentOps no longer ships the out-of-session
orchestration substrate — scheduled, between-session knowledge compounding now
runs via **GC** (Gas City). Do not invent a replacement `$ao` workflow.

## What stays in AgentOps

The KEEP knowledge primitives the dream cycle was built on are unchanged and
remain on-demand: `$harvest` (promote `.agents/` knowledge), `$forge` (mine
transcripts into learnings), `$compile` (Mine → Grow → Defrag → Lint), and
`$inject` (JIT decay-ranked context). The nightly-CI dream-cycle proof job in
`.github/workflows/nightly.yml` still runs against the checked-in corpus and is
built on those primitives, not on the retired CLI engine.

## Delineation vs $evolve

`$evolve` owns the daytime code-compounding layer (operator-driven, via `$rpi`).
The knowledge-compounding layer this skill used to own now lives outside
AgentOps (GC). Both still share the fitness-measurement substrate via
`corpus.Compute` / `ao goals measure`.

## See Also

- [How It Works](../../docs/how-it-works.md)
- [CLI Reference](../../cli/docs/COMMANDS.md)
