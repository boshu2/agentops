---
name: your-skill-name
description: 'What this skill does. Triggers: "trigger phrase", "other phrase".'
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
metadata:
  tier: execution
---

# your-skill-name

## Purpose

What this skill does and why it exists.

## When to Use

- Trigger condition one
- Trigger condition two
- Trigger condition three

## Inputs

- Required user input
- Relevant repo or runtime assumptions

## Instructions

1. First concrete step.
2. Main execution flow.
3. Validation or closeout step.

## Output

- Artifact, decision, or state change the skill should produce

## Examples

```text
Example prompt or invocation
```

## Troubleshooting

- Symptom: what goes wrong
  Fix: what to check or change

## References

- Add concrete `references/*.md` links here when the skill has them.
