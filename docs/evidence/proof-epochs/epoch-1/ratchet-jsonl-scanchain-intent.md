---
id: ratchet-jsonl-scanchain-intent
proof_epoch: 1
kind: repair
status: frozen
scope: gate-proof-integrity
author_context_id: claude-fable5-ratchet-jsonl-scanchain-author-20260728-session01DUtFAN2gC4ryUmkMEhDVuR
subject_commit: 06cc5f9a870137f62628cea691e51ac206bd42bf
subject_tree: 37a38d4dc7b4cff7456eca740bd6c656a2498a5d
predecessor_intent_ref: docs/evidence/proof-epochs/epoch-1/ratchet-sibling-failclosed-intent.md
origin: the predecessor repair report's own not-checked disclosure (the per-file added-hunk swallow), confirmed real against live source
---

# JSONL scan-chain fail-closed intent — successor to the sibling F1/F2 repair

## Why a successor, not an extension

The predecessor intent's authorized write scope and RS-KEEP froze exactly the
F1/F2 collector call sites and said "nothing else moves"; its wording does not
authorize widening. This successor is therefore its own frozen record, scoped
to the residual its repair report disclosed.

## Determination against live source

`scripts/lib/ratchet.sh:ratchet_added_hunk_matches` is deliberately tri-state
(`0` match, `1` no added match, `2` scan helper failed — "loud; never
conflated with 1"), and its own comment warns that callers using
`|| continue` still skip on rc 2. In `scripts/check-jsonl-scanner-ratchet.sh`
the per-file scan chain does exactly that, in two sites:

- **S1** — `added_hunk_has_scanner "$f" || continue`: a scan-helper death
  (awk killed/fork failure, or the whole-file grep dying in the empty-diff
  branch) returns rc 2 from the lib; `|| continue` skips the file; if it was
  the only new site the gate certifies green rc 0. **Same false-green class**
  as F1/F2: a scan nobody completed certified as "no new sites".
- **S2** — `file_trips()` uses `grep -q … || return 1`, conflating a grep
  death (rc ≥ 2) with "does not trip"; the loop's `file_trips "$f" || continue`
  then skips to the same false green.

Explicitly NOT in the class, verified against source and to be pinned by an
executed witness: a dead `git diff` INSIDE the matcher is swallowed by its
`|| true` and degrades to a whole-file scan — for a file that already passed
`file_trips` this MATCHES and the gate still flags rc 1. Fail-safe
(over-report) direction, not a false green. Likewise `is_grandfathered`'s
`&& continue` treats a pinned-file read refusal (rc 2) as "not pinned",
which flags rather than certifies — direction-safe, untouched. The preamble
gate's per-file helpers (`sources_preamble`, `exempt_reason`) also fail only
toward flagging — no false-green class there, untouched.

## Required criteria

- **SC-1 — per-file scan chain fail-closed.** In the jsonl gate loop, rc 1
  from `file_trips` or the added-hunk matcher means "skip" exactly as today;
  any other nonzero rc makes the gate refuse and exit 2 with the helper's
  stderr preserved. `file_trips` becomes tri-state internally so a grep death
  is distinguishable from a non-tripping file.
- **SC-2 — baseline-first hostile witnesses that distinguish rc 1 from rc 2.**
  Each witness first runs the gate sane over its real-violation fixture and
  requires rc 1 naming the file (the baseline that proves the fixture trips),
  then applies exactly one hostility:
  1. added-hunk scan death (awk shim triggered only when the lib's
     `RATCHET_HUNK_ERE` environment rides the call) → exit 2, refusal text,
     no green certification;
  2. whole-file scan death in `file_trips` (grep shim triggered only on the
     `\.jsonl` pattern) → exit 2, refusal text, no green certification;
  3. mid-loop dead `git diff` (shim; collection `diff-tree`/`rev-list`
     untouched) → still rc 1 NAMING the file — executed proof the degraded
     path over-reports rather than certifies.
- **SC-KEEP — nothing else moves.** No grandfather byte (growth or otherwise),
  no `scripts/lib/ratchet.sh` change, no blocking flag, no other gate or
  consumer, no governed regex. The predecessor's 21/21 sibling suites and the
  full battery stay green.

## Authorized write scope

Records first (this commit):

- `docs/evidence/proof-epochs/epoch-1/ratchet-jsonl-scanchain-intent.md`

then a RED tests-only commit:

- `tests/scripts/check-jsonl-scanner-ratchet.bats`

then a source-only child:

- `scripts/check-jsonl-scanner-ratchet.sh`

Nothing else.

## First useful checks

RED: witnesses 1 and 2 fail against the current source (today both hostilities
produce a green PASS rc 0 over the seeded violation); witness 3 passes today
and must keep passing (the fix may not satisfy the suite by reporting less).
GREEN: full jsonl suite passes; predecessor battery re-run green; shellcheck
clean; worktree clean. No PASS claim; fresh Sol remains the binding validator.
