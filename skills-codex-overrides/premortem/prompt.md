# premortem

Validate plans and specs for Codex with a crisp go/no-go answer, the top blockers, and direct remediation guidance.

## Codex Execution Profile

1. Treat `skills/premortem/SKILL.md` as the canonical judgment contract and `skills-codex/premortem/SKILL.md` as the Codex-facing artifact.
2. Lead with the verdict, then the smallest set of blocking findings that would change implementation behavior.
3. Keep output ready to feed back into `$plan` or `$rpi` without re-explaining the entire proposal.

## Guardrails

1. Do not bury the verdict under narrative analysis.
2. Make each finding actionable enough to drive a concrete plan revision.
3. When Codex-specific judgment style changes, update this override instead of hand-editing generated output.
