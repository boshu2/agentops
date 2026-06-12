# Beads Manifest — intent-amendment-pass-run-2-on-the-agent (REPAIRED)

> **Status: REPAIRED (id-scoped split) — drift-guard re-verified 2026-06-12.** Original defect
> (verdict 0.88, drift_guard=FAIL): the B85 acceptance was not bead-scoped — beads 13 and 21
> shared the single `^B85:` bats test, breaking 1:1 bead↔test parity (21 beads / 20 distinct
> tests) and leaving bead 13 red at its own close. An interim repair split the test but kept
> both under the `B85:` id (name-suffix filters); the final repair gives the second concern its
> own scenario id: **B94** (appended to `behaviors.md` §H as the split of B85's rollout-evidence
> concern; nothing else renumbered). `acceptance-tests/15-amend-install-chain.bats` now carries
> `B85: strict verify …` (bead 13) and `B94: rollout evidence …` (bead 21); the former trailing
> `&& scripts/check-rollout-evidence.sh` is folded INTO the B94 test body. Suite re-executed
> red-on-assertion (`1..21`, zero ok, zero harness crashes); totality gate checks B74..B94 each
> exactly once. Parity holds: 21 gate-passed keys == 21 manifest beads == 21 distinct tests,
> every ACCEPTANCE exactly one `bats -f '^B<n>:'` invocation selecting exactly one test. Thin
> acceptance sections named in [`validate-codex.md`](./validate-codex.md) carry explicit
> strengthen-the-test deliverables below.
>
> This manifest describes the gate-PASSED beads from [`beads.json`](./beads.json) (reconciled —
> beads.json now carries the same id-scoped `^B85:` / `^B94:` acceptance strings).
> Scenarios: **B74–B93 frozen + B94 (drift-guard split)**. All acceptance commands run from the repo root.
> Tracker writes (B77, B87, B90 rider) run as `br` from the main checkout
> `/Users/bo/dev/agentops` with `BEADS_DIR=$PWD/_beads` — **never from a worktree**.
> Run-1 spec §8 amendments (C16) were applied at spec-freeze, not by a bead. The former
> two-beads-one-scenario arrangement is dissolved: `run2-install-verify` owns B85 (strict
> verify, green at its own close); `run2-rollout-evidence` owns B94 (durable two-clone rollout
> evidence, the terminal lane).

## Conductor's mechanical findings (verbatim)

- coverage holes: **[none]**
- cycle_free: **true**
- rejected: **[]**

All 21 proposed beads passed the gate. 21/21 appear below.

## Created beads (tracker write 2026-06-12, run-local key → br id)

Written via `br` from the main checkout `/Users/bo/dev/agentops`
(`BEADS_DIR=$PWD/_beads`) after the drift-guard re-verification (red-red,
`1..21`/0 ok/0 crashes, 21↔21 parity). Overlap check against all open `ag-*`
beads: no existing run-2 bead; nothing merged or duplicated.

| # | Run-local key | br id |
|---|---|---|
| 1 | `run2-coverage-manifest` | `ag-run2-coverage-manifest-qc41n` |
| 2 | `run2-hermetic-check` | `ag-run2-hermetic-check-k9fea` |
| 3 | `run2-fixture-gatekeep` | `ag-run2-fixture-gatekeep-f1hw6` |
| 4 | `run2-audit-red` | `ag-run2-audit-red-0t6x6` |
| 5 | `run2-b57-repair` | `ag-run2-b57-repair-64mqb` |
| 6 | `run2-bead-d3-acceptance` | `ag-run2-bead-d3-acceptance-ckvzj` |
| 7 | `run2-regen-manifest` | `ag-run2-regen-manifest-x4ase` |
| 8 | `run2-count-markers` | `ag-run2-count-markers-wqiim` |
| 9 | `run2-install-chain` | `ag-run2-install-chain-ghu7y` |
| 10 | `run2-install-foreign-refuse` | `ag-run2-install-foreign-refuse-ti0kl` |
| 11 | `run2-install-idempotent` | `ag-run2-install-idempotent-rcee2` |
| 12 | `run2-install-crash-safe` | `ag-run2-install-crash-safe-v746h` |
| 13 | `run2-install-verify` | `ag-run2-install-verify-a754s` |
| 14 | `run2-lock-default` | `ag-run2-lock-default-3jh8t` |
| 15 | `run2-count-checker` | `ag-run2-count-checker-j944u` |
| 16 | `run2-gate-parity` | `ag-run2-gate-parity-1rp5p` |
| 17 | `run2-land-bin-seam` | `ag-run2-land-bin-seam-mlyd7` |
| 18 | `run2-doctrine-flip` | `ag-run2-doctrine-flip-xijbk` |
| 19 | `run2-arpk-disposition` | `ag-run2-arpk-disposition-n035w` |
| 20 | `run2-bead-sweep` | `ag-run2-bead-sweep-o1x4a` |
| 21 | `run2-rollout-evidence` | `ag-run2-rollout-evidence-uez2d` |

