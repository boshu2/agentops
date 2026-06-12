# Codex validation - proposed bead set

Input files:

- `docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/beads-manifest.md`
- `docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/behaviors.md`

Method:

- Read every proposed bead in the manifest and compared its `ACCEPTANCE` block to the frozen B74-B93 behavior contract.
- Checked selection mechanically with:

```bash
for id in B74 B75 B76 B77 B78 B79 B80 B81 B82 B83 B84 B85 B86 B87 B88 B89 B90 B91 B92 B93; do
  printf '%s ' "$id"
  bats --count docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests -f "^$id:"
done
```

Result: every B74-B93 filter selected exactly 1 bats test.

## Verdict

**21/21 crank-ready on the concrete-invocable acceptance gate.**

Every proposed bead has an acceptance command that is copy-paste invocable from the repo root. None are pure prose, and none are merely "see spec." The weak spots below are sufficiency/scope problems, not missing-command problems.

## Per-bead grade

| # | Bead | Behavior | Concrete invocable acceptance? | Grade note |
|---:|---|---|---|---|
| 1 | `run2-coverage-manifest` | B91 | YES | Thin: B91 is concrete, but duplicate-id and red-on-assertion enforcement are delegated to the checker rather than directly asserted in the bead acceptance. |
| 2 | `run2-hermetic-check` | B92 | YES | Thin: validates the wrapper on synthetic residue/HEAD cases, but does not itself prove every mutating B78-B81/B85/B86 verifier is actually routed through the wrapper/scratch path. |
| 3 | `run2-fixture-gatekeep` | B74 | YES | Direct B74 bats filter covers the tracked `scripts/gate.d` fixture behavior. |
| 4 | `run2-audit-red` | B75 | YES | Direct B75 bats filter invokes the checked-in red-on-assertion audit. |
| 5 | `run2-b57-repair` | B76 | YES | Direct B76 bats filter exercises post-push and pre-push rerun paths plus the dead-conditional lint. |
| 6 | `run2-bead-d3-acceptance` | B77 | YES | Direct B77 bats filter reads live `br` state and executes/validates the B62 acceptance command. |
| 7 | `run2-regen-manifest` | B78 | YES | Slightly thin: concrete B78 filter covers strict format, under/over declaration, source overlap, and actual regen write set; broad-glob/comment-format edge cases are left to the checker. |
| 8 | `run2-count-markers` | B80 | YES | Direct B80 bats filter checks marker conversion, regen restoration, and `regen-all.sh --check`. |
| 9 | `run2-install-chain` | B82 | YES | Direct B82 bats filter covers chain preservation and push behavior. Minor thinness: the probe assertion is stronger for beads/cockpit ordering than for explicitly naming the guard segment in the probe log. |
| 10 | `run2-install-foreign-refuse` | B84 | YES | Direct B84 bats filter covers hookless install, foreign-hook refusal variants, zero byte changes, and recognized-chain unaffected path. |
| 11 | `run2-install-idempotent` | B83 | YES | Direct B83 bats filter covers idempotent rerun and guard-only upgrade. |
| 12 | `run2-install-crash-safe` | B93 | YES | Direct B93 bats filter covers injected write/rename/chmod failures, intact hook, temp cleanup, backup, executable bit, and idempotent backup retention. |
| 13 | `run2-install-verify` | B85 | YES | Thin and not cleanly bead-scoped: the same B85 test also requires rollout evidence that belongs to `run2-rollout-evidence`; defect injection checks defects exist but not that each distinct defect token is named. |
| 14 | `run2-lock-default` | B89 | YES | Direct B89 bats filter covers deterministic lock dir, origin canonicalization, purity, and same-identity serialization. |
| 15 | `run2-count-checker` | B79 | YES | Slightly thin: concrete B79 filter checks manifest presence, clean checker run, bad path, and rogue-doc sweep; duplicate count-doc entries are only implicitly guarded. |
| 16 | `run2-gate-parity` | B81 | YES | Direct B81 bats filter invokes the structural parity checker and negative cases for comment, duplicate, empty list, and missing family. |
| 17 | `run2-land-bin-seam` | B88 | YES | Thin: concrete B88 filter proves helper override and installed push-time consult, but does not set `LAND_BIN` during install as the behavior text requires. |
| 18 | `run2-doctrine-flip` | B86 | YES | Direct B86 bats filter runs the doc sweep, checks pinned docs, and plants a negative direct-push instruction in a clone. |
| 19 | `run2-arpk-disposition` | B87 | YES | Thin: concrete B87 filter reads live `br`, but does not run `bv` triage and relies heavily on body greps rather than verifying the full status/label/dependency state described in the behavior. |
| 20 | `run2-bead-sweep` | B90 | YES | Direct B90 bats filter checks fail-closed modes and runs the live bead-acceptance sweep. |
| 21 | `run2-rollout-evidence` | B85 | YES | Concrete terminal acceptance: B85 bats filter plus `scripts/check-rollout-evidence.sh`. It is the right home for the live-clone evidence portion, but shares the same broad B85 test with bead 13. |

## Thin ones

- `run2-install-verify` (B85): concrete but not bead-scoped; uses the same B85 filter that requires rollout evidence from a later bead, and only checks that defects exist rather than that each pinned defect token is distinct/named.
- `run2-rollout-evidence` (B85): concrete, but coupled to the same broad B85 bats test as `run2-install-verify`; acceptable as terminal evidence, weak as independent bead accounting.
- `run2-hermetic-check` (B92): tests the wrapper, not the full routing obligation that all mutating real-repo verifiers use it or an equivalent scratch/disposable path.
- `run2-coverage-manifest` (B91): concrete, but too much of the no-prose/red-on-assertion guarantee is delegated to the checker without direct bead-level negative coverage for duplicate ids and mapped-test redness.
- `run2-arpk-disposition` (B87): concrete, but misses the required `bv` triage assertion and verifies machine state mostly via `br ready` plus body text.
- `run2-land-bin-seam` (B88): concrete, but misses the "LAND_BIN set during install and subsequent push" shape; it proves push-time consult after install.
- `run2-regen-manifest` (B78): concrete, but does not directly exercise every strict-format edge named in the behavior, especially overbroad glob/comment-line policy.
- `run2-count-checker` (B79): concrete, but duplicate-entry rejection is implicit rather than directly negative-tested.

## Single biggest systemic gap

The manifest proves invocability by pointing nearly every bead at one `bats -f '^Bxx:'` filter, but it does not always keep the executable acceptance bead-scoped. The clearest example is B85 split across `run2-install-verify` and `run2-rollout-evidence`: bead 13 cannot fully satisfy its own acceptance without bead 21's live rollout evidence. That means the set is crankable, but closure accounting can still drift unless broad scenario tests are split into bead-local filters or the bead dependencies/acceptance text explicitly mark which command is terminal-only.
