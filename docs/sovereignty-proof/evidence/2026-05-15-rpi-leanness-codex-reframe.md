# Council verdict — RPI-leanness reframe (2026-05-15)

> **Provenance.** Excerpt from the original local council verdict at `.agents/council/2026-05-15-rpi-leanness-council.md` (gitignored — runtime corpus). Committed here so the [sovereignty proof page](../index.md#case-study-1--rpi-leanness-reframe-2026-05-15) has a falsifiable, in-repo source.

**Date:** 2026-05-15
**Mode:** mixed · deep (5 voices: Codex gpt-5.5, Claude agile-coach, Claude DDD-architect, Claude token-economist, Claude skeptic)
**Scope:** RPI loop token/context cost + DDD-hexagonal-for-skills + Agile-native planning
**Verdict:** decided — slice 1 = "Packet-as-contract + slice-first planning"

---

## The reframe (unanimous, including the independent Codex voice)

**"RPI is waterfall" is the wrong diagnosis.** RPI's execution phase is already wave-based and fairly Agile. The real disease is narrower and more fixable:

> Discovery + planning manufacture the **plan.md as a shared god artifact**, then every later phase rehydrates pieces of it. Bloat is an *artifact-boundary* bug, not a *methodology* failure. — codex (gpt-5.5), echoed by the skeptic seat.

**Evidence cited in the verdict (file:line — each resolves at HEAD):**

- A typical `plan.md` is 20–44 KB and is re-read ~8× per RPI lifecycle (discovery, pre-mortem, crank Step 4 acceptance-inject, crank Step 5.5 close, validation). See `skills/crank/SKILL.md`.
- `skills/plan/SKILL.md:112` requires symbol-level detail for **every file**.
- `skills/plan/SKILL.md:160` mandates a full file-dependency matrix.
- `skills/plan/SKILL.md:170` defines the full canonical plan template.
- `skills/plan/SKILL.md:166` names plan.md as runtime input (not just commentary).
- `skills/crank/SKILL.md` builds a "full project context" briefing per worker.
- `.agents/rpi/context-shards/latest.json` is generated but never consumed — dead infrastructure (the path is gitignored; verifiable via `find ~/.agents/rpi -name "context-shards"`).

---

## Where the voices agreed

| Point | Agile coach | DDD architect | Token economist | Skeptic | Codex |
|---|---|---|---|---|---|
| Plan.md is a god-artifact; split it | ✅ | ✅ (anemic untyped port) | ✅ (~40% of bloat) | ✅ (it's a bug) | ✅ (lever #1) |
| Slice-first: plan ONLY slice 1 in detail | ✅ | ✅ | ✅ | ✅ (after compression) | ✅ (lever #2) |
| `execution-packet.json` becomes the contract, plan.md = commentary | ✅ | ✅ (= `PlanPort`) | ✅ | ✅ | ✅ |
| Delete/wire dead `context-shards` | — | — | ✅ (1-line win) | ✅ | ✅ (lever #5) |
| Workers get bead criteria, NOT full-project briefing | ✅ | ✅ | ✅ | — | ✅ (lever #3) |
| Do NOT replace YAML acceptance criteria with Gherkin | — | — | — | ✅ (protect gates) | ✅ |
| Do NOT build "RPI 2.0" DDD meta-framework now | — | ✅ (cargo-cult risk) | — | ✅ (that IS the waterfall) | ✅ (the biggest trap) |

---

## DDD/hexagonal-for-skills — verdict

Real **if applied to artifacts and capabilities, not folder cosplay** (codex + DDD seat agree). A skill already *is* a bounded context; the artifacts between skills *are* the ports — they're just untyped and unbounded today.

Bounded contexts: Discovery · Planning · Execution · Validation · Knowledge · Runtime.

**Decision: do ONE port now.** Slice 1 formalizes `ExecutionPacketPort` — `execution-packet.json` becomes the typed, ≤8 KB contract between Planning → Execution. The other 7 ports are slices 2…N stubs, NOT scoped now. Doing all of them = the big-bang waterfall the maintainer is escaping.

---

## Outcome (postscript, 2026-05-15)

After crank wave 1, an in-flight branch was found: **PR #275** `crank/ddd-hexagonal-2026-05-12` (OPEN, ~130 files) — it already implements most of the council's "thin slice 1": `cli/internal/domain/packet/ExecutionPacket` aggregate + `packet_repo.go` adapter (the `ExecutionPacketPort`), `docs/templates/slice-validation.md` + `intent-issue.md` + `docs/architecture/operating-loop.md` (slice-first / BDD-intent doctrine), and `cli/internal/ports/{llm,storage,tracker}.go` + `ADR-0001-ddd-hexagonal-adoption.md`.

**Decision:** PR #275 became canonical; `soc-etwf.1/.2/.3` were closed as superseded. The mis-scoped methodology epic was averted.

---

## Why this is the sovereignty proof artifact (case 1)

Without the Codex (gpt-5.5) reframe — independently arrived at by a model trained outside Anthropic — the operator would have planned a methodology rewrite. The Codex voice forced the diagnosis layer down to "artifact-boundary bug," which turned a multi-week refactor into a single-port surgical change. **That is the sovereignty value: an independent vendor catches what the resident vendor's prior would have rubber-stamped.**
