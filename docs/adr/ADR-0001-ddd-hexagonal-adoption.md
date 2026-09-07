# ADR-0001: Adopt DDD + Hexagonal Architecture

- **Status:** Accepted, amended for selected CDLC 2026-09-06 (2026-05-12)
- **Author:** AgentOps maintainers (DDD+Hexagonal v1 epic)

## Active disposition — 2026-09-06 CDLC adoption

Retain DDD meaning boundaries, explicit effect seams and testable invariants.
Supersede the original Decision's mandatory `ExecutionPacket` aggregate and
universal `domain/ports/adapters` package prescription. Current composition is
owned by [Go CLI architecture](../architecture/go-cli.md): focused application
services and consumer-owned ports where an actual effect earns the seam, with
app seams elsewhere. No retired execution packet, semantic phase engine or
new package hierarchy is authorized. Native BD owns work; Git owns revisions;
runtimes own execution; fresh judges assess meaning. T06 owns later Go evidence
mechanics; T04 adopts the contract only.

The original context, decision and consequences below remain historical
rationale for the retained architecture, not permission to rebuild the packet.

## Historical context

[`docs/cdlc.md`](../cdlc.md), [`PRODUCT.md`](https://github.com/boshu2/agentops/blob/main/PRODUCT.md), and the `ddd-bounded-context` / `hexagonal-architecture` rows in [`PRACTICE-REGISTRY.md`](https://github.com/boshu2/agentops/blob/main/PRACTICE-REGISTRY.md) already identify DDD + Hexagonal as the load-bearing architectural style. The encoding existed implicitly in `skills/rpi/references/phase-data-contracts.md` (linked-intent packet), `skills/domain/` (ubiquitous language), and the practice slug registry — but it was not mechanically enforced. This left the architecture as documentation rather than code-as-truth.

## Decision

Adopt DDD + Hexagonal as encoded architecture, starting with `ExecutionPacket` as the tracer-bullet aggregate root. The inner hexagon lives at `cli/internal/domain/`, port interfaces at `cli/internal/ports/`, and concrete adapters at `cli/internal/adapters/<name>_<flavor>/`. Mechanical enforcement via:

1. Per-skill `hexagonal_role` frontmatter validated by `scripts/validate-skill-frontmatter.sh`.
2. Auto-generated [`docs/contracts/context-map.md`](../contracts/context-map.md) with a CI drift gate.
3. `Validate()` invariants on aggregates, exercised by `pgregory.net/rapid` property tests.
4. Domain-purity import-boundary check (`cli/internal/domain/` must not import any non-domain `cli/internal/*` subpackage).

## Consequences

**Positive**

- Architectural sovereignty: the contracts live in code, not slides.
- Cross-runtime portability: any runtime that respects the ports can substitute for the current one.
- Testability: invariants are property-testable; adapters are integration-testable in isolation.
- Layered evolution: future migrations (more domain types, more ports, more adapters) follow an obvious pattern.

**Negative**

- One additional architectural layer to maintain — ports + adapters + domain.
- Initial scaffold is more code than a single-layer alternative would be.
- Risk of premature port abstraction: only port things with a second implementation in sight. `IssueTracker` ports `bd` + tasklist; `LLMClient` ports multi-provider; `PacketRepository` is single-impl today and is honest about that.

## References

- Alistair Cockburn, 2005. *Hexagonal Architecture* — <https://alistair.cockburn.us/hexagonal-architecture/>.
- Eric Evans, 2003. *Domain-Driven Design*.
- Bertrand Meyer. *Object-Oriented Software Construction* — Design by Contract.
- [`PRACTICE-REGISTRY.md`](https://github.com/boshu2/agentops/blob/main/PRACTICE-REGISTRY.md) — canonical practice slugs for `ddd-bounded-context`, `hexagonal-architecture`, and `adr`.
- [`docs/cdlc.md`](../cdlc.md) — CDLC doctrine and narrow-waist framing.
- [Ports and Adapters](../architecture/ports-and-adapters.md) — this ADR's companion overview.
- [Context Map](../contracts/context-map.md) — auto-generated map of bounded contexts.
