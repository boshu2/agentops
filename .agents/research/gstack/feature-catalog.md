# Feature Catalog: gstack

- Generated: 2026-05-01
- Source: code-proven from `/home/boful/dev/personal/gstack` (HEAD `6e1625c0`, v1.25.0.0)
- Total: 45 skills + 55 helper scripts + 2 compiled binaries + 1 setup installer + 11 host adapters

> The auto-generated catalog (overwritten here) only captured 9 docs-derived "control-plane" groups. Real product surface is broken down by lane below.

## Skills by Lane

### Plan-mode reviews (8)

| Slug | impl | anchor |
|---|---|---|
| `office-hours` | `local-skill` | `office-hours/SKILL.md` |
| `plan-ceo-review` | `local-skill` | `plan-ceo-review/SKILL.md` |
| `plan-eng-review` | `local-skill` | `plan-eng-review/SKILL.md` |
| `plan-design-review` | `local-skill` | `plan-design-review/SKILL.md` |
| `plan-devex-review` | `local-skill` | `plan-devex-review/SKILL.md` |
| `plan-tune` | `local-skill` | `plan-tune/SKILL.md` + `scripts/question-registry.ts` |
| `autoplan` | `local-skill` | `autoplan/SKILL.md` |
| `design-consultation` | `local-skill` | `design-consultation/SKILL.md` |

### Implementation + review (10)

| Slug | impl | anchor |
|---|---|---|
| `review` | `local-skill` | `review/SKILL.md` |
| `codex` | `mixed (Codex API)` | `codex/SKILL.md` (+`.tmpl`) |
| `investigate` | `local-skill` | `investigate/SKILL.md` |
| `design-review` | `local-skill+browse` | `design-review/SKILL.md`, `browse/dist/browse` |
| `design-shotgun` | `local-skill+browse` | `design-shotgun/SKILL.md` |
| `design-html` | `local-skill` | `design-html/SKILL.md` |
| `devex-review` | `local-skill+browse` | `devex-review/SKILL.md` |
| `qa` | `local-skill+browse` | `qa/SKILL.md`, `chrome-cdp` |
| `qa-only` | `local-skill+browse` | `qa-only/SKILL.md` |
| `scrape` | `local-skill+browse` | `scrape/SKILL.md`, `browser-skills/` |
| `skillify` | `local-skill` | `skillify/SKILL.md` |

### Release + deploy (7)

| Slug | impl | anchor |
|---|---|---|
| `ship` | `mixed (gh CLI)` | `ship/SKILL.md`, `bin/gstack-{next-version,pr-title-rewrite.sh}` |
| `land-and-deploy` | `mixed (gh CLI)` | `land-and-deploy/SKILL.md` |
| `canary` | `mixed (gh CLI + browse)` | `canary/SKILL.md` |
| `landing-report` | `local-skill` | `landing-report/SKILL.md` |
| `document-release` | `local-skill` | `document-release/SKILL.md` |
| `setup-deploy` | `local-skill` | `setup-deploy/SKILL.md` |
| `gstack-upgrade` | `local-skill` | `gstack-upgrade/SKILL.md`, `bin/gstack-{update-check,relink}` |

### Operational + memory (8)

| Slug | impl | anchor |
|---|---|---|
| `context-save` | `local-skill` | `context-save/SKILL.md`, `bin/gstack-timeline-log` |
| `context-restore` | `mixed (Conductor opt.)` | `context-restore/SKILL.md`, `bin/gstack-timeline-read` |
| `learn` | `local-skill` | `learn/SKILL.md`, `bin/gstack-learnings-{log,search}` |
| `retro` | `local-skill` | `retro/SKILL.md` |
| `health` | `local-skill` | `health/SKILL.md`, `scripts/skill-check.ts` |
| `benchmark` | `local-skill+browse` | `benchmark/SKILL.md` |
| `benchmark-models` | `mixed (multi-API)` | `benchmark-models/SKILL.md`, `scripts/eval-*.ts`, `bin/gstack-model-benchmark` |
| `cso` | `local-skill` | `cso/SKILL.md`, `bin/gstack-security-dashboard` |
| `setup-gbrain` | `mixed (Supabase)` | `setup-gbrain/SKILL.md`, `bin/gstack-gbrain-*` (5+lib) |

