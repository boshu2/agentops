# AgentOps — Technical Architecture Summary (Codebase Archaeology)

> Produced by the codebase-archaeology skill (swarm audit, 2026-06-11).
> Method: documentation-first orientation → entry points → key types → data flow →
> configuration → integration → tests, synthesized from Explore-agent fan-out
> (evidence gathered by a prior worker; this report is the synthesis pass).

---

## Executive Summary

**AgentOps** is a context-compiler and agent-operations substrate: a Go CLI (`ao`)
plus a 167-skill corpus that automates agent bookkeeping (attempts, decisions,
citations, verdicts, handoffs, learnings) and encodes a DevSecOps-style CDLC —
the "knowledge flywheel" — so that what agents learn compounds across sessions
and runtimes. It implements **hexagonal architecture (ports & adapters)** over
**five bounded contexts**, with a decentralized gate registry, a decay-ranked
knowledge retrieval engine, and a promotion ratchet that moves knowledge from
provisional capture to canonical doctrine.

**Key statistics:**
- ~382K Go LOC, **62% of it tests** (~2.3:1 test:code in the CLI)
- `cli/cmd/ao/`: one flat `main` package, 620 `.go` files (271 non-test), ~90 top-level cobra commands
- `cli/internal/`: ~70 bounded-context packages
- `skills/`: 167 skill directories (103 with `references/`), tiered + hexagonal-role-tagged
- `scripts/`: ~253 shell scripts (validate-\*, check-\*, generate-\*, evolve-\*, nightly-\*)
- `schemas/`: ~35 JSON schemas (verdict.v1, execution-packet, provenance.v1, handoff.v1, …)
- CI: 67 jobs consolidated into ~10 purpose-grouped jobs in `.github/workflows/validate.yml`
- State: **mid-3.1 "lean image" reduction** (hookless, push-to-main, ao↔MTO boundary work)

**The one-paragraph mental model:** everything is a loop around a corpus. Work
enters as beads/packets, executes through fractal RPI loops (research → plan →
implement), is gated by a check registry (local pre-push + CI backstop), and the
residue — learnings, citations, verdicts — feeds back into `.agents/` where
decay-ranked retrieval (`ao lookup`/`ao inject`) re-injects it into the next
session. The skills are the instruction layer; the CLI is the deterministic
layer ("CLI for deterministic, skills are instructions").

---

## Doctrine Stack (read in this order)

| Altitude | Document | Role |
|---|---|---|
| North star | `docs/3.0.md` | What AgentOps 3.0 is: hookless-first CDLC, SDLC↔CDLC loop, four-practice waist |
| Spine | `docs/architecture/operating-loop.md` | 7-move agent doctrine; fractal rpi/evolve/crank loops; three ratchet rules. **Primary navigation.** |
| Contracts | `docs/contracts/bounded-contexts.yaml` (SSOT), `docs/contracts/context-map.md` (generated) | BC1 Corpus → BC2 Validation → BC3 Loop → BC4 Factory → BC5 Runtime |
| Workflow | `CLAUDE.md` / `AGENTS-{WORKFLOW,CI,CODEX,RUNTIME}.md` | Push-to-main model (ag-qidx), worktree-mandatory, bead-cited changes |

---

## Entry Points

- `cli/cmd/ao/` — the single binary entry. **One-command-per-file with `init()`
  registration** into a flat package; ~90 top-level cobra commands grouped as:
  start, core, workflow, config, comms, knowledge.
- Global flags: `--dry-run`, `--verbose`, `-o/--output`, `--json`, `--config`.
- Skills (`skills/*/SKILL.md`) are the second entry surface: slash-command
  instructions that *call* `ao` subcommands rather than re-implement logic.
- Flagship skill entry points: `/rpi` (orchestrates discovery → crank →
  validate under a strict delegation contract), `/evolve` (autonomous loop to
  dormancy), `/forge` (transcript mining → promotion ratchet), `/council`
  (multi-judge verdicts), `/validate` (8-mode budget).

**Fossils (confirmed 2026-06-11):** root `cmd/`, root `internal/`, and
`manifests/` contain **zero Go files** — empty directory skeletons left from the
deleted daemon era (ADR-0009). They are not entry points; treat them as
candidates for removal in the 3.1 teardown.

---

## Key Types (the five everything revolves around)

| Type | Location | Purpose |
|---|---|---|
| `Candidate` | `cli/internal/types/types.go:107-219` | The knowledge unit: maturity state, MemRL utility, CASS confidence, supersession chain (max depth 3), gold/silver/bronze tiers |
| `FitnessVector` | `cli/internal/corpus/fitness.go:38-47` | 7 corpus-health metrics — the flywheel's health gauge |
| `GateVerdict` | `cli/internal/ports/gate_runner.go:16-19` | PASS/WARN/FAIL/SKIP/UNKNOWN + reason + 4KB logTail — the unit of validation truth |
| `NextWorkEntry` | `cli/internal/rpi/types.go:49-72` | The unit of queued work feeding the loop |
| `Chain` / `ChainEntry` | `cli/internal/ratchet/ratchet.go:130-181` | 7-step research→post-mortem pipeline with step locking and cycle tracking — the ratchet that prevents phase-skipping |

