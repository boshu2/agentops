# Claude image — CORE shipped DIRECT

> Unit 2 (`cp-ytub`) of the `cp-gqu` image-skills EPIC. Spec: `IMAGE-CORE.md`
> §1 (the 61 CORE slugs), §2a (Claude recipe), §2d (contract table), §3a
> (Claude operator skills). Source corpus: agentops `8172d7e7a` (the merged,
> gate-green distilled corpus).

## What this is

The **manifest + README + verify** that *declare* the Claude image. It does **not**
copy or duplicate any skill files. The Claude harness consumes
`agentops/skills/<slug>/` **in place**; this bundle only points at them.

## Recipe — direct, zero conversion

`SKILL.md` is **Claude-native**: YAML frontmatter (`name` / `description`) plus a
markdown body, with `references/` and `scripts/` siblings. Claude reads it
verbatim. There is **no transform, no wrapper, no twin** — the `skills/` tree is
already the Claude image's skill set.

This is the KEY FINDING of `IMAGE-CORE.md` §2: `SKILL.md` is portable across all
vendors; **only Codex** needs a format conversion (the `skills-codex/` twin).
Claude ships the source tree directly.

| Vendor | Source | Transform | Packaging shell |
|---|---|---|---|
| **Claude (this image)** | `skills/<slug>/` | **none (direct)** | skills path / marketplace plugin |
| Gemini/AGY | `skills/<slug>/` | none (direct) | Antigravity plugin wrapper |
| Codex | `skills/<slug>/` | CONVERSION → `skills-codex/<slug>/` | Codex skills + plugins |

## The skill set — 36 skills

- **34 CORE** (the "image mind"): **24 method-core** (the operating loop,
  AgentOps-owned) + **10 tool-op-core** (operating the substrate). The original
  IMAGE-CORE.md 61-slug list, resolved through the skill-consolidation ledger
  (2026-07-04 refresh, age-085q) — retired slugs dropped, merged slugs replaced
  by their successors.
- **2 Claude operator skills** (§3a): thin skills that teach the worker to drive
  Claude's first-class control surface — `workflow-builder`, `cc-hooks`.

The authoritative list (slug + path + `ship: direct`) is `manifest.json`.

## How the Claude harness discovers them

The skill set is exposed to Claude as a **skills directory** discoverable by the
harness — either via the **skills path** the harness scans, or as a **marketplace
plugin** that registers `skills/<slug>/SKILL.md` entries. Either way the discovery
unit is the verbatim `skills/<slug>/` directory; no per-skill packaging step runs.

## Install path

```bash
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-claude.sh | bash
```

The installer uses the Claude marketplace plugin path:

```bash
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace
```

## Primitives wrap usage, not files

Claude's first-class primitives — **Workflows**, **Agent subagents**, and
**scheduled tasks** — wrap the *usage* of these skills, not the skill files
themselves. A Workflow orchestrates `worker → validate → tie-break`; a subagent is
dispatched against a skill; a scheduled task fires a tick. None of them transform
or repackage `SKILL.md`. The operator skills in §3a are exactly the thin layer that
teaches the worker to drive those primitives.

## Verify

```bash
bash images/claude/verify.sh   # exit 0 iff every manifest slug exists at skills/<slug>/SKILL.md
```

`verify.sh` reads the slug list from `manifest.json` and asserts each
`skills/<slug>/SKILL.md` exists in the corpus. Exit 1 on any missing skill.
