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
| Compression invariant for prompt/packet/handoff content | **Context Density Rule** | "density thing", "token density", "compression rule" |
| Decay-ranked retrieval mechanism | **`ao inject`** | _no drift_ — the command is the canonical surface |

BC1 is largely drift-free at the aggregate level. The two terms to lock: **ContextPacket** for the bundle the session consumes (vs. `Context` as the BC1 generic concept), and **Context Density Rule** for the CDLC invariant that every context token carries intent, boundary, evidence, decision, constraint, or next action.

### BC2 Validation

| Concept | Canonical | Legacy names to retire |
|---|---|---|
| Named check that produces a Verdict | **Gate** | "check" (verb), "validator", "validation step" |
| Single execution of a Gate | **Check** (run noun) | "validation", "verify run" |
| Structured pass/fail/warn outcome | **Verdict** | "result", "outcome" (when scoped to a Gate) |
| Public assertion in `factory-claim-ledger.example.json` | **Claim** | "assertion", "claim marker", "AOP-CLAIM" (marker is fine, the noun is Claim) |
| File or artifact backing a Claim | **Evidence** | "proof", "backing" |
| GOALS.md strategic intent | **Directive** | "goal" (Directive is the BC2 aggregate; "goal" can refer to fitness-measurement output) |

**Ranked drift #1 (Gate / Check / Validation / Validator) resolution (refined cycle 130):** `Gate` is the BC2 aggregate noun. The audit found that what the catalog called "90 scripts inconsistent" is actually mostly fine:

- **Zero `scripts/check-*.sh` headers use the noun "Validator".** (None.)
- **38 of ~90 scripts already use "Gate" in their headers.** The rest don't repeat the noun in their headers; they just describe what they check.
- **`cli/internal/ratchet.Validator`** is the only `Validator` type in Go code. It's a ratchet-specific concept (not a check-script validator). KEEP.
- "validate" / "validates" used as English verbs in header descriptions ("Validates that ao doctor runs without failures") is fine — that's normal English, not a noun-confusion drift.

Cycle-130 finding: **drift #1 is audit-only.** The codebase organically drifted toward "Gate" already. No code or doc renames needed. The contract was over-eager about this drift size. The `Check` (single Gate invocation) terminology pin remains useful for future code; current code uses no conflicting noun.

**Ranked drift #3 (Claim / Assertion / Evidence) resolution:** `Claim` is the BC2 noun for what the project says publicly. `Evidence` is what backs it. `daemon.QueueClaim` (the Go type) was a naming collision (different concept entirely — leasing a job slot, not asserting a public claim) — **renamed to `QueueLease` in cycle 126** (commit ahead, 108 Go refs touched across cli/internal/daemon, cli/internal/rpi, cli/internal/llmwiki, cli/cmd/ao).

### BC3 Loop

| Concept | Canonical | Legacy names to retire |
|---|---|---|
| One iteration of /evolve, /rpi, /crank, /dream | **Cycle** | "iteration", "loop pass", "run" (run is fine where Phase context applies) |
| One discovery→implementation→validation arc inside a Cycle | **Phase** | _no drift_ |
| Claim about what a change will achieve | **Hypothesis** | _no drift_ — new term, cycle 51 |
| Terminal-state criteria for the autonomous loop | **Convergence** | _no drift_ — new term, cycle 51 |
| Harvested next-work, ready bead, or generator output | **WorkItem** | "task", "next-work entry" |

**Ranked drift #2 (Cycle / Loop / Iteration / Run) resolution (refined cycle 129):** `Cycle` is the BC3 aggregate noun. The audit found that what looked like "Run drift" is mostly NOT drift:

