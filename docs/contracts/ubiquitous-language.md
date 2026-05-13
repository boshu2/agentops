# Ubiquitous Language Contract

> **Status:** Draft
> **Decision:** Each domain concept has one canonical name per bounded context. New code, docs, and skills MUST use the canonical name; renames of legacy occurrences land as single-concern commits per the schedule below.
> **Consumers:** every contributor; `scripts/check-ubiquitous-language.sh` (future); 5 wave-1 rename PRs under epic soc-5yuy.
> **Source:** `.agents/research/2026-05-12-bounded-contexts-and-ports.md` § "5 ubiquitous-language drifts".

## Why this contract exists

The bounded-context inventory (cycle 51 research) surfaced 5 ranked drifts where the same domain concept has 3–5 different names across the codebase. Naming drift is a tax on every subsequent cycle: readers can't tell whether "Cycle" in `lifecycle/` refers to the same thing as "Iteration" in `overnight/` or "Run" in `rpi/`. This contract pins the canonical names per bounded context (BC1 Corpus, BC2 Validation, BC3 Loop, BC4 Factory, BC5 Runtime — see the source research doc for the full bounded-context map) and provides the migration schedule.

The contract is descriptive of where we want to land, not historical. Existing code may still use legacy names; rename PRs land mechanically per the schedule below.

## Canonical terms by bounded context

### BC1 Corpus

| Concept | Canonical | Legacy names to retire |
|---|---|---|
| Durable artifact under `.agents/learnings/` | **Learning** | _no drift_ |
| Promoted recurring friction under `.agents/patterns/` | **Pattern** | _no drift_ |
| Runtime-mined intake (JSONL) | **Finding** | _no drift_ |
| Record that a Learning was applied | **Citation** | _no drift_ |
| Assembled retrieval bundle delivered to a session | **ContextPacket** | "context bundle", "retrieval packet" |
| Decay-ranked retrieval mechanism | **`ao inject`** | _no drift_ — the command is the canonical surface |

BC1 is largely drift-free at the aggregate level. The one term to lock: **ContextPacket** for the bundle the session consumes (vs. `Context` as the BC1 generic concept).

### BC2 Validation

| Concept | Canonical | Legacy names to retire |
|---|---|---|
| Named check that produces a Verdict | **Gate** | "check" (verb), "validator", "validation step" |
| Single execution of a Gate | **Check** (run noun) | "validation", "verify run" |
| Structured pass/fail/warn outcome | **Verdict** | "result", "outcome" (when scoped to a Gate) |
| Public assertion in `factory-claim-ledger.example.json` | **Claim** | "assertion", "claim marker", "AOP-CLAIM" (marker is fine, the noun is Claim) |
| File or artifact backing a Claim | **Evidence** | "proof", "backing" |
| GOALS.md strategic intent | **Directive** | "goal" (Directive is the BC2 aggregate; "goal" can refer to fitness-measurement output) |

**Ranked drift #1 (Gate / Check / Validation / Validator) resolution:** `Gate` is the BC2 aggregate. `Check` is a single invocation of a Gate. `Validator` (where used in Go) is the concrete adapter type — fine to keep but it implements a Gate. The 90 `scripts/check-*.sh` filenames stay (the file name describes the action; the concept inside is the Gate they enforce). The new term in code/docs should be **Gate**.

**Ranked drift #3 (Claim / Assertion / Evidence) resolution:** `Claim` is the BC2 noun for what the project says publicly. `Evidence` is what backs it. `daemon.QueueClaim` (the Go type) is a naming collision and should rename to `QueueLease` (different concept entirely — leasing a job slot, not asserting a public claim).

### BC3 Loop

| Concept | Canonical | Legacy names to retire |
|---|---|---|
| One iteration of /evolve, /rpi, /crank, /dream | **Cycle** | "iteration", "loop pass", "run" (run is fine where Phase context applies) |
| One discovery→implementation→validation arc inside a Cycle | **Phase** | _no drift_ |
| Claim about what a change will achieve | **Hypothesis** | _no drift_ — new term, cycle 51 |
| Terminal-state criteria for the autonomous loop | **Convergence** | _no drift_ — new term, cycle 51 |
| Harvested next-work, ready bead, or generator output | **WorkItem** | "task", "next-work entry" |

