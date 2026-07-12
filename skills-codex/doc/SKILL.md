---
name: doc
description: 'Generate docs from repo truth. Triggers: "doc", "generate repo docs", "validate documentation".'
---

# Doc Skill

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

Generate and validate documentation for any project.

## Constraints

- Ground every documentation claim in inspected repository evidence, because plausible but stale prose is a documentation defect.
- For OSS scaffold requests, create missing docs only by default; never update or overwrite an existing doc unless the user explicitly confirms, because it may encode policy or project history. Treat `refresh` as a separate opt-in path and confirm its target writes with the user before proceeding.
- Report unresolved gaps explicitly, because generated prose is not proof of documentation coverage.

## Execution Steps

Default mode is deliberately thin — a frontier model runs it correctly with no payload. Given `$doc [command] [target]`:

1. **Detect project type** — `ls package.json pyproject.toml go.mod Cargo.toml` + existing `docs/`; classify CODING / INFORMATIONAL / OPS.
2. **Run the command** — `discover` (grep undocumented funcs), `coverage` (documented vs total), `gen [feature]` (read code → stamp function/class markdown), `all`, or `validate`.
3. **Write the report** to `.agents/doc/YYYY-MM-DD-<target>.md` (coverage %, generated, gaps, validation issues), then report coverage + gaps to the user.

Full step-by-step detail — grep recipes, function/class + code-map templates, the report skeleton, key rules, worked examples, and the troubleshooting table — lives in **[references/default-mode.md](references/default-mode.md)** (moved there in the generic-craft trim). Read it when you need the exact shapes; otherwise just do the three steps.

## Output Specification

- **Path:** write the default report to the artifact directory `.agents/doc/`; OSS scaffold mode creates missing repository docs only by default. The separate OSS `refresh` path may update an existing doc only after explicit user confirmation.
- **Filename:** reports use the filename convention `YYYY-MM-DD-<target>.md`; repository documentation keeps its canonical filename.
- **Format:** outputs are Markdown; the report schema records coverage percentage, generated artifacts, gaps, and validation issues.
- **Validation command:** validate the skill contract with `bash skills-codex/doc/scripts/validate.sh`, then run the relevant repository documentation checks before completion.
- **Downstream handoff:** return changed paths, validation results, coverage or remaining gaps, and blocked decisions; these results are consumed by the requesting workflow and verification membrane.

## Quality Checklist

- Every factual claim cites inspected code, configuration, or existing documentation.
- Generated prose preserves useful project-specific detail and avoids unsupported boilerplate.
- Completion reports name the validators run and disclose unresolved gaps.

## Reference Documents

- [references/default-mode.md](references/default-mode.md) — default mode (code/API docs): the full Steps 1-7 detail — grep recipes, function/class + code-map templates, report skeleton, worked examples, troubleshooting
- [references/generation-templates.md](references/generation-templates.md)
- [references/prose-and-report-workmanship.md](references/prose-and-report-workmanship.md)
- [references/project-types.md](references/project-types.md)
- [references/validation-rules.md](references/validation-rules.md)
- [references/de-slopify.md](references/de-slopify.md) — Remove AI writing artifacts from docs
- [references/architecture-report.md](references/architecture-report.md) — Generate technical architecture documents

## Examples

```bash
$doc                    # default: docs for the changed surface (references/default-mode.md)
$doc --mode=readme      # gold-standard README, council-validated
$doc --mode=oss         # full OSS doc pack
```

## Troubleshooting

| Problem | Fix |
|---------|-----|
| Default mode feels heavyweight | Read references/default-mode.md — or ask the model directly for simple docs |
| README mode verdict fails | Re-run with the council findings addressed (see the readme-mode references above) |
