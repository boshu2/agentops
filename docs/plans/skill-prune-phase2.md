# Skill Catalog Prune — Phase 2 Execution Plan (swarm-ready)

**Status:** Phase 1 (the plan) is committed — `docs/contracts/skill-dispositions.yaml`
holds 39 `merge-review` (folds) + 1 `cut-review` (cut). This doc is the **execution
work-list** the swarm consumes directly (bd is retired as of 2026-06-11 — see
"Gate preamble — bd neutrality" below; tracker is br at `_beads/`).

**Goal:** 105 → ~65 distinct agentops skills.

## ⚠️ Hard constraint — why this can't be naive-parallel

agentops is **factory-managed**: every skill change ripples through shared,
generated consistency artifacts. The shared-surface enumeration (expanded by S0 to
match S24's real write scope — was 6 items):

1. `registry.json`
2. `docs/reference/agentops-skill-domain-map.md` (S0: pinned to its full path — was listed bare)
3. `docs/contracts/skill-dispositions.yaml`
4. codex twins + hashes (`skills-codex/**`)
5. `skills-codex-overrides/catalog.json`
6. the hexagonal/context maps
7. `docs/contracts/critical-skills.txt` (S0 addition — S24 removes the `using-agentops` line)
8. `docs/contracts/skill-lease-audit.md` (S0 addition)

**A skill change hand-edited without the full regen
breaks ~7 pre-push gates** (proven 2026-06-11). So:

- **Content-augment (per fold-target) is parallelizable** — distinct target SKILL.md files.
- **Removals + regen + gate-green is a SINGLE sequential final pass** — one writer of the shared surfaces.

## Canonical fold recipe (each worker, per fold-target)

1. **Augment** the target `skills/<target>/SKILL.md`: graft any UNIQUE triggers /
   capabilities from each source skill into the target's `description` + body, so
   the model still fires the target for the sources' use-cases. If the target
   already covers a source, just confirm coverage (note it).
2. **Do NOT** remove source dirs or edit `skill-dispositions.yaml` — that's the
   final pass (shared single-writer surface).
3. **Gate (local):** `heal-skill --check --strict skills/<target>` exits 0.

## Pinned numbers (frozen by slice S0, 2026-06-11 — graded against, do not drift)

- **Fold-targets: exactly 21** (the table below lists 21 target rows + 1 cut; the old
  "22" headline was a miscount, fixed by row count).
- **Removal dirs: exactly 40** = 39 *existing* fold-source skill dirs + 1 cut
  (`reverse-engineer-rpi`). The work-list names 41 sources, but `validation` and
  `security-suite` are **phantoms** (no `skills/<name>/` dir exists) — excluded from
  removal counts; they only get disposition bookkeeping.
- **Post-pass count: `ls skills | wc -l` = 68** (from 108). Caveat: 68 includes the
  non-skill `_fixtures` entry (= 67 actual skills by the S0 freeze arithmetic);
  precisely, the 68 also counts the non-skill files `catalog.json` and
  `SKILL-TIERS.md`, so distinct skill *dirs* = 65 — consistent with the Goal line
  (105 → 65).
- **Grading fixture:** wave-1 workers are graded against the **pinned trigger-phrase
  manifest** `docs/plans/ag-s43tg-trigger-manifest.md` (one frozen phrase per the 39
  existing sources; `grep -Fqi` in the target SKILL.md flips red → green), NOT
  against phrases chosen post-hoc.

### Acceptance Row 1 — restated as the proxy it is

"No user-visible capability vanishes" **means**: every pinned manifest trigger phrase
remains grep-discoverable in its fold target (the pinned-manifest proxy), supplemented
by S24's routing spot-check. It is a proxy, not a semantic guarantee.

## The work-list (21 fold-targets + 1 cut)

| Target | ← folds in (sources) |
|---|---|
| `validate` | vibe, validation*, bead-completion-audit |
| `review` | bug-hunt, codebase-audit, ubs |
| `refactor` | complexity |
| `security` | deps, security-suite* |
| `council` | multi-model-triangulation, cross-vendor-trust-gate |
| `eval-outcomes` | scenario |
| `discovery` | brainstorm, design |
| `plan` | planning-workflow |
| `rpi` | operating-loop-skill, operating-loop-workflow |
| `crank` | burndown, ship-loop |
| `cass` | casr, cass-memory |
| `inject` | session-bootstrap, using-agentops |
| `status` | quickstart |
| `recover` | trace |
| `flywheel` | ratchet |
| `agy-native` | agy-mcp-plugins, agy-project-worktree-permissions, agy-rules-workflows, agy-sidecar-scheduled-tick |
| `cc-hooks` | cc-cron-ticks, cc-loop-driver, cc-subagents, cc-worktree-isolation |
| `codex-exec` | codex-goals, codex-mcp-plugins, codex-sandbox-evidence |
| `ntm` | ntm-browser-test-coordination, ntm-review-worker-orchestration |
| `pr-prep` | pr-research |
| `implement` | pr-implement |
| **CUT** | reverse-engineer-rpi (no target — just remove) |

\* `validation` and `security-suite` are NOT yet in `skill-dispositions.yaml`
(they weren't classified). Add their disposition rows in the final pass.

## S0 resolutions (recorded 2026-06-11)

### using-agentops — DECLARED REMOVED (folded into `inject`)

The earlier "survivor body scrub" framing is **dropped**. `using-agentops` is a fold
source like any other: its trigger surface folds into `inject` (pinned fixture in the
trigger manifest), `skills/using-agentops/` is one of the 40 removal dirs, the embedded
copy `cli/embedded/skills/using-agentops` is **deleted**, and its line in
`docs/contracts/critical-skills.txt` is removed — all in S24's atomic commit.

### Intent boundedContext amendment — narrow BC5 write authorized

This plan's bounded context is **BC4-Factory**, with one explicit carve-out:
**"BC5-Runtime limited to retiring `cli/embedded/skills/using-agentops` (+ its
`cli/Makefile` cp recipe and `validate-embedded-sync.sh` check_file enumeration) in
the S24 atomic commit."** No other cross-context write is declared or permitted.

### Gate preamble — bd neutrality

bd is retired (2026-06-11); the pre-push gate's bd preamble must be **neutral**:
`bd hooks run pre-push` exits 0 or 3, or `bd` is off PATH entirely — either is a
pass. Run with `BEADS_HOOK_TIMEOUT` set so a dead bd can't hang the gate.

## Final pass (ONE worker / orchestrator, after all augments)

1. Remove the **exactly 40** source skill dirs (39 existing fold sources + the cut
   `reverse-engineer-rpi`; phantoms `validation`/`security-suite` have no dirs):
   `git rm -r skills/<source>` + `skills-codex/<source>` + `images/*/skills/<source>`.
2. In `docs/contracts/skill-dispositions.yaml`, **flip rows to a terminal state
   (`merged-into:<target>` / `cut` + date); rows are never deleted** (and add
   `validation`, `security-suite` rows / handle them). *(S0 rewrite — the old "remove
   their rows" instruction contradicted the S23/S24 executed policy; the ledger keeps
   history.)*
3. For survivors with changed twins: `bash skills/converter/scripts/convert.sh skills/<name> codex` → land the twin into `skills-codex/<name>/`.
4. `bash scripts/append-codex-override-entry.sh <name>` for any new override gaps.
5. `bash scripts/regen-all.sh` + `bash scripts/generate-skill-domain-map.sh`.
6. **Also completes `account-rotation`** (local commit `3384f5638`) factory parity in this same pass — it has the identical un-regenerated debt.
7. **Gate MUST be 0 fail:** `bash .git/hooks/pre-push origin x </dev/null`.
8. `link-skill --all --relink`; commit; push.

## Dispatch

Work off `skill-dispositions.yaml` (`grep merge-review`) + this table. Suggested:
`atm spawn agentops --cc=N`, send each pane one fold-target ("augment `<target>`,
fold in `<sources>`, recipe in docs/plans/skill-prune-phase2.md"), then run the
final pass once all augments are merged. Tend to gate-green before push.

See memory `agentops-skills-are-factory-managed` for the factory-gate detail.
