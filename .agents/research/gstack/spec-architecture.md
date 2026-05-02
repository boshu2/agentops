# Architecture Spec: gstack

- Date: 2026-05-01
- Source: code-proven from `/home/boful/dev/personal/gstack` (HEAD `6e1625c0`, v1.25.0.0)
- Guardrails: authorized public-source analysis. No proprietary prompt/source reconstruction.

## Scope

**In-scope:** repo layout, runtime model, host-adapter pattern, skill bundle shape, state-resolution layer, binary surface, eval/test taxonomy, multi-host install model.

**Out-of-scope:** the actual prompt content of each `SKILL.md` (treated as IP — index by name + lane only), evaluation rubrics/scoring details, hosted gbrain/Supabase implementation.

## High-Level Model

gstack is **a multi-host AI-skill bundle with a Bun-native helper toolbelt and two compiled binaries**. It is not a single application — it's a deployment layer that installs 45 skills + 55 helper scripts into 9 different agent hosts (Claude, Codex, Cursor, OpenClaw, Cursor, Opencode, Slate, Hermes, Kiro, Gbrain).

```
┌────────────────────────────────────────────────────────────────┐
│  USER + HOST AGENT (Claude Code, Codex CLI, Cursor, ...)       │
│           invokes /office-hours, /ship, /qa, ...               │
└────────────────────────────────┬───────────────────────────────┘
                                 │ slash command
                                 ▼
┌────────────────────────────────────────────────────────────────┐
│  SKILL LAYER (45 dirs, each with SKILL.md)                     │
│  Plan-mode (8) | Impl+review (10) | Release (7) |              │
│  Ops+memory (8) | Browser (4) | Safety (5) | root meta         │
└─────┬────────────────┬────────────────┬────────────────────────┘
      │ shells out     │ shells out     │ shells out
      ▼                ▼                ▼
┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────┐
│ bin/        │  │ browse      │  │ External services (control- │
│ gstack-*    │  │ (Chromium)  │  │ plane):                     │
│ (55 helpers)│  │             │  │ • Anthropic API             │
│             │  │ make-pdf    │  │ • Codex API                 │
│ + state via │  │ (compiled)  │  │ • Supabase (gbrain)         │
│ gstack-paths│  │             │  │ • GitHub (gh CLI)           │
└──────┬──────┘  └─────────────┘  │ • Conductor workspaces      │
       │                          └─────────────────────────────┘
       ▼
┌─────────────────────────────────────────┐
│ STATE LAYER                             │
│ ${GSTACK_HOME} or ${CLAUDE_PLUGIN_DATA} │
│   • plans/                              │
│   • learnings/                          │
│   • timeline/, telemetry/, questions/   │
└─────────────────────────────────────────┘
       ▲
       │ install
┌──────┴────────────────────────────────────┐
│ HOST ADAPTERS (hosts/*.ts, 9 hosts)       │
│ claude · codex · cursor · openclaw ·      │
│ opencode · slate · hermes · kiro · gbrain │
│ (+ factory.ts, index.ts)                  │
└───────────────────────────────────────────┘
       ▲
       │ ./setup (40 KB shell, idempotent)
       ▼
┌───────────────────────────────────────────┐
│ INSTALLER                                 │
│ ./setup → bin/dev-setup → host adapter    │
│ → copies SKILL.md to host's skills dir    │
│ → wires bin/ into PATH                    │
└───────────────────────────────────────────┘
```

## Component Map

### 1. Skill layer (data-plane, local)
- **Where:** 45 top-level dirs each with `SKILL.md` (template: `<name>/SKILL.md.tmpl`).
- **Generated, not authored:** `bun run gen:skill-docs` materializes per-host SKILL.md from `.tmpl`. Source of truth = templates. Per-host variants emitted via `--host <name>`.
- **Trust boundary:** runs inside the host harness's permission model. Each skill declares `allowed-tools` in YAML frontmatter.
- **Evidence:** `*/SKILL.md`, `*/SKILL.md.tmpl`, `scripts/gen-skill-docs.ts`.

### 2. Helper toolbelt (data-plane, local)
- **Where:** `bin/` (55 entries — see `spec-cli-surface.md`).
- **Pattern:** Skills do not contain logic; they shell out to `bin/gstack-*`. This keeps prompts thin and concentrates testable logic in shell/TS.
- **State resolution centralized in:** `bin/gstack-paths` (sourced via `eval "$(bin/gstack-paths)"`). Single point that resolves `GSTACK_HOME` / `CLAUDE_PLUGIN_DATA` / `CLAUDE_PLANS_DIR`.
- **Trust boundary:** local FS + child-proc spawn. No network unless a specific helper makes one (gbrain-sync, telemetry-sync).

### 3. Binary surface (data-plane, local)
- `browse` — headless Chromium under Bun. Contract: ~100ms/command. Source: `browse/src/`. Built to `browse/dist/browse` via `bun run build`.
- `make-pdf` — markdown → PDF. Source: `make-pdf/`. Built to `make-pdf/dist/pdf`.
- **`browse/src/claude-bin.ts`:** resolves `claude` CLI via `Bun.which()` with `GSTACK_CLAUDE_BIN` override. Enables WSL routing on Windows.