**Dependency wiring:** all intra-set edges from the per-bead **Deps** lines below
were wired 1:1 (`br dep add <child> <parent>`, run-local keys mapped to the ids
above; `br dep tree ag-run2-rollout-evidence-uez2d` verified). Cross-set edge:
`ag-run2-b57-repair-64mqb` → `ag-recovery-abort-iodl9` (the "run-1 land.sh
engine beads" cross-run prerequisite named in this bead's Why — B76's test
executes real crash-recovery lands; `ag-recovery-abort-iodl9` owns B57 and
transitively pulls the `ag-m8b-push-spine-ns7zw` engine chain). `br ready`
post-write: only `ag-run2-coverage-manifest-qc41n` ready (the DAG root) — correct.

**ag-arpk disposition applied (B87 prerequisite, 2026-06-12):** keep
merge-queue planned — dep edges added `ag-arpk` → `ag-meta-suite-closure-eg2yn`
(run-1 closure) and `ag-arpk` → `ag-run2-rollout-evidence-uez2d` (B94 terminal,
the re-evaluation trigger), disposition comment on the bead names the
host-local-lock residual and the residual-handling choice; status stays OPEN
(deferred-with-reason, now BLOCKED in triage, no longer unclaimed-ready).
`ag-run2-arpk-disposition-n035w` (B87) verifies/finalizes it mechanically.

---

## 1. run2-coverage-manifest

**Title:** B91 coverage manifest + checker: map every appended behavior B74-B94 to a mechanical verifier (C13)

**Why:** Spec §11.1: this gates every other bead — no appended behavior may be satisfied by prose alone. Creates `<run2>/coverage-manifest.txt` (one line per behavior B74–B94, kind in {bats,script,cmd}; each behavior id maps to exactly one test) and `<run2>/acceptance-tests/check-coverage-manifest.sh` that fails naming missing/duplicate ids, unselected bats refs, and missing/non-executable scripts. **Strengthen-the-test deliverable (codex thinness):** extend the B91 bats test with DIRECT negative coverage — a planted duplicate behavior id must be rejected by name, and a manifest entry whose mapped test is not red-on-assertion (prose/no-op verifier) must fail — asserted in the test body, not delegated wholesale to the checker.

**scenario_ref:** B91

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B91:'
```

**Deps:** (none — DAG root)

---

## 2. run2-hermetic-check

**Title:** B92 hermetic-verification wrapper scripts/with-hermetic-check.sh (C12)

**Why:** Real-repo verifiers must never dirty the operator checkout. Wrapper captures `git status --porcelain` + HEAD before/after the wrapped command, exits nonzero printing 'verifier residue' naming every leftover path / SHA mismatch, otherwise propagates the wrapped exit status. Records pre/post itself so a mid-run crash cannot hide residue. **Strengthen-the-test deliverable (codex thinness):** extend the B92 bats test beyond the synthetic wrapper cases with a ROUTING audit — mechanically enumerate every mutating real-repo verifier this run ships (the B78–B81, B85/B94, B86 scripts) and fail naming any that neither dispatches through `with-hermetic-check.sh` nor confines all writes to a disposable clone/scratch path; the full routing obligation is asserted, not assumed.

**scenario_ref:** B92

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B92:'
```

**Deps:** run2-coverage-manifest

---

## 3. run2-fixture-gatekeep

**Title:** B74 harness repair: track scripts/gate.d/.gitkeep in seed_fixture + literal self-check (C1)

**Why:** Root cause of the 10-test redirect-crash class: git cannot track an empty dir, so lane clones lacked `scripts/gate.d`. `seed_fixture` writes `.gitkeep` before its `git add -A` commit; `run-acceptance.sh`/`helpers.bash` gain a post-seed self-check failing with the exact string 'fixture defect: scripts/gate.d untracked'.