- `lifecycle.CloseLoopIngestResult` — "loop closure" is a different concept (closing the feedback loop), not a Cycle synonym. KEEP.
- `overnight.IterationSummary` — Dream-loop-internal terminology. KEEP scoped to Dream; cross-BC alignment would be `DreamCycleSummary` but isn't urgent.
- `JobTypeRPIRun` enum, `RPIRunJobSpec`, `RPIRunJobSpecFromPayload`, `RPIRunRequest`, `RPIRunResult`, `RPIRunCache`, `RPIRunRegistryDir` — these are SERIALIZED CONTRACTS (JSON job specs, filesystem path constants). Renaming would break on-disk format + cross-process job spec compatibility. KEEP.
- `RPIRunner`, `RPIRunnerOptions`, `RPIRunExecutor` — legitimate Runner/Executor naming. KEEP.
- `RPIRuntimeCommand`, `RPIRuntimeMode` — NOT a Run concept; contains "Run" as part of "Runtime" (substring coincidence). KEEP.
- `discoverRPIRuns`, `serveRPIRuns` — operator-facing CLI commands. Renaming = user-facing breaking change. KEEP.
- `rpi.RPIRun` (the artifact struct) — 132+ `Iteration*` identifiers are all Dream-internal; 20+ `RPIRun*` identifiers are serialized-contract or named-typed Runner code.

Cycle-129 finding: **drift #2 is audit-only.** The "Run/Iteration" usage in the codebase is mostly conceptually correct — RPI's Run is its execution pattern (analogous to GitHub Actions' CI Run); Dream's Iteration is its convergence-loop counter. Both are scoped uses, not generic drift. No code renames are needed; the contract was over-eager to flag this.

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

**Ranked drift #5 (Session) resolution (refined cycle 125, re-refined cycle 131):** scope per BC. Four distinct Session-shaped concepts exist, each scoped by its package qualifier:

- **AgentSession** (BC5; `agentworker.AgentSession` — ephemeral provider instance for Claude/Codex/etc.)
- **OperatorSession** (BC5; canonical name for interactive CLI session)
- **TranscriptSession** (BC1 Corpus — extracted knowledge from a Claude Code transcript .jsonl). Currently lives as `storage.Session` + `search.Session`. The package qualifier IS the disambiguator: `storage.Session` reads unambiguously as "the storage package's Session = transcript-mined knowledge bundle" in context.
- **GasCitySession** (gascity package — daemon-managed worker session API). Published-API surface, keep as-is.

Cycle-131 audit: a narrow rename of `storage.Session → storage.TranscriptSession` and `search.Session → search.TranscriptSession` would touch 342 word-bounded `Session` refs across 6 packages (storage, search, formatter, forge, context, cmd/ao). That blast radius is 3x cycle 126's QueueClaim rename (108 refs) — too big for one cycle, and like cycle 130's drift #1, the audit reveals the codebase already has unambiguous names: the Go package qualifier provides the BC scope. No rename needed.

The `daemon` package should NOT have a `Session` type — anything that looked like one is either an AgentSession, OperatorSession, TranscriptSession, or GasCitySession. Daemon manages **WorkerSlots** and **DaemonJobs**, not Sessions.

## Ranked drift #4: Skill vs Primitive vs Pattern vs Practice

This drift cuts across BC1 and BC5, so it gets its own section.

| Term | Canonical meaning | BC |
|---|---|---|
| **Skill** | Invocable harness module (e.g., `skills/evolve/SKILL.md`, `/evolve`). One per harness adapter via HarnessPort. | BC5 Runtime |
| **Pattern** | Promoted recurring friction stored under `.agents/patterns/` or `skills/*/references/*-pattern.md`. Reusable across cycles. | BC1 Corpus |
| **Practice** | Declared discipline citation in a primitive file (the `practices: [slug]` annotation atop SKILL.md / hook / script / eval / schema files). Links the primitive back to corpus practices in `PRACTICE-REGISTRY.md`. | BC1 Corpus metadata |
| **Primitive** | Atomic unit of the AgentOps corpus that declares its practices. Includes Skills, hooks, schemas, evals, cli/ Go files. | BC1 Corpus (the unit being annotated) |

The pinned distinction: **Skill is invokable**, **Pattern is reusable knowledge**, **Practice is a citation**, **Primitive is the annotated unit**. Don't say "skill" when you mean "pattern"; don't say "pattern" when you mean "practice".

### Sub-overload: "primitive" carries three senses (audit cycle 128)

