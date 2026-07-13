# Codex image bundle — 29 CORE twins

**`cp-gqu` image-skills EPIC · Unit 4 · bead `cp-eoxc`.** Spec:
`control-plane/IMAGE-CORE.md` §1 (the 61 CORE slugs), §2c + §2d (the Codex
conversion recipe), §3b (Codex operator skills), §4 Unit-4 (what this unit
consumes). Cross-image laws: `control-plane/IMAGES.md`.

This directory **packages** the current CORE-29 subset as the **Codex image**. Codex is
the **only** vendor that needs a format **conversion** — Claude and Gemini/AGY
consume `skills/<slug>/SKILL.md` directly, but Codex consumes a converted twin at
`skills-codex/<slug>/`.

> **This bundle does NOT re-run the converter.** The `skills-codex/` twins already
> exist for the current Codex corpus (61 canonical implementations plus 4 compatibility
> packages). The job here is to declare the CORE-29 subset as the Codex image and **prove** those 29 twins are present, converted,
> and hash-consistent with their source. Packaging only — nothing under `skills/`
> or `skills-codex/` is edited.

## Contents

| File | Purpose |
|---|---|
| `manifest.json` | The Codex image set: 29 CORE slugs (20 method + 9 tool-op), each with its `skills-codex/<slug>/` twin path, source path, twin files, and `codex_override_catalog` wave + treatment; plus the Codex operator skill. |
| `verify.sh` | Presence gate: for each CORE slug confirm `SKILL.md` + `prompt.md` + `.agentops-generated.json` exist in its twin; then run the hash-drift gate. Exit 0 = clean; missing/stale twins are flagged, never silently passed. |
| `README.md` | This file. |

## The conversion recipe (IMAGE-CORE §2c)

Codex does **not** consume `skills/<slug>/SKILL.md` directly. Each skill has a
**converted twin** at `skills-codex/<slug>/`, produced by the **`converter`** skill
(`$converter skills/<slug> codex`). The converter pipeline is `parse → convert →
write`: parse the source SkillBundle (frontmatter + body + `references/` +
`scripts/`), convert to the Codex target, write the twin. `--all codex`
regenerates the whole mirror.

Each `skills-codex/<slug>/` twin carries:

- **`SKILL.md`** — Codex-native phrasing of the skill.
- **`prompt.md`** — the Codex prompt form.
- mirrored **`references/`** + **`scripts/`**.
- **`.agentops-generated.json`** — the drift marker recording the source→generated
  relationship and hashes:

  ```json
  { "generator": "manual-maintained", "source_skill": "skills/beads",
    "layout": "modular",
    "source_hash": "86bfef…", "generated_hash": "0960ad…" }
  ```

A top-level **`skills-codex/.agentops-manifest.json`** holds the
**`codex_override_catalog`** — a per-skill Codex treatment map organized in waves
(`backbone`, `core-execution`, `analysis-authoring`, `contribution-workflow`,
`security-focused`, `catalog-parity`) plus a catalog hash. `manifest.json` records
each CORE slug's `codex_wave` + `codex_treatment` from this catalog.

### CORE-29 wave distribution

| Wave | CORE slugs |
|---|---:|
| `backbone` | 9 |
| `core-execution` | 6 |
| `analysis-authoring` | 0 |
| `catalog-parity` | 14 |
| **total** | **29** |

## The integrity gate

**`scripts/regen-codex-hashes.sh`** recomputes each twin's `generated_hash` (and
the manifest hashes) after any `skills-codex/` edit:

- **`--check`** — the CI **drift gate**. Exit 0 means every twin is in sync with
  its source; non-zero means a twin drifted and must be regenerated. `verify.sh`
  runs this as its final, authoritative step.
- **`--only <slug>`** — scopes a single-skill PR so it doesn't sweep unrelated
  drift.

`verify.sh` is presence-only on its own; the hash gate is what proves the twins
are *converted and in sync*, not just present.

## Verify

Fresh install:

```bash
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash
```

```bash
# from the agentops repo root (or anywhere — verify.sh resolves the root itself)
bash images/codex/verify.sh          # presence of all 29 twins + hash drift gate -> exit 0

# the authoritative drift gate on its own
scripts/regen-codex-hashes.sh --check   # exit 0 = twins in sync with source
```

## Base

- agentops base commit: `8172d7e7ab6b43a6fb2624c4be8ac3816d6a24d5`
- distilled-state reference: `7af9eb342` (the post cf0+dkf state IMAGE-CORE
  enumerates the corpus against).

## Related images

- **Claude** (Unit 2, `cp-ytub`): direct `skills/<slug>/` — zero conversion.
- **Gemini/AGY** (Unit 3, `cp-7uih`): direct SKILL.md inside an Antigravity plugin
  wrapper — zero content conversion.
- **Codex** (this, Unit 4, `cp-eoxc`): the **only** vendor with a real format
  conversion — the `skills-codex/` twin + the hash-drift gate.