**scenario_ref:** B74

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B74:'
```

**Deps:** run2-coverage-manifest

---

## 4. run2-audit-red

**Title:** B75 standing red-on-assertion gate: `<base>/audit-red.sh` over the run-1 suite (C2)

**Why:** Makes red-on-assertion a mechanical standing gate, not a one-time fix: forces the SUT red via LAND_BIN (exit-97 placeholder), derives EXPECTED count from `bats --count` cross-checked against the B91 coverage manifest (never hardcoded 73), asserts zero ok / zero harness crashes, classifies every failure trace into a `@test` body (the ten previously-poisoned tests checked by name), and runs `find_identical_if_else` over the base suite. Spec §11.2: depends on C1 (crash-free base) and C13 (reads the manifest). **Repair-pass addendum (validation observation):** one transient lane-clone flake was observed in B82 during cross-family validation — exactly the crash class B75 outlaws; the trace classifier must treat any clone/setup flake as a harness crash (red verdict naming the test), never as an acceptable intermittent.

**scenario_ref:** B75

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B75:'
```

**Deps:** run2-fixture-gatekeep, run2-coverage-manifest

---

## 5. run2-b57-repair

**Title:** B76 repair the B57 dead conditional in `<base>/08-crash-recovery.bats`: materially distinct post-push vs pre-push arms (C3)

**Why:** B57's if/else had byte-identical branches (dead code). Rewrite: post-push crash phases (push, pre-release) assert rerun exit 0 + 'already landed' + unchanged remote patch-id set; pre-push phases (rebase, regen-write, regen-commit, gate) assert exit 0 + skill lands exactly once. No land.sh design change (run-1 M8b behavior); the audit-red lint keeps dead conditionals extinct. Full green also requires the run-1 land.sh engine beads (cross-run prerequisite, outside this DAG).

**scenario_ref:** B76

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B76:'
```

**Deps:** run2-audit-red

---

## 6. run2-bead-d3-acceptance

**Title:** B77 amend ag-d3-fixture-guard-yk7rq: runnable ^B62 ACCEPTANCE replaces the prose recipe (C14)

**Why:** Tracker edit via `br` FROM THE MAIN CHECKOUT `/Users/bo/dev/agentops` (`BEADS_DIR=$PWD/_beads`) — never from a worktree. ACCEPTANCE becomes exactly one runnable backticked criterion (`bats docs/plans/bdd-foundry/acceptance-tests -f '^B62'` plus required env); the ^B25 smoke proxy and every manual operative verb removed. Depends on the harness repair so the stated filter fails on assertion, not crash.

**scenario_ref:** B77

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B77:'
```

**Deps:** run2-fixture-gatekeep

---

## 7. run2-regen-manifest

**Title:** B78 real-repo regen write set: scripts/regen-manifest.txt + scripts/check-regen-manifest.sh (C6)

**Why:** Cutover lane. Strict-format manifest of regen-all.sh's fully-generated write set; checker derives reality from an ACTUAL regen run in a disposable clone (pass 1: written set ⊆ declared; pass 2: `git rm` declared, rerun, recreated == declared), rejects format defects and source-owned overlap naming the line. Includes the migration: regen-all.sh tolerates regenerating from a tree where its outputs are absent. Hermetic by construction (B92). **Strengthen-the-test deliverable (codex thinness):** extend the B78 bats test with the strict-format edges named in the behavior — an overbroad glob entry and a malformed comment line must each be rejected naming the offending line — not left implicit in the checker.

**scenario_ref:** B78

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B78:'
```

**Deps:** run2-coverage-manifest, run2-hermetic-check

---

## 8. run2-count-markers

**Title:** B80 generator-owned skill-count marker blocks in the ~11 prose docs + scripts/count-docs.txt + generator wired into regen-all.sh (C7 part 1)

**Why:** Every skill-count occurrence in the manifested docs converts to `<!-- count:skills -->N<!-- /count -->`; `scripts/sync-skill-counts.sh` becomes marker-driven and is invoked from regen-all.sh so edit-then-regen byte-restores values and `regen-all.sh --check` passes on the cutover commit. Sequenced after run2-regen-manifest: both edit `scripts/regen-all.sh` (shared write surface).

**scenario_ref:** B80

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B80:'
```

