---
id: go-g0-g2-landing-readiness-repair
proof_epoch: 1
lane: go-g0-g2
kind: repair
status: frozen
scope: test-and-evidence-only
subject_commit: 77b2dfef90694df65ebfda724978e9a433ccce3e
subject_tree: 73f1e131da03aab200cb51c9ea30236783c3e986
reconciliation_ref: docs/evidence/proof-epochs/epoch-1/go-g0-g2-landing-readiness-reconciliation.md
integration_review_sha256: 9ce5a4bb24432939c9fb06e57d6e34007a53cf0b07b74ba667c325ba975b20fd
sol_audit_sha256: 0b5f09a78590c228b46ede22f837fe1fbce649ddd7967fdb659dd69a763a18c2
production_behaviour_change: none
---

# Go G0–G2 landing-readiness repair intent

## Outcome

Two of the three landing-surface blockers are closed inside this lane without
touching production behaviour: the test-only `WaitDelay` tightness (W1) and the
erasable binding-evidence chain (W2). The third (W3) is recorded as an
integration-only obligation and is deliberately not resolved here.

## Required criteria

- **GLR-1 — test budget aligned with production, structurally.** The three
  `WaitDelay` sites in `cli/internal/subprocess/run_unix_test.go` reference the
  production constant `defaultWaitDelay` rather than an independent literal, so
  the two cannot drift apart again. **No production file changes**, and no
  production behaviour changes: `defaultWaitDelay` keeps its value and every
  non-test byte under `cli/` is identical to the parent.

- **GLR-2 — the aligned suite is still mutation-lethal.** Raising the test budget
  must not weaken the certified property. Hostile mutations of the cleanup path
  — never escalating to SIGKILL, and dropping `defaultWaitDelay` to zero — must
  still be killed by the focused suite, proven in disposable copies with an
  unmutated control green first.

- **GLR-3 — stable under load and repeat.** The focused subprocess suite passes
  repeated high-count shuffled runs **under deliberate contention**, since the
  observed failure appeared only under process-table and I/O load. A quiet-run
  pass alone does not discharge this criterion.

- **GLR-4 — the PASS chain is durable and self-resolving.** Both binding verdicts
  and every artifact they reference are preserved **byte-identically** in a
  tracked epoch evidence bundle, with a content-addressed mapping from each
  original `.agents/ao/...` reference to its bundled path and digest.
  - The original verdict bytes are **not edited** — editing them would break
    `aef3a8a3…` and `fadb23c2…`.
  - No tracked copy may carry a broken reference: every reference in a bundled
    artifact must resolve through the mapping.
  - The originals under `.agents/ao/` are **left in place**, untouched.

- **GLR-5 — erratum recorded, obligation stated.** The overbroad `OnStart`
  sentence is corrected by erratum in the reconciliation record, and the
  obligation that the final integrated `verdict.v3` must cite the narrower true
  property is stated durably.

- **GLR-6 — integration obligation recorded, not discharged.** The
  `cli/internal/adapters/eval` merge conflict is recorded as requiring an explicit
  combined resolution plus exact post-integration revalidation. **This lane does
  not resolve it.**

- **GLR-7 — residuals carried, not silently widened.** The doctor capability split
  and the non-native runtime limits are recorded as residuals. No source change
  addresses either.

- **GLR-KEEP — no regression.** Every fact both reports recorded as sound
  continues to hold: canonical digests of all three verdicts, distinct
  author/validator identities, complete per-criterion evidence, resolvable
  evidence refs, preserved FAIL lineage, and the full green gate set.

## Non-goals

- Do not change any production file. `defaultWaitDelay` keeps its value; no
  non-test byte under `cli/` moves.
- Do not resolve the landing-branch merge conflict in this lane.
- Do not edit the bytes of any existing verdict, receipt, manifest, scope index,
  effect receipt, or intent.
- Do not delete or relocate the originals under `.agents/ao/`.
- Do not mint a verdict, integrate, merge, publish, push, or touch `main`.
- Do not widen source to address the doctor capability split or non-native
  runtime coverage.
- Do not self-validate. The author of this repair judges nothing.

## Authorized write scope

Records first:

- `docs/evidence/proof-epochs/epoch-1/go-g0-g2-landing-readiness-reconciliation.md`
- `docs/evidence/proof-epochs/epoch-1/go-g0-g2-landing-readiness-repair-intent.md`

then, and only then:

- `cli/internal/subprocess/run_unix_test.go` — the three `WaitDelay` sites only
- `docs/evidence/proof-epochs/epoch-1/go-g0-g2-binding-evidence/**` — the durable
  bundle

Nothing else.

## First useful checks

Confirm at the exact parent that the three test sites read `100 * time.Millisecond`
while `run.go:16` reads `500 * time.Millisecond`, and that both verdicts and all
their references are untracked. Then, after the repair: rerun the focused suite
repeatedly under contention; reseed both cleanup mutations and require zero
survivors; verify every bundled artifact is byte-identical to its original and
that every reference resolves through the mapping; and rerun the full owned, race,
shuffle, vet, lint, mod, and cross-compile gate set.