**Ranked drift #2 (Cycle / Loop / Iteration / Run) resolution:** `Cycle` is the BC3 aggregate. `lifecycle.CloseLoopIngestResult` (Go) is fine — "loop closure" is a different concept (the act of closing the feedback loop), not a synonym for Cycle. `overnight.IterationSummary` (Go) is Dream-loop-specific terminology only; rename to `DreamCycleSummary` would align across BCs but isn't urgent. `rpi.Run` should be deprecated outside Phase context — RPI executions are Cycles with Phases inside.

### BC4 Factory

| Concept | Canonical | Legacy names to retire |
|---|---|---|
| Anything being promoted | **Artifact** | _no drift_ |
| Entry-gate decision: does the artifact qualify? | **Admission** | "factory admission decision", "admit decision" |
| Output record: what came out, what proof, what cost | **Yield** | _no drift_ |
| State transition from one tier to the next | **Promotion** | "promote", "advance" |
| Registry of admitted/yielded artifacts | **Manifest** | "registry" (where scoped to Factory; `registry.json` is a Manifest of the BC5 Runtime inventory, not Factory) |

**Cross-BC collision:** `Admission` is a BC4 Gate-decision concept. `daemon.agentopsd-control-plane.md` uses "admission" as a job-queue state. The Go type for the latter should rename to `LeaseGrant` or `Admitted` (qualifier) to avoid collision; documentation can say "queue admission" but the canonical Factory concept stays `Admission`.

### BC5 Runtime

| Concept | Canonical | Legacy names to retire |
|---|---|---|
| Daemon-owned execution capacity | **WorkerSlot** | _no drift_ |
| Ephemeral provider instance (Claude / Codex / Cursor / OpenCode) | **AgentSession** | "worker session", "provider session" |
| Operator-facing interactive CLI session | **OperatorSession** | "CLI session", "user session" |
| Queued unit of work | **DaemonJob** | _no drift_ |
| Runtime-specific skill/hook implementation | **Harness** | _no drift_ |
| Claude Code event (PreToolUse, etc.) | **HookEvent** | "tool event", "claude event" |

**Ranked drift #5 (Session) resolution (refined cycle 125):** scope per BC. Four distinct Session-shaped concepts exist:

- **AgentSession** (BC5, ephemeral provider instance — Claude/Codex/etc.)
- **OperatorSession** (BC5, interactive CLI session)
- **TranscriptSession** (BC1 Corpus — extracted knowledge from a Claude Code transcript .jsonl). Currently bare `storage.Session` (93 refs) and `search.Session` (4 refs). Cycle 125 surfaced this — was NOT in the original drift catalog.
- **GasCitySession** (gascity package — daemon-managed worker session API; 79 refs across `Session`, `SessionCreateRequest`, `SessionListParams`, `SessionListResponse`, `SessionGetOptions`, `SessionSubmitRequest`, `SessionSubmitResponse`). Conceptually overlaps with AgentSession but the gascity API surface uses Session as the user-facing noun. Rename to AgentSession would break a published API contract; keep as-is, document the alias in code.

The `daemon` package should NOT have a `Session` type — anything that looked like one is either an AgentSession, OperatorSession, or TranscriptSession. Daemon manages **WorkerSlots** and **DaemonJobs**, not Sessions.

## Ranked drift #4: Skill vs Primitive vs Pattern vs Practice

This drift cuts across BC1 and BC5, so it gets its own section.

| Term | Canonical meaning | BC |
|---|---|---|
| **Skill** | Invocable harness module (e.g., `skills/evolve/SKILL.md`, `/evolve`). One per harness adapter via HarnessPort. | BC5 Runtime |
| **Pattern** | Promoted recurring friction stored under `.agents/patterns/` or `skills/*/references/*-pattern.md`. Reusable across cycles. | BC1 Corpus |
| **Practice** | Declared discipline citation in a primitive file (the `practices: [slug]` annotation atop SKILL.md / hook / script / eval / schema files). Links the primitive back to corpus practices in `PRACTICE.md`. | BC1 Corpus metadata |
| **Primitive** | Atomic unit of the AgentOps corpus that declares its practices. Includes Skills, hooks, schemas, evals, cli/ Go files. | BC1 Corpus (the unit being annotated) |

The pinned distinction: **Skill is invokable**, **Pattern is reusable knowledge**, **Practice is a citation**, **Primitive is the annotated unit**. Don't say "skill" when you mean "pattern"; don't say "pattern" when you mean "practice".

## Rename schedule (per epic soc-5yuy children)