A cycle-128 audit found `primitive` is used for three distinct concepts. The BC1-Corpus sense above is the canonical one for the cross-cutting `practices: [slug]` annotation. The other two are scoped, not drift, but flagged here so readers don't conflate them:

| Sense | Used in | Distinguishing context |
|---|---|---|
| **BC1 Corpus Primitive** | `PRACTICE-REGISTRY.md`, `practices: [slug]` annotations atop SKILL.md / hooks / scripts | "the annotated unit of the AgentOps corpus" |
| **Domain Primitive** | `skills/domain/SKILL.md` + its `references/*-primitive.md` files | Architecture-as-domain — 6 structural primitives (Entry, Index, Citation, Primitive, Slice, Anti-Pattern). Scope: the `domain` skill only. |
| **Runtime primitive** (Codex/Claude tool features) | `skills/converter/SKILL.md`, `skills/standards/SKILL.md` ("prohibited primitives", "Claude primitive labels") | Atomic tool/feature exposed by a runtime (e.g., a Claude-only hook event). Scope: converter + standards skills. |

These three senses do NOT need renaming — each is scoped to its surface. The flag is descriptive: when reading `primitive` in code, check the surrounding skill/file to pick the right sense.

## Rename schedule (per epic soc-5yuy children)

| Drift | Resolution | Cycle slot |
|---|---|---|
| #1 Gate / Check / Validation | Pass through ~90 `check-*.sh` headers; pin `Gate` in Go type renames where applicable | soc-5yuy.1 |
| #2 Cycle / Loop / Iteration / Run | Deprecate `Run` outside Phase context; `lifecycle.CloseLoopIngestResult` keeps "loop" (different concept) | soc-5yuy.2 |
| #3 Claim / Assertion / Evidence | ✓ DONE cycle 126: `daemon.QueueClaim` → `QueueLease` (108 refs renamed; consumers updated) | soc-5yuy.3 |
| #4 Skill / Primitive / Pattern / Practice | Audit cross-references in `skills/*/SKILL.md` and `PRACTICE-REGISTRY.md`; correct where the wrong term is used | soc-5yuy.4 |
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
| #1 Gate / Check / Validation | `script header references "Validator"` outside Go code | ✓ AUDITED cycle 130: 0 script headers use "Validator"; 38/~90 already use "Gate"; rest use neither noun. cli/internal/ratchet.Validator is the only Go Validator (legitimate ratchet concept). No drift; no renames needed. | `grep -l 'Validator' scripts/check-*.sh` |
| #2 Cycle / Loop / Iteration / Run | `rpi.Run` callers outside Phase context | ✓ AUDITED cycle 129: most usage is serialized-contract enums (JobTypeRPIRun), filesystem path constants (RPIRunRegistryDir), legitimate Runner naming, or "Runtime" substring-coincidence. No drift; no renames needed. | `grep -rn 'rpi\.Run\b\|RpiRun\b' cli/` |
| #3 Claim / Assertion / Evidence | `QueueClaim` references | ✓ 0 (down from 111 at cycle 123 baseline); cycle 126 sed rename + green tests | `grep -rn 'QueueClaim' cli/ scripts/ docs/` |
| #4 Skill / Primitive / Pattern / Practice | mixed terms in `skills/*/SKILL.md` cross-references | ✓ AUDITED cycle 128: no systemic misuse; `primitive` overload documented as 3 scoped senses (no renames needed) | (audit per file; no single grep) |
| #5 Session | bare `type Session struct` declarations | ✓ AUDITED cycle 131: 4 distinct concepts disambiguated by package qualifier (agentworker.AgentSession, storage.Session/search.Session = TranscriptSession, gascity.Session, future OperatorSession). 342 word-bounded `Session` refs across 6 packages would need atomic rename — too big and not necessary since package qualifier disambiguates. No renames needed. | `grep -rn 'type Session ' cli/` |

Excluded from counts: `cli/testdata/` (transcript fixtures), test
files (`*_test.go`) where Session/Claim mock types are legitimate.

## Cycle log

