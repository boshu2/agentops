# Reverse-Engineering Report: gstack

- **Date:** 2026-05-01
- **Source:** `/home/boful/dev/personal/gstack` (HEAD `6e1625c0`, v1.25.0.0)
- **Upstream:** https://github.com/garrytan/gstack.git
- **Mode:** repo-only (no binary analysis; Bun-built `dist/` artifacts not present in this checkout)
- **Authorization:** public open-source repo; no `--authorized` flag needed (binary mode would require it)

## TL;DR

gstack is **a multi-host AI-skill bundle**: 45 slash-command-invokable skills + 55 helper scripts + 2 compiled binaries (`browse`, `make-pdf`) + 11 host adapters, deployable via a single 40 KB `setup` shell installer to any of 9 supported agent harnesses (Claude Code, Codex, Cursor, OpenClaw, Opencode, Slate, Hermes, Kiro, Gbrain). Runtime is **Bun**, not Node.

## Why the auto-extractor needed enrichment

The generic `reverse_engineer_rpi.py` extractor produced thin output — only 26 docs slugs and 2 binaries. It missed:

1. **The actual product surface** — 45 skills as top-level dirs with `SKILL.md` (not `docs/features/*`).
2. **The helper toolbelt** — 55 `bin/gstack-*` scripts skills shell out to.
3. **The multi-host architecture** — 11 adapters under `hosts/`.
4. **Bun-native runtime** — flagged as "Node package" because it has `package.json`.
5. **The templated-skill pattern** — `<skill>/SKILL.md.tmpl` → `<skill>/SKILL.md` per host via `bun run gen:skill-docs`.

The spec files in this directory have been hand-enriched with code-proven evidence. `feature-registry.yaml` validates clean against the canonical validator.

## Files in this directory

| File | What it is |
|---|---|
| `README.md` | this file |
| `feature-inventory.md` | full code-proven product surface (45 skills + 55 helpers + 11 adapters), grouped by lane |
| `feature-registry.yaml` | machine-checkable registry with anchors per group; passes `validate-feature-registry.py` |
| `feature-catalog.md` | all 45 skills + helper families + host adapters as tables |
| `validate-feature-registry.py` | validator wrapper (delegates to canonical script) |
| `spec-architecture.md` | 6-component architecture model with ASCII diagram and trust boundaries |
| `spec-code-map.md` | repo layout, SaaS boundary, feature-to-code anchors per skill lane |
| `spec-cli-surface.md` | three CLI surfaces (compiled binaries, installer, helper toolbelt) + slash commands + env vars |
| `spec-artifact-surface.md` | templated artifacts (`*.tmpl`), compiled artifacts, runtime state dirs |
| `spec-clone-vs-use.md` | decision heuristic for users; black-box vs white-box trade-offs |
| `spec-clone-mvp.md` | spec for an **original** 5-skill bundle that mirrors gstack's *shape* without copying its prompts |
| `docs-features.txt` | raw docs slug inventory |
| `analysis-root-path.txt` | `/home/boful/dev/personal/gstack` |
| `contracts/` | machine-checkable contract subset (auto-generated; sorted, stable keys) |
| `analysis-root/` | empty placeholder used by the validator |

## How to re-run

```bash
# From this agentops repo root:
python3 ~/.claude/plugins/cache/agentops-marketplace/agentops/2.38.0/skills/reverse-engineer-rpi/scripts/reverse_engineer_rpi.py \
  gstack \
  --mode=repo \
  --local-clone-dir=/home/boful/dev/personal/gstack \
  --output-dir=.agents/research/gstack/

# Re-validate the registry
python3 .agents/research/gstack/validate-feature-registry.py
```

The auto-script will overwrite the spec files with thin templates — the enrichments here are hand-authored and intentional. If you re-run, restore from git.

## Key boundaries (use this to scope follow-up work)

- **Black-box use:** `./setup` installs everything; `/<skill>` is invokable. No source needed.
- **Clone necessary for:** new host adapter, prompt edits, eval lane, custom build of `browse` or `make-pdf`.
- **Cannot clone:** Anthropic / OpenAI Codex / Gemini APIs; Supabase gbrain backend; Conductor workspaces.

## Guardrails honored

- Public-source analysis only (gstack is open source under `LICENSE`).
- No proprietary prompt content reconstructed in reports — index by name + lane only.
- No secrets in outputs (only references to `.env.example` env-var names).
- `dist/` binary anchors omitted from registry because the checkout isn't built; source dirs referenced instead.
