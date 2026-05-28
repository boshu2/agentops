---
title: Substring-based sed renames overreach across same-prefix concepts
date: 2026-05-13
tags: [refactor, rename, sed, audit-then-execute, anti-pattern]
source: .agents/evolve/cycle-history.jsonl cycle 126
maturity: lessons-learned
companion: 2026-05-13-bc-ports-wire-up-arc.md
---

# Substring sed rename overreach

Cycle 126 shipped `daemon.QueueClaim → daemon.QueueLease` (108 Go refs)
via `find ... -name '*.go' | xargs sed -i 's/QueueClaim/QueueLease/g'`,
caught a real semantic bug in the same cycle via the post-commit
`PreToolUse:Bash` hook's diff display, and shipped a corrective
follow-up. This learning captures the lesson durably.

## What went wrong

The sed pattern `s/QueueClaim/QueueLease/g` is a substring match. It
caught identifiers from TWO different concepts:

1. **Intended:** `daemon.QueueClaim` (struct, lease semantics —
   has fields `ClaimToken`, `LeaseEpoch`, `LeaseExpiresAt`).
   Correctly renamed to `QueueLease`.
2. **Over-reach:** `rpi.ErrQueueClaimConflict`,
   `rpi.RequireQueueClaimOwner`, and the cli/cmd/ao wrappers
   `errQueueClaimConflict` / `requireQueueClaimOwner`. These are
   about **work-item claim coordination in
   `.agents/rpi/next-work.jsonl`** — when two workers race to
   claim the same harvested work item. That IS a Claim concept
   (per the BC2 contract: Claim = public assertion of a work
   slot), not a Lease.

The renamed code compiled and tests passed because the symbols are
just identifiers — the build doesn't care what concept they encode.
The bug was semantic, not syntactic.

## How it was caught

The `PreToolUse:Bash` hook on the second `git commit` showed the
staged diff for self-review. The smoking gun was:

```
func EnsureQueueItemClaimable(...) error {
    ...
    return ErrQueueLeaseConflict   // ← Claim API, Lease error name
}
```

`EnsureQueueItemClaimable` kept Claim-language (sed only matched
`QueueClaim`, not `Claimable`), but the error it returned was
renamed to `ErrQueueLeaseConflict`. The semantic mismatch was
visible at a glance.

## The rule

**Before any bulk sed rename across packages:**

1. Find the type definition (`grep -rn 'type <OldName>\b'`).
2. **Enumerate every identifier that contains the substring** —
   not just the type itself. Use `grep -rn '\b[A-Za-z]*<OldName>[A-Za-z]*\b'`
   or `grep -roE '\w*<OldName>\w*'` and inspect the unique set.
3. For each enumerated identifier, classify which concept it
   belongs to (the type-def concept vs sibling concepts that
   share the prefix incidentally).
4. Sed only on identifiers that match the target concept.
5. After commit, **re-read the diff** to look for semantic
   inconsistencies — APIs that kept old language returning errors
   that took new language, or vice-versa.

## Cycle-126 worked example

```bash
# Step 1: type def
grep -rn 'type QueueClaim\b' cli/
# → cli/internal/daemon/jobs.go:67: type QueueClaim struct {

# Step 2: enumerate ALL identifiers containing "QueueClaim"
grep -roE '\w*QueueClaim\w*' cli/ scripts/ docs/ \
  --exclude-dir=testdata | sort -u

# What I should have seen:
#   QueueClaim                       (the struct — daemon, rename)
#   ErrQueueClaimConflict            (rpi, WORK-ITEM claim — KEEP)
#   RequireQueueClaimOwner           (rpi, WORK-ITEM claim — KEEP)
#   errQueueClaimConflict            (ao wrapper — KEEP)
#   requireQueueClaimOwner           (ao wrapper — KEEP)
#   claimForPlansSpec                (test helper, "claim" verb — unaffected)

# Step 3: classify
# Step 4: sed only on the struct + its method-receiver params
# Step 5: post-commit diff re-read
```

## When this matters most

Renames where the substring is a noun that has BOTH a domain
concept (BC1/2/etc.) AND an incidental code identifier. Common
examples to be careful with for future soc-5yuy children:

- **Drift #1 (Gate vs Validator):** `cli/internal/ratchet.Validator`
  has nothing to do with `scripts/check-*.sh` validators. Mass
  `Validator → Gate` sed would break it.
- **Drift #2 (Run vs Cycle):** `CIRun` (BC2 port), `RPIRun` (rpi
  package), `ContextVariantRun` (eval) — all legitimate "Run"
  identifiers. Narrow renames only.
- **Drift #5 (Session):** already-prefixed Sessions
  (`AgentSession`, `GCSession`, `GasCitySession`,
  `CLIFallbackSession`) are unaffected. Only the bare
  `type Session struct` declarations need the rename.

## Anti-pattern signal

If a single sed across N files moves a counter from "K → 0"
**and** N is large enough you can't diff-review the changes by
eye, the rename almost certainly over-reached. Lower N by
restricting the file set, or use `gopls rename` / IDE
refactoring tooling that knows about identifier scope.

## See also

- `docs/learnings/2026-05-13-bc-ports-wire-up-arc.md` — the
  parent arc; cycle 126 was the first concrete rename to follow.
- `docs/contracts/ubiquitous-language.md` cycle log entry for
  2026-05-13 cycle 126 — the inline correction record.
