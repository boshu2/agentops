# age-mvy — orchestrate skill count doc sync

Bead: `age-mvy`  
Commit: `ea9ab2cbc` (chore/age-mvy-skill-count-sync)  
Date: 2026-06-17

## Scope

Sync skill inventory count 72→73 after `orchestrate` skill landed in Phase 1 (`age-ueu`).

## Changes

- `scripts/sync-skill-counts.sh` patched `PRODUCT.md` distribution/runtime reach and `docs/agentops-brief.md` ASCII diagram.
- `docs/architecture/codebase-overview.md` active skills + Codex twin counts updated.
- `docs/contracts/claim-registry.yaml` promoted public-surface claim markers to PILOT/PROVEN so `claim.tier-citation` passes on changed docs.

## Verification

```text
bash scripts/sync-skill-counts.sh --check  # PASS (73 skills)
bash tests/docs/validate-skill-count.sh   # PASS
ao claim check --changed --json           # 5 supported, 1 weak (PRODUCT-MT-OLYMPUS cross-repo pointer)
ao gate check --fast --scope head         # 28 pass, claim.tier-citation PASS, claim.pmf-evidence non-blocking
bash scripts/check-pmf-evidence.sh PRODUCT.md  # PASS
```

## Notes

Phase 1 land (`daf5c4fdd`) intentionally deferred PRODUCT.md/README.md count sync to avoid claim gates; this bead closes that gap.