| Drift | Resolution | Cycle slot |
|---|---|---|
| #1 Gate / Check / Validation | Pass through ~90 `check-*.sh` headers; pin `Gate` in Go type renames where applicable | soc-5yuy.1 |
| #2 Cycle / Loop / Iteration / Run | Deprecate `Run` outside Phase context; `lifecycle.CloseLoopIngestResult` keeps "loop" (different concept) | soc-5yuy.2 |
| #3 Claim / Assertion / Evidence | Rename `daemon.QueueClaim` → `QueueLease`; update consumers | soc-5yuy.3 |
| #4 Skill / Primitive / Pattern / Practice | Audit cross-references in `skills/*/SKILL.md` and `PRACTICE.md`; correct where the wrong term is used | soc-5yuy.4 |
| #5 Session | Rename ambiguous `daemon.Session` types per BC; add canonical `AgentSession` and `OperatorSession` where missing | soc-5yuy.5 |

Each rename ships as a single-concern commit demonstrating the 5 update principles (single concern, drift test, sibling citation, fitness delta, clean branch point). The rename itself IS the fitness delta (e.g., `legacy "Loop" references: N → 0`).

## Sibling pattern

Matches the existing meta-contract shape from `docs/contracts/update-principles.md` (cycle 52 commit `0d5fd66a`): top-level heading, frontmatter consumers list, body sections per scope, anti-claim disclaimer (next section), catalogued in `docs/documentation-index.md`. Structural-floor gate auto-validates on push.

## Anti-claim

Not claiming these are the only naming decisions worth pinning. The five ranked drifts here are the ones the bounded-context inventory measured as highest-pain (cross-referenced in multiple files with semantic conflict). Other terms may surface in future audits; they get added here as discovered.

Not claiming legacy names must be retired _immediately_. The rename schedule is per-cycle; each rename is bounded and reversible. Code can use legacy names until its rename cycle fires; documentation should start using canonical names from contract-landing day.

Not claiming this contract overrides PRODUCT.md or GOALS.md. Where those documents use a legacy term, the rename gets harvested as follow-up work, not retroactively edited.

## Companion artifacts

- Source: `.agents/research/2026-05-12-bounded-contexts-and-ports.md` § "5 ubiquitous-language drifts"
- Meta-contract: `docs/contracts/update-principles.md`
- Rescope plan: `docs/plans/2026-05-12-rescope-evolve-and-architecture.md`

## Current drift baseline (2026-05-13 cycle 123)

Captured so each rename PR can claim a measurable fitness delta
("X→Y for term Z"). Re-measure with the grep commands listed; each
soc-5yuy child PR ratchets one of these counts toward 0.

| Drift | Measurement | Baseline | Grep |
|---|---|---|---|
| #1 Gate / Check / Validation | `script header references "Validator"` outside Go code | ~90 `scripts/check-*.sh` files (filenames stay; headers/docs use "Gate") | `grep -rln 'scripts/check-' scripts/` |
| #2 Cycle / Loop / Iteration / Run | `rpi.Run` callers outside Phase context | _measure on demand_ | `grep -rn 'rpi\.Run\b\|RpiRun\b' cli/` |
| #3 Claim / Assertion / Evidence | `QueueClaim` references | 111 (vs 3 `QueueLease`) | `grep -rn 'QueueClaim' cli/ scripts/ docs/` |
| #4 Skill / Primitive / Pattern / Practice | mixed terms in `skills/*/SKILL.md` cross-references | _measure on demand_ | (audit per file; no single grep) |
| #5 Session | bare `type Session struct` declarations | 3 (`cli/internal/{search,storage,gascity}/types.go`); refined cycle 125 — these are 3 DIFFERENT concepts (storage/search = TranscriptSession 97 refs; gascity = published-API Session 79 refs, keep as-is) | `grep -rn 'type Session ' cli/` |

Excluded from counts: `cli/testdata/` (transcript fixtures), test
files (`*_test.go`) where Session/Claim mock types are legitimate.

## Cycle log

- 2026-05-12 cycle 58: contract written; rename schedule binds soc-5yuy.1–.5 to specific drift resolutions.
- 2026-05-13 cycle 123: added current-drift baseline section so rename PRs have a starting-count to ratchet against. QueueClaim sits at 111 refs (vs 3 QueueLease); `type Session struct` appears in 3 packages.
- 2026-05-13 cycle 125: refined drift #5 — the 3 bare `Session` types are 3 different concepts, not one. Added TranscriptSession (BC1) as missing canonical name. gascity.Session is a published-API surface — rename out of scope; keep + alias-document. storage.Session (93 refs) + search.Session (4 refs) rename to TranscriptSession is the actual soc-5yuy.5 unit; gascity stays.
