# Orchestrator Compression Anti-Pattern

## Summary

Top-level orchestrator skills (`/rpi`, `/discovery`, `/validation`) are vulnerable to compression: the agent inlines sub-skill work instead of delegating via separate `Skill()` calls. This happened live in the 2026-04-19 MkDocs rebuild session: the agent explicitly chose to compress RPI into three direct phases, then never called `Skill(skill="discovery")`, `Skill(skill="crank")`, or `Skill(skill="validation")`. Phase 3 validation was skipped entirely until the user asked whether post-mortem validation had happened.

The compression passed a strict MkDocs build and an inline two-judge vibe review, so it looked mechanically successful. The knowledge flywheel did not turn: no forged learnings, no post-mortem artifact, no retro, and no structured council verdict.

## Detection

Look for these phrases in live sessions or transcripts:

- "I'll compress this into one pass"
- "I'll do discovery inline"
- "I already know what to do"
- "Nested `Skill()` calls waste context"
- "Tests pass, so validation is done"
- A claimed `/rpi` completion with no distinct `Skill(skill="discovery"|"crank"|"validation")` invocations

Positive detection: an `/rpi` session should show distinct `Skill()` tool calls at phase boundaries, each producing its own completion marker. Anything less is compressed.

## Corrective Action

1. Delegate to `Skill(skill="discovery", args=...)`, wait for completion, then delegate to `Skill(skill="crank", ...)`, then delegate to `Skill(skill="validation", ...)`.
2. Do not substitute `Agent()` for `Skill()`. `Agent()` spawns parallel work; `Skill()` invokes a declared workflow contract.
3. Honor phase gates. Phase 2 to Phase 3 is mandatory. Phase 3 failure returns to implementation, then retries validation.
4. Use supported escapes for speed: `--quick`, `--fast-path`, `--no-retro`, or `--no-forge`. These scale gate depth or scope; they do not skip phases.

## Rationalizations To Reject

| Rationalization | Why It Is Wrong |
| --- | --- |
| "I know what discovery would say." | Delegation produces a written artifact future sessions can read. |
| "Nested `Skill()` wastes context." | Context is cheaper than losing the artifact chain. |
| "The sub-skill is just instructions I can follow inline." | The sub-skill owns an artifact, gate, and retry policy. |
| "This is a small task, full RPI is overkill." | Use `--fast-path`; it still delegates. |
| "The user wants speed." | Time-box gates with `--quick`; do not skip phases. |

## Compression Signature: Pre-Mortem Substitution (2026-06-14)

In a `/discovery` run, the agent skipped the delegated `/pre-mortem` and
substituted two things that looked like adversarial review but were not:

1. **An inline "honest risk" section** the author wrote in the discovery packet
   itself. Autocorrelated — same blind spots as the plan.
2. **A citation of an earlier adversarial siege** that had refuted an INPUT
   premise (whether the goal was right), not the implementation plan. Different
   artifact, different failure modes.

The agent claimed "pre-mortem baked in." The operator caught it. A subsequent
cross-family (Codex) pre-mortem found three real problems the inline section
missed: over-narrowed moat positioning, a vanity-metric ruler, and no
pre-registered decision rule.

### Detection phrases

- "Pre-mortem is baked into the honest-risk section"
- "The adversarial siege already covered this"
- "A related council already ran"
- Any pre-mortem claim where `author_id == judge_id`

### Fix

Added to `skills/shared/references/strict-delegation-contract.md`:
**Pre-Mortem Anti-Rationalization Clause** — explicit list of what does NOT
count as a pre-mortem (inline risk section, prior-premise adversarial pass,
"related council already ran"). Pre-mortem = DELEGATED + INDEPENDENT + fresh-
context on THIS plan.

Added to `skills/pre-mortem/SKILL.md`:
**Step 2.10: Pre-Registered Decision Rule** — strategy/experiment/one-way-door
plans require a pre-registered decision rule (what kills the claim, what
redirects) before judges deliberate. Without it, pre-mortem is unfalsifiable.
**Cross-family reviewer** requirement for one-way-door plans.

## Cross-References

- `skills/rpi/SKILL.md`
- `skills/discovery/SKILL.md`
- `skills/validation/SKILL.md`
- `skills/shared/references/strict-delegation-contract.md`
