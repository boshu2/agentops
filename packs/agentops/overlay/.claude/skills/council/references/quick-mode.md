# Quick Mode (`--quick`)

Single-agent inline validation. No subprocess spawning, no Task tool, no Codex. The current agent performs a structured self-review using the same output schema as a full council.

**When to use:** Routine checks, mid-implementation sanity checks, pre-commit quick scan. Use full council for important decisions, final reviews, or when cross-perspective disagreement is valuable.

## Quick Mode Execution

1. **Gather context** (same as full council -- read target files, get diffs)
2. **Skip agent spawning** -- no Task tool, no background agents
3. **Perform structured self-review inline** using this template:

```
Analyze the target as a single independent reviewer.

Target: {TARGET_DESCRIPTION}

Context:
{FILES_AND_DIFFS}

Respond with:
1. A JSON block matching the council output_schema:
   {
     "verdict": "PASS | WARN | FAIL",
     "confidence": "HIGH | MEDIUM | LOW",
     "key_insight": "Single sentence summary",
     "findings": [
       {
         "id": "(optional) Stable finding ID for cross-skill correlation",
         "severity": "critical | significant | minor",
         "category": "security | architecture | performance | style",
         "description": "What was found",
         "location": "file:line if applicable",
         "recommendation": "How to address",
         "fix": "Specific action to resolve this finding",
         "why": "Root cause or rationale",
         "ref": "File path, spec anchor, or doc reference"
       }
     ],
     "recommendation": "Concrete next step"
   }
2. A brief Markdown explanation (2-5 paragraphs max)
```

4. **Write report** to `.agents/council/YYYY-MM-DD-quick-<target>.md`
5. **Label clearly** as `Mode: quick (single-agent)` in the report header

## Quick Mode Report Format

```markdown
# Council Quick Check: <target>

**Date:** YYYY-MM-DD
**Mode:** quick (single-agent, no multi-perspective spawning)
**Target:** <description>

## Verdict: PASS | WARN | FAIL

<JSON block>

## Analysis

<Brief explanation>

---
*Quick check -- for thorough multi-perspective review, run `/council validate` (default mode).*
```

## Quick Mode Limitations

- No cross-perspective disagreement (single viewpoint)
- No cross-vendor insights (no Codex)
- Lower confidence ceiling than full council
- Not suitable for security audits or architecture decisions -- use `--deep` or `--mixed` for those
