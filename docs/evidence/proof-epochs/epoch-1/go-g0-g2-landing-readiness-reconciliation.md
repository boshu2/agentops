---
id: go-g0-g2-landing-readiness-reconciliation
proof_epoch: 1
lane: go-g0-g2
kind: reconciliation-record
status: durable
subject_commit: 77b2dfef90694df65ebfda724978e9a433ccce3e
subject_tree: 73f1e131da03aab200cb51c9ea30236783c3e986
branch: codex/go-g0-g2-fast-exit-repair-20260724
integration_review_ref: /tmp/agentops-opus5-go-g0-g2-integration-review.md
integration_review_sha256: 9ce5a4bb24432939c9fb06e57d6e34007a53cf0b07b74ba667c325ba975b20fd
integration_review_disposition: REQUEST_CHANGES
sol_audit_ref: /tmp/agentops-go-g0-g2-fresh-sol-validation-audit.md
sol_audit_sha256: 0b5f09a78590c228b46ede22f837fe1fbce649ddd7967fdb659dd69a763a18c2
sol_audit_disposition: PASS (advisory)
binding_verdict_v2: aef3a8a340ac2315e0278d8f09e1158c37c62cb46617ae037c26f21aac8f563b
binding_verdict_v3: fadb23c2bcce38aa02f2e59d01d400296dfc18bf7c5ee58a8799dcae9544e6f1
preserved_fail_verdict: 8a1d6d59
---

# Go G0–G2 landing-readiness reconciliation

## Two reports, both correct, answering different questions

Both are preserved by digest and neither is overturned. They do **not** conflict;
they answer different questions, and conflating them is the failure this record
exists to prevent.

| | Integration review | Sol validation audit |
|---|---|---|
| sha256 | `9ce5a4bb…` | `0b5f09a7…` |
| Disposition | **REQUEST_CHANGES** | **PASS** (advisory) |
| Question answered | *May this lane land into current `main`?* | *Is the binding PASS trustworthy for the exact tree?* |
| Scope | landing surface | exact-lane semantics |

**The distinction, stated once and load-bearing:**

- **Exact-lane semantic readiness: MET.** Every gate is green on
  `77b2dfef9` / `73f1e131`, both binding verdicts recompute canonically with
  distinct author and validator identities and complete per-criterion evidence,
  the certified cleanup property is mutation-lethal under attack, and the FAIL
  lineage is preserved. Both reports agree on this.
- **Landing readiness: NOT MET.** Three landing-surface conditions block, none of
  which is a defect in the repair.

A `PASS` on exact-lane semantics is **not** authorization to land. This record
fixes that boundary so a later reader cannot cite the Sol audit as landing
clearance.

Independently reconfirmed here at the exact subject: both verdict artifacts
recompute their filenames under the canonical rule (compact, key-sorted, with
`artifact_digest` omitted) — `aef3a8a3…` raw sha256 `0dd1725f…`, 7,555 bytes;
`fadb23c2…` raw sha256 `9a39eaa8…`, 12,322 bytes.

## Findings carried into repair

### W1 — structurally tight test-only `WaitDelay` (repaired in this lane)

Three sites in `cli/internal/subprocess/run_unix_test.go` (lines 73, 97, 124) pin
`WaitDelay: 100 * time.Millisecond` while production uses
`defaultWaitDelay = 500 * time.Millisecond` (`cli/internal/subprocess/run.go:16`).
The tests therefore assert SIGKILL propagation *plus* pipe drain inside one-fifth
of the production budget. The reviewer observed 1 failure in 9 runs, only under
process-table and I/O contention, in exactly the two G0–G2 cleanup properties this
lane certifies.

Non-reproduction is **not** clearance for a timing flake. The structural tightness
is the durable finding, and it is repaired here — test-only, with **no production
behaviour change**.

### W2 — the binding PASS chain is untracked and erasable (repaired in this lane)

`.gitignore:56-58` excludes `/.agents/ao/*` except `config.yaml`. Measured at this
subject: **139 files** under `.agents/ao`, of which **138 are untracked** —
including both binding verdicts and every artifact they reference. The FAIL
lineage is tracked under `docs/evidence/proof-epochs/epoch-1/verdicts/`; the PASS
that would authorize landing is not. Cleaning this worktree destroys the only copy.

Reference closure, measured rather than assumed:

- `fadb23c2…` (v3) cites **5** `.agents/` paths — intent snapshot `13cea861…`,
  `before-manifest.json`, `effect-receipt.json`, `end-manifest.json`,
  `scope-index.json`. All 5 resolve.
- Its criteria cite **20 distinct** evidence-receipt digests, **all 20** of which
  resolve to files in `check-receipts/` (24 receipts present).
