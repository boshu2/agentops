# JSM Skill Standards Snapshot

This page records pattern-only observations from the user-local JSM skill corpus inspected on 2026-05-16. It is a structural reverse-engineering note, not a content import: do not copy proprietary skill body text, examples, prompts, reference prose, or scripts into AgentOps.

## Snapshot

Local commands showed:

| Measure | Value |
|---|---:|
| Installed skill packages | 118 |
| `jsm verify --json` verified | 118 |
| `jsm verify --json` failures | 0 |
| Total files under skills root | 3,377 |
| Skill packages with `references/` | 111 |
| Skill packages with `scripts/` | 47 |
| Skill packages with `assets/` | 28 |
| Skill packages with `subagents/` | 22 |
| Skill packages with `SELF-TEST.md` | 35 |
| `references/` files | 1,951 |
| `scripts/` files | 613 |
| `assets/` files | 242 |
| `subagents/` files | 333 |
| Executable files under `*/scripts/*` | 0 |
| `jsm validate` passing packages | 98 |
| `jsm validate` failing packages | 20 |
| Validate failures caused by `>50` package files | 20 |

`jsm list --workspace global --json` reported 118 installed skills, no pinned skills, and no pending updates at the time of inspection.

## Product Shape

The corpus has two distinct shapes.

| Shape | What it optimizes for | AgentOps implication |
|---|---|---|
| Package-clean skill | `jsm validate` passes, usually under 50 files, compact `SKILL.md`, references loaded on demand | Use as the default publishable marketplace target. |
| Mega skill | 50-202 files, many references, scripts, assets, and subagent packets | Treat as a product bundle or split into smaller publishable skills before JSM publishing. |

The important correction: JSM installation and verification can succeed for mega skills that fail the simple package validator. AgentOps should not confuse "installed and verified" with "ready for marketplace push."

## Structural Pattern

High-quality JSM skills separate concerns aggressively:

- `SKILL.md` is the operational router: trigger fit, triage, workflow phases, tool choice, stop conditions, and pointers to deeper material.
- `references/` holds the detailed playbooks, checklists, matrices, command notes, failure modes, schemas, and evidence requirements.
- `scripts/` holds repeatable helpers for scan, doctor, verify, score, scaffold, transform, or report generation. In installed JSM packages these script files are not executable.
- `assets/` holds templates, manifests, report skeletons, examples, schemas, or other payloads that should not live in the main prompt body.
- `subagents/` holds role packets for large workflows that need delegated reviewers, implementers, auditors, or domain specialists.
- `SELF-TEST.md` is used as a trigger and behavior harness for many stronger skills.

The repeated design move is context budgeting: the always-loaded description is tiny, `SKILL.md` is a kernel, and everything expensive is loaded only when the current task proves it needs that context.

## Packaging Rules for AgentOps

For AgentOps skills intended for JSM-style distribution:

1. Keep `SKILL.md` as a kernel, not a book.
2. Move long domain context to explicitly linked `references/*.md` files.
3. Move helper logic over roughly 20-30 shell lines into `scripts/`.
4. Add `SELF-TEST.md` for user-facing execution, judgment, and product skills.
5. Keep package-clean skills at or under 50 files when they are meant to pass `jsm validate`.
6. For larger skills, either split the package or mark it as a mega-skill distribution profile.
7. Normalize exported `scripts/` files to non-executable mode before `jsm validate` or `jsm push`.
8. Review all `jsm validate` secret warnings even when they are examples or placeholder strings.
9. Keep copied assets and references real files. Do not use symlinks.
10. Preserve AgentOps attribution rules: pattern-only absorption and explicit source footers where applicable.

## Quality Gates

Before publishing an AgentOps skill through a JSM-style lane, run:

```bash
scripts/check-jsm-export.sh --json skills/<name>
jsm verify --json
```

For repo-native AgentOps skills, also run the existing local gates:

```bash
bash skills/heal-skill/scripts/heal.sh --strict
bash scripts/validate-skill-frontmatter.sh --strict
bash tests/docs/validate-skill-count.sh
```

If the same skill must satisfy both AgentOps repo gates and JSM publishing gates, use an export step that copies the skill into a temporary package directory and normalizes file modes there. Do not weaken repo-runtime checks just to satisfy an external package validator.

Supporting references:

- [JSM CLI Capability Map](jsm-cli-capability-map.md)
- [JSM Clean-Room Extraction Policy](jsm-clean-room-extraction-policy.md)
- [Skill Quality Rubric](skill-quality-rubric.md)
- [JSM-Informed AgentOps Gap Audit](jsm-agentops-gap-audit.md)
- [JSM-Informed Pilot Upgrade Backlog](jsm-pilot-upgrade-backlog.md)

## Adoption Plan

Use this standard to raise AgentOps skills in four passes:

1. Add `SELF-TEST.md` to high-value user-facing skills.
2. Add an export validator that runs `jsm validate` against a temporary package copy with non-executable scripts.
3. Split or profile any skill package that crosses the 50-file JSM validator limit.
4. Introduce optional `assets/` and `subagents/` conventions for product-grade skills, while keeping normal AgentOps skills compact.

---

**Source:** Pattern-only inspection of the user-local JSM skill corpus on 2026-05-16. No proprietary source text copied.
