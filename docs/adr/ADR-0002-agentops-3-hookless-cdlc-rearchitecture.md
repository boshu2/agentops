# ADR-0002: AgentOps 3.0 Hookless-First CDLC Rearchitecture

- **Status:** Proposed (2026-05-15)
- **Author:** AgentOps maintainers
- **Supersedes:** none
- **Builds on:** [ADR-0001](ADR-0001-ddd-hexagonal-adoption.md)

## Context

AgentOps has converged on a clearer thesis: it is a context-native SDLC for
LLM agents under token scarcity. The core product is not a hook bundle. The
core product is software-engineering practice encoded into compact,
verifiable, reusable agent context.

Recent evidence changes the architecture posture:

- RPI token measurement found a trivial cycle costing 10.35M tokens and $7.48,
  with 97.6% cache-read. The dominant cost was resident stacked context.
- The workbench A/B result for the hook/skill layer showed `aggregate_delta =
  0` at v1 difficulty.
- `hooks/hooks.json` currently wires 43 hook entries across 12 runtime events
  after removing three prompt-only injection hooks, while product docs and CI
  still need to treat hooks as adapters instead of architectural primitives.
- The DDD/hexagonal inventory already points to bounded contexts and ports as
  the cleaner abstraction.

Hooks were useful as early experiments, but they have not earned default
architectural status for 3.0.

## Decision

AgentOps 3.0 will be hookless-first.

The core architecture is:

1. **Work Lifecycle** - objective, intent, slice, wave, execution packet, work
   order, phase run.
2. **Context Compiler** - context packet, citation, token budget,
   density/freshness/relevance scoring.
3. **Evidence and Trust** - acceptance criterion, validation lane, evidence,
   verdict, gate result.
4. **Knowledge Flywheel** - finding, learning, ratchet, promotion rule,
   provenance.
5. **Skill Catalog** - skill definition, runtime projection, practice
   citation, lease on life.
6. **Runtime Shell** - harnesses, runtime adapters, event sinks, CLI, daemon,
   filesystem, `bd`, git, LLM providers.

Only the first five are domain bubbles. Runtime Shell is an adapter shell.

Hooks are reclassified as optional runtime adapters. A hook may remain only if
it earns a lease on life:

- It has a typed owner port.
- It has no default prompt injection.
- It either blocks a deterministic unsafe mutation, emits a typed event, or
  runs an explicit validation lane.
- It has test or eval evidence showing positive value.
- It can be disabled without breaking the core CDLC loop.

## Consequences

### Positive

- Lower resident context and fewer hidden runtime side effects.
- Clearer product story: context compiler and evidence loop, not hook magic.
- Stronger hexagonal boundary: Codex, Claude, OpenCode, hooks, CLI, daemon,
  `bd`, git, and LLM providers are adapters.
- Easier testing: gates, event subscribers, and explicit commands are easier
  to exercise than hidden lifecycle hooks.
- Better portability across runtimes that do not expose equivalent hooks.

### Negative

- Some behavior currently hidden in hooks must become explicit commands,
  ports, or phase steps.
- Install and docs paths that promise automatic hooks need migration.
- CI has many hook-specific checks that must be retired or recast.
- Operators may initially miss automatic closeout and safety nudges unless the
  replacement UX is crisp.

## Migration Rule

Do not delete hooks before a replacement exists.

Each hook gets one disposition:

- remove
- convert to `GateRunnerPort`
- convert to `EventBusPort` subscriber
- convert to explicit `ao` command or skill step
- retain as optional runtime adapter

Default 3.0 install should be hookless. Optional hook profiles may exist after
they are eval-proven.

## Acceptance

This ADR is accepted when:

- The 3.0 plan defines bounded contexts, ports, and hook dispositions.
- The default install path no longer requires hooks.
- RPI can run a full discovery-to-validation cycle without hooks.
- Hookless RPI token cost is remeasured against the 10.35M token baseline.
- Hook A/B evals demonstrate no regression or identify specific hooks worth
  retaining as optional adapters.

## References

- [CDLC](../cdlc.md)
- [Operating Loop](../architecture/operating-loop.md)
- [Ports and Adapters](../architecture/ports-and-adapters.md)
- [Context Map](../contracts/context-map.md)
- [AgentOps 3.0 Hookless CDLC Rearchitecture Plan](../plans/2026-05-15-agentops-3-hookless-cdlc-rearchitecture.md)
