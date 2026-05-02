# Clone vs Use Spec: gstack

- Date: 2026-05-01
- Source: code-proven from `/home/boful/dev/personal/gstack` (HEAD `6e1625c0`, v1.25.0.0)

## Use It (Black-Box) — Recommended Default

**What you get without cloning:** Run `./setup` against your host (Claude Code / Codex / Cursor / OpenClaw / Slate / Hermes / Kiro / Opencode / Gbrain). The installer drops 45 SKILL.md files into the host's expected skills dir and wires `bin/gstack-*` into PATH. From then on, `/<skill-name>` is invokable inside the host.

**Verifiable contracts:**
- Slash-command surface: 45 invocable skills (see `feature-inventory.md` lanes)
- Compiled binaries: `browse` (~100ms/cmd), `make-pdf`
- Helper toolbelt: `bin/gstack-*` (55 entries) on PATH after install
- State indirection: `GSTACK_HOME`, `CLAUDE_PLUGIN_DATA`, `CLAUDE_PLANS_DIR`
- Env keys: `ANTHROPIC_API_KEY` (eval lane only), `GSTACK_CLAUDE_BIN` + `GSTACK_CLAUDE_BIN_ARGS` (Windows/WSL routing)

**What you cannot verify black-box:**
- Internal prompt design of each skill (treat as opaque IP — read `SKILL.md` if you need to inspect; do not re-distribute proprietary prompts).
- Eval rubric details.
- The exact gbrain Supabase schema.

## Clone It (White-Box)

**Reasons to clone:**
1. **Add a new host** — copy `hosts/openclaw.ts` shape, register in `hosts/factory.ts`, run `bun run gen:skill-docs --host <new-host>`.
2. **Patch a skill prompt** — edit `<skill>/SKILL.md.tmpl`, regenerate via `bun run gen:skill-docs`.
3. **Add a new skill** — new top-level dir with `SKILL.md.tmpl`, follow `SKILL.md.tmpl` (root) shape, register in `scripts/discover-skills.ts` if not auto-detected.
4. **Run the eval lane locally** — needs `ANTHROPIC_API_KEY` (or `GEMINI_API_KEY`, etc. for cross-model benchmarks).
5. **Audit security** — `/cso` gives an OWASP+STRIDE pass over your repo, but the audit logic itself is in `cso/SKILL.md` + `bin/gstack-security-dashboard`.

**What clone gives you over use:**
- Source for `browse/src/` (Bun + CDP wrapper) — can fork for custom browser primitives.
- All 11 host adapters (`hosts/*.ts`) — extension model is documented in code.
- Eval scripts (`scripts/eval-*.ts`) — can adapt for local model benchmarking.
- Test taxonomy and CI gating (`.github/`, `bun run test:*`).

## Hosted / Control-Plane Boundaries (Cannot Be Cloned)

| Surface | Hosted by | Implication for clone |
|---|---|---|
| Codex API | OpenAI | `/codex` requires OpenAI auth; cannot replicate without API key + ToS adherence |
| Anthropic eval lane | Anthropic | `bun run test:eval*` needs API key |
| gbrain Supabase backend | Supabase project | Cross-machine sync needs your own Supabase project (`bin/gstack-gbrain-supabase-provision` automates) |
| GitHub PR lifecycle | GitHub | `/ship`, `/land-and-deploy` need `gh` CLI auth |
| Conductor workspaces | Conductor | `/landing-report`, `/context-restore` use Conductor APIs when present (graceful fallback otherwise) |

## Redaction / Handling

- **Do not commit extracted skill prompt content into your own repo verbatim** — gstack is licensed (see `LICENSE`); respect the project's terms.
- **Do not paste proprietary prompts into reports.** This spec set indexes by name + lane only.
- **Do not commit `.tmp/`** — clone targets land there by default.
- **Do not check in env keys.** `.env.example` is the only env file in the repo by design.

## Decision Heuristic

| You want to... | Choose |
|---|---|
| Use the 45 skills as-is in your editor | **Use** (`./setup`) |
| Modify one prompt for your taste | **Use + override** (drop a `~/.claude/skills/<name>/SKILL.md` after install — local wins) |
| Add a new host (e.g., a new agent harness) | **Clone**, add `hosts/<host>.ts`, contribute upstream |
| Build your own competing skill bundle | **Inspired by, not copy of** — follow the `bin/<tool>` + `<skill>/SKILL.md` + host adapter pattern; write original prompts |
| Run the eval/benchmark suite | **Clone** + provide API keys |
| Just want `/qa` or `/ship` | **Use** |
