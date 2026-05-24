# Examples + Troubleshooting Template

> Reference template for adding `## Examples` and `## Troubleshooting` sections to skills. Workers MUST follow this format exactly.

## Section Placement

- `## Examples` goes BEFORE `## See Also` (or at end of file if no See Also)
- `## Troubleshooting` goes AFTER `## Examples`, BEFORE `## See Also`

## Append-vs-Create Rules (4 cases)

1. **Neither section exists:** CREATE both `## Examples` and `## Troubleshooting` before `## See Also`
2. **Only `## Examples` exists:** APPEND new examples below existing ones; CREATE `## Troubleshooting` after Examples
3. **Only `## Troubleshooting` exists:** CREATE `## Examples` before Troubleshooting; APPEND new rows to existing table
4. **Both exist:** APPEND new examples and new troubleshooting rows to existing sections (don't rewrite)

## Examples Format

```markdown
## Examples

### <Scenario Title>

**User says:** `/<skill> <args>`

**What happens:**
1. Agent does X
2. Agent does Y
3. Output written to Z

**Result:** Brief description of outcome
```

Each example MUST include:
- A realistic trigger phrase showing how a user invokes the skill
- Step-by-step behavior description (what the agent actually does)
- Expected output or result

## Troubleshooting Format

```markdown
## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Error message or symptom | Why it happens | How to fix |
```

Each entry MUST include:
- A specific, recognizable error message or symptom
- The root cause explanation
- A concrete fix (command, config change, or workaround)

## Per-Tier Requirements

| Tier | Skills | Examples | Troubleshooting | Word Budget |
|------|--------|----------|-----------------|-------------|
| Tier 1 | council, crank, vibe | 3+ scenarios | 3+ entries | Examples: max 400 words |
| Tier 2 | research, plan, implement, pre-mortem, post-mortem, rpi | 2+ scenarios | 2+ entries (skip if exists) | Examples: max 250 words |
| Tier 3 | swarm, codex-team, evolve, release, quickstart, handoff | 2+ scenarios | 2+ entries (skip if exists) | Examples: max 250 words |
| Tier 4 | bug-hunt, complexity, doc, product, status, trace, inbox, knowledge, retro | 2 scenarios | 2-3 entries | Examples: max 250 words |
| Internal | extract, flywheel, forge, inject, provenance, ratchet, standards, using-agentops | 1-2 scenarios | 2 entries | Examples: max 200 words |

**Note:** `shared` is excluded — it's a reference collection, not a skill.

Troubleshooting: max 200 words per skill across all tiers.

Total SKILL.md word count MUST stay under 5000 words. Verify with `wc -w SKILL.md`.

## Quality Bar

- Examples must reflect actual skill behavior (not placeholder text)
- Troubleshooting entries must describe real failure modes users encounter
- Internal skills: show programmatic invocation (how other skills call them)
- User-facing skills: show natural language triggers a human would type
