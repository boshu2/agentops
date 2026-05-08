# Previous-run audit (anchor: nightly merge commit 60911ff)

Anchor SHA: 60911ff (Nightly 2026-05-07 #252 merged to main)
Anchor branch `origin/nightly/2026-05-07` no longer exists (deleted post-merge);
audit anchored on the merge commit instead.

## Fitness comparison

- Anchor final: code-driven 19/19 passing, 1 skip, runtime-artifact 2/2 passing
  (transient — Dream wrote `.agents/defrag/latest.json` during the run; that
  file is gitignored and does not propagate to main).
- Today's baseline: code-driven 19/19 passing, 1 skip, runtime-artifact 0/2.
- Code-driven delta: **0** (steady at 100%).
- Runtime-artifact flips: 2 fail → expected, NOT counted in headline.

## Anchor failures resolved

None — anchor had zero code-driven failures.

## Anchor failures persisting (expected, runtime-artifact)

- `compile-freshness` w=4 (tagged runtime-artifact; flips every run, excluded
  from headline).
- `compile-no-oscillation` w=4 (same).

## Regressions (was passing, now failing)

None on code-driven goals. Two runtime-artifact goals flipped fail, which is
the steady-state behaviour for these goals between Dream invocations.

## PRs merged since anchor

11 commits since 60911ff (3 PR merges + 8 direct chore/fix commits on main):

- #264 Wave 1A-D: factory-claim-ledger reconciliation (soc-e4ulx)
- #263 fix(hooks): reconcile stale .agents/.gitignore deny-all with parent allowlist (soc-rv5p)
- #262 docs(positioning): wiki-framing sweep across all surface docs (soc-9xn0)
- ce9d42f fix(pre-push): keep fast two-pass advisory scoped
- #261 CI toil-reduction sweep: registry determinism + codex auto-refresh + local CI dry-run (3 beads)
- 0143220 chore(registry): correct knowledge_stores to match CI checkout
- 6e4f866 fix(heal,lint): recognize cross-skill `../<sibling>/references/foo.md` paths (closes soc-8gi2)
- ab5cc4f chore(registry): correct swarm reference_count after worker-specs.md add
- c283648 chore(codex): refresh swarm hashes after worker-specs cross-link
- 48774a8 docs(swarm): cross-link worker-specs.md from /swarm
- fb77ea0 chore(registry): refresh registry.json after worker-specs.md copy

## Notes

- Yesterday's audit noted "tomorrow's nightly will have a real anchor"; that
  property is preserved — `.agents/nightly/2026-05-07/baseline-goals.json` and
  `final-goals.json` are tracked in main and were diffed directly.
- bd remains unavailable (canonical VM image fix is one-time repo work).
- `.agents/rpi/next-work.jsonl`, `.agents/findings/registry.jsonl`,
  `.agents/evolve/cycle-history.jsonl`, and `.agents/goals/*/attempts.jsonl`
  are absent in the working tree — corpus is dormant. This is the same
  precondition that drives `flywheel-compounding` to SKIP.