- 2026-05-12 cycle 58: contract written; rename schedule binds soc-5yuy.1–.5 to specific drift resolutions.
- 2026-05-13 cycle 123: added current-drift baseline section so rename PRs have a starting-count to ratchet against. QueueClaim sits at 111 refs (vs 3 QueueLease); `type Session struct` appears in 3 packages.
- 2026-05-13 cycle 125: refined drift #5 — the 3 bare `Session` types are 3 different concepts, not one. Added TranscriptSession (BC1) as missing canonical name. gascity.Session is a published-API surface — rename out of scope; keep + alias-document. storage.Session (93 refs) + search.Session (4 refs) rename to TranscriptSession is the actual soc-5yuy.5 unit; gascity stays.
- 2026-05-13 cycle 131: **drift #5 RESOLVED via audit-only.** Cycle 125 first-execution audit had said the 3 bare Session types were 3 different concepts and the storage+search subset (97 refs) was the actual unit. Cycle 131 pre-rename substring audit shows the full surface is 342 word-bounded `Session` refs across 6 packages — too big for one cycle, and the Go package qualifier (storage.Session, search.Session, gascity.Session, agentworker.AgentSession) already disambiguates the 4 concepts. No rename needed. soc-5yuy now 5/5 closed (only .3 was an actual rename; all 4 others audit-only).
- 2026-05-13 cycle 130: **drift #1 RESOLVED via audit-only.** Surveyed scripts/check-*.sh headers — 0 use "Validator", 38 use "Gate", rest describe their function. Only Go Validator is cli/internal/ratchet (legitimate ratchet-specific concept). The catalog's "90 scripts inconsistent" claim was overstated; the codebase organically drifted toward "Gate" already. Fourth soc-5yuy child to close via audit-only (joins .2 cycle 129, .4 cycle 128). Pattern: 3 of the 5 catalog flags turned out to be audit-only resolutions.
- 2026-05-13 cycle 129: **drift #2 RESOLVED via audit-only.** Enumerated all `RPIRun*` (~150 identifiers) and `Iteration*` (132 identifiers). Most usage is serialized-contract enums (JobTypeRPIRun), filesystem path constants (RPIRunRegistryDir), legitimate Runner naming, substring-coincidence ("Runtime"), or scoped Dream-internal counters. Contract over-flagged this — the actual codebase has Run/Iteration semantically correct. No renames needed. Third soc-5yuy child to close via audit-only (after cycle-128 drift #4 and cycle-126 drift #3 via rename).
- 2026-05-13 cycle 128: **drift #4 RESOLVED via audit-only.** Surveyed `primitive`/`skill`/`pattern`/`practice` usage across SKILL.md, PRACTICE-REGISTRY.md, contracts. Found no systemic misuse — Skill is consistently "invokable", Pattern is "reusable knowledge", Practice is "citation annotation". `primitive` IS overloaded across 3 distinct concepts (BC1 Corpus Primitive vs Domain Primitive in skills/domain/ vs Runtime primitive in converter/standards) — added a sub-overload table to the contract documenting all 3 senses as scoped (not drift). No renames needed; the audit IS the resolution.
- 2026-05-13 cycle 126: **drift #3 RESOLVED.** `daemon.QueueClaim` → `QueueLease` rename shipped: 108 Go refs across cli/internal/{daemon,rpi,llmwiki} + cli/cmd/ao. Audit-then-execute: pre-rename audit (cycle-125 pattern) showed no split-concept surprise for the daemon struct itself, but post-commit self-review caught an over-broad sed in the same cycle — `rpi.ErrQueueClaimConflict` / `rpi.RequireQueueClaimOwner` (about work-item claim coordination in `.agents/rpi/next-work.jsonl`, NOT the daemon job-slot lease) were also renamed by the substring match. Reverted just those identifiers; daemon-side `QueueLease` stays. Lesson: substring-based sed can over-reach across different concepts that share a prefix; an audit needs to enumerate ALL identifiers containing the substring, not just the type definition. First soc-5yuy child to close.
- 2026-05-15: added **Context Density Rule** to BC1 and `skills/domain/references/context-density-rule.md`. This gives the "density thing" an explicit DDD name and keeps token-scarcity doctrine in CDLC/operating-loop surfaces rather than in the practice registry.
