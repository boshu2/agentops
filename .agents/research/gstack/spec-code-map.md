# Code Map Spec: gstack

- Date: 2026-05-01
- Source: code-proven from `/home/boful/dev/personal/gstack` (HEAD `6e1625c0`, v1.25.0.0)

## SaaS Boundary (Explicit)

| Lane | Local? | Hosted dependency |
|---|---|---|
| Skill invocation (`/<name>`) | local | host harness (Claude/Codex/Cursor/etc.) |
| Helper toolbelt (`bin/gstack-*`) | local | none (a few hit network — see below) |
| `browse` binary | local | none (uses local Chromium via CDP) |
| `make-pdf` | local | none |
| `/codex` skill | mixed | OpenAI Codex API |
| `/cso`, `/review`, `/qa` evals | mixed | Anthropic API (`ANTHROPIC_API_KEY`) |
| `gstack-brain-*` (gbrain sync) | mixed | Supabase (gbrain backend) |
| `/ship`, `/land-and-deploy`, `/canary` | mixed | GitHub (`gh` CLI), CI provider, deploy target |
| `/context-save`, `/context-restore` | local | optional Conductor workspace API |

## Packages / Components

| Component | Location | Role | Evidence |
|---|---|---|---|
| Skill bundle | `<skill>/` (45 dirs) | Slash-command-invokable agent role | `*/SKILL.md`, `*/SKILL.md.tmpl` |
| Skill template engine | `scripts/gen-skill-docs.ts` | Renders `SKILL.md` from `.tmpl` per host | `bun run gen:skill-docs` |
| Helper toolbelt | `bin/` (55 entries) | Shell/TS helpers skills shell out to | `bin/gstack-*`, `bin/dev-{setup,teardown}`, `bin/chrome-cdp` |
| State resolver | `bin/gstack-paths` | Canonical resolver for `GSTACK_HOME` / plugin paths | sourced via `eval "$(...)"` |
| Browse binary | `browse/src/`, `browse/dist/browse` | Headless Chromium wrapper, Bun-compiled | `package.json` bin entry, `browse/SKILL.md` |
| Claude bin resolver | `browse/src/claude-bin.ts` | `Bun.which()` + `GSTACK_CLAUDE_BIN` override | code reference in `AGENTS.md` |
| make-pdf binary | `make-pdf/`, `make-pdf/dist/pdf` | Markdown → PDF | `package.json` bin entry, `make-pdf/SKILL.md` |
| Host adapters | `hosts/*.ts` (11) | Per-host install + render logic | `hosts/{claude,codex,cursor,gbrain,hermes,kiro,openclaw,opencode,slate,factory,index}.ts` |
| Open-source `openclaw` adapter | `scripts/host-adapters/openclaw-adapter.ts` | OpenClaw-specific hooks | TS file |
| Setup installer | `./setup` (40 KB shell) | Idempotent installer; detects host, copies skills | invoked as `bun run setup` |
| Eval CLI suite | `scripts/eval-{compare,list,select,summary,watch}.ts` | LLM-as-judge eval lifecycle | `bun run eval:*` |
| Test taxonomy | `scripts/test-free-shards.ts`, package `test:*` scripts | Tiered tests by API spend | `bun run test:{free,e2e,audit,gate,evals,gemini,codex,periodic,windows}` |
| Slop-scan | `scripts/slop-diff.ts`, `slop-scan.config.json` | Content lint | `bun run slop`, `bun run slop:diff` |
| Skill health dashboard | `scripts/skill-check.ts` | Health metrics for all skills | `bun run skill:check` |
| App / runtime | `scripts/app/`, `scripts/resolvers/` | Server / runtime orchestration | `bun run server`, `bun run start` |
| Question registry | `scripts/question-registry.ts` | AskUserQuestion tuning data | feeds `/plan-tune` |
| One-way doors | `scripts/one-way-doors.ts` | Decision-archetype helper | called from review skills |
| Archetypes / models | `scripts/archetypes.ts`, `scripts/models.ts` | Persona + model catalogs | TS modules |
| Top-level docs | `AGENTS.md`, `ARCHITECTURE.md`, `BROWSER.md`, `CHANGELOG.md`, `CLAUDE.md`, `CONTRIBUTING.md`, `DESIGN.md`, `ETHOS.md`, `README.md`, `SKILL.md`, `TODOS.md`, `USING_GBRAIN_WITH_GSTACK.md` | Authored knowledge surface | repo root |

## Feature-to-Code Anchors

This section aligns with `feature-inventory.md` (45-skill product surface).

### Plan-mode reviews
| Feature | Anchor | Helpers shelled out |
|---|---|---|
| `/office-hours` | `office-hours/SKILL.md` (+`.tmpl`) | `gstack-question-log`, `gstack-paths` |
| `/plan-ceo-review` | `plan-ceo-review/SKILL.md` | (review-log family) |
| `/plan-eng-review` | `plan-eng-review/SKILL.md` | — |
| `/plan-design-review` | `plan-design-review/SKILL.md` | — |
| `/plan-devex-review` | `plan-devex-review/SKILL.md` | — |
| `/plan-tune` | `plan-tune/SKILL.md` | `scripts/question-registry.ts` |
| `/autoplan` | `autoplan/SKILL.md` | sequences the four `/plan-*` skills |
| `/design-consultation` | `design-consultation/SKILL.md` | — |

