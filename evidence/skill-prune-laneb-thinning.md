# Lane B — Jeff-adapter thinning ledger (ag-pj51)

Phase 1 of the skill prune. 10 of 18 Jeff-adapter skills exceeded 200 body lines and were
thinned to adapter form (AgentOps doctrine only; upstream-binary re-documentation replaced
with pointers at the tools' self-describing surfaces: --help / robot-docs / capabilities).
The other 8 (bead-tracker-migration 188, using-atm 185, dcg 185, beads-bv 175, beads-br 164,
ntm-review-worker-orchestration 151, sbh 133, acfs 131) were already ≤200 — untouched.

| Skill | Before | After | Δ |
|---|---:|---:|---:|
| ntm | 656 | 188 | -468 |
| cass | 524 | 156 | -368 |
| vibing-with-ntm | 484 | 193 | -291 |
| beads-workflow | 320 | 141 | -179 |
| agent-mail | 293 | 145 | -148 |
| rch | 266 | 163 | -103 |
| caam | 241 | 108 | -133 |
| cass-memory | 234 | 90 | -144 |
| beads | 211 | 153 | -58 |
| ntm-browser-test-coordination | 206 | 199 | -7 |
| **total** | **3435** | **1536** | **-1899 (-55%)** |

Gates (run independently by the orchestrator, not self-reported):
- heal-skill --check --strict: ALL 10 PASS (exit 0)
- skill-auditor: no regression — pre-existing output-spec-explicit FAIL unchanged on 9;
  ntm improved FAIL→WARN; rubric scores improved on 5 (e.g. cass 22→24, vibing 25→26)
- Frontmatter (incl. descriptions) byte-identical on all 10 (git diff grep)
- All existing references/*.md remain linked; one new reference added
  (vibing-with-ntm/references/DECISION-AIDS.md — re-homed decision tables)

NOT done in this lane (per Phase 1 hard stops): no skill deleted/retired, no Olympus
moves, catalog (using-agentops) untouched.
