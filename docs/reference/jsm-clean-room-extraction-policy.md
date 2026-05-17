# JSM Clean-Room Extraction Policy

This policy governs all AgentOps work that learns from the locally installed JSM skill corpus or the `jsm` CLI. The goal is to extract transferable engineering patterns while keeping proprietary JSM content out of AgentOps.

## Principle

AgentOps may learn from structure, metadata, validation behavior, and repeated patterns. AgentOps must not copy JSM skill content.

## Allowed Observations

These are safe to record in AgentOps-owned artifacts:

| Category | Examples |
|---|---|
| Counts | number of skills, files, references, scripts, assets, subagents, self-tests |
| Paths | local package paths and repo-owned artifact paths |
| Filenames | `SKILL.md`, `SELF-TEST.md`, `references/`, `scripts/`, `assets/`, `subagents/` |
| Metadata | skill names, versions, hashes, license flags, compatibility tags, install/verify status |
| CLI behavior | command names, flags, JSON shapes, success/failure classes |
| Package shape | file-count bands, directory conventions, script-mode conventions |
| Validation outcomes | `jsm validate` pass/fail class, warnings, file-count limit classification |
| Derived categories | "package-clean", "mega skill", "kernel", "reference map", "export profile" |
| Aggregate examples | anonymized or path-only examples that do not copy content |

## Disallowed Material

Do not copy or paraphrase proprietary skill content into AgentOps artifacts:

- skill body prose
- prompt text
- examples written inside JSM packages
- reference document prose
- script bodies or algorithms
- templates or assets
- subagent role packet text
- long descriptions beyond what is needed for metadata identification
- any passage that preserves distinctive wording from the source

The working rule is stricter than ordinary citation: avoid copying more than five consecutive words from an external skill corpus unless the text is a generic filename, command, or identifier.

## Required Attribution

When a derived AgentOps artifact was informed by JSM, include one of the attribution patterns in `skills/standards/references/jsm-attribution.md`.

For docs under `docs/reference/`, use a footer:

```markdown
---

**Source:** Pattern-only inspection of the user-local JSM corpus. No proprietary source text copied.
```

For skill references, use the per-reference footer pattern from the attribution standard.

## Safe Workflow

1. Work from backups or read-only local installs.
2. Prefer `jsm list`, `jsm verify`, `jsm graph`, `jsm related`, `jsm validate`, and filesystem inventory.
3. Record counts, paths, command output classes, and derived rules.
4. Before committing, scan new artifacts for copied JSM prose or script fragments.
5. Keep implementation scripts generic and AgentOps-owned.

## Mutating Commands

These commands are out of scope unless the operator explicitly requests them:

- `jsm install`
- `jsm install-all`
- `jsm sync`
- `jsm push`
- `jsm upgrade`
- `jsm rollback`
- `jsm pin`
- `jsm unpin`
- `jsm effectiveness record`
- `jsm cass mark`
- `jsm cass unmark`
- `jsm cass mine` without `--dry-run`

## Review Checklist

Before landing any JSM-informed artifact:

- [ ] The artifact contains only allowed observations or AgentOps-authored guidance.
- [ ] It does not copy JSM prose, prompts, examples, references, scripts, templates, or role text.
- [ ] It distinguishes facts from derived recommendations.
- [ ] It cites the inspection date and source corpus at a pattern level.
- [ ] It does not expose auth, session, history, cache, or telemetry data.
- [ ] It does not require mutating JSM commands to reproduce.
- [ ] It names validation commands and expected pass/fail classes.

## Handling Borderline Cases

If a lesson depends on specific JSM wording, do not include it. Restate the underlying engineering rule from first principles, or omit the lesson.

If a script behavior is useful, describe the input/output contract and write new AgentOps-owned code. Do not translate the original implementation line by line.

If a package contains proprietary templates or assets, record only that an asset category exists and what role that category plays.

---

**Source:** Pattern-only inspection of the user-local JSM corpus. No proprietary source text copied.
