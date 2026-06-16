# Evidence — M2: Codex/local recovery state-machine (age-d16-self-hosting-route-nkr.3)

**Bead:** `age-d16-self-hosting-route-nkr.3` — "Recovery state-machine for the Codex/local runtime."
**Arc:** Port the Claude-only fix-forward policy into the Codex/local runtime + add the two missing branches. THE unattended-blocker.

## Contract (the bead's scenarios)

1. Given a Codex/local run that hits a failure, When recovery triggers, Then it branches **fix-forward | re-scope-as-new-acceptance | andon** UNATTENDED and updates/closes the bead accordingly.
2. Given a re-scope branch, When taken, Then the failure becomes a new acceptance (new/updated scenario), with crisp terminal behavior (no spin, no silent defer, no mis-close).

## What existed (re-baseline, verified file-by-file)

- **Policy to port:** `.claude/workflows/ship-beads.js` (≡ `bead-crank.js`) — a per-PR RECONCILE state machine: `polling → flake-rerun [budget 1] → fix-forward (regen drift) → merge → confirm-merged → bead-closed`; terminal states `merged|blocked|abandoned`. **Claude-Workflow-only** — it spawns Claude subagents and runs in the Workflow engine; it cannot run on the bo-mac Codex/local runtime.
- **The Codex/local runtime** = `codex exec` + local shell. The only existing driver, `scripts/run-rpi-phases.sh`, does the banned **silent defer** on failure: `timeout … codex exec … || echo "Phase N timed out or failed"`. No recovery state machine exists on that side.
- **`ao beads stale` / `ao beads resume`** (`cli/cmd/ao/beads_{stale,resume}.go`) — claim-transfer recovery only; NOT failure-branching.
- `scripts/reconcile-pr.sh` — the M3 merge/pawl acceptance door (owns the binding "accepted"/close verdict). Recovery must NOT duplicate it.

## What M2 adds

`scripts/recovery-statemachine.sh` — a **pure-shell, Codex/local** recovery state machine (no Claude, no Workflow engine). It ports the fix-forward policy and adds the two missing branches:

| `--failure-kind` | branch | bead action | exit |
|---|---|---|---|
| `drift` / `flake` | **fix-forward** (remediate → recheck, bounded by `FIX_FORWARD_BUDGET=1`) | comment `recovered`; bead stays progressable | `0` |
| `rescope` (+`--rescope-scenario`) | **re-scope-as-new-acceptance** | `br create` a follow-up bead whose body IS the new scenario, `blocks:`-deps the original; label+comment the original | `0` |
| `hard` / `auto` / unknown | **andon** (default-safe) | label `andon` + comment reason; bead left OPEN | `3` |

**Crisp terminal invariants (structurally enforced, asserted by tests):**
- **No spin** — fix-forward is bounded by `FIX_FORWARD_BUDGET=1`; on exhaustion it *escalates to andon*, never loops. No unbounded loop exists in the script (`"spin":false` is emitted by construction).
- **No silent defer** — every path emits exactly one structured terminal JSON line AND performs exactly one bead mutation. Missing args / missing rescope scenario → andon (loud), never a quiet pass. Replaces `run-rpi-phases.sh`'s `|| echo "failed"`.
- **No mis-close** — recovery NEVER calls `br close`. Closing-as-done is the merge/pawl door's authority (`reconcile-pr.sh`, M3). rescope → original blocked-by-new (open); andon → open+labeled.

## Acceptance test (failure-injection, not happy-path)

`tests/scripts/recovery-statemachine.bats` — 10 cases, each driven by an **injected failure** with `br` and the recheck/remediate commands stubbed (deterministic, offline):

```
1..10
ok 1 fix-forward: red recheck recovers after one remediation -> recovered, exit 0
ok 2 fix-forward: recheck stays red -> escalates to andon, exit 3, bounded remediation
ok 3 rescope: files a new acceptance bead blocking the original -> rescoped, original not closed
ok 4 rescope: non-zero br create escalates to andon (no silent defer under set -e), exit 3
ok 5 rescope: missing --rescope-scenario falls to andon (no silent defer), exit 3
ok 6 andon: hard failure labels + comments the bead, does NOT close it, exit 3
ok 7 classify: unknown failure-kind defaults to andon (default-safe), exit 3
ok 8 usage: missing --bead exits 2
ok 9 usage: missing --failure-kind exits 2
ok 10 dry-run: andon decision emitted with NO bead mutation
```

- Case 2 asserts **no spin** (remediation ran ≤ `FIX_FORWARD_BUDGET`).
- Cases 4/5/10 assert **no silent defer** (failing `br create` / missing scenario both reach a loud andon; dry-run still decides).
- Cases 1/3/6 assert **no mis-close** (`! grep "^close"`).

### Merge-to-main pawl (fresh-context refuter) — REFUTED → fixed

The first commit was **REFUTED** by an independent fresh-context refuter: the rescope branch captured `new_id="$(br create …)"` directly, so under `set -euo pipefail` a non-zero `br create` (locked DB, stale-DB refusal, bad `--deps`) **aborted the script before the `|| andon` guard** — exit 7, no terminal line, no bead mutation = a **silent defer in production**, which the always-`exit 0` stub `br` could not surface. Reproduced directly (buggy: `exit 7`, no JSON; fixed: andon JSON + label + `exit 3`). Fixed by capturing inside an `if !` condition that escalates to andon; **case 4** locks the regression. Re-refute after the fix: holds.

## Gate

- `shellcheck -x scripts/recovery-statemachine.sh` — clean.
- `ao gate check --fast --scope head` — **19 checks, 19 pass, 0 warn, 0 fail** (`shell.shellcheck-changed` PASS on the new script).
- Full bats suite runs in CI (`validate.yml`) on push.

## Scope boundary

Non-goals held: no Claude executor, no parked Linux daemon, no new gate/ledger schema, no `br close` authority (M3 owns it). Downstream M4/M5 untouched. The live driver wire (replacing `run-rpi-phases.sh`'s silent defer with a `recovery-statemachine.sh` call on a bead-aware run) is the epic done-test's integration step, not this slice.
