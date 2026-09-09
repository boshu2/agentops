# RPI traversal

This page owns exact evidence semantics for the lean [RPI operating charter](../../skills/rpi/SKILL.md).
AgentOps is the operations layer for agentic engineering: portable skills and
independent judgment, not a scheduler, tracker, Git workflow or autonomous
controller. [Vocabulary](../contracts/ubiquitous-language.md) names the source
boundaries.

```text
accepted intent
  -> Plan only for missing shape or consequential uncertainty
  -> Implement, direct repairs and cheap discriminating checks
  -> required integration checks
  -> fresh author-distinct Validate over the exact subject
  -> repair known findings within real bounds and revalidate changed content
  -> completed outcome or truthful stopped result
```

The agent owns the authorized outcome through finish. A new planning artifact,
Recall pass, Learn worksheet, specialist or scheduler is not required for a
trivial clear change. Evidence may disprove an assumption: revise the approach
within unchanged accepted outcome and scope. Changing acceptance or expanding
authority needs the caller. Known failures stay implementation work.

## Intent, subject and checks

Use the caller-owned bead, issue or conversation as the accepted intent. Add
acceptance, non-goals, scope and the first discriminating check only where
missing or ambiguous. Scope covers source owners and their generator-owned
companions as a class. An exact acceptance snapshot stays fixed for a judgment;
approach notes may change without changing accepted outcome or scope.

The runtime carries a durable intent reference and the digest of exact resolved
bytes. If no durable source exists, `ao provenance snapshot-intent` accepts a
file or stdin and an explicit existing protected external non-Git evidence root.
No workspace fallback or second model-authored planning packet is allowed.
Native handoffs preserve necessary context and unknown identities honestly;
startup episode association is used when selected, not a trivial-change toll.

Implement runs a right-reason RED for behavioral work, or an honest green
baseline for documentation, relocation or pure refactor work. Make the smallest
coherent change, fix understood errors directly and use fast checks to falsify
it. Run repository-required integration checks at the final integration boundary.
Valid exact-input receipts can establish routine facts; do not replay an expensive
suite without an acceptance need, relevant new change or unresolved concern.

The runtime derives actual changed paths, author context and `subject-manifest.v1`.
Where changed paths affect bound acceptance evidence, run
`ao provenance evidence-orphans --root <repo-root>` with one `--changed <path>`
per actual changed path and preserve its JSON receipt. Exit 0 means the scan
completed, not that no orphan exists; exit 2 is incomplete. Recapture affected
required evidence and refresh the receipt when repairs affect those bindings.

`subject-manifest.v1` contains normalized relative paths, file/symlink/deletion
kinds, executable bits, content/target digests, declared roots and exclusions,
an optional base-manifest digest and a canonical digest. Git metadata is optional
and cannot establish semantic acceptance. Existing `ao provenance` manifest,
verify-manifest, snapshot-intent, digest, store-verdict, verify-verdict and
verify-subject helpers establish structural facts, not semantic judgments.
They add no tracker, queue, Git delivery or runtime authority.

## Fresh final Validate

A fresh author-distinct context reads the exact subject, unchanged acceptance
and authorized evidence independently. A new role in the author's context is
not fresh. Observe actual runtime/model/context identities; unknown or colliding
identities and unattested freshness produce NOT_PROVEN. An attestation is a
declared trust fact, not cryptographic isolation.

