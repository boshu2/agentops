# Fitness-run verdict — the graduation ruler (age-gc-mvp-w2-nuiw.4)

**NUDGE COUNT = 5 → NOT-YET GRADUATED** (0 nudges = graduation). One real non-toy
quest — `csv-stats.sh` with a 4-clause default-FAIL contract and an 18-assertion
`test.sh` — was driven end-to-end through the **pack** membrane
(`packs/agentops-membrane`, imported into the warm native bd/dolt city `gc-city`
via a local plain-path import; doctor law0 + membrane-health green; builder=claude,
cross-family judges codex/gpt + agy/gemini). The **build lane graduated with zero
nudges**: the claude/opus builder set up its worktree, respected RBAC (read
`test.sh`, never `CONTRACT.md`), implemented a correct 49-line `csv-stats.sh`, and
turned `./test.sh` green (18/18) untended. All 5 nudges were in the
**verifier-delivery path**, mapping to 3 root gaps: **(A, pack-fixable, the
biggest)** the pack's `[steps.check].check.path = "membrane/close-gate.sh"`
resolves against the city root but `gc import` never materializes the gate scripts
into the city, so the control-dispatcher **quarantined and terminated every
workflow** on the check step (fix: a pack install hook that copies `membrane/`
into the city, or a pack-relative check path) — this forced NUDGE 1 (copy scripts)
and NUDGE 3 (manual `close-gate.sh`, since gc has no re-fire-a-quarantined-step
verb); **(B, config-fixable)** the agy lane blocked on a `gc prime` trust modal
(NUDGE 2 — pre-trust the provider); **(C, needs upstream gc, the deepest)**
`gc session submit` delivered the verification but the busy interactive agy pane
never consumed it — it queued behind AGY's own self-exploration loop, needing a
send-Enter (NUDGE 4, failed) then an esc-interrupt + submit (NUDGE 5, worked); the
durable fix is a **non-interactive/headless verifier lane** or a gc
submit→consume guarantee. The **door's own logic is sound**: gpt returned a clean
nonce-verified CONFIRMED/pass (read-only enforced, 6 evidence), the deterministic
`finalize.sh` fail-closed to **REFUTED** on a hard finding, and the quest bead
stayed **OPEN** (no false CONFIRM; human merges nothing). The run did **not reach
CONFIRMED** — and a **bonus pack-fixable finding** explains why more rounds
wouldn't help: the weak agy/gemini lane raised an **unsatisfiable false-positive**
("file outside quests/csv-stats/") caused by a **diff-frame mismatch** — the
review diff is quest-repo-relative (`csv-stats.sh`) while the contract non-goal is
city-relative — so a bounded redo loop would burn `max_attempts` on a file that is
already correctly placed. Native observability worked for the evidence trail
(`gc events` run_id/step correlation) but **cost was not captured** (`gc costs`
UNPRICED for sub-backed models; `.gc-home/usage.jsonl` absent). Wall time ≈ 25 min
(build ~10 min, gate+nudges ~15 min); 1 build round + 1 gate round. `~/.gc`
verified frozen (byte-identical, 494 files). **Disposition:** `gc-city` is a
**validated reference implementation, not yet a zero-touch lane** — Gaps A and the
diff-frame false-positive are fixable in our pack now (the `.6` residual-gap
helpers should absorb the city-materialization step); Gap C needs upstream gc or a
headless verifier lane. Full timeline + numbered nudge log: `gc-city/FITNESS-RUN.md`.
