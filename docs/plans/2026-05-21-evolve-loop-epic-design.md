---
date: 2026-05-21
epic: soc-g2qd
status: design-memo
phase: 0c
---

# Evolve Loop Epic — Design Memo

Cross-cutting decisions for the 6 sub-beads of `soc-g2qd` (/evolve --mode=loop). Consumed by Wave 1 (soc-hwax, soc-6svt, soc-ij7e) and Wave 2 (soc-mlbm, soc-un0m, soc-g34d) workers.

## The boundary

- **Skill (`skills/evolve/SKILL.md`)** = prompt the agent reads. Authors *intent*.
- **CLI (`cli/cmd/ao/evolve*.go`)** = mechanical enforcement + state. Owns *invariants*.

Rule: anything the skill can fail to enforce goes in the CLI.

## Architectural decisions

### A1 — `--mode=loop` flag (soc-hwax)

CLI: `ao evolve --mode string` (default burst). Mechanical no-stop: `ao evolve write-stop-marker` exits 1 under `--mode=loop`. Operator-written markers bypass via `ao evolve operator-stop`.

### A2 — preferences.yaml (soc-6svt)

Path: `.agents/evolve/preferences.yaml`. Schema: `schemas/evolve-preferences.v1.schema.json` (draft-07). Loader: `cli/internal/evolve/preferences.go::Load(ctx) (*Prefs, error)`. Subcommand: `ao evolve config --show` (YAML output, or `--json`). Resolution order: defaults → preferences.yaml → CLI flag. Invalid keys/types: ERROR with file:line. Schema fields v1: `schema_version, mode_default (burst|loop), scope_filter.{productive_threshold, scout_streak_halt}, recommended_pointer_strict, halt_signals (list), generator_layers_enabled`.

### A3 — Versioned cron template (soc-ij7e)

Canonical: `skills/evolve/templates/cron-loop-mode.md`. Engine: Go `text/template`. Required vars: `.ShippedCommits`, `.NextRecommendedBead`, `.SubBeadsFiledThisCycle`, `.TestsDelta`, `.CronSelfAdjustCounter`. VERBATIM-PRESERVE markers: HTML-comment pairs with SHA-256 in frontmatter. Renderer + verifier: `cli/internal/evolve/template.go`.

### A4-A6 (Wave 2)

`ao cron self-adjust`, `ao evolve next-work`, `ao evolve blocked`. Out of scope for Wave 1.

### A7 — SKILL.md mode-aware text

In-file HTML-comment conditionals. If brittle, fall back to two-file split (`SKILL.burst.md` + `SKILL.loop.md` + stub). Decision deferred to W1a (soc-hwax worker) based on diff complexity.

## Wave-1 file ownership

| Worker | Bead | Owns |
|---|---|---|
| W1a | soc-hwax | `skills/evolve/SKILL.md` (Steps 1/3/7 + Flag table only) + `skills/evolve/references/loop-mode.md` + `cli/cmd/ao/evolve.go` (--mode flag) + `cli/cmd/ao/evolve_test.go` + `cli/cmd/ao/evolve_write_stop_marker.go` + `_test.go` + this memo file |
| W1b | soc-6svt | `schemas/evolve-preferences.v1.schema.json` + `cli/internal/evolve/preferences.go` + `_test.go` + `cli/cmd/ao/evolve_config.go` + `_test.go` + `.agents/evolve/preferences.yaml.template` + `.gitignore` negation rule |
| W1c | soc-ij7e | `skills/evolve/templates/cron-loop-mode.md` + `cli/internal/evolve/template.go` + `_test.go` + `cli/internal/evolve/testdata/` |

## Out-of-scope (Phase 3, not this session)

- Bead-graph audit
- Cron-fire handoff continuity primitive
- Operating-model schema CI gate
