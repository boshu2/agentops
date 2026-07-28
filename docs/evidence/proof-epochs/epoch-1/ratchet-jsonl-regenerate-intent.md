---
id: ratchet-jsonl-regenerate-intent
proof_epoch: 1
kind: repair
status: frozen
scope: gate-proof-integrity
author_context_id: claude-fable5-ratchet-jsonl-regenerate-author-20260728-session01DUtFAN2gC4ryUmkMEhDVuR
subject_commit: 350db4b4525a8434ee93373723622c8772ef8c68
subject_tree: e07d11e19bcfd1eaf4a53a2586b4f02bd211c70c
predecessor_intent_refs:
  - docs/evidence/proof-epochs/epoch-1/ratchet-sibling-failclosed-intent.md
  - docs/evidence/proof-epochs/epoch-1/ratchet-jsonl-scanchain-intent.md
binding_validation_ref: /tmp/agentops-ratchet-siblings-review-350db4b4-sol.md
binding_validation_sha256: f11e38bc54b2022d8f887590904f9cecb427b3a56e5fe2ff020b1e77430875ae
origin: Sol's executed residual finding at 350db4b45 — the regenerate path is fail-open on enumeration-helper death, and the prior report's prune-guard claim is FALSE
---

# JSONL `--regenerate` fail-closed intent — third successor in the sibling chain

## The finding (Sol, executed; independently reproduced before this freeze)

In `check-jsonl-scanner-ratchet.sh --regenerate`, `compute_grandfather_set`
enumerates candidates via `< <(grep -rl … 2>/dev/null || true)` and filters
each with `grep -q '\.jsonl' … || continue`. Both swallow helper death:

- a dead `grep -rl` (rc > 1) becomes an EMPTY candidate set;
- a dead per-file `grep -q` silently drops that file.

`ratchet_regenerate` (the shared lib) is already atomic and refuses on a
failing entries-fn — but the entries-fn **cannot fail**, so a dead enumeration
looks like a legitimate empty set and the gate rewrites a POPULATED grandfather
to header-only, prints `regenerated … (0 files)`, and exits 0. Reproduced at
the exact subject with a five-entry snapshot and a grep shim (rc 3 on `-rl`):
exit 0, all five pins destroyed.

**The predecessor report's residual-risk claim that "a death there under-pins,
which the prune guard then surfaces" is FALSE and is retracted:** missing
grandfather entries are legal shrinkage; the stale-entry loop checks pinned
entries that no longer trip, never entries that vanished. This mutating
success cannot be trusted under helper failure.

## Required criteria

- **RG-1 — enumeration fail-closed.** In `compute_grandfather_set`, a
  candidate-enumeration death (`grep -rl` rc > 1) or a per-file whole-file scan
  death (`grep -q` rc > 1) returns rc 2 with the helper's stderr preserved.
  `grep -rl` rc 1 (genuinely no candidate files) remains a legitimate empty
  set; per-file rc 1 remains a legitimate skip.
- **RG-2 — atomicity of failure.** On any enumeration failure, `--regenerate`
  exits nonzero (2 via the existing `ratchet_regenerate … || exit 2`) and the
  original grandfather bytes are byte-identical afterward — including under
  PARTIAL enumeration (paths emitted before the death must never reach the
  rewritten file; the repair accumulates entries and emits only on full
  success).
- **RG-3 — baseline-first hostile witnesses.** Each witness first proves the
  sane behavior on its own fixture (populated snapshot + tripping files;
  `--regenerate` exits 0 and writes the expected entries), then applies exactly
  one hostility and asserts: nonzero exit, refusal text visible, and the
  pre-hostility grandfather bytes preserved byte-exactly:
  1. enumeration death (`grep` shim dying on `-rl`);
  2. partial enumeration (shim emits one plausible path, then dies);
  3. per-file whole-file scan death during regenerate (shim dying on the
     `\.jsonl` pattern).
- **RG-KEEP — nothing else moves.** Check-mode semantics untouched (all
  check-path code byte-identical); no other grandfather file, no
  `scripts/lib/ratchet.sh` line, no gate registration, no blocking flag. The
  full existing battery (both sibling suites, python-ratchet, ratchet-lib,
  atomic-write at its documented 17/18, r3, preamble-lib, dolt-crosscheck, the
  eight consumers, the negative-witness Go closure, live gate) stays at its
  Sol-recorded baseline. Ordinary shrink/regenerate behavior (rewriting to
  FEWER or zero entries when files genuinely stopped tripping) is preserved
  and pinned by the baseline halves of the witnesses.

## Non-goals

- No repair of `ratchet_regenerate` (already fail-closed and atomic; the lib
  is out of scope and needs no change).
- No change to the check mode, its comparators, or its witnesses.
- No PASS claim, no merge, no push; fresh Sol remains the binding validator.

## Authorized write scope

Records first (this commit):

- `docs/evidence/proof-epochs/epoch-1/ratchet-jsonl-regenerate-intent.md`

then a RED tests-only commit:

- `tests/scripts/check-jsonl-scanner-ratchet.bats`

then a source-only child:

- `scripts/check-jsonl-scanner-ratchet.sh` (`compute_grandfather_set` only)

Nothing else.

## First useful checks

RED: all three witnesses fail against the current source (hostility runs exit
0 and mutate the snapshot). GREEN: full jsonl suite passes including the three
witnesses; shellcheck clean; full ratchet battery at baseline; worktree clean.
