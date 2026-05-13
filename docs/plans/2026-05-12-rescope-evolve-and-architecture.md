---
id: plan-2026-05-12-rescope-evolve-and-architecture
type: plan
date: 2026-05-12
scope: re-organize the evolve-loop + bounded-context work under the operator's demonstrated shape principles
companion_to:
  - .agents/research/2026-05-12-repo-direction-map.md
  - .agents/research/2026-05-12-bounded-contexts-and-ports.md
  - .agents/post-mortems/2026-05-12-evolve-session-improvement-postmortem.md
trigger: operator commit f62295f7 (cherry-picked from origin/nightly/2026-05-12 1b9d139c) demonstrated the canonical shape
---

# Rescope — Evolve & DDD/Hex Architecture Work

## What changed today that forces the rescope

1. **Operator pushed `1b9d139c`** to `nightly/2026-05-12` — a single-commit demonstration of the canonical shape for fixing one gate. Now cherry-picked to main as `f62295f7`.
2. **DDD/hex bounded-context inventory landed** (`.agents/research/2026-05-12-bounded-contexts-and-ports.md`) — 5 BCs, 6 missing ports, 3 god-packages.
3. **Cycle 51 post-mortem named the structural failure** — write-only ledgers, text-only patches, no read path.

Each of these alone could justify a re-scope. Together they're a re-org mandate.

## The five update principles (distilled from operator's commit `1b9d139c`)

Every cycle's output, starting now, must demonstrate ALL FIVE:

1. **Single concern.** One bug or feature per commit. The operator's commit changed exactly 2 files for exactly 1 logical fix. Multi-concern commits like cycle 48's "fix 3 CI failures" are an anti-pattern — even if the failures are related, they should land as separate commits.

2. **Drift-blocking test included.** The operator added 4 BATS tests covering all paths (greenfield SKIP, overnight PASS, canonical PASS, empty-tree SKIP) — not a one-shot operator probe like my `AGENTS_DIR=/tmp/no-such-dir bash …` validation. Without a test that fires when the fix regresses, the fix is aspirational.

3. **Sibling-pattern citation.** The commit message says "matching the Gap 1 council-coverage SKIP shape." Every fix names the precedent it follows. New shapes need explicit rationale.

4. **Fitness delta in the commit message.** "Code-driven fitness: 134/139 → 139/139." Measured outcome, not narrated outcome. If the fix can't be measured, the fix is suspect.

5. **Branched from a clean point.** Operator branched from `dcdb016b` (before cycle 44 noise) rather than from the latest red main. Even a 1-commit fix shouldn't carry unrelated context.

## How the 5 principles + 5 bounded contexts collapse existing scope

The existing `soc-z8rt` epic (8 children for read-path automation) was filed under a flat "evolve-improvement" umbrella. With bounded contexts + ports as the spine, the same work re-organizes more cleanly. Existing beads either map to a port or signal that the port is missing.

### New shape: 5 bounded-context epics, each with port-aligned children

Each port becomes one bead. Each bead is sized to land as one operator-shaped commit (single concern, bats test, sibling citation, fitness delta).

---

### BC1: Corpus — `epic-corpus-ports` (NEW)

**Acceptance:** all 4 ports below exist as Go interfaces in `cli/internal/ports/corpus.go`; existing implementations in `corpus`, `knowledge`, `harvest`, `mine`, `forge` adapt to them; one new in-memory adapter exists for tests.