Default to a fresh reviewer from the author's model family. `--cross-model
[model]` is a skill prompt option adding an authorized different-family judge,
not an AO command flag. An explicitly required leg remains required until the
caller changes it; unavailable diversity yields `diversity_unsatisfied` /
NOT_PROVEN. Risk deepens inspection rather than automatically adding families
or specialists. Use [model-dispatch](../../skills/agent-native/references/model-dispatch.md)
with caller/native bounds, no fixed ten-minute cap and no renewed allowance.

Validate derives subject identity at the beginning and end, confirms intent
continuity and complete changed-path coverage, and judges each acceptance
criterion against real evidence. Proven out-of-scope change or failed acceptance
is FAIL. Subject mutation, digest mismatch, incomplete coverage, missing
freshness or necessary evidence is NOT_PROVEN. PASS requires nonempty checked
scope and top-level evidence, evidence for every criterion, and empty
`not_checked`. Its meaning is unverified in-scope acceptance; do not empty it by
relabelling a necessary finding as optional. Non-goals remain in accepted intent,
bounded proof in criterion reasoning, and residual risk in the report.

Classify commands before running them. Mutating checks (regeneration, sync,
formatting) run against a disposable copy or committed subject, never overwrite
the judged working tree. Rerun acceptance-critical or uncertain checks; a
receipt is a claim to inspect, and structure alone cannot prove behavior.
Changes to checks, fixtures, tolerances or specifications must satisfy original
intent; weakening the oracle to get green is FAIL.

Keep all necessary findings from selected judges with stable IDs/classes.
Recurrence or unknown cause calls for causal examination; counts alone cannot
prove progress, regression or a wrong design. Preserve disagreement; neither
majority vote nor the author's preferred review certifies PASS. Validate reads
and returns judgment; implementers repair and obtain new fresh judgment over
changed exact content.

When requested by a caller or declared consumer, Validate authors `verdict.v2`
and uses `ao provenance store-verdict` for structural verification and atomic
storage in the explicit protected external non-Git root. No default write to
consumer `.agents/` is implied. Legacy requested proof remains preserved.
The existing schemas are unchanged. Ledger availability never affects semantic
validity; a stored artifact does not prove acceptance.

## Progress, stalls and finish

Take the smallest acceptance-advancing action or resolve a consequential
uncertainty. Known defects get direct repair. Reserve real remaining capacity
for integration, final judgment, repairs and a compact handoff. Respect native
and caller time/cost/quota and any explicit repair bound. Retry counts alone are
not a spent time/quota budget; new subjects, compaction and helpers reset none.

A genuine causal stall (unknown cause, recurrence, no progress or wrong objective)
admits at most one bounded fresh helper per incident when authorized and within
bounds. Supply failed assumption, evidence and one discriminating question.
Resume with a testable different approach; an unhelpful response ends the attempt.
Do not create a helper chain, rename the same incident or consult help for an
understood failed check. Cancellation, refusal and spent real bounds skip help.
Keep recovery state in the native handoff only when needed to protect evidence.

Optional [outer-goal guidance](../../skills/rpi/references/outer-goal.md) stays
outside the core. The native controller owns aggregate enforcement and selected
future outcomes. Objective text is no proof of stop or budget enforcement.
The grandfathered pure Python [fixed-dispatch adapter](../../skills/rpi/references/bounded-adapter.md)
retains its optional once-only dispatch and finite review-round contract; it is
not a native execution engine or the authority for the lean charter's repairs.

Return the outcome, exact changed subject, strongest checks and material unchecked
acceptance. `NOT_PLANNED` and `NOT_BUILT` are progress statuses, not verdicts.
Do not report incomplete capability as complete. Interactive prose is sufficient;
`rpi-report.v1` and durable verdicts are optional unless a declared consumer needs
them. No new status ledger, process packet or lifecycle command is introduced.

## Optional Memory and specialists

[Memory](../../skills/memory/SKILL.md) supplies on-demand recall, separately
budgeted mining/learning, and curation/qualification/retirement. BD owns work,
status and handoffs; Git content; native/CASS sources episode evidence; and
caller-selected reviewed external Markdown topic pages reusable claims.
Reuse/update an existing topic page. An entry gives applicability, action,
support, limits and invalidation. Single incidents stay narrow; generalized
rules need stronger evidence and later reapplication. Retain rare useful
constraints; no blind TTL/deletion. Learning can remove rules and no-change is
valid. Saved pages prove no benefit; later task outcomes must establish reuse
helped, with harmful, failed and null outcomes visible.

This lean path admits public or already-cleared trial inputs only. It does not
implement native restricted-source isolation or grant automatic transcript
access. [ADR-0016](../adr/ADR-0016-state-tiers.md) governs protected external
non-Git staging and exact independent factual-support and destination-disclosure
review before any Git object/index/stash/import. Preserve legacy `.agents/`
evidence and owner recovery controls. Structure, labels and same-user subprocesses
are not confidentiality enforcement. Restricted-source productization remains
unavailable until its actual controls and adversarial evidence exist.

Anti-ceremony, Premortem, Postmortem, Council, research, genies and factory/runtime
adapters remain optional. A selected factory stays behind its own coordinator
and supervisor doors. No hard specialist or Memory dependency is added to RPI.
The three core skill dependencies describe available operations, not mandatory
worksheets. [ADR-0017](../adr/ADR-0017-loop-as-control-flow-not-knowledge.md)
records this lean amendment; previous selection/phase-lock prescriptions are
historical where superseded. No evolve root or scheduler is restored.
