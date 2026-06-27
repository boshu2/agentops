# Land Queue Pain Manifest

This manifest names the regressions the land-queue suite must keep dead. If a
future edit breaks one of these tests, the failed pain name below is the thing
that regressed.

| Pain | Proof tests |
|------|-------------|
| `rebase-race` | `tests/land-queue/land-lane.bats`: `lane lands 3 queued beads in order, each gated once, no force-push`; `a second concurrent lane refuses to start (singleton lock held)`; `watch lane exits on TERM, frees the lock, and only then admits a successor`; `drain is crash-safe: a re-run does not re-land already-done beads`. `tests/land-queue/e2e-acceptance.bats`: `integrated land queue kills rebase-race, catch-22, flaky-unrelated-red, and Actions dependence`. |
| `commit-bound-pawl catch-22` | `tests/land-queue/postrebase-pawl-stamp.bats`: `pawl-land rebases onto fresh origin/main before stamping and pushes once`; `pawl-land aborts a conflicting rebase cleanly and does not push`. `tests/land-queue/e2e-acceptance.bats`: `integrated land queue kills rebase-race, catch-22, flaky-unrelated-red, and Actions dependence`. |
| `flaky-unrelated-red` | `tests/land-queue/flaky-retry.bats`: `pkg extraction parses failing package and shuffle seed from race log`; `flake passing on package retry lands and files quarantine bead record`; `deterministic package retry dead-letters with failing package and seed`. `tests/land-queue/e2e-acceptance.bats`: `integrated land queue kills rebase-race, catch-22, flaky-unrelated-red, and Actions dependence`. |
| `github-actions-rate-limit` | `tests/land-queue/assert-no-actions.bats`: `startup guard passes for the real land scripts (no validate.yml config-assertion)`; `guarded local landing leaves gh run list count unchanged`; all runtime shim rejection tests for `gh workflow run`, `gh pr create`, `gh run rerun`, and unknown `gh run` verbs; the direct `guard-gh` allow/block matrix. `tests/land-queue/e2e-acceptance.bats`: `integrated land queue kills rebase-race, catch-22, flaky-unrelated-red, and Actions dependence`. |

Regression runner: `scripts/land-queue-test.sh` executes the complete
CI-independent suite: post-rebase pawl stamping, branch submit, lane behavior,
flaky retry, Actions-free guard, and integrated e2e acceptance.
