---
package: cli/internal/context
status: active
owner: agentopsd
contract_source: cli/internal/context/run.go (ReadinessGreen/Amber/Red/Critical), cli/internal/context/packet.go
---

# cli/internal/context

Context packet assembly: turns `.agents/` artifacts plus session signals into a budgeted, readiness-graded packet that downstream agents consume at session start.

## Ownership

- **Owner:** agentopsd extraction track (epic `agentops-tqc`).
- **Concept:** a context "packet" is the bundle injected into a fresh agent session — knowledge, briefings, transcript tail, ranked intel — sized to a token budget and tagged with a readiness color.
- **Public stdlib name shadow:** the package imports `stdcontext "context"` — when editing this package, remember `context.X` refers to *this* package, not the stdlib `context.X`.

## Interfaces

- **Assembly:**
  - `assemble.go` — top-level assembly orchestration.
  - `run.go` — runtime entry (`Run`, readiness constants).
  - `packet.go` — `Packet` shape (the bundle written to disk / streamed).
  - `options.go` — caller-tunable knobs.
- **Budgeting:**
  - `budget.go` — token-budget enforcement.
  - `summarize.go` — summarization for over-budget items.
- **Trust and ranking:**
  - `trust_policy.go` — which sources are trusted at which readiness level.
  - `ranked_intel.go` — ranked intel feed (typically driven by `cli/internal/search` results).
- **Briefing surface:**
  - `explain.go` — explain why an item was/was not included.

## Non-obvious rules

- **Readiness has four levels, not three.** `GREEN`, `AMBER`, `RED`, `CRITICAL` (see `run.go`). Many call sites only model green/amber/red — `CRITICAL` is the "stop, do not proceed" tier and must be propagated, not collapsed.
- **Transcript tail is byte-capped.** `TranscriptTailMaxBytes = 512 KiB`. Don't raise this without measuring impact on token counts at the call site — `ao inject` and bead context both consume from this.
- **Filename sanitization is regex-driven.** `filenameSanitizerRE = [^a-zA-Z0-9._-]+` strips everything else. If you need to preserve a character (e.g., colon), update the regex AND grep for inverse assumptions in callers.
- **Issue ID detection is case-insensitive and prefix-based.** `issueIDRE` matches `\bag-[a-z0-9]+\b` (case-insensitive). The `ag-` prefix is the hard-coded agentops bead prefix — changing it without coordinating with `bd` config breaks issue auto-linking.
- **`Trust policy` gates inclusion before budget.** Items are filtered by trust first, then ranked, then budgeted. A trust-policy bug looks like a ranking bug — check `trust_policy.go` first.
- **`explain.go` is operator-facing.** It produces human-readable "why this was included" output. Keep its language stable so operators can grep for known phrases.

## Cross-references

- Parent epic: `agentops-tqc` (Olympus → agentopsd extraction).
- Skill: `skills/inject/SKILL.md` (the primary consumer surface), `skills/recover/SKILL.md`.
- Pattern source: olympus per-folder `AGENTS.md` ownership convention.
- Sibling packages: `cli/internal/search` (provides ranked intel), `cli/internal/daemon` (serves packets via daemon job results), `cli/internal/overnight` (dream consumes packets at iteration boundaries).
