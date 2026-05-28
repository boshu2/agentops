# Strangler Plan — order of operations

> First-principles teardown, 2026-05-28. Companion: [spine.md](spine.md), [slop-map.md](slop-map.md).
> Strangler-fig, not rewrite: keep the artifact shipping and CI-green at every step. Each wave is a coherent arc = one PR (per repo workflow). Ordered by confidence × independence: highest-confidence, zero-coupling cuts first.

## Sequencing principle
Cut in this order so each wave de-risks the next and nothing rolls back the prior:
1. **Dead weight first** (no behavior, no coupling) — proves the teardown is safe, builds momentum.
2. **Severed compat** (already-orphaned) — closes the gc-bridge removal cleanly.
3. **Projection collapse** (the real win) — kills generators + gates together. Highest leverage, medium risk.
4. **Skill-tree decision** — the biggest single reduction, but needs an operator thesis call (Option A/B/C).
5. **Refactor-gated deletes last** — legacy RPI lane needs caller migration; do it when nothing else is in flight.

## Waves

### Wave 1 — Delete dead packages  · `chore` · 1 PR · LOW risk
- `rm -r` the 6 zero-import packages (S1): `safety`, `wikiworker`, `feedbackcompiler`, `plans`, `worker`, `domain`.
- Gate: `cd cli && go build ./... && go vet ./... && go test ./...` after each removal.
- Watch: blank-imports / cmd registration (esp. `internal/domain` vs `skills/domain`).
- **−5,033 lines. Zero behavior change.** Bead: new `chore`, `#trivial` candidate.

### Wave 2 — Delete severed GasCity compat · `refactor` · 1 PR · LOW risk
- Pre-gate: `grep -r "internal/gascity\|internal/bridge" packs/ plugins/` → must be empty.
- `rm -r internal/gascity internal/bridge/gc.go cli/cmd/ao/worktree_gc_test.go`.
- Rewrite the 2 `doctor/fix_bridges.go` hints to drop `agentopsd` references (thesis #1: no sovereign daemon).
- **−3,399 lines.** Closes the soc-2rtm0 arc. Bead: link `discovered-from` soc-2rtm0.

### Wave 3 — Collapse projections to single sources · `refactor` · 2–4 PRs · MED risk
The leverage wave. Each sub-PR removes one committed projection + its generator + its drift gate. Order easiest-first:
1. **context-map → SKILL.md frontmatter.** Merge the fields; delete `validate-context-map-drift`.
2. **skill counts.** One canonical count, all 6 docs derive it; delete `sync-skill-counts.sh` + the doc-release count check.
3. **catalog.json / skill-domain-map.json → derived-on-read** from frontmatter; delete `check-skill-catalog-drift`, `validate-skill-domain-map-golden`.
4. **registry.json + COMMANDS.md → build artifacts** (gitignore them, generate in CI/build); delete `registry-check`, `cli-docs-parity`.
5. **AGENTS-CI.md** already generated — make manifest canonical so it's *always* regenerated, never hand-edited.
- Gate each: the deleted CI job must be gone AND the remaining build must be green AND the artifact must still be produced where consumers need it (plugins read `registry.json` — confirm build emits it before un-committing).
- **~15 CI jobs → gone. validate.yml ~80K → ~50K.** Beads: one per projection, share an epic.

### Wave 4 — Skill-tree decision · `refactor`/`feat` · scope depends on choice · MED risk
**BLOCKED ON OPERATOR DECISION (Option A/B/C in slop-map S3).** Do not start until Bo picks:
- **A (generate codex from skills/):** build a `skills/` → `skills-codex/` generator (tool-name swaps + apply the 32 overrides), gitignore `skills-codex/`, delete the 7 `validate-codex-*` gates + hand-maintenance. Largest engineering lift in the plan.
- **B (drop codex):** `git rm -r skills-codex skills-codex-overrides`, delete 7 gates + the parity scripts. **−72k md lines, −7 gates.** Fastest, most aligned with one-runtime thesis.
- **C (keep):** no work; accept the tax. Only if Codex is a real paying surface.
- Bead: blocked until decision; tag `bd human`.

### Wave 5 — Merge over-fragmented packages · `refactor` · 1 PR · LOW risk
- Fold the 1-import packages (S7) inline: `compile`, `llmwiki`, `resteer`, `bench`, `scope`; audit `adapters`.
- Opportunistic tidiness, not thesis. Gate: build+test green.
- **~6k lines relocated (not deleted), fewer package boundaries.**

### Wave 6 — Legacy RPI caller migration · `refactor` · multi-PR epic · HIGH effort
- The soc-1gbpz refactor. Migrate callers off `rpi_loop_supervisor` / `rpi_c2_events` / `rpi_phased_tmux` / `rpi_parallel` symbols, THEN delete.
- Do this LAST, alone, nothing else in flight (touches 13+ files). Per repo rule: do NOT write new tests/features for these; migrate then remove.
- **−2,514 lines** once callers are clean.

## Session-scope guard
Repo workflow caps autonomous sessions at 2–4 PRs, post-mortem at ≥5. This plan is **6 waves = 6+ PRs**, so it spans multiple sessions by design. Suggested first session: **Waves 1–2** (−8.4k dead lines, two low-risk PRs, proves the teardown is safe). Waves 3–4 are the leverage but need the skill-tree decision and careful CI surgery — separate session(s).

## Tracking
File one epic + one bead per wave before cutting (bd is mandatory here). Each PR cites its bead, ships in a worktree, squash-merges green. Reference these three spec files as `Evidence:` in PR bodies.

## Honest expected outcome
- **Mechanical (Waves 1–2,5–6):** ~17k Go lines removed, ~121k remaining (≈88% of current). Real but modest.
- **Architectural (Waves 3–4):** the actual rebuild — ~15–22 CI jobs gone, ~8 generator scripts gone, up to 72k md lines gone, the projection-sync tax eliminated. This is where "rebuild from first principles" actually lands: not fewer lines of code, but **one source of truth per fact** so the meta-system stops policing itself.
