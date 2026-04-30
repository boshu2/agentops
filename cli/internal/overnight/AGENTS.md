---
package: cli/internal/overnight
status: active
owner: agentopsd
contract_source: docs/contracts/dream-run-contract.md
---

# cli/internal/overnight

The Dream nightly compounding loop: bounded outer loop over the local `.agents/` corpus that ingests, reduces, and measures without touching code or git.

## Ownership

- **Owner:** agentopsd extraction track (epic `agentops-tqc`); Dream subsystem.
- **Doc-level contract:** `doc.go` is the package's authoritative contract — read it before touching any file here. The repo-level contract is `docs/contracts/dream-run-contract.md`.
- **Three-stage wave:** INGEST (parallel-safe, read-only) → REDUCE (serial, mutative, checkpointed) → MEASURE (parallel-safe, read-only). Halts on wall-clock budget, plateau, or fitness regression.

## Interfaces

- **Loop entry:** `loop.go` (`Loop`, `Run`) is the main outer loop driver. `longhaul.go` extends it for long-running multi-night sessions.
- **Checkpoint overlay:** `checkpoint.go` (+ `checkpoint_clone_darwin.go` and `checkpoint_clone_fallback.go`) implements 2-phase commit semantics for REDUCE writes. The Darwin-specific clone fast path is platform-gated by build tags.
- **Fitness vector:** `fitness.go` produces the per-iteration fitness signal that drives plateau detection.
- **Generator sidecars:** `generator_sidecars.go` aggregates sidecar findings written under the run output dir during INGEST.
- **Findings router:** `findings_router.go` routes findings to `.agents/rpi/next-work.jsonl` during REDUCE.
- **External watchlist:** `external_watchlist_generator.go` generates the watchlist consumed by external observers.

## Non-obvious rules

- **Anti-goals are load-bearing** (see `doc.go`):
  - Never mutate source code.
  - Never invoke `/rpi` or any code-mutating flow.
  - Never touch git (no commits, branches, pushes, remotes).
  - Never create symlinks.
  - Never fan out to swarm/gc agents inside iterations — only bounded in-process generator goroutines, REDUCE remains the serialized writer.
- **Mutation surface is closed.** The only subpaths Dream writes (and therefore checkpoints) are: `.agents/learnings/`, `.agents/findings/`, `.agents/patterns/`, `.agents/knowledge/`, `.agents/rpi/next-work.jsonl`. Plus harvest writes to `~/.agents/learnings/` (the global hub). Everything else under `.agents/` is untouched.
- **Concurrency guard with harvest.** A guard in `cli/cmd/ao/harvest.go` prevents manual harvest from racing a live Dream run. Don't bypass it.
- **REDUCE is serial, by contract.** Even though INGEST and MEASURE are parallel-safe, REDUCE must remain the single serialized queue writer. Adding parallelism here would break the checkpoint invariant.
- **Delineation vs `/evolve`.** `/evolve` is the day-time loop (code + knowledge, operator-driven, full `/rpi` per cycle). Dream is the nightly loop (knowledge-only, bounded, never `/rpi`). Both share `ao goals measure` plus the new `ao corpus fitness` as fitness sources of truth.

## Cross-references

- Parent epic: `agentops-tqc` (Olympus → agentopsd extraction).
- Contract: `docs/contracts/dream-run-contract.md` (v2 contract pinning the mutation surface).
- Skill surfaces: `skills/dream/SKILL.md`, `skills/evolve/SKILL.md`.
- Pattern source: olympus per-folder `AGENTS.md` ownership convention.
- Sibling packages: `cli/internal/daemon` (`dream.run`, `dream.stage` job types), `cli/internal/goals` (fitness measurement), `cli/internal/harvest` (concurrency-guarded global hub writes).
