# Dual-Pane ATM — Work-Split Matrix

## Selection guide

| If the goal is… | Pattern | Why two panes |
|---|---|---|
| Ship a vertical slice with doc + code | **Disjoint ownership** | Different surfaces, same bead; no file collision |
| Reduce author blind spots before writing | **Explore + write** | Opus maps; Codex implements with fresh context |
| Land risky change with skeptic | **Author + refuter** | Lighter than full duel; refuter read-only |
| Compare two design drafts | **Dual draft → synthesize** | Orchestrator merges; no shared write path |
| Add Gemini-sub / adversarial third lane | **Tri-vendor (+AGY)** | Opus + Codex stay primary builders; AGY pane for adversarial review, red-team read, or AGY-native `/research` when matrix needs a third vendor | AGY reserves disjoint read or `.agents/dual-pane/` artifact paths; orchestrator still synthesizes |
| Score ideas adversarially | **Not this skill** → `dueling-idea-wizards` | Scoring/reveal phases, not build |

## Pattern detail

### Disjoint ownership

Best for: registry overlay + CLI gate, schema + implementation, docs + tests in separate trees.

| Lane | Typical scope | Reserve example | Dispatch |
|---|---|---|---|
| Opus | Policy, contracts, narrative | `docs/contracts/` `schemas/` | `/implement <bead>` or `/doc` |
| Codex | Go CLI, gates, scripts | `cli/` `scripts/check-*.sh` | `/implement <bead>` |

**Integration:** orchestrator merges or sequential PRs; integration order declared upfront (schema before CLI consumer).

### Explore + write

Best for: unfamiliar subsystem, architecture spike before slice.

| Lane | Mode | Reserve | Dispatch |
|---|---|---|---|
| Opus | Read-only | *(none)* or read paths only | `/research` — output `EXPLORE.md` |
| Codex | Write | `cli/foo/` `tests/` | `/implement` after explore artifact exists |

**Gate:** Codex dispatch waits until `EXPLORE.md` (or br note) exists — explore lane finishes first.

### Author + refuter (light)

Best for: pre-land skeptic pass without full council; smaller than dueling-idea-wizards.

| Lane | Mode | Reserve | Dispatch |
|---|---|---|---|
| Opus | Write | author's file manifest | `/implement` or `/refactor` |
| Codex | Read-only | none | `/review` or refuter packet (see pre-land-refuters) |

**Evidence:** refuter must cite paths + line refs; orchestrator fixes forward.

### Dual draft → synthesize

Best for: architecture options, API shape, naming debates (collaborative, not scored).

| Lane | Output | Reserve |
|---|---|---|
| Opus | `.agents/dual-pane/DRAFT_OPUS.md` | orchestrator-owned dir only |
| Codex | `.agents/dual-pane/DRAFT_CODEX.md` | same dir, disjoint filenames |

Orchestrator synthesizes; optional `/council` if pawl-level decision.

## Reserve examples (copy-paste)

```bash
# Claim registry slice (CEP-style)
--reserve "docs/contracts/claim*.yaml" "schemas/claim*.json" "cli/internal/gates/claim*.go" "cli/cmd/ao/claim*.go"

# Docs-only vs cli-only
--reserve "docs/" "cli/"

# Single-package Go + its tests
--reserve "cli/internal/foo/" "cli/internal/foo/*_test.go"
```

## Collision rules

1. **DERIVED surfaces** (`registry.json`, `cli/docs/COMMANDS.md`, generated maps) — only one lane may touch per session; usually sequential, not parallel.
2. **Same bead, two writers** — OK only with disjoint reserves + worktrees; orchestrator owns merge.
3. **≥2 paths overlap** — stop; re-cut split or escalate to `am` with explicit lease, not hope.

## vs dueling-idea-wizards

| | dual-pane-atm | dueling-idea-wizards |
|---|---|---|
| Posture | Collaborative handoff | Adversarial cross-score |
| Phases | Spawn → dispatch → tend → synthesize | Study → ideate → score → reveal → report |
| Output | Shipped artifact or merged design | `DUELING_WIZARDS_REPORT.md` + score matrix |
| Typical duration | One bead / one slice | Multi-phase ideation session |