### Browser + agent integration (4)

| Slug | impl | anchor |
|---|---|---|
| `browse` | `compiled-binary` | `browse/SKILL.md`, `browse/src/`, `browse/dist/browse` |
| `open-gstack-browser` | `local-skill+chromium` | `open-gstack-browser/SKILL.md`, `extension/`, `connect-chrome` symlink |
| `setup-browser-cookies` | `local-skill` | `setup-browser-cookies/SKILL.md` |
| `pair-agent` | `local-skill` | `pair-agent/SKILL.md` |

### Safety + scoping (5)

| Slug | impl | anchor |
|---|---|---|
| `careful` | `local-skill (advisory)` | `careful/SKILL.md` |
| `freeze` | `local-skill (hard block)` | `freeze/SKILL.md` |
| `guard` | `local-skill` | `guard/SKILL.md` (careful + freeze) |
| `unfreeze` | `local-skill` | `unfreeze/SKILL.md` |
| `make-pdf` | `compiled-binary` | `make-pdf/SKILL.md`, `make-pdf/dist/pdf` |

### Root meta (2)

| Slug | impl | anchor |
|---|---|---|
| `gstack` | `umbrella-skill` | `SKILL.md` (root, name=gstack, version 1.1.0) |
| `codex/SKILL.md` | `host-variant` | `codex/SKILL.md` (Codex-specific render) |

## Helper Scripts by Family (`bin/`)

| Family | Count | Anchors |
|---|---:|---|
| `gstack-brain-*` | 9 | `bin/gstack-brain-{init,enqueue,consumer,reader→consumer,sync,restore,uninstall}` |
| `gstack-gbrain-*` | 5+lib | `bin/gstack-gbrain-{detect,install,repo-policy,source-wireup,supabase-provision,supabase-verify}` + `gstack-gbrain-lib.sh` |
| `gstack-{telemetry,timeline,review,question,learnings}-{log,read,search,sync,preference}` | 10 | logging/reading utilities |
| `gstack-{config,paths,relink,extension,uninstall,update-check,next-version,repo-mode,settings-hook}` | 9 | repo state + lifecycle |
| `gstack-{platform-detect,pr-title-rewrite.sh,patch-names,open-url,slug,jsonl-merge,diff-scope,global-discover.ts}` | 8 | cross-platform + utility |
| `gstack-{analytics,community-dashboard,security-dashboard,model-benchmark,specialist-stats,builder-profile,developer-profile,taste-update,team-init,session-update,codex-probe}` | 11 | profiles + dashboards |
| Standalone | 4 | `chrome-cdp`, `dev-setup`, `dev-teardown`, `make-pdf` |

## Host Adapters (`hosts/*.ts`)

| File | Host |
|---|---|
| `claude.ts` | Claude Code |
| `codex.ts` | Codex CLI |
| `cursor.ts` | Cursor IDE |
| `gbrain.ts` | gbrain (cross-machine memory) |
| `hermes.ts` | Hermes |
| `kiro.ts` | Kiro |
| `openclaw.ts` | OpenClaw |
| `opencode.ts` | Opencode |
| `slate.ts` | Slate |
| `factory.ts` | Detector + selector |
| `index.ts` | Re-export surface |

## Aggregate Counts

| Surface | Count |
|---|---:|
| Skills (top-level dirs with SKILL.md) | 45 |
| Helper scripts (`bin/`) | 55 |
| Compiled binaries (`package.json` `bin`) | 2 |
| Host adapters | 11 |
| Top-level docs (md files at root) | 12 |
| Build/dev scripts (`bun run *`) | 25+ |
