# CLI Surface Spec: gstack

- Date: 2026-05-01
- Analysis root: `/home/boful/dev/personal/gstack` (HEAD `6e1625c0`, v1.25.0.0)
- Runtime: Bun (not Node — `bun.lock` present, every script invokes `bun`)

## Three CLI Surfaces

gstack ships three distinct invokable surfaces. The auto-extractor only found surface (1).

### 1. Compiled binaries (declared in `package.json` `bin`)

| Binary | Source | Purpose |
|---|---|---|
| `browse` | `./browse/dist/browse` | Headless Chromium wrapper. Skills invoke as `$B <command>` (~100ms/cmd). Built from TypeScript via `bun run build`. |
| `make-pdf` | `./make-pdf/dist/pdf` | Markdown → publication-quality PDF. |

### 2. Top-level installer

| Entrypoint | Type | Purpose |
|---|---|---|
| `./setup` | Shell (40 KB) | Idempotent installer. Detects host (Claude / Codex / Cursor / etc.), installs skills, wires `bin/` into PATH. Re-runnable. |
| `bin/dev-setup` | Shell | Dev-mode setup (aliased as `bun run setup`). |
| `bin/dev-teardown` | Shell | Reverse of dev-setup (aliased as `bun run archive`). |

### 3. Helper toolbelt (`bin/`, 55 entries)

The `bin/gstack-*` family is the operator/skill toolbelt. Skills shell out to these. Notable:

- **State resolution:** `bin/gstack-paths` — sourced via `eval "$(bin/gstack-paths)"`. Honors `GSTACK_HOME`, `CLAUDE_PLUGIN_DATA`, `CLAUDE_PLANS_DIR`. Canonical state-root resolver across hosts.
- **Memory sync (gbrain):** `gstack-brain-{init,enqueue,consumer,reader,sync,restore,uninstall}` — the cross-machine memory pipeline.
- **gbrain integration:** `gstack-gbrain-{detect,install,repo-policy,source-wireup,supabase-provision,supabase-verify}` + `gstack-gbrain-lib.sh`.
- **Telemetry / logging:** `gstack-{telemetry,timeline,review,question,learnings}-{log,read,search,sync,preference}` (10 scripts).
- **Per-skill helpers:** `gstack-codex-probe`, `gstack-community-dashboard`, `gstack-security-dashboard`, `gstack-model-benchmark`, `gstack-specialist-stats`, `gstack-builder-profile`, `gstack-developer-profile`, `gstack-taste-update`.
- **Repo state:** `gstack-{config,repo-mode,relink,extension,uninstall,update-check,next-version}`.
- **Cross-platform:** `gstack-platform-detect`, `gstack-pr-title-rewrite.sh`, `gstack-patch-names`, `gstack-open-url`, `gstack-slug`.
- **Browser/CDP:** `chrome-cdp` (Chrome DevTools Protocol bridge).

### 4. Slash-command surface (45 skills)

These aren't OS-level binaries — they're invoked through the host harness as `/<skill-name>` and live as `<skill>/SKILL.md`. The `setup` script installs them into the host's expected location (e.g., `~/.claude/skills/gstack/<name>/`).

Full list in `feature-inventory.md`. Lanes: plan-mode reviews (8), implementation+review (10), release+deploy (7), operational+memory (8), browser+agent (4), safety+scoping (5), root meta (1).

## Help Text

`browse --help` and `make-pdf --help` would produce golden-test fixtures, but the binaries are not built in this checkout. To capture: `bun run build && ./browse/dist/browse --help > .agents/research/gstack/golden/browse-help.txt`.

## Config / Env (Code-Proven)

From `.env.example`:
- `ANTHROPIC_API_KEY` — required for `bun run test:eval` (LLM-as-judge eval lane)

From `AGENTS.md` + `bin/gstack-paths`:
- `GSTACK_HOME` — overrides default state root
- `CLAUDE_PLUGIN_DATA` — Claude Code plugin data dir (used when running as plugin)
- `CLAUDE_PLANS_DIR` — Claude Code plans dir
- `GSTACK_CLAUDE_BIN` — overrides resolved `claude` binary path (`Bun.which()` fallback)
- `GSTACK_CLAUDE_BIN_ARGS` — JSON array, prepended args for `claude` invocation (e.g., `'["claude"]'` to run via WSL: `GSTACK_CLAUDE_BIN=wsl GSTACK_CLAUDE_BIN_ARGS='["claude"]'`)

## Notes For 1:1 Fidelity

- The CLI contract is the **union of three surfaces**: compiled binaries + setup installer + 55-script toolbelt + 45 slash commands. A clone that only ships the binaries is a sub-product.
- Capture `--help` output of `browse` and `make-pdf` post-build as golden fixtures.
- Skills are the user-facing surface; the toolbelt is the implementation. A reverse-engineering effort that targets only the binaries misses the product entirely.
- Bun, not Node — runtime substitution requires re-validating every script's `bun`-specific assumptions (`Bun.which`, top-level await, `import.meta.path`, `Bun.file()`).
