# Skill prune recon — razor pass (2026-06-10)

**Razor (Bo, 2026-06-10):** AgentOps = the inner loop — produce work end-to-end.
Anything in the repo that isn't the inner loop is either Mount Olympus's (the outer
gate), a Jeff-tool adapter (adopt, don't own), or cruft.

**Usage ground truth:** all explicit skill invocations across 880MB / 27 projects of
Claude transcripts (`<command-name>` mining). Caveats: (a) auto-fired Skill-tool
invocations and Codex-side usage are under-counted; (b) skills referenced *by other
skills* (catalog/registry roles) legitimately show zero direct invocations. Zero ≠
dead; zero = no recorded demand — confirm cross-references before retiring.

## Headline

- **~30 skills carry 100% of recorded explicit demand.** Top: discovery (105),
  goals (35), rpi (32), post-mortem (29), operationalizing-expertise (13),
  evolve (12), crank (10), council (10), research (8), idea-wizard (6).
- **The RPI/flywheel core IS the inner loop in practice** — usage confirms the razor.
- **80 resident skills have zero recorded invocations ever.**

## Buckets (166 resident skills)

| Bucket | Count | Disposition |
|---|---:|---|
| INNER-LOOP | 57 | Keep in agentops; sharpen; modularize the 14 oversized bodies |
| OLYMPUS (outer gate) | 7 | Move to mt-olympus: council, red-team, multi-model-triangulation, eval-outcomes, cross-vendor-trust-gate, bead-completion-audit, beads-compliance-and-completion-verification (+ review: project-reality-check, silent-novice-test) |
| JEFF-ADAPTER | 18 | Thin to adapter (point at tool's own --help/robot docs + AgentOps doctrine only): ntm×3, using-atm, vibing-with-ntm, agent-mail, beads×4, cass, cass-memory, dcg, caam, rch, acfs, sbh, ubs |
| RUNTIME-ADAPTER | 16 | agy-×7, cc-×5, codex-×4 — consolidate to ~3 (one per runtime); these are inner-loop *bindings*, not 16 distinct skills |
| CRUFT-CANDIDATE | ~68 | Zero-usage generic-expertise tail (rust-×6, codebase-×6, testing/fuzz/metamorphic/golden, perf/triage one-offs, gh-/gcloud/ssh vendor cribs, *-hygiene-sweep, etc.) — retire to an archive branch or library repo; not the inner loop |

## Notable judgment calls (not mechanical)

- `using-agentops` — zero invocations but it's the catalog other skills require. KEEP.
- `swarm`, `codebase-audit`, `workflow-builder` — low-but-nonzero usage + registry-listed. KEEP or merge into a sibling.
- `skill-builder`/`heal-skill`/`skill-auditor`/`forge`/`curate` — the producer chain. KEEP but **add the missing editor**: an admission gate (new skill requires demand evidence) + a periodic usage-ranked GC. This is what prevents regrowth.
- `system-performance-remediation`, `storage-watchdog-ops`, `system-tuning` — fleet ops, not the inner loop → dotfiles/fleet-ops territory, not agentops.

## Fan-out cut (wave plan, one lane per cluster, no file overlap)

1. **Lane A — OLYMPUS extraction** (7-9 skills → mt-olympus). Clean dir moves + catalog updates.
2. **Lane B — Jeff-adapter thinning** (18 skills). Body → thin adapter; biggest token win (ntm 656, cass 524, vibing-with-ntm 484 lines).
3. **Lane C — runtime-adapter consolidation** (16 → ~3).
4. **Lane D — cruft retirement** (~68 skills → archive). Mechanical after a cross-reference sweep (grep each candidate's name across skills/docs/cli for inbound links).
5. **Lane E — the editor** (new): admission gate + usage-GC wired into evolve/flywheel.

Each lane = one bead, worktree-isolated, catalog (`using-agentops`) updated last by a single closer to avoid collisions.
