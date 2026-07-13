# Docs-build known warnings (dispositions)

`scripts/docs-build.sh --check` runs `mkdocs build --strict` end-to-end and then
reinterprets the result against an allowlist of **dispositioned** warnings. The
check is CLEAN when the only strict warnings are enumerated, intentional
cross-references; **any warning not on the allowlist fails the build**, so a new
or accidental broken link still breaks `--check`. Nothing is globally suppressed
(no `--no-strict`) — this is a baseline, not a blanket waiver.

- **Allowlist (source of truth):** `tests/docs/mkdocs-strict-allowlist.txt` — the
  verbatim `WARNING -  …` lines that are tolerated (whole-line, exact match).
- **Runner:** `scripts/docs-build.sh --check` (also wired as pre-push gate step
  25a — advisory in the fast cockpit lane, blocking in CI / the bushido refinery).
- **Complementary checker:** `tests/docs/validate-links.sh` +
  `tests/docs/broken-links-allowlist.txt` validate on-disk link targets across
  `docs/`, `skills/`, `skills-codex/`, `cli/`; `--check` is the end-to-end
  MkDocs-render complement.

## Why these warnings exist

The AgentOps knowledge graph deliberately links from `docs/` narrative pages out
to the **real repo artifacts** they describe — skills, scripts, schemas, eval
data, and the canonical repo-root docs. Those artifacts are intentionally **not**
part of the published MkDocs site (`docs_dir: docs`), so MkDocs cannot resolve the
link and emits a strict `links.not_found` WARNING. The link is correct for a repo
or GitHub reader; it is simply outside the site boundary. Re-pointing such a link
at an in-site generated stub (e.g. a `gen-files` skill page) would break the
GitHub reader, so the deliberate choice is to keep the repo-true link and
disposition the warning.

## Dispositioned classes (the 82 tolerated warnings)

All 82 allowlisted warnings were verified to resolve to a **real repo file on
disk**; none is an accidental typo. They group as:

| Class | Target (outside `docs/`) | Example referrer | Rationale |
|-------|--------------------------|------------------|-----------|
| Skill cross-refs | `../skills/<name>/SKILL.md`, `../../skills/<name>/SKILL.md` | `GLOSSARY.md`, `SKILLS.md`, `context-lifecycle.md`, `contracts/pawls.md`, `architecture/*` | Points at the canonical skill source (SSOT). The in-site skill pages are generated stubs; the SKILL.md is the real target for repo readers. |
| Scripts | `../../scripts/*.sh` (`pawl-review.sh`, `pawl-verdict.sh`, `reconcile-pr.sh`, `evolve/halt-check.sh`) | `contracts/pawls.md`, `doctrine/operating-discipline.md` | Executable source; not site content. |
| Schemas | `../../schemas/*.json` (`learning.v1`, `pawl-verdict.v1`) | `contracts/corpus-learning-seam.md`, `doctrine/operating-discipline.md` | Declared contracts; live under `schemas/`, not the site. |
| Eval artifacts | `../../evals/membrane/**` (json/jsonl/sh/py scorecards + derived checks) | `evals/harvest-*.md`, `evals/local-membrane-vs-codex-*.md`, `evals/membrane-escape-harvest-no-escape.md` | Raw eval evidence kept as repo artifacts, not re-hosted in the site. |
| Repo-root docs | `../../GOALS.md`, `../../PRODUCT.md`, `../../AGENTS-{WORKFLOW,CI,CODEX,RUNTIME}.md`, `../../README.md#…`, `../../cli/docs/COMMANDS.md`, `../../skills/SKILL-TIERS.md`, `../../.claude/workflows/operating-loop.js` | `architecture/codebase-overview.md`, `architecture/control-loop-model.md`, `architecture/workflow-conformance-pattern.md`, `doctrine/lineage-and-theory.md`, `comparisons/vs-hosted-code-review.md` | Canonical repo-root files that are the source of truth; the site links to them rather than duplicating. |
| Skill-relative schema | `schemas/audit-report.json` in the generated `skills/heal-skill.md` page | (generated) | The link resolves in the skill's own tree (`skills/heal-skill/schemas/audit-report.json`); the schema subfolder is not copied into the generated page. Source is `skills/**` (out of scope to edit). |

## Non-fatal INFO notices (not counted by strict, not allowlisted)

MkDocs also logs INFO-level notices that do **not** abort a strict build and so
need no allowlist entry. They are recorded here for completeness:

- **Excluded templates (13):** links from `documentation-index.md`,
  `architecture/intent-to-loop-hexagon.md`, and `architecture/operating-loop.md`
  to `templates/*.md`. Templates are intentionally excluded from the built site
  but referenced for repo readers. Intentional.
- **`../../skills/domain/references/` (3):** directory links from
  `architecture/canonical-loop-model.md` (×2) and `architecture/operating-loop.md`
  to a skill reference directory outside the site. No in-site target; intentional.
- **`acceptance-tests/` (2):** from
  `plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests.md`
  and `plans/bdd-foundry/canonicalize-bdd-foundry-workflow/spec.md`. These point at
  a real **directory of test files** (`.bats`, `.sh`, `.go.txt`), not the sibling
  `acceptance-tests.md` doc — MkDocs' "did you mean `.md`" suggestion would
  misdirect the link, so it is left as-is. Intentional.
- **`skills/discovery.md#open-ended-path-…` (1):** a same-page anchor on the
  generated discovery skill page. The source is `skills/discovery/SKILL.md`
  (`skills/**`, out of scope for docs-only work); the anchor resolves in the
  rendered page. Left for the skill owner.

## Maintenance

- When a doc link breaks: **fix the link first.** Only add a line to
  `tests/docs/mkdocs-strict-allowlist.txt` when the target is a real repo artifact
  that is deliberately outside the docs site and there is no in-site equivalent.
- After an intentional doc move, run `scripts/docs-build.sh --check`; it prints any
  un-dispositioned warning verbatim so the allowlist line can be updated.
- Keep the allowlist entries as the exact `WARNING -  …` lines MkDocs emits.
