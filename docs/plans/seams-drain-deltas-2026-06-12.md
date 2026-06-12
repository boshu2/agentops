# Seams epic → routing-drain deltas for the active main pass (ag-xwjlc / W2.1)

> Handoff per `.agents/council/2026-06-12-dirty-main-attribution.md`: the seams
> epic does NOT edit shared/regen surfaces while your pass is in flight — these
> are the deltas to fold into your single-writer pass (or into the next regen).
> Every count below was measured fresh 2026-06-12; re-verify at apply time.

## 1. Doc routing references (check with `scripts/check-doc-skill-refs.sh`, new on branch chore/ag-seams-wave013)

At committed HEAD (4d7248136), 3 live violations in docs/architecture/operating-loop.md:
- L82 `/validation` → `/validate` (fold landed; the doc still cites the retired name)
- L91 `/harvest` → `/curate` (**fold blessed by Bo 2026-06-12** — previously absent from the settled map)
- L91 `/retro` → `/post-mortem` (or mark as explicit retired-note)

The moment your uncommitted skill deletions land, ALSO dangling:
- L82 `/vibe` → `/validate` (settled fold)
- L91 `/ratchet` → `/flywheel` (your own skill-dispositions.yaml S24)
- L43 `/brainstorm`, `/design` → `/discovery` (settled folds)
- SKILL-TIERS.md: 18 vibe/ratchet/retro/harvest tokens at HEAD (regen surface — your regen covers it; verify with the doc checker after)

## 2. Frontmatter context_rel edges

At HEAD, `scripts/audit-skill-metadata.sh` reports 5 unresolved (all
session-bootstrap citing doc files as skill slugs — AGENTS.md,
AGENTS-WORKFLOW.md, AGENTS-CI.md, AGENTS-CODEX.md, AGENTS-RUNTIME.md; needs a
doc-ref field, not `with:`). After your deletions land, also expect:
beads→ratchet, red-team→vibe, security→vibe (+ any of acfs/agy-native/
codex-exec/release/workflow-builder edges whose targets your pass removes —
the host dirty tree measured 8 total). Re-run the audit post-commit; the
baseline-pin bats case in tests/scripts/audit-skill-metadata.bats carries a
clearly-marked EXPECTED_UNRESOLVED to update when you drain.

## 3. CI wiring (validate.yml is yours right now)

Add to the `contracts-sync` job (advisory first, strict after the drain):
```yaml
- name: skill-metadata edges (advisory until drain lands)
  run: bash scripts/audit-skill-metadata.sh || true   # flip: drop '|| true'
- name: doc skill refs (advisory)
  run: bash scripts/check-doc-skill-refs.sh           # add --strict to flip
```
Both scripts have full bats coverage on the branch (21/21 green).

## 4. bd-caller drain (all five files are dirty in your pass)

skills/{crank,status,using-atm,evolve,recover}/SKILL.md still instruct agents
to run `bd`. Each `bd` mention → `BEADS_DIR=$PWD/_beads br ...` or an
explicitly-marked retired-note (CLAUDE.md declares bd retired 2026-06-11).

## 5. beads skill fold

skills/beads/SKILL.md is bd-first and carries a context_rel edge → ratchet.
Recommend: fold into beads-br (now carries the persist_intent port contract,
see branch) + retire the dir, consistent with cull-skills-not-lines.

## 6. Catalog/disposition rows for the four new skills (branch chore/ag-seams-wave013)

operationalize, reality-check, toil-mining, continuity-loop need rows in
skills/using-agentops/SKILL.md + docs/contracts/skill-dispositions.yaml
(both yours). operationalize/reality-check/toil-mining should then flip to
`user-invocable: true` (authored false only because the catalog files were
lane-owned at authoring time).

## 7. Merge order for the branch

After your pass commits: merge `chore/ag-seams-wave013` → run the full regen
set (registry, context-map, codex twins, catalog, critical-skills) → pre-push
gate → push. The branch deliberately contains NO regen output.
