---
record_type: implementation-intent
proof_epoch: 1
status: frozen
experiment: p1-regen-prospective-v3-repair4
author_context_id: claude-opus5:session_013fqQZAMZVFswA3oFrCDAFf:p1-regen-prospective-v3-repair4-author
invocation_id: root:regen-repair4:4bd9f73a-8de1-4e8a-8b77-65ee532658e4
base_commit: e6fdd51e51e3d3e449e938289f20d0df72c27705
base_tree: 262dbf3215dba9a3d747ce577b5f958f0593129c
branch: codex/p1-regen-prospective-v3-repair4-20260728
---

# regen-all.sh fail-fast mode — repair4 implementation intent

## Caller-supplied invocation identity

```text
root:regen-repair4:4bd9f73a-8de1-4e8a-8b77-65ee532658e4
```

Supplied by root/caller **before** any Implement artifact was built. It is bound
**verbatim**: never invented, derived, reformatted, or re-minted. This record is
frozen before the source change so the identity cannot be back-fitted.

## The defect, reproduced at the pristine base

```text
git ls-files -s scripts/regen-all.sh   ->  100644
./scripts/regen-all.sh --check         ->  rc=126
```

The script carries a `#!/usr/bin/env bash` shebang but is tracked
non-executable. Every existing caller (the Makefile, the gate runner) passes an
explicit interpreter, and the existing suite copies the script and invokes it as
`bash <path>` — blind to the mode by construction — so the regression stayed
green throughout.

## Approved behavior (unchanged from the approved source shape)

- `scripts/regen-all.sh`: **content byte-identical**, mode `100644` → `100755`.
- `tests/scripts/regen-all-fail-fast.bats`: add direct-execution mode witnesses.

## Required criteria

- **R4-MODE-01.** Tracked mode is `100755` and the file is executable.
- **R4-DIRECT-01.** Under **direct** execution the seeded generator failure stops
  the run and the exit code is **not** 126.
- **R4-126-01.** A deliberately non-executable fixture is refused with **exactly**
  126, never reaching the seeded failure. Without this, R4-DIRECT-01 asserting
  merely "non-zero" could not distinguish a mode regression from the failure
  being proven.
- **R4-CONTENT-01.** `scripts/regen-all.sh` content is unchanged; the effect
  kernel must classify it `MODE_CHANGED`, not `MODIFIED`.
- **R4-RECEIPT-FINAL-01.** *(the repair4 constraint)* Every `check-receipt.v1`
  referenced by the effect receipt binds the **FINAL** subject manifest digest,
  never the BEFORE digest. Receipts are therefore built only **after** the final
  manifest is frozen, which is only possible after the last tracked mutation.
- **R4-COMPLETE-01.** Observation roots are literally the repository include
  `["."]` with all `COMPLETE_RUNTIME_EXCLUSIONS`; effect coverage is `COMPLETE`
  with zero undeclared paths.
- **R4-SCOPE-01.** The tracked delta from base is exactly four paths.

## Prior defect this round repairs

Repair3 bound the caller-supplied invocation ID correctly but executed its
checks **before** freezing the final manifest, so every `check-receipt.v1`
carried the BEFORE subject-manifest digest. A receipt that names a manifest
predating the change it attests cannot evidence that the checks ran against the
subject actually produced. Repair4 reorders: freeze final, then execute, then
derive.

## Ordering (prospective, kernel order)

1. Verify the pristine base tree clean; reproduce the defect.
2. Freeze the BEFORE manifest from the pristine tree.
3. Mint the intent snapshot; freeze the scope index.
4. **Records commit** — this file and the repair4 record only.
5. Apply the source change — mode plus witnesses.
6. **Freeze the FINAL manifest** after that last tracked mutation.
7. Execute every declared check and build its receipt against the **FINAL**
   manifest digest.
8. Derive the COMPLETE effect receipt and the prepared invocation.
9. **Source commit.**

## Non-goals

- No verdict, judgment ID, or validator context ID. Minting any would be
  self-validation.
- No merge, push, projection regeneration, release, or ratchet mutation.
- No tracked packet copy: every runtime artifact stays under the excluded
  `.agents/ao` subdirectories.
- No tracked mutation after the final manifest is frozen.
