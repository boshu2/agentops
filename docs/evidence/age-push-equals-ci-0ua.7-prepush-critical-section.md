# age-push-equals-ci-0ua.7 - pre-push critical section

**Date:** 2026-06-19
**Bead:** `age-push-equals-ci-0ua.7`

## Problem

The pre-push path used one host-wide lock for both pure validation work and
mutable bookkeeping. It also invoked `post-land-provenance-emit.sh` during the
default pre-push path. That script can create a new commit after Git has already
selected the `local_sha` being pushed, producing stale pawl verdicts and
multi-push bookkeeping loops.

## Fix

1. Build the gate binary at a per-run `mktemp` path instead of `/tmp/ao-gate`.
2. Run the push-to-main full race suite before acquiring the host-wide push lock.
3. Acquire the lock only for mutable gate/provenance/pawl side effects.
4. Make post-land provenance explicit in pre-push via
   `AGENTOPS_PROVENANCE_EMIT_POST_LAND=1`; the default hook path does not create
   provenance commits.
5. Install a pre-push stdin snapshot/replay preamble so legacy hook segments that
   consume stdin cannot starve `pre-push.local` of the `refs/heads/main` push
   record needed for full-race and pawl routing.
6. Keep `pawl-land.sh` documentation aligned with the explicit after-push
   reconciliation model.

## Proof

```bash
sh -n scripts/hooks/pre-push.local
bash -n scripts/install-pre-push-gate.sh
bash -n scripts/pawl-land.sh
bats tests/scripts/pre-push-local.bats tests/scripts/install-pre-push-gate.bats
git diff --check
```

`tests/scripts/pre-push-local.bats` now includes a temp-repo behavior test that
stubs `go`, `ao`, pawl, and post-land provenance, simulates pre-push stdin for a
main push, and proves the default hook path leaves `HEAD`, the index, and
`docs/provenance/ledger.jsonl` unchanged.

`tests/scripts/install-pre-push-gate.bats` reproduces a prior pre-push segment
consuming stdin and proves the installed wrapper replays the same push record to
`pre-push.local`.
