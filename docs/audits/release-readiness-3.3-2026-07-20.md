# Release-readiness audit — 3.3.0 "Cathedral Cut" (2026-07-20)

> **Fix status:** B1 and M1–M13 (the blocker and all majors) were resolved the
> same day on branch `claude/release-wrapper-fixes-33`. M7 resolved per the
> product decision that npx (universal) leads and Claude/Codex plugins are
> encouraged, with checkout + `ao skills link` as the contributor path. Minors
> 1–9 below remain open except where a major's fix subsumed them (minor 9 via
> M10). The findings below describe the pre-fix state.

**Scope:** skills + CLI + `.agents/` integration, the 3.2 → 3.3 migration story, and the
new-user experience, audited at origin/main tip `735580d1c` (worktree
`llm-wiki-integration-research-44c35a`). Method: deterministic checks run inline
(build, vet, full test suite, `ao gate check --full`, every tombstone exercised
against the built binary), then a 10-agent workflow — five dimension auditors
(new-user, 3.2-migration, skills-corpus, docs-site, `.agents` runtime), each
finding adversarially re-verified by an independent skeptic. 27 findings kept,
0 refuted.

## Verdict

**The product core is genuinely release-ready; the release *wrapper* is not.**

The Cathedral Cut itself holds together: the loop doctrine is told identically
by README, PRODUCT.md, AGENTS.md, the four core skills, and the binary; every
removed 3.2 verb (including subcommands like `skills edit`, `goals trace`,
`session memory`) returns exit 1 with a correct, specific replacement pointer;
the legacy `~/.agentops` config fallback works exactly as documented (verified
behaviorally); all six curl installers are proper refusing tombstones that print
the new path; plugin manifests carry all 48 skills with zero parity drift; no
live skill and no current narrative doc instructs a removed surface; and the
deterministic floor is fully green — build + vet clean, **all 60 Go test
packages pass**, **`ao gate check --full`: 67/67 pass, 0 warnings**.

What is not ready is everything a stranger touches *around* that core: the
published docs site, the release notes, and three self-documentation surfaces
of the CLI. One blocker, twelve majors, eight minors — almost all cheap to fix,
but several sit on the exact first-five-minutes path.

## Blocker

| # | Where | Defect |
|---|---|---|
| B1 | [docs/_hooks/gen_cli_reference.py:61-78](../_hooks/gen_cli_reference.py) | The generated docs-site **CLI Overview page** (`/cli/`) tells a new user to install via `bash <(curl …/scripts/install.sh)` — a tombstone that exits 2 — then to run `ao rpi phased "fix the flaky auth test"` — an unknown command — and claims skills live in `~/.claude/skills/`. The published first-run page fails at both steps and misstates where skills live. Verified in the built `_site/cli/index.html`. |

## Major findings

**CLI self-documentation contradicts the binary**

| # | Where | Defect |
|---|---|---|
| M1 | `cli/internal/commands/robotdocs/module.go:107` | `ao robot-docs` — the handbook `ao --help` tells every agent to run first — prescribes `ao inject "<topic>"` as step 4 of its canonical workflow. `inject` is gone: unknown command, **no tombstone, no MIGRATION.md row**, breaking `--help`'s "every removed surface has a row" promise. |
| M2 | `cli/internal/commands/config/module.go:274` | `ao config --help` documents ~14 `AGENTOPS_*` env vars for subsystems that no longer exist (RPI runtime/worktree, Dream, Council tiers, flywheel auto-promote). Zero behavioral consumers in the 3.3 binary; `docs/ENV-VARS.md` says the loop needs no env vars. Two authoritative surfaces tell opposite stories. |
| M3 | `cli/internal/flywheelapp/metrics.go:38` | `ao flywheel status` counts knowledge only under legacy `.agents/learnings|patterns|findings`, while `ao doctor fix` declares `.agents/ao/learnings` canonical and **renames files into it** — running the doctor's own fix silently zeroes flywheel metrics. Two shipped surfaces fight over where knowledge lives. |

**Release story contradicts the shipped binary**

