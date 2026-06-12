# ag-s43tg — Resume Plan: Finish the Prune, Aim It at the MTO Worker Image

**Written:** 2026-06-12, after operating-loop run `wf_94fd6da2-b09` (167 agents,
~7.4M tokens, 98 min) died on the session limit with only S24 + closeout left.
Combines the workflow's validated state with the mt-olympus/olympusd integration
context (`mt-olympus/docs/AGENTOPS-MTO-INTEGRATION.md`).

---

## 1. State ledger (verified on disk, not self-reported)

**DONE**
- **21/21 fold augments green** on per-target branches `aug/<target>` (e.g.
  `aug/validate` = 8bf369ef1, `aug/review` = dfe6ba55a, `aug/cass` = c62a0a9fe).
  Each grafts the sources' pinned trigger phrases into the target SKILL.md;
  graded against the frozen manifest, not post-hoc judgment.
- **S0 artifacts staged:** `docs/plans/ag-s43tg-trigger-manifest.md` (39 pinned
  fixtures, RED-verified pre-augment) + `docs/plans/skill-prune-phase2.md` now
  tracked (was untracked). Headline corrected: **21 targets** (not 22), 40
  removal dirs (39 fold sources + 1 cut `reverse-engineer-rpi`), post-pass
  count = 68 entries (67 skills + `_fixtures`).
- **Gate rehearsal ran:** bd preamble neutral (bd retired); the SKU drift
  reproduces with edits stashed — it is the pre-existing un-regenerated
  `account-rotation` debt (local commit 3384f5638, currently `main` ahead-1),
  which the final pass clears.
- **Council verdicts:** go-with-changes ×3 (pre-mortem, scope, correctness);
  changes already absorbed into the slice plan (S0 freeze, ledger
  rows-never-deleted policy, BC5 amendment for `cli/embedded/skills/using-agentops`).

**PARTIAL (dirty working tree on main — the interrupted S24 single-writer pass)**
- `docs/contracts/skill-dispositions.yaml`: new `historical:` mapping (terminal
  rows for the phantom `validation` + `security-suite`; policy = flip rows to
  `merged-into:<target>`/`cut`, never delete).
- caam codex-twin deletions + codex catalog/manifest edits (account-rotation
  parity regen, partially applied).
- Nothing committed; no source dirs removed (`ls skills | wc -l` still 108).

**UNTOUCHED**
- Merging the 21 `aug/*` branches into main.
- The ~40 source-dir removals (`skills/`, `skills-codex/`, `images/*/skills/`,
  `cli/embedded/skills/using-agentops` + its Makefile recipe +
  `validate-embedded-sync.sh` enumeration — the declared BC5 carve-out).
- Full regen (`convert.sh` twins, `append-codex-override-entry.sh`,
  `regen-all.sh`, `generate-skill-domain-map.sh`), gate-green, the ONE atomic
  commit, push. Closeout (bead ag-s43tg, post-mortem, ratchet).

## 2. Finish-S24 sequence (single writer, inline — no swarm needed)

The remaining work is mechanical and ordered; it is exactly what the plan's
hard constraint reserves for one writer:

1. **Merge** the 21 `aug/*` branches into main (disjoint SKILL.md files; trivial).
2. **Remove** the 40 source dirs + the BC5 embedded copy of `using-agentops`
   (+ Makefile cp recipe + embedded-sync enumeration + its
   `docs/contracts/critical-skills.txt` line).
3. **Ledger:** flip all 39 `merge-review` + 1 `cut-review` rows to terminal
   states with date; keep the drafted `historical:` rows.
4. **Regen everything:** codex twins for changed survivors → override entries →
   `regen-all.sh` → `generate-skill-domain-map.sh`. This same pass clears the
   account-rotation parity debt riding on unpushed 3384f5638.
5. **Gate:** `bash .git/hooks/pre-push origin x </dev/null` must exit 0
   (heal-skill --strict, schema/frontmatter, body-refs, skill-flow closure,
   scenario-test linkage, regen-all --check, context-map drift). Fix-forward on
   red; the gate naming a stale surface is the designed trip-wire, not a thing
   to disarm.
6. **One atomic commit** (removals + ledger + regen + the two tracked plan docs
   + trigger manifest), push main (carries 3384f5638 with it).
   `git revert <sha>` restores everything together.
7. **Verify:** `ls skills | wc -l` = 68; pinned-manifest grep all-GREEN;
   `link-skill --all --relink`; `validate.yml` green on main.
8. **Closeout:** `BEADS_DIR=$PWD/_beads br close ag-s43tg` + post-mortem.

**Verdict on the original "disarm the contract tests" ask:** not needed and not
wanted. Every gate trip observed in rehearsal was either (a) sequencing (fixed
by single-writer + regen) or (b) pre-existing debt (account-rotation). The
gates are the factory's immune system — the same fail-closed shape olympusd
enforces at fleet scale. Inputs change; gate logic doesn't (intent non-goal #3).

## 3. MTO alignment — why this prune is integration work

Per `AGENTOPS-MTO-INTEGRATION.md` (spec frozen): AgentOps narrows to the
**worker-intelligence layer** olympusd dispatches — worker-as-image, with
`ao` + **skills on the worker PATH** + `.agents/` corpus; MTO/Themis is the
sole writer of binding verdicts; `ao validate`/`ao gate` outputs are **claims
in evidence.json**, never verdicts.

- **The pruned corpus IS the worker image.** 105 → 67 skills is a direct cut to
  what every dispatched MTO worker carries. The spec still says "82 skills" —
  stale after this lands.
- **Fold survivors match the narrowed core:** `validate`/`review`/`council`
  survive as claims-producers (in-session usefulness), consistent with the
  binding gate living in olympusd. Do not grow verdict semantics into them.
- **Registry discipline is the contract surface:** WS6 plans distributing
  olympusd *through* the AgentOps channel (v0.5+); `registry.json` +
  the regen gates are what make that channel stable.

## 4. Follow-up beads to file (not in this arc)

1. **Worker-image manifest** — declare which of the 67 survivors ship in the
   MTO dispatch image (vs operator-only skills like wealth-mentor, dad-nas);
   update AGENTOPS-MTO-INTEGRATION.md §5's "82 skills" figure. (Coordinates
   with mt-olympus WS4/WS6.)
2. **Fix the operating-loop workflow's reviewer seat** — it dispatches subagent
   type `agentops:code-reviewer`, which doesn't exist in the harness; every
   slice burned 1-3 retries on it before self-repairing with a valid type.
3. **The 7 `refactor` + 27 `update` dispositions** — explicitly out of scope
   here (non-goal #1); next coherent arc after the count lands.
4. **67 → 77-target reconciliation** — memory says target 77, this pass lands 67;
   confirm the delta is intentional (it over-achieves; if some folds prove too
   aggressive, un-fold by reverting the specific aug commit, not the pass).
