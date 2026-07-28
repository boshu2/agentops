---
id: ratchet-jsonl-regenerate-repair3-intent
proof_epoch: 1
kind: repair
status: frozen
scope: jsonl-reader-migration
author_context_id: claude-opus5-ratchet-jsonl-repair3-author-20260728-session013fqQZAMZVFswA3oFrCDAFf
subject_commit: 753e0aadbd1eaea9bb63becd56d50b9d4ffb93e6
subject_tree: 7bddfc8d903a6e64abc48418815c1d1c8a86b34a
predecessor_intent_ref: docs/evidence/proof-epochs/epoch-1/ratchet-jsonl-regenerate-intent.md
predecessor_intent_sha256: 54d8c099e5c95efc146e3e533cb71fac1b12e9aec1277364a70dd80a585fe112
review_report_ref: /tmp/agentops-ratchet-regenerate-review-753e0aadb-sol.md
review_report_sha256: be92401bb56426e2bc4f910dfb619345055080e8a230625112b21fa77feea957
adr_ref: docs/adr/ADR-0016-state-tiers.md
adr_sha256: 14dba837f6e5946b9f02a2a60bb409c682c35b865f53602199092723ba974aec
grandfather_ref: scripts/.jsonl-scanner-grandfather
grandfather_sha256_required_unchanged: a15f326f172e0346aadc17c0f4498861eb61e8c07df88c6e554f214bc90beb8e
---

# JSONL scanner ratchet — repair 3 intent

Every prior intent, record, and acceptance criterion keeps its exact bytes.
Nothing is amended, narrowed, or reinterpreted. This is an addition-only
successor experiment.

## Why this increment exists

A fresh, author-distinct validator returned **REQUEST_CHANGES / semantic FAIL**
against exact subject `753e0aadb` (tree `7bddfc8d9`). The fail-closed repair
itself was judged effective on all three helper-death classes. One criterion
failed: **RG-KEEP / the caller's `no grandfather growth` requirement.**

A normal, successful `--regenerate` on the real exact tree exits `0` and grows
the tracked snapshot from **five entries to seven**, moving the grandfather
digest from `a15f326f172e0346aadc17c0f4498861eb61e8c07df88c6e554f214bc90beb8e`
to `bcdbf4d1cee4553b240cfa163f89d5f9ab96a945ebeeabddcc5a9a91033716aa`. The two
added entries are:

```text
+cli/internal/evidence/citations.go
+cli/internal/provenancegraph/store.go
```

Both genuinely trip the whole-file heuristic today: each mentions a `.jsonl`
path and each invokes `bufio.NewScanner(`. This is **pre-existing snapshot
drift**, not a defect the prior repair introduced — the grandfather blob is
byte-identical at base `350db4b45` and subject `753e0aadb`. The branch diff has
no growth; executed regeneration does.

## The design error being corrected

The prior rounds treated the grandfather as the thing to hold still while the
tree drifted underneath it. The honest correction is the reverse: **make the
tree stop tripping the heuristic**, so a normal regeneration naturally
reproduces the five-entry snapshot. Growth is not suppressed, waived, or pinned
away — the two offenders are migrated to the blessed helpers and therefore stop
being offenders.

**No grandfather edit of any kind is authorized by this intent** — no growth, no
prune, no reordering, no comment change. The file must remain byte-identical at
`a15f326f…`, and it must remain so *as the natural output of regeneration*, not
because regeneration was prevented from running.

## Required criteria

- **RG3-NOGROWTH-01 — normal regeneration is a no-op on the snapshot.**
  `bash scripts/check-jsonl-scanner-ratchet.sh --regenerate` exits `0`, reports
  five files, and leaves `scripts/.jsonl-scanner-grandfather` **byte-identical**
  to `a15f326f172e0346aadc17c0f4498861eb61e8c07df88c6e554f214bc90beb8e`.
- **RG3-IDEMPOTENT-01 — regeneration is idempotent at five entries.** A second
  consecutive regeneration exits `0` and again changes nothing. Idempotence must
  hold *at the five-entry digest*, not after a first prohibited growth.
- **RG3-MIGRATE-01 — the two named readers use the blessed helpers.**
  `cli/internal/evidence/citations.go` and
  `cli/internal/provenancegraph/store.go` read JSONL through
  `cli/internal/storage` `ScanJSONL` / `ScanJSONLFile`, not a raw
  `bufio.NewScanner`. Neither file may be added to the grandfather.
- **RG3-BEHAVIOR-01 — each package's observable behavior and error semantics are
  preserved.** Specifically: `LoadCitations` still returns `(nil, nil)` for a
  missing ledger, still wraps an open failure as `open citation ledger: %w`,
  still wraps a scan failure as `scan citation ledger: %w`, and still returns
  the **partial** citations collected before a scan error. The provenance store
  keeps its existing reader contract and error wrapping. No import cycle is
  introduced.
- **RG3-PRESERVE-01 — the already-fixed helper-death behavior survives.** All
  three witnesses (complete candidate-enumeration death, partial enumeration
  then death, per-file whole-file scan death) remain fail-closed and atomic:
  nonzero exit, `refusing to regenerate` on stderr, no `regenerated` success
  line, and the grandfather byte-identical by `cmp`. Their test IDs and
  assertions are unchanged.
- **RG3-RED-01 — the failure is demonstrated before it is fixed.** A RED test
  proves exact-tree regeneration grows `5 → 7` and **names both offending
  paths**, so the repair cannot be shaped against a target chosen afterwards.

## Non-goals

- No grandfather growth, prune, or edit. If the file changes at all, this repair
  has failed.
- No change to check mode, the gate registry, blocking posture, flags, or
  `scripts/lib/ratchet.sh`.
- No unrelated refactor of either Go package beyond replacing the raw scanner.
- No projection regeneration, no merge, no push, no release, no `verdict.v*`.
- ADR-0016 posture is unchanged: the blessed helpers already live in
  `cli/internal/storage`; this adds no new authority and no new shipped file.

## Authorized write scope

- `docs/evidence/proof-epochs/epoch-1/ratchet-jsonl-regenerate-repair3-intent.md`
- `tests/scripts/*jsonl*.bats` (RED and regression witnesses)
- `cli/internal/evidence/citations.go`
- `cli/internal/provenancegraph/store.go`

Nothing else. In particular **not** `scripts/.jsonl-scanner-grandfather` and
**not** `scripts/check-jsonl-scanner-ratchet.sh`.

## Commit ordering

1. **Records-first:** this intent, before any test or source byte.
2. **RED:** a test that fails at the current tree by demonstrating the 5→7
   growth and naming both offenders.
3. **Source:** the two reader migrations, turning the RED test green.

## First useful checks

Reproduce the growth against `753e0aadb` in a disposable clone (done before this
record was frozen: `a15f326f…` → `bcdbf4d1…`, `+cli/internal/evidence/citations.go`,
`+cli/internal/provenancegraph/store.go`). Then require regeneration to report
five files and leave the snapshot byte-identical, twice in a row; the three
helper-death witnesses to stay red-on-hostility; and the targeted package tests,
the full JSONL suite, the sibling ratchet suites, and `go build`/`go test` to
pass.
