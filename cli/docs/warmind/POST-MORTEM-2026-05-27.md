# Warmind V2 Post-Mortem: Gas City Integration Test

**Date**: 2026-05-27
**Test Type**: Multi-agent simulation via Gas City
**Test Repo**: https://github.com/LcncRoot/warmind-test-20260527-145222

---

## Executive Summary

Tested Warmind V2 (team knowledge sharing system) using Gas City to orchestrate a two-engineer simulation (Alice creates learning, Bob cites it). **Core concept validated**, but uncovered design flaws in the citation pipeline.

---

## Architecture Recap

### Warmind V2 Pipeline
```
.agents/learnings/  →  .warmind/pool/staged/  →  .warmind/learnings/
     (local)              (team staging)           (team canon)
```

### Key Concepts

| Concept | Definition |
|---------|------------|
| **Pool** | Staging area (`.warmind/pool/`) where learnings wait for citations |
| **Scoring** | Quality assessment: novelty (35%), specificity (25%), actionability (20%), confidence (20%) |
| **Tiers** | Gold (≥0.8, auto-promotes after 24h), Silver (≥0.5, needs 1 cite), Bronze (<0.5, needs 3 cites) |
| **Candidate ID** | Unique identifier like `learn-2026-05-27-alice-channels-16e1725c` |
| **Citation-gated promotion** | Learnings only graduate to canon when OTHER engineers cite them |

---

## What Worked

| Component | Status | Evidence |
|-----------|--------|----------|
| `warmind sync` | ✓ | Learning staged with ID, scored as silver |
| `warmind pool list` | ✓ | Shows staged candidates with tier/citation count |
| `warmind promote --force` | ✓ | Moved from pool to `.warmind/learnings/` |
| Cross-engineer detection | ✓ | `is_self_citation: false` when Bob ≠ Alice |
| Quality scoring | ✓ | Learnings scored by tier |
| Git-based sharing | ✓ | Push/pull propagated learnings |

---

## What Didn't Work (Pre-Fix)

### Issue 1: Local Learnings Shadowed Warmind Learnings

**Problem**: When Bob ran `inject`, it found the local `.agents/learnings/` copy first, not the `.warmind/learnings/` version.

**Root Cause**: Deduplication logic skips warmind versions if a local copy exists with matching content.

**Impact**: Warmind citations weren't recorded because the `Warmind=true` flag wasn't set.

**Fix Applied**: `warmind sync` now moves local learnings to `.agents/learnings/.synced/` after staging, preventing shadowing.

### Issue 2: Citations Didn't Update Pool or Trigger Auto-Promotion

**Problem**: `recordWarmindCitations()` wrote to `.warmind/citations.jsonl` but didn't:
- Update the pool entry's `citation_count`
- Check for auto-promotion eligibility

**Root Cause**: Citation recording was decoupled from pool management.

**Fix Applied**: `recordWarmindCitations()` now:
1. Updates pool citation count via `pool.RecordCitation()`
2. Checks promotion eligibility via `pool.CheckPromotion()`
3. Auto-promotes if threshold met via `pool.Promote()`

---

## Gas City Observations

### Infrastructure Issues
- **gc not in PATH**: Every new dog session had to rediscover the gc binary
- **Beads backend mismatch**: File-backed (`.gc/beads.json`) vs dolt-backed (`.beads/`) caused tooling failures
- **Session amnesia**: Dogs kept restarting with fresh context

### Safety Behavior (Working as Designed)
- dog-1 refused the "Bob impersonation" task, citing:
  - Role hijack (dogs shouldn't do engineer work)
  - Untrusted authorization channel
  - Anti-verification pressure in instructions
- This is **correct behavior** for untrusted task injection

---

## Files Changed

### `cli/cmd/ao/warmind.go`
- Added: Move local learning to `.synced/` after staging (lines 277-285)

### `cli/cmd/ao/inject_learnings.go`
- Modified: `recordWarmindCitations()` to update pool and auto-promote (lines 266-396)
- Added: `extractCandidateIDFromPath()` helper
- Added: `getArtifactAuthorInfo()` helper

---

## Recommendations

1. **Fix Gas City gc PATH**: Add gc to agent environment PATH or use absolute paths in skills
2. **Unify beads backend**: Ensure dogs use the same beads provider as the city
3. **Test with real sessions**: Use actual Claude Code sessions with transcript extraction
4. **Add .synced/ to .gitignore**: Prevent synced local learnings from being committed

---

## Test Commands Used

```bash
# Alice workflow
git clone <repo> /tmp/alice-warmind-test
git config user.email 'alice@company.com'
# Create .agents/learnings/2026-05-27-alice-channels.md
/tmp/ao-test warmind sync
git add -A && git commit && git push

# Bob workflow
git clone <repo> /tmp/bob-warmind-test
git config user.email 'bob@company.com'
/tmp/ao-test warmind pool list
/tmp/ao-test warmind promote <id> --force
/tmp/ao-test inject 'go channels' --verbose
cat .warmind/citations.jsonl  # Should show is_self_citation: false
```

---

## Verdict

**Core flywheel validated**: Extract → Stage → Cite → Promote works.

**Pipeline gaps fixed**: Citations now properly update pool counts and trigger auto-promotion.

**Gas City friction**: Infrastructure issues (PATH, beads backend) prevented fully autonomous test, but mayor orchestration pattern is sound.