### Implementation + review
| Feature | Anchor | Helpers shelled out |
|---|---|---|
| `/review` | `review/SKILL.md` | `gstack-review-log`, `gstack-review-read` |
| `/codex` | `codex/SKILL.md` (+`.tmpl`); `scripts/preflight-agent-sdk.ts` | OpenAI Codex API |
| `/investigate` | `investigate/SKILL.md` | — |
| `/design-review` | `design-review/SKILL.md` | `browse` binary, `chrome-cdp` |
| `/design-shotgun` | `design-shotgun/SKILL.md` | `browse`, image diff |
| `/design-html` | `design-html/SKILL.md` | — |
| `/devex-review` | `devex-review/SKILL.md` | `browse`, telemetry |
| `/qa` | `qa/SKILL.md` | `browse`, `chrome-cdp` |
| `/qa-only` | `qa-only/SKILL.md` | same as `/qa`, no edits |
| `/scrape` | `scrape/SKILL.md` | `browse`, `browser-skills/` |
| `/skillify` | `skillify/SKILL.md` | promotes scrape session into `browser-skills/` |

### Release + deploy
| Feature | Anchor | Helpers shelled out |
|---|---|---|
| `/ship` | `ship/SKILL.md` | `gstack-next-version`, `gstack-pr-title-rewrite.sh`, `gh` CLI |
| `/land-and-deploy` | `land-and-deploy/SKILL.md` | `gh`, deploy target detection |
| `/canary` | `canary/SKILL.md` | `browse` (post-deploy probe) |
| `/landing-report` | `landing-report/SKILL.md` | reads ship-queue state |
| `/document-release` | `document-release/SKILL.md` | — |
| `/setup-deploy` | `setup-deploy/SKILL.md` | detection only |
| `/gstack-upgrade` | `gstack-upgrade/SKILL.md` | `gstack-update-check`, `gstack-relink` |

### Operational + memory
| Feature | Anchor | Helpers shelled out |
|---|---|---|
| `/context-save` | `context-save/SKILL.md` | `gstack-timeline-log` |
| `/context-restore` | `context-restore/SKILL.md` | `gstack-timeline-read`, Conductor (optional) |
| `/learn` | `learn/SKILL.md` | `gstack-learnings-log`, `gstack-learnings-search` |
| `/retro` | `retro/SKILL.md` | timeline + telemetry reads |
| `/health` | `health/SKILL.md` | `scripts/skill-check.ts` |
| `/benchmark` | `benchmark/SKILL.md` | `browse` (Core Web Vitals) |
| `/benchmark-models` | `benchmark-models/SKILL.md`; `scripts/eval-*.ts`; `bin/gstack-model-benchmark` | Anthropic/Codex/Gemini APIs |
| `/cso` | `cso/SKILL.md` | `bin/gstack-security-dashboard` |
| `/setup-gbrain` | `setup-gbrain/SKILL.md`; `bin/gstack-gbrain-*` (5+lib) | Supabase |

### Browser + agent integration
| Feature | Anchor | Helpers |
|---|---|---|
| `/browse` | `browse/SKILL.md`; `browse/src/`; `browse/dist/browse` | local Chromium via CDP |
| `/open-gstack-browser` | `open-gstack-browser/SKILL.md`; `connect-chrome` symlink | `chrome-cdp`, `extension/`, sidebar UI |
| `/setup-browser-cookies` | `setup-browser-cookies/SKILL.md` | reads from real-browser profile |
| `/pair-agent` | `pair-agent/SKILL.md` | OpenClaw / remote-agent bridge |

### Safety + scoping
| Feature | Anchor | Mechanism |
|---|---|---|
| `/careful` | `careful/SKILL.md` | inline advisory prose, advisory only |
| `/freeze` | `freeze/SKILL.md` | hard block (not advisory) |
| `/guard` | `guard/SKILL.md` | careful + freeze together |
| `/unfreeze` | `unfreeze/SKILL.md` | reverses freeze |
| `/make-pdf` | `make-pdf/SKILL.md`; `make-pdf/dist/pdf` | publication-quality PDF generator |

## Notes

- "Docs say" surface (`docs-features.txt`) is intentionally separate from this map. Most `docs/designs/*` entries are design notes, not yet shipped features (e.g., `BUN_NATIVE_INFERENCE`, `ML_PROMPT_INJECTION_KILLER`, `SELF_LEARNING_V0`).
- "Skills as code" pattern means every prompt change is a code change; treat `<skill>/SKILL.md.tmpl` as authored source and the rendered `SKILL.md` as build output.
- `bin/gstack-paths` is the single point of state-path resolution; any clone effort that re-implements state paths must mirror its env-var precedence (`CLAUDE_PLUGIN_DATA` > `GSTACK_HOME` > default).
- `browse/src/claude-bin.ts` is the cross-platform `claude` resolver — required reading for any host that wraps `claude` (e.g., WSL routing).
