# Slop Map — what to delete, merge, keep

> First-principles teardown, 2026-05-28. Companion: [spine.md](spine.md), [strangler-plan.md](strangler-plan.md).
> Every row is evidence-grounded (file paths + measured line/import counts). "Slop" = serves projection-consistency, not the four-point thesis.

## Verdict in one table

| # | Slop site | Size | Type | Verdict | Confidence |
|---|---|---|---|---|---|
| S1 | 6 zero-import Go packages | 5,033 lines | dead code | **DELETE** | High — verified 0 importers |
| S2 | `internal/gascity` + `internal/bridge/gc.go` + `worktree_gc_test.go` | 3,399 lines | severed compat | **DELETE** (after packs/ grep) | High |
| S3 | `skills-codex/` + `skills-codex-overrides/` | ~72k md lines, 5.9M | parallel runtime | **GENERATE or DROP** | Med — needs decision |
| S4 | ~6 generators + ~15 drift-gate CI jobs | 15 of 65 jobs | projection police | **COLLAPSE** with sources | High |
| S5 | Doc sprawl (`AGENTS.md` vs `CLAUDE.md`; inline practice lists) | ~99 lines + dupes | redundant prose | **MERGE / cite** | Med |
| S6 | Legacy RPI lane (`rpi_loop_supervisor`, `rpi_c2_events`, `rpi_phased_tmux`, `rpi_parallel`) | 2,514 lines | tangled-but-live | **REFACTOR then delete** | Low — needs caller migration (soc-1gbpz) |
| S7 | Consolidation candidates (1-import pkgs: `compile`, `llmwiki`, `resteer`, `bench`, `scope`, `adapters`) | ~6,033 lines | over-fragmented | **MERGE inline** | Med |

**Net deletable now (S1+S2): ~8.4k lines of Go, zero behavior change.**
**Net collapsible (S3+S4): ~72k md + ~15 CI jobs + ~8 scripts, the real win.**

---

## S1 — Dead Go packages (DELETE, no refactor)
Verified `grep -rl 'internal/<pkg>"'` → 0 importers each:
```
cli/internal/safety/           1,470
cli/internal/wikiworker/       1,403
cli/internal/feedbackcompiler/   658
cli/internal/plans/              434
cli/internal/worker/             368
cli/internal/domain/             200   (NOTE: distinct from skills/domain; verify no cmd registration)
```
Risk: a package could be referenced by `cmd/ao` via blank import or registration, not path import. Mitigation: `go build ./... && go test ./...` after each `rm -r`. One PR, one bead.