| # | Where | Defect |
|---|---|---|
| M4 | `CHANGELOG.md:8-10` | `[3.3.0]` is dated 2026-07-17 with an empty `[Unreleased]`, but ~30 user-visible commits landed after 07-17 — `ao eval` (#921), default-build `ao flywheel`, the PreToolUse policy engine, CLI cleanup #919 — none mentioned anywhere. The release notes lie by omission about what the binary contains. |
| M5 | `docs/MIGRATION.md:43` | "`ao eval` returned in **3.4**" — a release that doesn't exist. eval ships in the 3.3 binary this doc describes; a 3.2 eval user would defer upgrading or migrate off a surface they're about to receive. |
| M6 | `CHANGELOG.md:42` + `docs/3.3.md:39` + `docs/releases/2026-07-17-v3.3.0-notes.md:47,83` + `docs/CHANGELOG.md:42` | Four release surfaces claim a **50**-skill corpus; every projection and the binary agree on **48** (catalog.json, SKILL-ROUTER, SKILL-TIERS, `ao skills check/list`). The flagship "every projection agrees" claim is contradicted by the release notes themselves. |
| M7 | `README.md:26` | The front door says "**Prefer** a managed bundle" (plugin install) while UPGRADING, install-day2-ops, 3.3.md, the CHANGELOG, and the installer tombstones all declare plugins legacy migration-only. The README never presents the canonical checkout + `ao skills link` as *the* install path. New users are pointed at the path every other surface tells them to undo. |

**The published docs site**

| # | Where | Defect |
|---|---|---|
| M8 | `.github/workflows/docs.yml:42` vs `mkdocs.yml:22` | The deploy workflow runs `mkdocs build --strict`; the build aborts with **84 warnings** (config deliberately sets `strict: false`, the flag overrides it). The 3.3 site cannot be published via its own pipeline — so the public site stays frozen on the old deploy. |
| M9 | `docs/overrides/main.html:5` | The site-wide banner on every published page says "**AgentOps 2.x** — the operational layer for coding agents." for the 3.3 release. |
| M10 | `mkdocs.yml:17` | `exclude_docs` covers only `convergence/operationalized/`, so ~176 internal files build into the public site — `audits/` (2.1 MB), `plans/`, `handoffs/`, `council-log/`, `sovereignty-proof/`, a `TEMP-…-HANDOFF` scratch doc — and dominate search (4,751 index entries; audits 1,072 + plans 654 vs ~40 nav pages). Inside-baseball outranks the real docs. |
| M11 | `docs/newcomer-guide.md:18-20` | Getting-Started nav page links premortem/council/postmortem at `../skills/<slug>/SKILL.md` — outside `docs_dir`, all three 404 on the site. |
| M12 | `docs/SCHEMAS.md:7-18` | The Reference→Schemas page is six links to `../schemas/*.schema.json`; all six 404 on the site. The page is a complete dead-end. |

**Skills corpus**

| # | Where | Defect |
|---|---|---|
| M13 | `skills/rch/references/TROUBLESHOOTING.md:287-326`, `MACHINE_INTROSPECTION.md:174,180` | The rch recovery flow instructs **5 scripts and 8 reference files that don't exist** (`skills/rch` has no `scripts/` at all) — imperative self-fix steps, not prose. `ao skills check` reports 0 errors because its link audit doesn't cover reference-to-reference or script paths, so this class ships green. |

## Minor findings

1. `cli/cmd/ao/main.go:8` — fallback version `3.3.0-rc`; the documented `go install` path yields a binary self-reporting a pre-release against 3.3.0 manifests (no `cli/vX` tags exist, so `@latest` always carries the fallback).
2. `cli/internal/config` — during the documented legacy-config fallback, `ao config --show` prints the deprecation warning and then "Home: …/.agents/ao/config.yaml (not found)" with the value misattributed "(from flag)" — the exact state UPGRADING step 2 walks users through looks broken.
3. `cli/internal/initapp/initapp.go:16` — `ao init` creates a layout `ao doctor diff` immediately flags as incomplete (missing `sessions`, `index`) and never creates the documented `intents/sha256` store (reproduced in a fresh dir).
4. `cli/internal/doctor/fix_cliconfig.go:120` — the missing-`br` repair hint says to run `ao beads dir`, an unknown command in 3.3.
5. `.agents/ao/config.yaml` — the only in-tree config example sets `tracker: br` with a comment describing resolution behavior the 3.3 binary no longer has; the key is silently ignored.
6. `skills/doc/references/architecture-report.md:38,144,170` + `validation-rules.md:21` — Quick Start instructs `scaffold-report.py` and `doc-validate.py`, neither shipped.
7. `docs/ROADMAP.md:3,17,21` — dead links to `3.0.md` (retired in the Cut) and `curation-pipeline.md`.
8. `docs/documentation-index.md:8-11` — the README-footer docs index opens with four repo-root links that 404 on the published site.
9. `docs/TEMP-RPI-LOOP-IMPROVEMENT-HANDOFF-2026-07-15.md` — 19.5 KB internal cross-machine scratch context, published and searchable (subsumed by M10, listed for individual deletion).

## The 3.2 user's migration, as it stands

Received today, a 3.2 upgrader gets a **genuinely excellent mechanical
migration** — the best part of the whole release: every dead verb they type
tells them exactly what replaced it, their old config keeps working with a
polite warning, the old installers refuse with the new path printed, and
UPGRADING.md's three actions are accurate and sufficient. That's the 10-star
part, and it's already real.

What breaks the spell is the *story around it*: the migration guide tells them
eval is a future-release feature their own binary already has (M5), the
changelog omits a third of what shipped (M4), the release notes miscount the
corpus they can trivially recount (M6), and if they visit the website it greets
them with a 2.x banner (M9), a first-run page built on dead commands (B1), and
search results full of internal audits (M10). None of it blocks the mechanical
upgrade; all of it erodes the trust the tombstone work earned.

## Suggested fix order (release-cut checklist)

1. **B1** — rewrite `emit_index()` to the real 3.3 install + quickstart. One function.
2. **M9** — banner to 3.3. One line.
3. **M4-M6** — one release-notes pass: fold post-07-17 surfaces into `[3.3.0]`, fix the date, `3.4`→`3.3`, `50`→`48` (×5 sites).
4. **M1, M2** — prune `robot-docs` step 4 and the dead env-var block (+ add an `inject` row or hint).
5. **M10 + M8** — extend `exclude_docs` to the internal dirs and TEMP files; drop `--strict` from docs.yml (or fix the 84 warnings — most vanish once M10-M12 land).
6. **M11, M12** — repoint newcomer-guide and SCHEMAS links at site-servable targets.
7. **M7** — decide the one true install story (checkout+link per every 3.3 doc, or rehabilitate plugins) and make the README lead with it.
8. **M3, M13** — flywheel roots → `.agents/ao/learnings`; prune rch's phantom references (and consider extending `ao skills check` to script/reference-to-reference paths so the class can't ship green again).
9. Minors opportunistically; `main.go` version bump belongs to the tag-cut itself.

## What was checked and found clean (so it isn't re-audited)

Build/vet/full test suite (60 pkgs), `ao gate check --full` (67/67),
all 20 tombstoned verbs + 3 subcommand tombstones, the 6 installer tombstones,
legacy config fallback behavior, plugin manifest ↔ corpus parity (48/48, drift 0),
every README relative link, cli/docs/COMMANDS.md ↔ binary parity (19 commands),
skills-corpus references to removed verbs/skills (zero live instructions),
cross-skill directory references, `.agents/ao` evidence-path agreement across
README/docs/skills/binary, provenance ledger integrity (615 records), and
MIGRATION/UPGRADING/FAQ/GLOSSARY/ARCHITECTURE narrative accuracy.

---
*Produced by a 10-agent audit workflow (5 auditors + 5 adversarial verifiers,
~1.0 M tokens, 356 tool calls) plus inline deterministic checks. Every finding
above was independently re-verified against the tree or the built binary; 0 of
27 findings were refuted.*