`cli/internal/` layers (~70 packages):
- **Knowledge core:** corpus, knowledge, search, types
- **Validation & gates:** gates, ports, ratchet, quality, verdictledger
- **RPI execution:** rpi, lifecycle, orchestration, liveness, worktree
- **Goals & measurement:** goals, goalstrace, goalsfitness, eval, evalsubstrate
- **Skills:** skills, skillshealth, skillsresolve, plugin
- **Storage & config:** storage, paths, config, lockfile
- **Analysis & refining:** refinery, harvest, mine, provenance, drrebuild, drwitness
- **Bridges & ports:** ports, adapters, bridge, canon, compiler

---

## Data Flow

### Flow 1 — knowledge retrieval (`ao lookup --query`)

```
config.Load
  → collectLearnings (walk .agents/learnings/*.md)
  → inverted-index search (terms → doc-set, score by match count)
  → decay ranking: utility * (1 - e^(-decayCount/λ))
  → global_weight 0.8 downranks cross-repo knowledge
  → citations appended to .agents/ao/citations.jsonl   ← MemRL reward feedback
```

The citation append is the load-bearing step: retrieval *is* the reward signal.
MemRL Q-value update `u ← (1-α)u + αr` (init 0.5); CASS confidence decays;
maturity transitions provisional → candidate → established → anti-pattern at
thresholds 0.55/0.3/0.2; tier progression observation → learning (2+ cites) →
pattern (3+ sessions) → skill (tested) → core (10+ uses).

### Flow 2 — validation (`ao gate check --fast`)

```
decentralized check registry (init() per check)
  → changed-file routing via Match[] globs (predicate-parity guard, ag-qidx GA7)
  → dual dispatch: native Go CheckFunc XOR subprocess scripts/check-*.sh (GateRunnerPort)
  → FailFast locally vs collect-all in CI
  → Fast/Full tiers → GateVerdict
```

This mirrors `scripts/pre-push-gate.sh` (39 numbered checks, scope modes
auto/staged/worktree/head, `--fast` ~10-20s cached, verdict log for claim
verification). Native-Go parity is **in progress, not done** — `ao gate check`
is the default wall, with the legacy shell gate retained as the
`AGENTOPS_GATE_BASH=1` fallback until the sunset criterion is met.

### Flow 3 — the flywheel (macro loop)

```
bead/packet → /rpi (research → plan → implement) → gate → push-to-main
   → residue captured in .agents/ (learnings, council, proof, ledger)
   → /forge mines transcripts → promotion ratchet (pending → learnings → canon)
   → ao inject / ao lookup re-injects into the next session
```

---

## Configuration

Precedence: **flags > `AGENTOPS_*`/`AO_*` env > `.agentops/config.yaml` >
`~/.agentops/config.yaml` > defaults.**

Path resolution (resolved once via `paths.Resolve()`):
`AO_HOME` > `CLAUDE_PLUGIN_DATA/.agents` > git-toplevel/`.agents` > cwd/`.agents`,
with per-subdir `AO_*` overrides (`AO_LEARNINGS_DIR`, …).

Config sections: output, paths (`global_weight` 0.8), rpi
(worktree_mode/runtime_mode/commands), flywheel (`auto_promote_threshold` 24h),
models, dream, compile.

---

## Skills + Validation Layer (the second half of the system)

- **Skill contract:** SKILL.md frontmatter declares `name`, `hexagonal_role`,
  `consumes`, `produces`, `context_rel`, `metadata{tier, dependencies,
  stability}`, `output_contract`, `skill_api_version`. The context map
  (`docs/contracts/context-map.md`) is **generated from this frontmatter** and
  drift-gated in CI (`validate-context-map-drift`) — the inventory is never
  hand-edited.
- **Tiers:** execution 72, judgment 27, orchestration 13, knowledge 12,
  library 11, meta 10, session 6, product 5, background 4, cross-vendor 3,
  contribute 3. **Roles:** supporting 99, domain 31, driving-adapter 25,
  driven-adapter 8, generic 3.
- **Cross-runtime parity:** `skills-codex/` carries generated manifests +
  `prompt.md` per skill; `skills-codex-overrides/` holds 11 bespoke; manifest
  hashes drift-checked by `scripts/validate-codex-parity-drift.sh`.