**Deps:** run2-regen-manifest

---

## 9. run2-install-chain

**Title:** B82 install engine v2 segment model: --install CHAINS onto the beads-managed pre-push hook + --hook-pre-push guard dispatch through the LAND_BIN seam (C5 §5.1-5.2)

**Why:** The real repo's pre-push IS a live beads chain — clobbering it is the catastrophic failure. Ordered-segment parser (beads segment byte-preserved, pre-push.local never opened, guard segment the only owned bytes), documented order printed by `--help`, guard body dispatches via `${LAND_BIN:-$(git rev-parse --show-toplevel)/scripts/land.sh} --hook-pre-push` (B63 nonce permit / B17 rejection). Head of the scripts/land.sh write-surface chain.

**scenario_ref:** B82

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B82:'
```

**Deps:** run2-coverage-manifest

---

## 10. run2-install-foreign-refuse

**Title:** B84 pinned foreign-hook policy: install on hookless, REFUSE on unrecognized — across all control-flow trap variants (C5 §5.1)

**Why:** Policy pinned at freeze (run-1 spec §8 amendment 1, already applied): exit nonzero matching 'not recognized' + 'chain manually', zero byte changes, no guard text anywhere under `.git/hooks/`, unconditional across exit-0 head / trailing exec / no shebang / non-executable variants; printed in `--help`. Serialized on the scripts/land.sh surface after run2-install-chain.

**scenario_ref:** B84

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B84:'
```

**Deps:** run2-install-chain

---

## 11. run2-install-idempotent

**Title:** B83 --install idempotency + guard-segment-only upgrade (C5 §5.3 part 1)

**Why:** Rerun with current guard: exit 0, 'already installed', hook byte-identical, backup untouched. Older guard version: rewrite ONLY the guard segment bytes, log 'guard upgraded <old> -> <new>', beads segment + pre-push.local sha256-identical. Serialized on the scripts/land.sh surface.

**scenario_ref:** B83

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B83:'
```

**Deps:** run2-install-foreign-refuse

---

## 12. run2-install-crash-safe

**Title:** B93 crash-safe install: temp-file + atomic rename, backup at .git/hooks/pre-push.pre-land-install.bak, LAND_TEST_INSTALL_FAIL fault seams (C5 §5.3 part 2)

**Why:** Every write path composes the full hook, writes `pre-push.tmp.$$`, chmod +x, backs up the prior hook (only when content changes), atomic `mv` — never in-place. Fault seams write/rename/chmod (`LAND_TEST_MODE=1`) abort AT that step: exit nonzero naming the failed step, surviving hook byte-identical, no tmp wreckage; `--help` documents backup path + recovery. Serialized on the scripts/land.sh surface.

**scenario_ref:** B93

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B93:'
```

**Deps:** run2-install-idempotent

---

## 13. run2-install-verify

**Title:** B85 strict --install --verify: pinned JSON keys, five named defect rejections, naked-clone diagnosis, host-local-lock residual in --help + scripts/check-rollout-evidence.sh (C5 §5.4 + C10 script)

