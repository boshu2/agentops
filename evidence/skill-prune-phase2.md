# Skill prune — Phase 2 execution (disposition ledger)

Bead: ag-if7p (ledger, Lane D) / ag-pj51 next-work item 1 — **Bo-gated, gate granted 2026-06-11** (interactive session, AskUserQuestion "Phase 2 skill cull" selected).
Ledger: `evidence/skill-prune-dispositions.md` (45 RETIRE, 13 MERGE-INTO).
Archive: branch `archive/skill-prune-phase2-20260611` (pre-cull tree @ origin/main).

## Ledger reconciliation

49 of the 58 RETIRE/MERGE rows were already executed by Phase 1's product-audit
commit (5e4f7e58a, retire 52). Remaining on disk: **9**.

| Skill | Disposition | Action taken |
|---|---|---|
| codebase-archaeology | MERGE-INTO:codebase-audit | distilled into references/REPORT-MODES.md; dir + codex twin removed |
| codebase-briefing-report | MERGE-INTO:codebase-audit | same |
| codebase-pattern-extraction | MERGE-INTO:codebase-audit | same |
| codebase-report | MERGE-INTO:codebase-audit | same |
| codebase-risk-audit | MERGE-INTO:codebase-audit | same + its context_rel (supplier-to plan/validate) adopted by codebase-audit |
| expertise-to-procedure | RETIRE | removed; its `skill` artifact consumer (cross-vendor-trust-gate) rewired to `converted-skill` (skill-builder) |
| idea-option-forge | RETIRE | removed |
| release-readiness-gate | RETIRE | removed |
| research-software | RETIRE | removed (incl. agy plugin list + bundle) |

Skill count: 114 → **105** (97 user-facing + 8 internal).

## Inbound-ref sweep (tests/ + scripts/ + .github/ — per learning 2026-06-11-skill-extraction-must-sweep-test-couplings)

- `scripts/skill-flow-standalone.txt`: 3 stale entries removed.
- `scripts/validate-agy-plugin.sh`: 10 stale core_skills removed (9 were Phase 1
  misses — script was failing on main); `.agy-plugin/skills/` bundle pruned to 17
  + 9 drifted SKILL.mds resynced. Validator now PASSES (was failing).
- `tests/docs/broken-links-allowlist.txt`: matches belong to the research skill's
  own reference files (same basename) — no change needed.
- zsh footnote: the first sweep pass used unquoted `$names` word-splitting, which
  zsh doesn't do — silently swept nothing. Re-ran with explicit splitting.

## Registry regeneration + gates

- `skills/SKILL-TIERS.md`: 61 stale rows removed (166 → 105; Phase 1 had not updated it).
- `scripts/sync-skill-counts.sh`: PASS, 12 files synced to 105.
- `skills-codex-overrides/catalog.json`: 114 → 105 entries; `skills-codex/.agentops-manifest.json`: 180 → 171.
- `scripts/refresh-codex-artifacts.sh`: full chain PASS (hashes, override coverage, lifecycle, parity).
- `docs/contracts/context-map.md`, `docs/reference/agentops-skill-domain-map.md`, skill catalog: regenerated, drift gates pass.
- `docs/contracts/skill-dispositions.yaml`: 9 rows removed (Phase 1 precedent).
- `scripts/validate-skill-flow.sh`: **GREEN — was red on main** with 5 orphans
  (Phase 1 fallout). Cull removed 3; `codebase-audit` wired with real edges;
  `handoff` allowlisted as standalone (session-continuity artifact writer).
- frontmatter / schema / isolation / body-refs gates: all pass.

## Residual / not done here

- 37 stale `skill-flow-standalone.txt` allowlist entries (WARN, non-fatal) — pre-existing; separate hygiene pass.
- Lane E (admission gate + usage-GC) and the skills-codex merge driver: separate next-work items, still queued.
- Bead close blocked on the br tracker migration (concurrent lane); cite this file as proof when closing.