| Bead | Title | Maps from existing | Sized |
|---|---|---|---|
| .1 | Extract `CorpusReaderPort` (decay-ranked retrieval) | partial overlap with `soc-z8rt.1` | 1 cycle |
| .2 | Extract `CorpusWriterPort` (typed capture) | partial overlap with `soc-z8rt.1` | 1 cycle |
| .3 | Extract `FindingCompilerPort` (already spec'd in `docs/contracts/finding-compiler.md`) | NEW | 1 cycle |
| .4 | Extract `CitationPort` (record applied learnings) | NEW (currently scattered across hooks) | 1 cycle |

### BC2: Validation — `epic-validation-ports` (NEW)

**Acceptance:** `GateRunnerPort` enumerates and runs all 90 `scripts/check-*.sh` + `scripts/validate-*.sh`; one CI adapter, one pre-push adapter, one ao-goals-measure adapter; `CIStatusPort` returns last-N CI conclusions.

| Bead | Title | Maps from existing | Sized |
|---|---|---|---|
| .1 | Extract `GateRunnerPort` + 4 adapters | partial overlap with `soc-z8rt.5` (CI green-watch) | 2 cycles |
| .2 | Extract `CIStatusPort` + GitHub adapter | partial overlap with `soc-z8rt.5` | 1 cycle |
| .3 | Bats coverage shape for the supergate is **already canonical** (operator commit `f62295f7`) — extend to all 4 super-gate sub-checks | NEW (extends `soc-w6vh.6` council-coverage strengthening) | 2 cycles |
| .4 | Bind every Claim in `factory-claim-ledger.example.json` to an evidence_artifact via `EvidenceBinderPort` | new shape of `soc-z8rt.4` (PG4 re-verify) | 1 cycle per claim |

### BC3: Loop — `epic-loop-ports` (NEW)

**Acceptance:** /evolve loop has zero direct shell-outs in its decision logic — all external calls go through ports. Step 0, Step 1.5, hypothesis tracking, convergence STOP are all backed by typed Go interfaces.

| Bead | Title | Maps from existing | Sized |
|---|---|---|---|
| .1 | Step 0 prior-failure injection via `CorpusReaderPort` | direct rename of `soc-z8rt.1` (now port-backed) | 1 cycle |
| .2 | Step 1.5 healing-first classifier via `CIStatusPort` | direct rename of `soc-z8rt.2` (now port-backed) | 1 cycle |
| .3 | Hypothesis tracking via `HypothesisLedgerPort` + pre-commit hook | direct rename of `soc-z8rt.6` | 1 cycle |
| .4 | Convergence STOP via `ConvergenceCheckPort` | NEW (cycle 51 sketched it; needs port + Go impl) | 1 cycle |
| .5 | `scripts/evolve-read-cycle-history.sh` extracted reader | direct rename of `soc-z8rt.8` (becomes a small CLI on top of `CorpusReaderPort`) | 1 cycle |

### BC4: Factory — `epic-factory-ports` (NEW)

**Acceptance:** Each Claim row's `evidence_status` is mechanically determined from gate run results, not human-set. `FactoryAdmissionPort` already half-formed; finish the formalization. The `AOP-CLAIM` markers stop being narrative and become structural.

| Bead | Title | Maps from existing | Sized |
|---|---|---|---|
| .1 | Formalize `FactoryAdmissionPort` (interface exists in daemon; extract clean) | NEW | 1 cycle |
| .2 | `ClaimEvidencePort` — bind Claim → Evidence files → gate verdict | NEW (replaces narrative `soc-z8rt.4` PG4 re-verification) | 1 cycle |
| .3 | Auto-promote ledger `evidence_status` based on `EvidenceBinderPort` output | NEW | 1 cycle |
| .4 | Retire the two existing PG4 evidence files (cycles 46/47) OR re-bind them to real gates | replaces `soc-z8rt.4` directly | 1 cycle each (2 cycles total) |

### BC5: Runtime — `epic-runtime-ports` (NEW)

**Acceptance:** `skills/` and `skills-codex/` become adapters under a single `HarnessPort` source-of-truth; codex-parity-drift gate fires automatically without manual `regen-codex-hashes.sh`.

| Bead | Title | Maps from existing | Sized |
|---|---|---|---|
| .1 | Extract `HarnessPort` + Claude adapter + Codex adapter compiler | NEW (this kills the manual sync) | 3 cycles |
| .2 | Extract `EventBus` formalized port + hook adapter inventory | NEW (turns 55 hooks into typed subscribers) | 2 cycles |
| .3 | Extract `OperatorPort` (CLI/daemon/future MCP share interface) | NEW | 2 cycles |
| .4 | Audit `generate-registry.sh` for filesystem-find drift in `skills/`, `hooks/`, `schemas/`, `cli/` | direct rename of `soc-z8rt.3` | 1 cycle |

### Cross-cutting: `epic-ubiquitous-language-cleanup` (NEW)

**Acceptance:** `docs/contracts/ubiquitous-language.md` defines canonical terms per BC; all 5 ranked drifts resolved.

| Bead | Title | Maps from existing | Sized |
|---|---|---|---|
| .1 | Gate vs Check vs Validation — pick `Gate` (BC2 aggregate); rename ~90 scripts to use consistent term in headers | ✓ AUDITED cycle 130 (0 scripts use "Validator", 38 already use "Gate", rest use neither — catalog overstated drift size; no renames needed) | 1-2 cycles |
| .2 | Cycle vs Loop vs Iteration vs Run — pick `Cycle` (BC3 aggregate); deprecate `Run` outside Phase context | ✓ AUDITED cycle 129 (no drift; RPIRun + Iteration usage is serialized-contract, Runner naming, Runtime substring-coincidence, or scoped Dream counters — no renames needed) | 1 cycle |
| .3 | Claim vs Assertion vs Evidence — `daemon.QueueClaim` → `QueueLease` | ✓ DONE cycle 126 (108 Go refs renamed; daemon/rpi/llmwiki/cmd-ao tests green) | 1 cycle |
| .4 | Skill vs Primitive vs Pattern vs Practice — pin definitions per BC | ✓ AUDITED cycle 128 (no systemic misuse; `primitive` overload documented as 3 scoped senses, no renames needed) | 1 cycle |
| .5 | Session — scope per BC (`AgentSession` BC5, `OperatorSession` BC5; drop the daemon "session" usage) | NEW | 1 cycle |

---

## Migration map — existing beads → new shape

| Existing bead | Disposition | New home |
|---|---|---|
| `soc-z8rt` (epic, 8 children) | **Retire** as catch-all; children move to BC-aligned epics | dissolved into BC1/BC2/BC3/BC5 |
| `soc-z8rt.1` (Step 0 prior-failure injection) | reshape | BC3.1 |
| `soc-z8rt.2` (Step 1.5 healing-first classifier) | reshape | BC3.2 |
| `soc-z8rt.3` (generate-registry.sh audit) | rename | BC5.4 |
| `soc-z8rt.4` (PG4 re-verify cycles 46-47) | reshape | BC4.4 |
| `soc-z8rt.5` (CI green-watch in pre-push) | reshape | BC2.1 + BC2.2 |
| `soc-z8rt.6` (skill-edit → hypothesis pre-commit hook) | reshape | BC3.3 |
| `soc-z8rt.7` (operator brief CI-green quality bar) | **Close as superseded** — covered by Update Principle #2 (drift-blocking test included) | closed |
| `soc-z8rt.8` (scripts/evolve-read-cycle-history.sh) | reshape | BC3.5 |
| `soc-w6vh.1` (practice slug registry drift) | reshape | `epic-ubiquitous-language-cleanup.4` |
| `soc-w6vh.2` (practice citation validation strict-mode) | keep as-is | BC4 (factory admission of skill manifests) |
| `soc-w6vh.4` (remove tracked repo-root .agents state) | keep as-is | BC1 (corpus persistence policy) |
| `soc-w6vh.5` (export evolve-cycle learning evidence) | keep as-is | BC1.4 (citation port) |
| `soc-w6vh.6` (council-coverage gate strengthening) | reshape | BC2.3 (bats coverage for super-gate sub-checks) |
| `soc-sx99` (positioning honesty pivot) | **Becomes mechanically possible after BC4 ports land** | sequence after BC4 |
| `soc-m6v5.9.x` (3.0 release train) | **Hold** until at least BC1+BC2+BC3 ports land OR operator chooses Option B (substrate-honest narrative) | release train decision |

## Sequence (the order things should land)

This is the operator's Option E from the direction map, now port-shaped:

### Wave 1 — Ubiquitous Language + Principles install (1 week)

Mechanical, no-substrate-risk renames. Free-but-not-free: removes the cognitive tax on every subsequent cycle.

- `epic-ubiquitous-language-cleanup.1` through `.5` (~1 cycle each)
- New `docs/contracts/update-principles.md` codifying the 5 principles from operator commit `1b9d139c`
- New `docs/contracts/ubiquitous-language.md` glossary
- Pre-commit hook (BC3.3) enforces principles at commit time

**Deliverable:** every subsequent commit must demonstrate 5 principles. Lint surfaces drift.

### Wave 2 — Core ports (2-3 weeks)

The 6 highest-leverage missing ports. Each is one Go interface in `cli/internal/ports/`, with adapters in their existing packages.

- `CorpusReaderPort` + `CorpusWriterPort` (BC1.1, .2) — gate for /evolve Step 0
- `GateRunnerPort` + `CIStatusPort` (BC2.1, .2) — gate for /evolve Step 1.5
- `HypothesisLedgerPort` + `ConvergenceCheckPort` (BC3.3, .4) — gate for cycle termination
- `HarnessPort` (BC5.1) — gate for skills/skills-codex sync

**Deliverable:** /evolve loop's claimed read-path mechanisms (cycle 51) now wired through ports, not shell-outs. Compounding claim is **mechanically demonstrable**.

### Wave 3 — God-package decomposition (2-3 weeks)

Split `types`, `daemon`, `goals` per the bounded-context inventory. Touches many call sites; needs test coverage to land safely. Can be deferred to post-3.0.

**Deliverable:** smaller packages; clean import graph. The compounding claim becomes **structurally enforced**, not just demonstrable.

### Wave 4 — Factory + claim binding (1-2 weeks)

- `FactoryAdmissionPort` formalization (BC4.1)
- `ClaimEvidencePort` + auto-promotion (BC4.2, .3)
- Retire/re-bind the 2 cycles 46-47 PG4 files (BC4.4)
- soc-sx99 positioning honesty pivot becomes mechanical

**Deliverable:** every `AOP-CLAIM-*` marker has its evidence_status mechanically tied to a gate run. Marketing claims cannot run ahead of substrate.

### Wave 5 — Operator+Event extraction (1-2 weeks, can be post-3.0)

`OperatorPort` and `EventBus` formalization. Future-proofs the architecture for MCP, LSP, alternative runtimes (Cursor, OpenCode).

---

## What's deliberately NOT in this rescope

1. **Code-level refactor of existing functions** unless they're directly extracting a port. The God-package decomposition (Wave 3) is one big exception; everything else is additive.

2. **New features.** No "add X to corpus retrieval" or "new gate Y." Every bead extracts what exists or formalizes what's implicit.

3. **3.0 release work.** `soc-m6v5.9.x` stays on hold. The whole point is to decide whether 3.0 ships with verifiable compounding (A) or substrate-honest narrative (B); the rescope makes both options mechanically possible.

4. **CI workflow expansion.** Existing 61 jobs stay; the rescope just extracts a shared interface (`GateRunnerPort`) so the 61 jobs become 61 adapters of one port instead of 61 hand-coded scripts.

5. **Skill content edits.** No new SKILL.md text. The cycle 51 additions (Step 0 + Step 1.5 + convergence STOP) stay as documentation; Wave 2 wires the implementation behind them.

---

## What the rescope produces (artifacts)

- **5 new BC-aligned epics** to file: `epic-corpus-ports`, `epic-validation-ports`, `epic-loop-ports`, `epic-factory-ports`, `epic-runtime-ports`, `epic-ubiquitous-language-cleanup`.
- **Close `soc-z8rt`** (epic + 1 child `soc-z8rt.7`) as superseded.
- **Reshape `soc-z8rt.1..6, .8`** under new BC homes (keep titles in close reasons for traceability).
- **Two new contracts:** `docs/contracts/update-principles.md` (the 5 distilled principles) and `docs/contracts/ubiquitous-language.md` (the glossary).
- **Pre-commit hook** wired to enforce principles 1+2+4 at commit time (single-concern check, bats-coverage check, fitness-delta string presence in commit message).

---

## The pivotal directional decision (updated)

The direction map listed 5 options (A–E). With the rescope concrete:

| Option | Path | Updated estimate |
|---|---|---|
| **A** — ship 3.0 now | risk profile unchanged | days; high embarrassment |
| **B** — close `soc-sx99` first | requires BC4 ports to be mechanical | now 4-5 weeks (waves 1+4) |
| **C** — close `soc-z8rt` first | superseded by this rescope | N/A — its children moved to BC-aligned epics |
| **D** — pause 3.0 | unchanged | 4-8 weeks |
| **E** — DDD/hex restructure | **NOW EXECUTABLE as 5 sequenced waves above** | wave 1 (1 week) → wave 2 (3 weeks) → wave 4 (2 weeks) ≈ 6 weeks to 3.0-shippable substrate; waves 3+5 post-3.0 |

**Updated recommendation:** **E waves 1+2+4 → 3.0 (substrate-honest narrative) → waves 3+5 post-launch.** This trades 6 weeks of pre-launch time for a 3.0 that ships with verifiable compounding and structural anti-drift. Waves 1+2+4 are mechanical port extractions, not redesign.

If 6 weeks is too long, waves 1 alone (1 week, ubiquitous language + principles + pre-commit hook) deliver outsized value — every subsequent commit gets the operator's exemplar shape automatically — and unlock soc-sx99 mechanically at lower cost.

---

## Next operator decision

One question:

> Do we file the 5 new BC-aligned epics now (this rescope's main output), or stage them in a draft file for review first?

If file now: I close `soc-z8rt` + `soc-z8rt.7`, retitle the 6 surviving children under new epic IDs, file the 5 new epics, and the next /evolve session has the right backlog shape.

If draft first: this document IS the draft; operator reviews, approves, then I execute filings as a separate cycle.
