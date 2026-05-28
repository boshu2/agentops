# AgentOps 3.0 Council Verdict Example

This is public sample output for the 3.0 explainer kit. It is an example
artifact shape, not PMF evidence or a claim that a live mixed council completed.

## Run Metadata

| Field | Value |
|---|---|
| Packet | `.agents/packets/agentops-3-launch.md` |
| Briefing | `.agents/rpi/briefing-current.md` |
| Decision | Should the 3.0 launch demo lead with council-first engineering judgment? |
| Mode | Mixed Claude/Codex when available; `/council --quick` fallback allowed for local proof |
| Artifact | `.agents/council/<run-id>/verdict.md` |

## Consolidated Verdict

**Status:** WARN

Lead with the council-first engineering judgment demo. Keep the daemon and
software-factory automation as the second-stage lane after the viewer has seen
one visible domain/practice packet and one verdict artifact.

The launch copy should be updated before publication to avoid unsupported
productivity, PMF, or autonomous-factory claims.

## Judge Notes

| Judge | Assessment |
|---|---|
| Claude product judge | PASS: council-first is the clearest first value because the user sees taste, judgment, and shared context immediately. |
| Claude engineering-practice judge | PASS: the domain/practice packet encodes DDD/TDD/BDD/review/release discipline before implementation begins. |
| Codex implementation judge | PASS: existing `ao quick-start`, `ao context assemble`, `/council`, bd, and `.agents/` surfaces can support the path. |
| Codex release-risk judge | WARN: public copy must avoid PMF and speed claims until exported evidence exists. |

## Evidence Used

- `PRODUCT.md`
- `GOALS.md`
- `docs/examples/agentops-3-domain-practice-packet.md`
- `docs/first-value-path.md`
- `docs/activation-profiles.md`
- `soc-m6v5.9.7`

## Required Follow-Up

1. Point README and launch content at the first-value path.
2. Show `.agents/council/<run-id>/verdict.md` before any daemon automation.
3. Use `bd create ... --description "From .agents/council/<run-id>/verdict.md"`
   for tracked follow-up work.
4. Keep `ao activate product-council` deferred until repeated PMF runs prove the
   profile is stable.

## Claim Posture

Allowed public language:

- "engineering operating system for agent teams"
- "disciplined engineering layer for agentic software development"
- "from agent opinions to engineering verdicts"
- "bring your agent, bring your harness"

Blocked until exported evidence exists:

- PMF claims.
- Specific productivity or speedup numbers.
- Fully autonomous factory as the first-value promise.
- Regulated or certified-operation claims.