## S2 — Severed GasCity compat (DELETE)
The gc-bridge glue (`gc_bridge.go`, `gc_events.go`, `rpi_phased_gc.go`) was already removed (soc-2rtm0). What dangles:
```
cli/internal/gascity/          3,108   (2 importers — both doctor diagnostic hints)
cli/internal/bridge/gc.go        291   (GCMinVersion compat)
cli/cmd/ao/worktree_gc_test.go  ~test  (tmux runID parsing tests)
```
Gate before delete: `grep -r "internal/gascity\|internal/bridge" packs/ plugins/`. If clean, delete + rewrite the 2 doctor hints to drop the `agentopsd` reference (consistent with thesis #1: no sovereign daemon).

## S3 — The triple skill tree (the biggest projection)
```
skills/                   75 dirs   82,733 lines   6.4M   ← SOURCE OF TRUTH
skills-codex/             75 dirs   70,371 lines   5.6M   ← hand-maintained parallel copy
skills-codex-overrides/   32 dirs    1,430 lines   340K   ← bespoke divergences
```
CLAUDE.md line 108: *"Codex skills are manually maintained."* This is the single largest maintenance tax in the repo: every skill edit must be hand-mirrored, then 7 CI gates (`validate-codex-*`) police the drift, then `regen-codex-hashes.sh` re-stamps hashes.

**Decision required (this is a thesis call, not a mechanical one):**
- **Option A — Generate:** build `skills/` → `skills-codex/` as a build artifact (regex tool-name swaps Edit→apply_diff etc. + the 32 overrides applied on top). Kills 7 gates + the hand-maintenance. Risk: codex semantics aren't purely mechanical.
- **Option B — Drop:** if Codex-runtime parity isn't a live product requirement, delete `skills-codex*` entirely. Kills 7 gates + 5.9M + the whole parity apparatus. Most aligned with "one runtime" (thesis #2).
- **Option C — Keep + own the cost:** status quo. Only defensible if Codex cross-runtime is a real, paying surface.

Recommend **A if Codex is a real target, B if it's aspirational.** Status quo (C) is the slop.

## S4 — Generators + drift gates (COLLAPSE with their sources)
~15 of 65 CI jobs exist solely to police derived artifacts. Each pairs a checked-in projection + a generator script + a drift gate. Collapse by making the projection generated-on-read or build-time-only (never committed):

| Projection (committed) | Generator | Drift gate | Fix |
|---|---|---|---|
| `registry.json` (131K) | `generate-registry.sh` | `registry-check` | generate at build, don't commit |
| `cli/docs/COMMANDS.md` | `generate-cli-reference.sh` | `cli-docs-parity` | build artifact |
| `skill-domain-map.json` | `generate-skill-domain-map.sh` | `validate-skill-domain-map-golden` | derive from registry |
| `skills/catalog.json` | `generate-skill-catalog.sh` | `check-skill-catalog-drift` | SKILL.md frontmatter is canonical |
| SKU catalog (JOIN of 4 files) | `generate-sku-catalog.sh` | `validate-sku-catalog-drift` | merge SKILL-TIERS.md + dispositions INTO frontmatter |
| `AGENTS-CI.md` | `generate-ci-jobs-table.sh` | `validate-ci-policy-parity` | manifest canonical, doc always regenerated |
| context-map vs SKILL.md frontmatter | — | `validate-context-map-drift` | merge into frontmatter |
| skill counts across 6 docs | `sync-skill-counts.sh` | doc-release-gate | single count, derived |

Each fix removes 1 committed file + 1 script + 1 gate. ~15 jobs → the validate.yml drops ~30% (80K → ~50K).

## S5 — Doc sprawl (MERGE / cite)
Mostly cleaner than the framing suggested. The 5 `AGENTS-*.md` files are a *clean* split (≤5% overlap). Real redundancy:
- `AGENTS.md` (99 lines) ~40% duplicates `CLAUDE.md` intro + index → reduce to index-only or delete.
- `PRODUCT.md` inlines an 8-practice list that `PRACTICE-REGISTRY.md` owns canonically → cite, don't inline.
- `registry.json`/`SDK-INVENTORY.md`/`.agents/` tracking: **NOT slop** — correctly managed (145M `.agents/` is 99% gitignored, 13 audit files tracked deliberately).

## S6 — Legacy RPI lane (REFACTOR, do not delete yet)
CLAUDE.md is explicit: these are load-bearing, deleting any breaks the build.
```
rpi_loop_supervisor.go   1,143   (rpiLoopSupervisorConfig, runRPISupervisedCycle — used by rpi_loop, rpi_nudge, 5+ rpi_phased_*, agentworker)
rpi_c2_events.go           313   (RPIC2Event, appendRPIC2Event — used by 8+ rpi_phased_*)
rpi_phased_tmux.go         375   (tmux helpers — used by rpi_phased_stream, rpi_nudge, rpi_serve)
rpi_parallel.go            683   (deprecated per comment, symbols still live)
```
This needs the caller-migration refactor tracked as **soc-1gbpz**, not a delete. Lowest-confidence, highest-effort. Do it last.

## S7 — Over-fragmented packages (MERGE inline)
1-import packages that add a package boundary for no reuse benefit:
```
internal/compile/   368  → fold into cmd/ao/compile.go
internal/llmwiki/  1649  → fold into internal/wiki/
internal/resteer/  1366  → fold into internal/evolve/
internal/bench/     259  → fold into retrieval_bench.go
internal/scope/     518  → 1 importer
internal/adapters/ 1873  → 0 imports, base types for ports/ (audit: inline or keep)
```
Lower priority — this is tidiness, not thesis. Do opportunistically.

---

## What is explicitly NOT slop (stop suspecting it)
- The 363k Go lines broadly — coherent domain modules, real tests.
- The large files (goals/commands.go 1387, etc.) — strategic modules, not generated padding.
- `.agents/` 145M — 99% gitignored by design; 13 tracked files are deliberate audit truth.
- The 5-way `AGENTS-*.md` split — clean modularization.
- 17 code-correctness CI gates + 12 structural + 8 proof gates — these earn their place.