### 4. Host-adapter layer (control-plane within the install model)
- **Where:** `hosts/*.ts` (11 files: 9 host adapters + `factory.ts` + `index.ts`).
- **Pattern:** factory selects adapter based on detected host; adapter knows where the host expects skills (`~/.claude/skills/`, `~/.cursor/...`, etc.) and how to render SKILL.md for that host.
- **Why this matters:** the same skill source produces 9 different on-disk shapes. `bun run gen:skill-docs --host codex` produces `codex/SKILL.md`; the equivalent for `cursor`, `opencode`, etc.

### 5. State + memory (mixed: local + hosted)
- **Local state root:** `${GSTACK_HOME:-~/.gstack}` or `${CLAUDE_PLUGIN_DATA}` (when running as Claude plugin). Subdirs: `plans/`, `learnings/`, `timeline/`, `telemetry/`, `questions/`, `reviews/`.
- **gbrain (cross-machine sync):** `gstack-brain-*` family + Supabase backend. Out-of-process consumer/enqueue pattern (`gstack-brain-enqueue` writes a queue entry, `gstack-brain-consumer` drains it). `gstack-brain-reader` is a symlink to `gstack-brain-consumer` — same binary, read-only mode signaled by argv0.

### 6. Eval / test taxonomy (developer surface)
- **Free tests** (`bun test` / `bun run test:free`): no API spend, run in CI.
- **Audited / gated** (`bun run test:audit`, `test:gate`): structural checks, slop-scan.
- **Periodic** (`bun run test:periodic`): scheduled CI lane.
- **Windows-safe subset** (`bun run test:windows`): curated list runs on `windows-latest`.
- **LLM-as-judge evals** (`bun run test:eval{s,:codex,:gemini}` + `eval:{compare,list,select,summary,watch}`): hits Anthropic/Codex/Gemini APIs. Costs API spend; gated by env keys.
- **e2e** (`bun run test:e2e{,:all}`): real browser, real flows.

## Trust Boundaries

| Boundary | What crosses it | Evidence |
|---|---|---|
| Local ↔ Anthropic API | Eval prompts, `/codex` second-opinion calls | `.env.example` requires `ANTHROPIC_API_KEY`; `scripts/eval-*.ts` |
| Local ↔ Codex API | Second-opinion via `/codex` skill | `codex/SKILL.md`, `scripts/preflight-agent-sdk.ts` |
| Local ↔ GitHub | PR open/merge/CI watch via `gh` CLI | `/ship`, `/land-and-deploy`, `/canary` skills |
| Local ↔ Supabase | gbrain memory rows | `bin/gstack-gbrain-supabase-{provision,verify}` |
| Local ↔ Conductor | Workspace-aware ship queue, context-restore handoff | `conductor.json`, `/landing-report`, `/context-restore` |
| Skill ↔ Host harness | `allowed-tools` frontmatter declares scope | every `SKILL.md` YAML frontmatter |

## Evidence Buckets (kept separate)

### Docs Say (claims, not necessarily code-proven)
- 26 docs slugs in `docs/` (full list in `docs-features.txt`); design notes for unbuilt/upcoming features under `docs/designs/*`.
- AGENTS.md prose: "browse binary provides headless browser access"; "Setup script (`./setup`) requires Git Bash or MSYS today"; "native PowerShell support is a future expansion".

### Code Proves (filesystem evidence)
- 45 skills × `SKILL.md` (templated from `.tmpl`).
- 55 `bin/gstack-*` helpers + `chrome-cdp` + `dev-setup` + `dev-teardown`.
- 2 compiled binaries declared in `package.json` `bin`: `browse`, `make-pdf`.
- 11 host adapters (`hosts/*.ts`).
- Bun-native runtime (`bun.lock`, every script invokes `bun`).
- Multi-host install via `./setup` (40 KB shell).

### Hosted / Control-Plane (network-bound, not in repo)
- Anthropic / Codex / Gemini APIs (eval + `/codex` lanes).
- Supabase (gbrain backend).
- GitHub (PR lifecycle).
- Conductor workspace API.

## Architectural Principles (Inferred from Layout)

1. **Skills are prompts, not code.** Logic concentrates in `bin/` and `browse/src/`; SKILL.md describes intent + invokes helpers.
2. **One-source, many-hosts.** `.tmpl` + host adapter = N on-disk shapes. New host = new adapter file under `hosts/`, no skill changes.
3. **State path indirection.** `bin/gstack-paths` is the single resolver — every helper sources it. Path changes happen in one place.
4. **Browse-as-primitive.** `$B <command>` is the standard interaction model for any skill that needs a browser.
5. **Tiered tests by API spend.** Free / audited / periodic / windows / eval lanes let CI run cheap checks always and expensive checks selectively.
