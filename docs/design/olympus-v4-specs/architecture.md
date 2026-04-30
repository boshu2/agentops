> **Status:** olympus v4 reference, NOT canonical for agentopsd. This file is a verbatim port from olympus/docs/specs/v4/ for cross-reference only. Where this disagrees with agentopsd's canonical design at `.agents/design/2026-04-28-design-agentops-daemon-gascity-vertical-slices.md`, **agentopsd canonical wins**.

# Olympus v4 Architecture

**Status:** Normative
**Date:** 2026-02-15

## What Olympus Is

Olympus is a **personal AI workspace daemon** that manages context, agents, and knowledge across all your repos. One user. One system. One daemon.

It replaces Gas Town with an opinionated system built around one insight: **context is the control plane, agents are the runtime, code is ephemeral, knowledge compounds forever**.

The `ol` CLI is the single entry point — used manually or driven by the daemon. Both paths share the same validation and ratchet guarantees.

## Principles

1. **Context is the control plane** — The context window IS the agent. Go code is plumbing.
2. **Beads = WIP, Git = accepted** — All workflow state in beads. Ratcheted work in git.
3. **No self-grading** — Validation is external and deterministic. Workers never judge their own output.
4. **Ratchets lock progress** — Merged code, closed beads, stored knowledge never go backward.
5. **Failures are information** — Post-mortems and retros harvest learnings. Failures compound into wisdom.
6. **Fresh context > rework** — Spawn a new agent with curated feedback rather than retrying in saturated context.
7. **Constraint injection > documentation** — Automated tests prevent recurrence. Markdown does not.

## HYDRA Layers

Five layers, each addressing a specific failure mode:

```
┌─────────────────────────────────────────┐
│  Layer 4: Knowledge Flywheel            │
│  Learnings → constraint tests → CI      │
├─────────────────────────────────────────┤
│  Layer 3: Validation Pipeline           │
│  Stage 1 (mechanical) → Stage 2 (merge) │
├─────────────────────────────────────────┤
│  Layer 2: HERO Loop                     │
│  Hunt → Embark → Ratchet → Odyssey      │
├─────────────────────────────────────────┤
│  Layer 1: Bead Compiler                 │
│  Spec → well-specified work unit        │
├─────────────────────────────────────────┤
│  Layer 0: Constraint Engine             │
│  Learnings compiled to automated tests  │
└─────────────────────────────────────────┘
```

**Removed from v3:** Strategy Selector layer. One bead = one attempt. No adaptive N, no diversity seeds, no strategy routing. If it fails, spawn fresh with feedback (Brownian Ratchet).

### Layer 0: Constraint Engine

Every learning compiles to an automated test. Documentation failed to prevent the dead-code-at-entry-point bug three times. A constraint test prevented it on first deployment.

### Layer 1: Bead Compiler

The bead is the intermediate representation — a well-specified unit of work. Beads with 3+ acceptance criteria, explicit file paths, and unambiguous language have dramatically higher first-attempt success rates. Investing in bead specification has ~3x more impact than investing in code generation speed.

### Layer 2: HERO Loop

Four-phase execution cycle: Hunt (find ready work) → Embark (claim + build context) → Ratchet (validate + lock progress) → Odyssey (handle failures). See `execution.md`.

### Layer 3: Validation Pipeline

Two-stage deterministic validation. No LLM can override a failing test. Stage 1: build + vet + test. Stage 2: merge gate. See `validation.md`.

### Layer 4: Knowledge Flywheel

Learnings feed back into Layer 0 as constraint tests. The output is tests and CI checks that enforce themselves, not documentation. See `knowledge.md`.

## What Olympus Absorbs

### From Gas Town (Tier 1: Absorbed)

| Gas Town Concept | Olympus v4 | What Changed |
|-----------------|------------|--------------|
| Mayor | Zeus + daemon | Split: Zeus handles phases, daemon is mechanical poll loop |
| Polecats | Claude sessions | Persistent workers → ephemeral fresh-context spawns |
| Convoys | Quests + waves | Implicit coordination → explicit dependency-based waves |
| Agent Mail | Beads state | Messages → persistent shared state (stigmergy) |
| Daemon (intelligent) | Daemon (mechanical) | Decision-maker → poll loop that spawns phases. No LLM in daemon. |
| Rig registry | `~/.olympus/config.yaml` | Same data, migrated via `ol init` |
| Crew directories | Same convention | Inherited directly |

**What Gas Town had that Olympus kills:** Supervision tree, multi-user model, identity system, federation, daemon heartbeat/patrols (never worked).

**Philosophy shift:** "Orchestrate workers" → "Control the context window". "Retry in place" → "Spawn fresh with lessons learned". "Daemon decides" → "Daemon is mechanical, decisions in phases".

### External CLIs (Tier 2: Composed)

| Tool | Commands | Integration | Why Not Absorb |
|------|----------|-------------|----------------|
| `ao` | 54 | `exec.Command("ao", ...)` in skills/hooks | Complex ML loops, own release cycle |
| `bd` | 110 | `internal/beads/client.go` wrapping bd CLI | Mature git-native storage, own release cycle |

**Boundary:** Olympus owns quest lifecycle and phase execution. ao owns knowledge lifecycle. bd owns issue state.

### Complementary (Tier 3: Standalone)