- **CI (`validate.yml`):** ~10 purpose-grouped jobs (collapsed from 67, ag-877):
  changes (path filter), go-gate-shadow (REQUIRED — `ao gate check --full`),
  skill-gates (REQUIRED), correctness, lint, security (gosec/gitleaks/trivy/
  hadolint), skills-integrity, contracts-sync, codex-parity, doctrine-proof,
  eval, skill-eval (per-skill grading, blocked at >1 FAIL), process-hygiene,
  summary aggregator. Tiers T0 (≤30s) / T1 (≤5min) / T2 (≤15min) — **all
  required**; I0 is informational-only.
- **`.agents/` corpus:** ~20 capture subdirs (research, design, proof, council,
  plans, goals, learnings, patterns, forge, rpi, swarm, …), `ledger/`,
  `agent-constitution.md`. Write surfaces are catalogued in
  `docs/contracts/agents-write-surfaces.md` and gate-enforced (check 3d).

---

## Architectural Patterns (what the codebase teaches)

1. **Hexagonal ports & adapters, compile-checked.** `cli/internal/ports` +
   `cli/internal/adapters`; 20 ports scaffolded (16 with production adapters,
   4 in-memory only), each with compile-time `var _ XPort` assertions and
   in-memory test doubles.
2. **Decentralized registry over central switch.** Both cobra commands and gate
   checks self-register via `init()` — the flat 620-file `cli/cmd/ao` package
   is the cost of this pattern at scale.
3. **Generated-or-gated inventory, never hand-edited.** Context map, codex
   manifests, COMMANDS.md, embedded resources (Makefile `sync-hooks` →
   `cli/embedded` Go embed) — drift is a CI failure, not a doc chore.
4. **Strict delegation contract.** `/rpi` may not collapse phases; council
   enforces author≠validator; the ratchet (`Chain`) locks step order.
5. **Context Density Rule.** The six-field context packet (Intent, Boundary,
   Evidence, Decision, Constraint, Next Action) is the narrow waist between
   skills, beads, and handoffs.
6. **Tests as the dominant artifact.** 622 `*_test.go` files, ~2.3:1
   test:code, heaviest suites in ratchet, gates, search, config (70KB), goals;
   a test-count-regression ratchet prevents abandonment. `make build` /
   `make test` (`-shuffle=on`) / `make lint`.

---

## External Dependencies / Integration Points

- **cobra** — CLI framework (one command per file, init() registration)
- **Filesystem-first storage** — `.agents/` markdown + JSONL (citations.jsonl,
  ledger.jsonl provenance, next-work.jsonl); no database in the hot path
- **Subprocess gate scripts** — `scripts/check-*.sh` via GateRunnerPort during
  the native-Go migration
- **bd/beads (Dolt)** — issue tracking (data-plane; control-plane work tracked
  in `~/dev/control-plane` with `br`)
- **NTM + MCP Agent Mail** — the live orchestration substrate (tmux swarms,
  locks/messaging). Gas City is reference-only; the CLI gc-bridge was removed
  (soc-2rtm0, ag-hfc).

---

## Hazards & Open Seams (for the next explorer)

- **Empty fossils:** root `cmd/`, root `internal/`, `manifests/` — 0 Go files,
  daemon-era skeletons. Delete candidates.
- **Legacy RPI lane:** `rpi_loop_supervisor.go`, `rpi_c2_events.go`,
  `rpi_phased_tmux.go`, `rpi_parallel.go` are live, tested, load-bearing legacy
  — extend tests when touched, add no new surface area, do not flat-delete
  (symbol fan-out across 13+ files; removal needs caller migration, soc-1gbpz).
- **Dual gate reality:** `pre-push-gate.sh` (2210 LOC, 39 checks) is still the
  default; `ao gate check` parity is partial. Don't report the shell gate as
  retired.
- **Flat 620-file main package:** command discovery is grep-shaped, not
  package-shaped; expect name collisions and shared-helper sprawl when adding
  commands.
- **Mid-reduction churn (v3.1.0):** the repo runs hot; doctrine docs and
  CLAUDE.md carry explicit supersession notices — trust executable >
  contracts > narrative (the repo's own precedence rule).

---

## Checklist Coverage

- [x] Documentation first (CLAUDE.md, docs/3.0.md, operating-loop.md)
- [x] Orientation: directory structure, ~382K LOC, 5 BCs
- [x] Entry points: cobra registry, skills surface, fossils flagged
- [x] Key types: Candidate, FitnessVector, GateVerdict, NextWorkEntry, Chain
- [x] Data flow: lookup, gate check, flywheel macro-loop
- [x] Config: precedence chain + path resolution
- [x] Integration: filesystem corpus, gate scripts, bd, NTM/Agent Mail
- [x] Tests: 62% of LOC, shape and ratchet documented
- [x] Reusable summary: this document
