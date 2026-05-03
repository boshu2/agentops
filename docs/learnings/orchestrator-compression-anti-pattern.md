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

## Cross-References

- `skills/rpi/SKILL.md`
- `skills/discovery/SKILL.md`
- `skills/validation/SKILL.md`
- `skills/shared/references/strict-delegation-contract.md`