- `aef3a8a3…` (v2) cites **zero** `.agents/` paths. Its `acceptance_digest`
  `4f6c81ce…` resolves to the **already-tracked** intent
  `go-g0-g2-finite-stdin-test-sync-intent.md`, and the content-addressed
  `.intent` snapshot is byte-identical to it. Its remaining `evidence_refs` are
  textual `fresh-check:` strings — which is precisely Sol's warning 1.

### W3 — the lane no longer merges cleanly, and the conflict is semantic (integration-only)

`git merge-tree --merge-base=735580d1c main 77b2dfef9` → rc=1, two content
conflicts in `cli/internal/adapters/eval/runtime.go` and `runtime_test.go`. The
single main-only commit `74795448a` touches exactly those files; the lane's
same-subject `d7029685e` is **not** patch-id equivalent, and the blobs genuinely
differ. Path overlap between the lane's 83 changed paths and main-only paths is
**zero** — zero path overlap did not mean no conflict, because both sides changed
the same package from a shared base.

**This is an integration-only obligation and is deliberately NOT resolved in this
lane.** Resolving it produces a third version of `adapters/eval/runtime.go` that
no verdict in this lineage covers. It requires:

1. an **explicit combined resolution** — not take-ours, not take-theirs, because
   the lane also changed `scenario_ab_agentic.go` and `scenario_ab_runtime.go` in
   the same package;
2. **exact post-integration revalidation** of the full Go gate set on the merged
   tree, because every race, shuffle, vet, lint, and cross-build receipt in this
   lane was produced against the lane's version of that package.

## Erratum — the `OnStart` sentence is overbroad

Sol warning 2. A checked sentence in the binding `verdict.v2` states that
`OnStart` runs **only after successful process-tree attachment**. The source calls
it after the attachment *attempt*, even when attachment returned an error.

**The verdict bytes are not edited** — that would break `aef3a8a3…`. This erratum
records the correction:

- **False as written:** `OnStart` runs only after successful attachment.
- **True and narrower:** `OnStart` runs after the attachment *attempt*, regardless
  of its outcome. In the exact GTS-1 execution, `Cmd.Start` succeeded, the PID was
  positive, cancellation was causally downstream of the callback, and the exact
  `CleanupCompleted` outcome rules out an attachment failure **in that run** — so
  GTS-1's acceptance is satisfied and the overstatement does not invalidate it.

**Obligation:** the final integrated `verdict.v3` must cite the **narrower true
property**, not the overbroad sentence. Carrying the original wording forward
would propagate a false claim into the record that authorizes the cutover.

## Residuals carried forward, not silently widened

Recorded as residuals rather than repaired, because repairing any of them would
widen source beyond the frozen scope:

1. **Doctor capability split** (Sol warning 3). Root `ao capabilities` leaves
   `ao doctor gc` exit codes unspecified while `ao doctor capabilities` publishes
   the doctor map including usage code 64. Observed codes were stable and
   non-conflicting. **No source widening in this lane.**
2. **Non-native runtime checks** (Sol warning 4; review residual 5). Linux and
   Windows were cross-compiled and vetted only; process-group and fast-exit
   semantics ran on Darwin arm64 alone. A Linux or Windows regression in
   process-group cleanup would be caught by nothing either report ran.
3. **v2 evidence is descriptive, not receipt-bound** (Sol warning 1). The
   full-suite, vet, lint, and module checks are textual `fresh-check:` strings
   rather than named content-addressed receipts. An auditability limitation, not
   a semantic failure; both reports independently replayed the claims.
4. **The lane is 132 commits, not a Go-only lane** (review I1). `main..77b2dfef9`
   is 710 files, +162,419 / −115,714. Landing this branch lands the entire
   skill-overhaul chain. Whether the Go fix is extracted or the whole branch lands
   is an integration decision this lane does not make.
5. **gofmt: 16 dirty files, none in the lane's changed set** (review I2). Lane tip
   16, lane base 16, main 4. gofmt is not an enforced CI gate. Landing takes the
   repo from 4 to 16.

## Recorded as sound — not to be re-litigated

Both reports confirmed, and this repair must not regress: canonical digest
identity of all three verdicts; distinct author/validator identities; complete
per-criterion evidence (4/4 GTS, 18/18 prior) with zero findings; every declared
evidence ref resolving and digest-checking; exact changed-path scope; preserved
FAIL lineage with five FAIL receipts; build, vet, pinned lint v2.11.4, full
race+shuffle, `go mod tidy -diff`, `go mod verify`, both supported cross-builds;
and mutation-lethality of the cleanup path under hostile attack.

## Authority

This record is evidence, not authority. It issues no verdict, creates no
`verdict.v3`, and does not alter either report's disposition. Operative criteria
live only in the frozen repair intent beside it.