**Why:** Machine-readable deterministic verify (guard_present, guard_version, chain, defects; exit codes in `--help`); distinct defect token each for stale version / duplicate segment / unpaired marker / missing exec bit / chain order — the bead-scoped test asserts the five reported tokens are pairwise DISTINCT (`sort -u` count == 5), not merely that defects are nonempty; pure resolution. Ships the rollout-evidence checker script (≥2 records, all keys, staleness via repo_sha blob-equality + guard_version match), proven bead-scoped: executable + rejects a stale SYNTHETIC manifest, with zero dependence on the checked-in evidence records (those are run2-rollout-evidence's B94 deliverable). **Bead-scoped done:** the `^B85:` test goes green at THIS bead's own close — no waiting on bead 21, no prose fallback.

**scenario_ref:** B85 (strict verify only — the rollout-evidence concern is split to B94)

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B85:'
```

**Deps:** run2-install-crash-safe

---

## 14. run2-lock-default

**Title:** B89 pinned LAND_LOCK_DIR production default: XDG state root + sha256 digest of the canonicalized origin identity (C5 §7)

**Why:** `lock_dir = ${XDG_STATE_HOME:-$HOME/.local/state}/land/<first-16-hex of sha256(canonical origin identity)>`; canonicalization (scp≡ssh, scheme/credentials/default-port stripped, lowercased host, trailing `.git` stripped, `file://`≡local path) pinned in `--help` with the word 'canonical'. Same identity ⇒ same lock_dir across the four GitHub spellings; `--status` stays pure and reports lock_dir; mutual exclusion flows through the default. Serialized on the scripts/land.sh surface.

**scenario_ref:** B89

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B89:'
```

**Deps:** run2-install-verify

---

## 15. run2-count-checker

**Title:** B79 scripts/land.sh --check-counts + repo-wide out-of-marker sweep + count-literal migration (C7 part 2)

**Why:** Validates count-docs.txt format naming the line, recomputes every marker-block value, sweeps `git ls-files -co --exclude-standard '*.md'` (tracked AND untracked — catches the planted rogue doc) for `\b[0-9]+\+?\s+skills?\b` outside marker blocks; migration fixes/manifests every existing out-of-marker literal so the sweep is 0 on the cutover commit. Also satisfies B79's `-x scripts/land.sh` assertion on the real repo (cutover of the run-1 D1 artifact). Serialized on scripts/land.sh after run2-lock-default; needs the marker conversion from run2-count-markers. **Strengthen-the-test deliverable (codex thinness):** extend the B79 bats test with a direct negative case — a planted duplicate count-doc entry must be rejected naming the duplicated path, not guarded only implicitly.

**scenario_ref:** B79

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B79:'
```

**Deps:** run2-count-markers, run2-lock-default

---

## 16. run2-gate-parity

**Title:** B81 validate.yml land-gate-families declaration + scripts/check-gate-parity.sh + land.sh --gate-families verb (C8)

**Why:** Exactly one workflow-level env key, each family token mapping to a real job/step; checker is a STRUCTURAL python3+PyYAML parse (never grep) rejecting commented-out/duplicate/empty declarations and unmapped or missing families; parity reads land.sh's list via the new `--gate-families` verb. CONSTRAINT (spec §11.3, recorded here per B81/B90 self-containment): the implementing lane MUST coordinate with or sequence after the active lane holding validate.yml — take an `am` reservation on `.github/workflows/validate.yml` before editing. Tail of the scripts/land.sh write-surface chain.

**scenario_ref:** B81

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B81:'
```

**Deps:** run2-count-checker

---

## 17. run2-land-bin-seam

**Title:** B88 one LAND_BIN invocation seam: run-1 helpers honor LAND_BIN, all 28 direct SUT invocations rerouted (C4)

**Why:** `land()`/`start_land()`/`status_json()` invoke `${LAND_BIN:-$lane/scripts/land.sh}`; every direct scripts/land.sh invocation in run-1 .bats files routes through the helpers (fixture-authoring heredocs exempt per `find_direct_sut_invocations`). The installed-hook half of the seam is run2-install-chain's guard dispatch (dependency); the spec's ao-land substrate note is already in run-1 spec §8 (C16, applied at freeze). **Strengthen-the-test deliverable (codex thinness):** extend the B88 bats test with the behavior's full shape — `LAND_BIN` exported DURING `--install` and honored at a SUBSEQUENT push through the installed hook (install-time seam + push-time consult), not only the post-install push-time consult.

**scenario_ref:** B88

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B88:'
```

**Deps:** run2-install-chain

---

## 18. run2-doctrine-flip

**Title:** B86 repo-wide landing-doctrine flip + scripts/check-doctrine-docs.sh (C9)

**Why:** CLAUDE.md Phases Land step + Branch+PR-shape Land row instruct scripts/land.sh; the seven sister docs name scripts/land.sh in every LIVE landing instruction; direct-push language survives only inside marked historical/superseded sections; sweep script greps the pinned doc list (each name literal in the script) for the operative phrases and documents the marker convention in its header. Depends on run2-install-chain so the hook description matches the installed chain order (beads segment + cockpit gate + land guard).

**scenario_ref:** B86

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B86:'
```

**Deps:** run2-install-chain

---

## 19. run2-arpk-disposition

**Title:** B87 disposition ag-arpk: keep merge-queue planned, sequenced after the land.sh epic, residual named (C15)

**Why:** Tracker edit via `br` FROM THE MAIN CHECKOUT `/Users/bo/dev/agentops` (`BEADS_DIR=$PWD/_beads`) — never from a worktree. Chosen path pinned by spec §6: keep merge-queue planned; `br dep add ag-arpk <land.sh epic id>` so `br ready --limit 0` and bv triage show it blocked, never unclaimed-ready. Body states deferral + re-evaluation trigger (cutover verified on both clones), the exact cross-host residual (host-local lock; merge queue the only listed serializer), and the residual-handling choice — machine state agrees with prose. **Strengthen-the-test deliverable (codex thinness):** extend the B87 bats test to verify MACHINE state first-class — `bv` robot triage (read-only) shows ag-arpk blocked/not-ready, and the dependency edge + status/labels are asserted via structured `br show` fields — with body greps demoted to corroboration, not the primary evidence.

**scenario_ref:** B87

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B87:'
```

**Deps:** run2-coverage-manifest

---

## 20. run2-bead-sweep

**Title:** B90 scripts/sweep-bead-acceptance.sh + normalize every landing-redesign bead body to full runnable commands (C11)

**Why:** Fail-closed sweep (`br` on PATH, main checkout `.git`, ledger dir, every id live-resolvable — any miss exits nonzero by name; never a skip, never cached prose) that extracts every ACCEPTANCE/regression command, rejects shorthand-only criteria, and EXECUTES each from a clean repo root with only the declared env — zero-selection, stale path, and harness death fail naming bead + command (shared audit-red trace classifier). Tracker rider via `br` from the main checkout: normalize the 16 existing + this run's new bead bodies (full bats path + filter + env). Depends on run2-audit-red (shared classifier, healthy harness) and run2-bead-d3-acceptance (its body is in the swept set).

**scenario_ref:** B90

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B90:'
```

**Deps:** run2-audit-red, run2-bead-d3-acceptance

---

## 21. run2-rollout-evidence

**Title:** B94 rollout evidence: run --install + --install --verify on BOTH live clones and check in `<run2>/rollout-evidence.jsonl` (C10)

**Why:** Spec §11.4 — the last lane, only after the C5-C9 cutover is on the real repo's main and both clones (Mac main checkout; bushido via `ssh bushido`) are pulled. One JSON record per host (host, repo_sha, guard_version, command, ISO-8601 timestamp, raw verify JSON), validated by `scripts/check-rollout-evidence.sh` (staleness rejected, not grandfathered). B94 is the drift-guard split of B85's rollout-evidence concern (behaviors.md §H). **Bead-scoped done:** the `^B94:` test — which itself executes the checker against the CHECKED-IN records from the real repo root and rejects a staleness-mutated copy — goes green. One invocation, one test: the former trailing `&& scripts/check-rollout-evidence.sh` is folded into the test body, not chained in the ACCEPTANCE.

**scenario_ref:** B94 (split from B85; terminal)

**ACCEPTANCE:**
```bash
bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f '^B94:'
```

**Deps:** run2-install-verify, run2-regen-manifest, run2-count-checker, run2-gate-parity, run2-doctrine-flip

---

## Dependency shape (summary)

- **Root:** run2-coverage-manifest (B91) gates everything.
- **Harness lane:** coverage-manifest → fixture-gatekeep → audit-red → b57-repair; fixture-gatekeep → bead-d3-acceptance; {audit-red, bead-d3-acceptance} → bead-sweep.
- **scripts/land.sh serialized write-surface chain:** install-chain → install-foreign-refuse → install-idempotent → install-crash-safe → install-verify → lock-default → count-checker → gate-parity. install-chain also fans out to land-bin-seam and doctrine-flip (off-chain, different surfaces).
- **Regen/count lane:** {coverage-manifest, hermetic-check} → regen-manifest → count-markers → (joins the land.sh chain at count-checker).
- **Tracker-only beads:** bead-d3-acceptance (B77), arpk-disposition (B87) — `br` from the main checkout only.
- **Terminal lane:** rollout-evidence (B94, the operational close) joins install-verify + regen-manifest + count-checker + gate-parity + doctrine-flip.
- **B85/B94 bead↔test parity (drift-guard repair):** the rollout-evidence concern is its own scenario id — install-verify owns `^B85:` (green at its own close), rollout-evidence owns `^B94:` (terminal). 21 beads ↔ 21 distinct tests, every ACCEPTANCE one id-scoped invocation / one test.