ntm (tmux), dcg (safety hooks), bv (beads viewer), cass (session search). No integration needed.

### Killed (Tier 4: Replaced)

gt CLI, claude_code_agent_farm, Gas Town operator, Agent Mail daemon. All replaced by Olympus.

## Runtime Model

```
┌─────────────────────────────────────┐
│           ol binary                  │
│  ┌──────────┐  ┌──────────────────┐ │
│  │ Daemon   │  │ Zeus + HERO      │ │
│  │ (Apollo  │  │ (phases, waves,  │ │
│  │  poll)   │  │  validation)     │ │
│  └────┬─────┘  └────────┬─────────┘ │
│       │                  │           │
│  ┌────┴──────────────────┴─────────┐ │
│  │     Subprocess Orchestration     │ │
│  │  exec("ol zeus step --quest")   │ │
│  │  exec("claude -p ...")          │ │
│  └──────────────┬──────────────────┘ │
│                 │                     │
│  ┌──────────────┴──────────────────┐ │
│  │      CLI Composition            │ │
│  │  exec("bd", ...) / exec("ao")  │ │
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

- **No agent processes.** All work happens through CLI commands or Claude sessions spawned as subprocesses.
- **No supervision tree.** If a subprocess fails, the HERO loop's Odyssey phase decides: retry with feedback or escalate.
- **No Agent Mail.** Coordination through beads state (stigmergy).
- **No in-memory-only state.** Crash recovery is file-backed: Zeus checkpoints + beads state + quest events + run ledger.

## Configuration

Via `.ol/config.yaml` or environment variables (prefix `OL_`):

| Key | Default | Purpose |
|-----|---------|---------|
| `max_demigods` | 4 | Maximum parallel agents per wave |
| `poll_interval` | 30s | Daemon poll interval |
| `validation_timeout` | 10m | Per-step validation timeout |
| `max_retries` | 3 | Retry budget per bead |
| `memrl_mode` | off | MemRL policy mode (`off\|observe\|enforce`); cannot override deterministic gate failures |

## Data Model

| Store | Format | Purpose |
|-------|--------|---------|
| Beads (`.beads/`) | JSONL via bd CLI | Work state: issues, deps, claims |
| Run Ledger (`.ol/runs/`) | JSON per attempt | Execution provenance |
| Quest Events (`.ol/quests/`) | JSONL per quest | Audit log |
| Context Bundles (`.ol/prompts/`) | JSON per attempt | Reproducible context |
| Daemon Events (`~/.olympus/state/`) | JSONL | Daemon audit trail |
| Config (`~/.olympus/config.yaml`) | YAML | Rig registry, daemon settings |

## Import Boundaries

```
cmd/ol/ → internal/* → pkg/*
```

No circular imports. Internal packages never import cmd/ol/. Constraint tests never import business logic.

## Primitive Registration Layers

The primitive package (`internal/primitive/`) is leaf-only — it defines interfaces, the Registry, the compatibility matrix, and stub implementations but cannot import other internal packages. This creates two distinct registration layers:

1. **Go library layer** (`internal/primitive/stubs.go`): Stub implementations that return `errNotWired`. Used when creating a Registry directly in Go code (e.g., tests, library consumers). Every primitive has a stub so `NewRegistry()` always returns a complete set.

2. **CLI adapter layer** (`cmd/ol/primitive_cmd.go`): Adapter implementations wrapping real internal packages (context, zeus, hero, knowledge, beads, validation). The CLI's `buildRegistry()` replaces stubs with adapters that delegate to the actual subsystems.

The separation exists because the primitive package must remain a leaf node in the import graph. Adapters live in `cmd/ol/` where importing everything is allowed.

**Daemon integration note:** Phase 1 daemon integration should export an equivalent of `buildRegistry()` from a non-leaf adapter package (not `cmd/ol/`) so the daemon can construct a fully-wired registry without duplicating adapter code.

## Recipe Advisory Sequences

Recipes are suggested primitive sequences, NOT enforced CanFollow chains. The 4 built-in recipes (`rpi`, `spike`, `tech-debt`, `full-feature`) express intent — the agent fills gaps with intermediate primitives as needed.

Six CanFollow gaps exist across the built-in recipes:

| Recipe | Gap | Missing Transition |
|--------|-----|--------------------|
| rpi | JUDGE → SPAWN | JUDGE cannot directly precede SPAWN |
| rpi | JUDGE → HARVEST | JUDGE not in HARVEST's predecessor set |
| spike | EXPLORE → JUDGE | EXPLORE not in JUDGE's predecessor set |
| tech-debt | MEASURE → EXECUTE | MEASURE not in EXECUTE's predecessor set |
| full-feature | JUDGE → JUDGE | JUDGE not in its own predecessor set |
| full-feature | JUDGE → SPAWN | Same as rpi gap |

`Composer.Validate()` checks CanFollow for explicitly composed sequences, but recipes bypass this — they are suggestions, not validated transition chains. This is intentional: recipes express high-level workflow intent while the agent determines the concrete primitive sequence that satisfies both the recipe's goals and the compatibility matrix.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success / more phases remain |
| 1 | Error |
| 2 | Human needed (approval gate, budget exceeded) |
| 42 | Quest complete (all phases done) |

Exit code 42 is an architectural property — every command that advances a state machine must support it.
