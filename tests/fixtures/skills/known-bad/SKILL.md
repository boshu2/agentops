---
name: known-bad
description: 'A test fixture that intentionally fails Pass-2 checks.'
---

# /known-bad — Auditor FAIL test fixture

This SKILL.md passes Pass 1 (frontmatter is structurally valid and name matches dir) but intentionally violates Pass-2 content-discipline checks. The auditor should return verdict FAIL with at least one fail-severity finding.

## How to use

Run the auditor and expect FAIL exit 1.

## Examples

```bash
bash skills/skill-auditor/scripts/audit.sh tests/fixtures/skills/known-bad
```

Note: there is intentionally no `## ⚠️ Critical Constraints` section, no `**Why:**` rationales, no `## Output Specification`, no `## Quality Rubric`, and no `Checkpoint:` markers. These omissions exercise multiple Pass-2 checks. The single FAIL severity check (`output-spec-explicit`) drives the FAIL verdict.
