---
type: research
date: 2026-04-13
source_bead: na-cj90
source_finding: f-2026-04-03-002
source_epic: dream-findings-router
---

# Proof-Backed Next-Work Suppression Normalization

## Objective

Verify and consume the harvested `dream-findings-router` item
`f-2026-04-03-002`: "Stale next-work suppression must require explicit
completion proof, not filename/history heuristics."

This cycle is a stale-duplicate normalization. It should not modify RPI loop
runtime behavior unless current source or tests disprove the existing
proof-backed implementation.

## Plan

- Check the current RPI loop queue reader and preflight classifier.
- Check tests that preserve retryability when failed queue rows lack completion
  proof.
- Check tests that consume only items with completed-run, execution-packet, or
  evidence-only closure proof.
- Consume only `f-2026-04-03-002` in `.agents/rpi/next-work.jsonl`.
- Emit a durable evidence-only closure packet for this normalization bead.

## Pre-Mortem

Scope mode: hold scope.

- Risk: reimplement a proof-backed path that already exists.
  Mitigation: require source, test, contract, changelog, and git-history proof
  before deciding the finding is stale.
- Risk: treat history-only matches as enough proof.
  Mitigation: cite history only as supporting context; use current source and
  focused tests as the authoritative evidence.
- Risk: consume the wrong queue item in a large router row.
  Mitigation: update only the item whose `id` is `f-2026-04-03-002` and
  recompute the entry aggregate after that item is consumed.
- Risk: leave the final router batch aggregate inconsistent.
  Mitigation: validate the queue against `validate-next-work-contract-parity.sh`
  after mutation.

Applied prevention check:

- `.agents/findings/f-2026-04-03-002.md`: stale next-work consumption should
  require explicit completion proof such as a matching run registry record or
  evidence-only closure packet, not filename or history heuristics.

## Evidence

Source proof:

- `cli/internal/rpi/helpers.go` implements `ShouldSkipLegacyFailedEntry` as a
  proof-only check: failed entries are skipped only when `CompletionEvidence` is
  present.
- `cli/cmd/ao/rpi_loop.go` keeps failed rows selectable in `readQueueEntries`
  and delegates proof filtering to preflight logic.
- `cli/cmd/ao/rpi_loop.go` implements `classifyNextWorkCompletionProof` for
  completed RPI runs, execution-packet proof paths, and evidence-only closure
  packets.
- `docs/contracts/next-work.schema.md` says `failed_at` is retry metadata, not
  completion proof, and that proof-backed preflight may consume already
  satisfied work only when it has completed-run or evidence-only closure proof.

Regression proof:

- `TestShouldSkipLegacyFailedEntry_NoProofKeepsAvailable` covers failed entries
  with lifecycle metadata and no completion proof.
- `TestShouldSkipLegacyFailedEntry_NoMetadataNoProofStaysAvailable` covers
  failed legacy rows with no lifecycle metadata and no completion proof.
- `TestReadQueueEntries_KeepsLegacyFailedEntriesSelectable` confirms failed
  rows without proof remain available for downstream preflight.
- `TestClassifyNextWorkCompletionProof_UsesProofRefBeforeTextFallback` confirms
  explicit `proof_ref` takes precedence over text fallback.
- `TestResolveLoopGoal_PreflightConsumesLegacyFailedEntryWithEvidenceOnlyClosure`
  and sibling-preflight coverage confirm stale work is consumed only when a
  durable evidence-only closure packet exists.

History proof:

```bash
git log --oneline --all --grep='proof-backed\|next-work\|CompletionEvidence' -- cli/cmd/ao/rpi_loop.go cli/cmd/ao/rpi_loop_test.go docs/contracts/next-work.schema.md
```

Relevant commits include `d87b93b1 fix(rpi): require proof for stale next-work
preflight`, `c8a31f15 fix(rpi): accept execution packet proof paths`, and
`8f9a8684 feat(rpi): cherry-pick proof-backed next-work visibility from
ag-k9jk`.

## Decision

The harvested finding is stale on current `main`. The implementation and
regression coverage already exist, so this cycle consumes only
`f-2026-04-03-002` with `na-cj90` as the normalization bead and records a durable
evidence-only closure packet.
