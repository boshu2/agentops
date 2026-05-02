# Clone MVP Spec (Original): gstack-shaped skill bundle

- Date: 2026-05-01
- Goal: A minimal, **original** skill bundle that mirrors gstack's *shape* (one-source-many-hosts, skills-call-helpers, state-via-resolver) without copying its prompts or proprietary surface.

## Goal

Implement a 5-skill bundle that follows the gstack architectural pattern, deployable to at least 2 hosts (Claude Code + one other).

## Non-Goals

- Reconstruct gstack's prompts, eval rubrics, or design system.
- Ship `browse` or `make-pdf` equivalents (out of scope for MVP).
- Cross-machine memory sync (gbrain equivalent).

## MVP Scope (5 skills)

Pick 5 from these archetypes — all are common-domain enough to clone safely without copying gstack content:

1. `office-hours`-shape — discovery / reframing skill
2. `review`-shape — pre-merge review skill
3. `ship`-shape — push + open-PR skill
4. `careful`-shape — destructive-command warning skill
5. `health`-shape — repo health dashboard skill

Each skill:
- Lives in its own top-level dir with `SKILL.md.tmpl` (templated)
- Declares `allowed-tools` in YAML frontmatter
- Shells out to `bin/<helper>` for any logic — no logic in the prompt
- Renders to per-host `SKILL.md` via the template engine

## Architecture

### Layout
```
my-bundle/
├── setup                      # idempotent installer (shell)
├── package.json               # bun runtime
├── bin/
│   ├── mybundle-paths         # state resolver (sources via eval)
│   ├── mybundle-config        # repo config reader
│   └── ...                    # one helper per skill that needs logic
├── hosts/
│   ├── factory.ts             # host detector + adapter selector
│   ├── claude.ts              # Claude Code adapter
│   └── codex.ts               # second host adapter
├── scripts/
│   ├── gen-skill-docs.ts      # template renderer (one .tmpl → N hosts)
│   └── discover-skills.ts     # enumerate skill dirs
├── office-hours/SKILL.md.tmpl
├── review/SKILL.md.tmpl
├── ship/SKILL.md.tmpl
├── careful/SKILL.md.tmpl
└── health/SKILL.md.tmpl
```

### Components

| Component | Responsibility |
|---|---|
| `setup` (shell) | Detect host → invoke matching `hosts/<host>.ts` → install skills + PATH |
| `hosts/factory.ts` | Pure detector: which agent harness is installed? |
| `hosts/<host>.ts` | Knows host's skills dir + per-host SKILL.md format |
| `scripts/gen-skill-docs.ts` | Renders `.tmpl` → per-host `SKILL.md` with host-specific tokens |
| `bin/mybundle-paths` | Single state-path resolver. Honors `MYBUNDLE_HOME` + host-specific overrides |
| `bin/<helper>` | All non-trivial logic lives here. Skills shell out, never embed |

### Interfaces

- **Skill → helper:** standard `bin/<name> [args]` shell-out. JSON on stdout.
- **Helper → state:** every helper sources `eval "$(bin/mybundle-paths)"` first.
- **Adapter → host:** writes SKILL.md files; returns install report (count, skipped, errors).

### Storage

- `${MYBUNDLE_HOME:-~/.mybundle}/state/` — per-skill JSONL logs
- `${MYBUNDLE_HOME}/cache/` — derived data (discovery results, etc.)
- No SaaS dependencies in MVP

## Security Invariants

- Skills run with explicit `allowed-tools`; default to read-only set.
- Helpers refuse to operate above the repo root unless `--global` flag is passed.
- `careful`-shape skill blocks (or warns about) any `rm -rf`, force-push, or DROP TABLE before execution.
- No env keys committed; `.env.example` only.

## Deterministic Test Harness

| Lane | What it checks | API spend |
|---|---|---|
| `bun test` | helper unit tests | none |
| `bun run test:gen-skill-docs` | `.tmpl` → `SKILL.md` golden-fixture diff per host | none |
| `bun run test:install` | dry-run against fake host dirs, assert layout | none |
| `bun run test:e2e` | spawn host harness, invoke a skill, assert side effect | varies |

## Out of Scope (Phase 2+)

- `browse`-shape compiled binary
- Eval / LLM-as-judge lanes
- Cross-machine memory sync (gbrain-shape)
- Conductor workspace integration
- Slop-scan / content lint
- More than 2 hosts

## Validation Criteria

MVP is "done" when:

1. `./setup` installs all 5 skills into Claude Code and one other host without manual steps.
2. Each skill is invokable as `/<name>` from inside the host.
3. Editing one `.tmpl` and re-running `bun run gen:skill-docs` updates both host outputs.
4. Adding a 6th skill takes < 10 min: new dir, new `.tmpl`, no other touch points needed.
5. `bin/mybundle-paths` is the only file that changes when state-path semantics change.
