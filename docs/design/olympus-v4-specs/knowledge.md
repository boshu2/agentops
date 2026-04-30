> **Status:** olympus v4 reference, NOT canonical for agentopsd. This file is a verbatim port from olympus/docs/specs/v4/ for cross-reference only. Where this disagrees with agentopsd's canonical design at `.agents/design/2026-04-28-design-agentops-daemon-gascity-vertical-slices.md`, **agentopsd canonical wins**.

# Knowledge Compounding

> *"Constraint tests prevent recurrence. Documentation does not."*

**Date:** 2026-02-15
**Status:** Draft (v4)
**Depends On:** `architecture.md` (Layer 0, Layer 4), `validation.md`

---

## The Flywheel

Knowledge compounds through a closed loop. Every quest makes the next quest cheaper.

```
Execute → Extract → Constrain → Inject → Execute (faster)
```

1. **Execute** — Agent works a bead, succeeds or fails.
2. **Extract** — `/post-mortem` and `/retro` pull learnings from the attempt.
3. **Constrain** — Learnings become `*_test.go` files that run in CI. Binary: the test blocks the anti-pattern or it doesn't exist.
4. **Inject** — `ao inject` loads relevant learnings into the next agent's context at spawn time.
5. **Execute** — Next agent starts with more knowledge and harder guardrails. Cycle repeats.

The flywheel has no "documentation" step. Markdown notes are intermediate artifacts. The terminal form of a learning is either a constraint test or injected context — never a wiki page.

---

## Learning Extraction

Learnings are extracted by two skills at the end of every quest:

| Skill | When | Output |
|-------|------|--------|
| `/post-mortem` | After quest completion or failure | `.agents/learnings/YYYY-MM-DD-<slug>.md` |
| `/retro` | After any significant work session | `.agents/learnings/YYYY-MM-DD-<slug>.md` |

### Learning File Format

Each learning file is a structured markdown document:

```markdown
# Learning: <title>

**Pattern:** <what happened>
**Anti-pattern:** <what to avoid>
**Constraint candidate:** yes | no
**Source:** <quest-id or bead-id>

## Detail

<freeform description of what was learned and why it matters>
```

The `constraint candidate: yes` flag marks learnings that should become automated tests. Not every learning qualifies — only those where a deterministic check can prevent recurrence.

### Extraction Rules

- Every completed quest produces at least one learning file. No exceptions.
- Failed quests produce learnings too. Failures are the highest-value input to the flywheel.
- Learnings are committed to git in `.agents/learnings/`. They are ratcheted — once committed, they persist.

---

## Constraint Injection

A constraint is a Go test that blocks an anti-pattern in CI. The bar is binary: either the test catches the problem, or don't write it.

### From Learning to Constraint

```bash
ol knowledge constraints          # Scan learnings for constraint candidates
ol knowledge constraints --generate-tests   # Generate *_test.go files from candidates
```

Generated tests land in the appropriate `*_test.go` file next to the code they protect. They run with `go test ./...` in Stage 1 validation. No special framework, no separate enforcement tool — just tests.

### What Makes a Good Constraint

- **Deterministic** — Same input, same result. No flaky checks.
- **Targeted** — Tests one specific anti-pattern. Fails with a clear message.
- **Self-documenting** — Test name and failure message explain the "why".

### What Doesn't Qualify

- Style preferences (use a linter instead).
- Observations that can't be mechanically verified.
- Learnings that require human judgment to evaluate.

---

## Knowledge Injection

`ao inject` loads relevant learnings into an agent's context window at spawn time. This is how accumulated knowledge reaches the next agent without that agent having to discover it independently.

### What Gets Injected

- Learnings relevant to the current bead's domain (semantic match via `ao`).
- Constraint test names that guard the area being worked on.
- Feedback from prior failed attempts on the same bead.

### Injection is Selective

Not all learnings are injected into every context. `ao inject` uses semantic similarity to select only relevant learnings. An agent working on validation doesn't need learnings about wave scheduling.

Context windows are finite. Injecting everything defeats the purpose. The system curates — the agent receives only what helps.

---

## Storage

| Artifact | Location | Lifetime |
|----------|----------|----------|
| Learning files | `.agents/learnings/*.md` | Permanent (git-ratcheted) |
| Constraint tests | `*_test.go` (co-located) | Permanent (git-ratcheted) |
| Knowledge index | `ao` knowledge base | Rebuilt from learnings on demand |
| Injection context | Agent context window | Ephemeral (per-session) |

The permanent artifacts — learning files and constraint tests — live in git. They survive agent deaths, context exhaustion, and repository clones. The knowledge index is derived and rebuildable. The injection context is intentionally ephemeral.

---

## References

- `docs/specs/v4/architecture.md` — Layer 0 (Constraint Engine), Layer 4 (Knowledge Flywheel)
- `docs/specs/v4/validation.md` — How constraint tests integrate with CI
- `docs/BROWNIAN-RATCHET.md` — Why ratchets compound and retries don't
