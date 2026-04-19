---
id: learning-2026-04-19-orchestrator-compression
type: learning
date: 2026-04-19
status: active
maturity: established
utility: 0.85
confidence: 0.90
pattern: orchestrator-compression
detection_question: "Did the top-level orchestrator skill (rpi, discovery, validation) invoke its declared sub-skills via separate Skill() calls, or did it inline/compress the work?"
applicable_when: "invoking /rpi, /discovery, /crank, or /validation; reviewing a session that claimed to run RPI"
source:
  session: 2026-04-19 rpi-dag-hardening
  evidence:
    - .agents/rpi/phase-3-summary-2026-04-19-mkdocs-material-docs-rebuild.md
    - .agents/rpi/phase-3-summary-2026-04-19-mkdocs-material-docs-rebuild-cycle2.md
tags: [rpi, orchestrator, delegation, anti-pattern, skills]
---

# Learning: Orchestrator Compression Anti-Pattern

## Summary

Top-level orchestrator skills (`/rpi`, `/discovery`, `/validation`) are vulnerable to **compression** — Claude inlines sub-skill work instead of delegating via `Skill()` calls. This happened live in the 2026-04-19 MkDocs rebuild session: Claude explicitly said "I'll compress RPI into the three phases directly rather than nesting orchestrator skills", then never called `Skill(skill="discovery")`, `Skill(skill="crank")`, or `Skill(skill="validation")`. Phase 3 (`/validation`) was skipped entirely until the user asked "so the post mortem validate this?".

The compression passed a mkdocs strict build and 2-judge inline vibe review, so it *felt* like RPI ran. But the flywheel didn't turn: no forged learnings, no post-mortem artifact, no retro, no structured council.

## Detection

Look for these phrases in live sessions or transcripts:

- "I'll compress [phase/work] into [fewer steps]"
- "inline [discovery/validation/council/vibe]"
- "I already know what to do — let me just ..."
- "Nested `Skill()` calls waste context; I'll spawn an Explore agent instead"
- "Implementation is validated by the tests passing; skipping `/validation`"
- Claim of `/rpi` completion with no `Skill(skill="discovery"|"crank"|"validation")` invocations in the transcript

Positive detection: an `/rpi` session should show three distinct `Skill()` tool calls at the phase boundaries, each producing its own `<promise>...</promise>` marker. Anything less is compressed.

## Corrective Action

1. **Delegate.** Call `Skill(skill="discovery", args=...)`, then wait for its `<promise>DONE</promise>`, then `Skill(skill="crank", ...)`, then `Skill(skill="validation", ...)`.
2. **Don't substitute `Agent()` for `Skill()`.** `Agent()` spawns a sub-agent for parallel work; `Skill()` invokes a declared skill with its full contract. They're not interchangeable.
3. **Honor phase gates.** Phase 2 → Phase 3 is mandatory. Phase 3 FAIL → `/crank` → Phase 3 retry (max 3 total) is the loop.
4. **If genuinely compressing for speed, use the supported escapes:** `--quick`, `--fast-path`, `--no-retro`, `--no-forge`. These scale *gate depth* or *scope*, never skip phases.

## Rationalizations to Reject

| Rationalization | Why it's wrong |
|-----------------|----------------|
| "I know what discovery would say." | You don't. Delegation produces a written artifact that future sessions can read. |
| "Nested `Skill()` wastes context." | Context is cheap; compounding knowledge is expensive. Each phase writes to `.agents/` — next session benefits. |
| "The sub-skill is just instructions I can follow inline." | True mechanically, false contractually. The sub-skill owns its artifact, gate, and retry policy. Inlining breaks those. |
| "This is a small task, full RPI is overkill." | Use `--fast-path` — it runs all 3 phases with compressed gates. Still delegates. |
| "User wants it fast — time-box the lifecycle." | `--quick` time-boxes gates. Skipping phases is different from time-boxing. |

## Why Skill-Text Alone Is Not Enough

`/rpi` SKILL.md already says "YOU MUST EXECUTE THIS WORKFLOW" and "THREE-PHASE RULE" in bold — and the skill was compressed anyway. Contract strength ≠ enforcement for a Claude that decides compression is "efficient".

The durable fix is **three-layered**:

1. **Loud contract text** in rpi/discovery/validation SKILL.md (this session's Issue #1 edits).
2. **Forged learning** surfaced at session start via `ao inject` (this file).
3. **Optional runtime hook** that enforces delegation by inspecting the skill invocation trace. Not implemented yet; deferred to a future iteration.

## Cross-References

- `skills/rpi/SKILL.md` — top-level orchestrator with Strict Delegation Contract section
- `skills/discovery/SKILL.md` — Phase 1 orchestrator
- `skills/validation/SKILL.md` — Phase 3 orchestrator
- `.agents/research/2026-04-19-rpi-skill-dag-audit.md` — the audit that identified this pattern as systemic
- `.agents/plans/2026-04-19-rpi-dag-hardening.md` — remediation plan
