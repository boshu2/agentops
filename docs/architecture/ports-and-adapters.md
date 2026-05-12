# Ports and Adapters

> One-page overview of AgentOps's hexagonal seam. Companion to [ADR-0001: Adopt DDD + Hexagonal Architecture](../adr/ADR-0001-ddd-hexagonal-adoption.md).

AgentOps adopts Alistair Cockburn's 2005 *Hexagonal Architecture* (a.k.a. Ports and Adapters) as the load-bearing structural style for the Go runtime. The inner hexagon — the domain — is the only thing the rest of the system is allowed to depend on. Everything that talks to a runtime, a filesystem, a tracker, an LLM, or a CI workflow is an adapter, plugging into a port the domain declares.

## Inner hexagon: domain

The domain lives at `cli/internal/domain/`. Its first inhabitant is the `ExecutionPacket` aggregate root at `cli/internal/domain/packet/`. The aggregate enforces four invariants in `invariants.go`:

- **I1 — `ErrPlanPathEmpty`.** `plan_path` is non-empty.
- **I2 — `ErrInvalidComplexity`.** `complexity ∈ {fast, standard, full}`.
- **I3 — `ErrInvalidTestLevel` / `ErrEmptyTestLevels`.** `test_levels` is non-empty and every entry is one of `{L0, L1, L2, L3}`.
- **I4 — `ErrEmptyProvenance`.** `provenance.created_at` and `provenance.source` are non-empty.

Invariants are exercised by `pgregory.net/rapid` property tests in `aggregate_property_test.go`. The domain package may not import from any other `cli/internal/*` subpackage — verified mechanically (see ADR-0001).

## Primary (driving) adapters

Primary adapters drive the domain from the outside: a human, an agent, or a workflow calls into them, and they translate the call into a domain operation. Categories currently in use:

- **CLI commands** in `cli/cmd/ao/` — every `ao <verb>` is a driving adapter.
- **Slash commands** in `skills/` — invocations like `/plan`, `/discovery`, `/validation`, `/vibe`.
- **MCP** — Model Context Protocol entry points exposed by `ao` and by skills.
- **Autonomous loops** — `/dream`, `/evolve`, the `agentopsd` daemon scheduler.
- **CI gates** — `scripts/*.sh` and `.github/workflows/validate.yml` jobs that drive validation against the same domain types they would in interactive runs.

## Secondary (driven) adapters

Secondary adapters are driven *by* the domain through a port interface — the domain calls them. The first concrete adapter is `cli/internal/adapters/storage_fs/`, which implements `ports.PacketRepository` against the local filesystem and is exercised by `t.TempDir()` L2 integration tests.

Future driven adapters (not delivered in v1, kept here as a design forecast):

- **Git adapter** — packet history, ratchet snapshots, ADR provenance.
- **Beads / tasklist adapter** — `ports.IssueTracker` against `bd` when available, falling back to the filesystem tasklist.
- **LLM-provider adapters** — `ports.LLMClient` against Claude, Codex, or local providers.

## Ports

Port interfaces live at `cli/internal/ports/`. Three are declared today:

- **`PacketRepository`** (`storage.go`) — abstracts ExecutionPacket persistence: save / load / load-latest.
- **`IssueTracker`** (`tracker.go`) — abstracts epic and issue creation, currently anticipating `bd` and a filesystem tasklist fallback.
- **`LLMClient`** (`llm.go`) — abstracts model completion calls behind a provider-neutral `Complete(ctx, prompt, opts)` shape.

Ports are deliberately narrow. New ports earn their place when at least a second implementation is in sight — the v1 set already meets that bar (tracker has two intended impls; LLM has many; storage has one today and is honest about that in ADR-0001).

## Hexagon diagram

```text
                     Primary (driving)
                ┌────────────────────────┐
   CLI       → │                          │ ← slash commands
   MCP       → │      ╔═══════════╗       │ ← /dream, /evolve
                 │      ║   domain  ║       │
                 │      ║  (inner)  ║       │
   CI gates  → │      ╚═══════════╝       │ ← scripts/*.sh
                 │            ▲              │
                 └────────────│──────────────┘
                              │  ports
                              ▼
              ┌──────────────────────────────┐
              │ Secondary (driven) adapters  │
              │  storage_fs   git   beads    │
              │  LLM provider  …             │
              └──────────────────────────────┘
```

The diagram is intentionally ASCII / `text` fenced. No raster images: agent-context-friendly, mkdocs-strict-safe, and diffable.

## How to add a new adapter

A new driven adapter is a four-step recipe.

1. **Declare the port interface** in `cli/internal/ports/<name>.go`. Keep it small (1–3 methods). Document each method in godoc. If a second implementation is not plausible within the next epic, defer.
2. **Create the adapter package** at `cli/internal/adapters/<name>_<flavor>/` (e.g., `storage_fs`, `tracker_beads`, `llm_claude`). The adapter only imports `cli/internal/domain/...` and `cli/internal/ports`; it must not import `cli/cmd/...` or any sibling adapter.
3. **Add a compile-time interface check** at the top of the adapter file:

   ```go
   var _ ports.<PortName> = (*Adapter)(nil)
   ```

   This catches signature drift the moment the port interface changes.
4. **Write L2 tests** against `t.TempDir()` or a real-ish backing store (a fake server, a temp git repo, a recorded LLM transcript). L2 first, L1 always — per [Go conventions](../standards/golang-style-guide.md).

## References

- Alistair Cockburn, 2005. *Hexagonal Architecture* — <https://alistair.cockburn.us/hexagonal-architecture/>.
- [ADR-0001: Adopt DDD + Hexagonal Architecture](../adr/ADR-0001-ddd-hexagonal-adoption.md) — the decision record this page operationalizes.
- [`PRACTICE.md`](https://github.com/boshu2/agentops/blob/main/PRACTICE.md) line 80 — the load-bearing claim.
- [Context Map](../contracts/context-map.md) — auto-generated bounded-context view of all skills by hexagonal role.
