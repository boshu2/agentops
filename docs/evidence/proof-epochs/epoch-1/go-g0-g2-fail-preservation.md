# Go G0-G2 fresh validation — terminal FAIL preservation

Date: 2026-07-24

Candidate:
`52ecde3d88b5a9f0e4fda9e88c14ccf7575a2849`

Fresh validator:
`codex-fresh-go-g0-g2-validator-2-20260724-52ecde3d8`

Verdict artifact:
`docs/evidence/proof-epochs/epoch-1/verdicts/83cb2e9095d61bc14a00bee250215367bd9697bd05ff56fe384b662965752680.json`

Verdict artifact digest:
`83cb2e9095d61bc14a00bee250215367bd9697bd05ff56fe384b662965752680`

## Result

`FAIL`

GO-1 through GO-4 and GO-7 passed. GO-5 and GO-6 failed because the shared
subprocess result exposes bounded output and exit facts but no cleanup outcome,
and cleanup errors are suppressed when cancellation or wait errors return
first. Windows process-tree behavior was inspected and cross-compiled only.

## Persistence transport

The fresh validator completed the semantic judgment and emitted the exact
six-field draft, but its managed `workspace-write` sandbox denied writes under
the worktree-local ignored `.agents` directory. The root runtime materialized
those exact validator-authored bytes without semantic edits and invoked the
frozen `validate_v3.py store-verdict` command. The store reverified the exact
intent, final manifest, complete effect receipt, active epoch-1 identity, typed
check receipts, and distinct author/validator context IDs before writing the
canonical verdict.

The canonical verdict bytes in this repository are byte-identical to the
runtime-stored artifact. The initial validator attempt that was terminated by
an automated policy false-positive produced no draft or verdict and has no
semantic status.
