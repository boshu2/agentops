# age-0tn — trunk-bound provenance merge_sha

**Bead:** `age-0tn`  
**Date:** 2026-06-17

## Problem

Pre-push provenance emit recorded local commit OIDs before push; some never became ancestors of `origin/main`, breaking `(bead_id, merge_sha)` mesh joins. Verdict emit only fired from `pawl-verdict.sh write`, not the post-land path.

## Fix

1. `ao provenance emit-landed --trunk-ref origin/main` filters to trunk ancestors only.
2. `scripts/post-land-provenance-emit.sh` runs after push (fetch + landed range + verdict emit).
3. Pre-push emit disabled by default (`AGENTOPS_PROVENANCE_EMIT_PRE_PUSH=1` opt-in legacy).
4. `scripts/check-provenance-merge-sha.sh` warns/fails on off-trunk merge_shas.

## Proof

```bash
bats tests/scripts/check-provenance-merge-sha.bats
cd cli && go test ./cmd/ao/ -run ProvenanceEmit -count=1
bash scripts/post-land-provenance-emit.sh   # after push
```

**Commit:** 0e52f8790 (land on main)
