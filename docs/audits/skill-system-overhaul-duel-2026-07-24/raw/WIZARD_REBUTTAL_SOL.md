# SOL formal rebuttal

Date: 2026-07-24
Identity: SOL
Inputs read completely: `WIZARD_REACTIONS_SOL.md`,
`WIZARD_REACTIONS_FABLE.md`

This rebuttal omits resolved issues. Kernel-first ordering, caller-owned
continuation, no skill-grantable `select_experiment`, compiler plus taxonomy
budget, proof epochs instead of rebaselining, contract-owned shared invariants,
`shared` retirement after relocation, Goal/Fitness vocabulary, and Security’s
evidence placement are consensus.

## Opponent claim 1 — Ambiguous CLI crossings may proceed by default

### Claim and exact disagreement

FABLE’s remaining CLI position is:

> When the T0 proof-chain trace is ambiguous or contested, continue the skill
> migration in parallel and block only crossings proven to be used.

SOL agrees that the CLI audit is a parallel release program, not a universal
prerequisite. The disagreement is narrower: an unresolved dependency may not
support a binding tranche PASS. Source edits and unrelated implementation may
continue, but the acceptance path is `NOT_PROVEN` until the ambiguity is
resolved. “Not proven used” is not equivalent to “proven unused.”

### Concrete source or migration evidence

- `AGENTS.md` says no evidence-backed verdict means the experiment is not
  proven, and missing or incomplete coverage yields `NOT_PROVEN`.
- The overhaul plan names mixed acceptance surfaces: Python frontmatter and
  mesh tools, shell regeneration, Bats, focused Go tests, `ao skills`,
  `ao gate check`, verdict checks, and runtime-specific probes.
- `cli/internal/gates/scriptrunner.go:136` is one verified unbounded-capture
  path. Later runtime/adaptor probes can also exercise audited subprocess,
  cleanup, or output behavior even if the initial schema tranche does not.
- The current broken orchestration-boundary gate demonstrates why a named
  acceptance command is not automatically a live, trustworthy proof path.

The command graph therefore has to include wrappers and descendants, not only
the executable names printed in the plan.

### Failure mechanism if FABLE’s ruling is adopted

A tranche invokes a wrapper believed to be safe. The wrapper reaches an
audited subprocess or output path that the static trace did not resolve. The
probe passes in the ordinary fixture but can buffer without bound, orphan a
descendant, leak an owned root, ignore dry-run semantics, or emit an output
shape the caller misreads. Because the crossing was merely “not proven,” the
tranche records PASS. The program then appears complete on a proof runner whose
relevant behavior was unknown.

This is not an argument to block all edits on all CLI findings. It is an
argument that an unknown proof dependency cannot establish a durable PASS.

### Falsifier

A complete acceptance-command inventory that expands every wrapper and child
process, maps each path to audited packages and effects, and demonstrates with
seeded fixtures that no unresolved path can affect exit status, evidence
identity, mutation, cleanup, boundedness, or output parsing would falsify the
need for a fail-closed default.

### Smallest synthesis rule

> Implementation may proceed in parallel; a tranche may issue PASS only when
> every acceptance-relevant CLI crossing is proven sound or proven unused.
> Unresolved crossings are `NOT_PROVEN`, not global work stoppages.

### Final confidence

**96%.**

## Opponent claim 2 — Distributed v3 population stands without an explicit publication barrier

### Claim and exact disagreement

FABLE states that the compiler tranche should contain grammar, parser,
fixtures, and strict Go consumers while the 49-file metadata population is
distributed to owning semantic tranches. SOL accepts that distribution.

The under-specified part is authority during the transition. FABLE’s reaction
does not preserve SOL’s required two-state rail: distributed v3 readiness may
be measured, but no partially populated v3 catalog may become authoritative.
One strict all-49 cutover is required after the distributed source edits.

### Concrete source or migration evidence

- `scripts/validate-skill-schema.sh` still validates v1 while
  `scripts/validate-skill-frontmatter.sh` validates v2.
- `cli/internal/skills/catalog.go` models neither the current semantic fields
  nor the proposed v3 contract and does not reject unknown fields, despite its
  parity claim.
- One current strict pass already reports 49 parsed sources with a missing
  optional field. Structural readability is therefore not semantic
  completeness.
- Generated routers, bounded-context membership, Codex projections, hashes,
  and CLI catalog readers all consume catalog-wide state. They cannot safely
  infer authority from a mixture of old and new entries.

This is the exact dual-authority defect the compiler is intended to remove.

### Failure mechanism if FABLE’s ruling is adopted

Early semantic tranches add v3 authority/effect/output declarations while
later skills remain v2. One generator begins reading the new fields, another
continues ignoring them, and the Go reader accepts the catalog because it does
not fail on unknown or mixed semantics. Generated views then advertise
authority for some skills, derive legacy tier for others, and omit effects for
the remainder. Every individual tranche can pass its local probe while the
catalog has no single meaning.

### Falsifier

An incremental catalog prototype would falsify the barrier if it can encode
per-entry versions, make every Python and Go consumer reject semantically mixed
sets, generate no authoritative projection until dependencies are complete,
and still avoid a second writable authority. That would be equivalent to the
barrier in another form; the current readers cannot do it.

### Smallest synthesis rule

> Populate v3 source contracts in owning tranches, but treat them as readiness
> data only; publish v3 authority exactly once after all 49 entries and every
> strict consumer pass the same golden corpus.

### Final confidence

**98%.**

## Underrated SOL decision 1 — Complete staging is required even with one writer

### Claim and exact disagreement

SOL’s revised publisher no longer requires an early multi-writer lock,
recovery journal, or exhaustive concurrency matrix. It does require every
owned projection to render and validate in a complete staging root before any
live generated path changes.

FABLE still proposes deferring full staging until parallel lanes exist, on the
ground that fail-fast serialization plus a manifest-last marker closes every
observed hazard. That underrates the single-writer failure mode.

### Concrete source or migration evidence

- `scripts/regen-all.sh` currently records failure and continues, proving the
  orchestration does not preserve a valid generation boundary.
- `scripts/generate-skill-mesh.py:249-325`,
  `scripts/codex-sync.sh:437-686`, and
  `scripts/regen-codex-hashes.sh:172-178` independently rewrite or remove
  derived paths.
- `skills/skill-builder/scripts/build.sh:31-40` and fix/heal paths are further
  direct writers.
- The overhaul performs catalog-wide source edits and then regenerates several
  shared projections. A validation failure halfway through that sequence is a
  normal single-process failure, not a concurrency edge case.

Manifest-last is a commit marker. It detects that publication did not commit;
it does not stop invalid bytes from reaching the live tree before the marker.

### Failure mechanism if the underrated decision is omitted

A direct writer deletes stale outputs and writes a subset of the new
generation. A later validation step fails. Fail-fast correctly stops, and the
old manifest correctly mismatches, but the working tree now contains neither
the previous valid generation nor the new valid generation. Subsequent checks,
diff review, or a rerun may consume the mixed tree. If a later manual step
updates the marker, the mixed state can be sealed.

Complete staging makes validation failure non-publishing. It contains the most
likely failure without needing a journal or concurrent writer.

### Falsifier

Instrument every current generator to fail after each mutation. If every
single-writer failure leaves all live owned paths byte-identical to the prior
valid generation—or automatically restores them before any consumer can run—
then complete staging is unnecessary. A manifest mismatch alone does not meet
that falsifier.

### Smallest synthesis rule

> Keep one serialized publisher and defer concurrency machinery, but require
> render-and-validate-completely-before-live-replacement from the first
> catalog-wide regeneration.

### Final confidence

**95%.**

## Underrated SOL decision 2 — Proof epoch identity must be a first-class contract field

### Claim and exact disagreement

Both reactions now adopt immutable proof epochs. FABLE suggests the epoch
digest could temporarily ride in `evidence_refs` or freshness notes until a
scheduled schema revision. SOL’s stronger revised rule is that epoch identity
must be a required, typed field in the verdict/report contract before epoch 1
exists.

This is not a request for another later schema churn. The kernel tranche already
changes the verdict writer/reader corpus and introduces `rpi-report.v2`; the
proof-contract identity belongs in that same atomic revision.

### Concrete source or migration evidence

- `schemas/verdict.v2.schema.json` currently has no dedicated
  validator-contract or proof-epoch field.
- `rpi-report.v1` cannot represent even the already promised opaque
  correlation object because its top level is closed; relying on untyped
  narrative carriage has already produced a live contract mismatch.
- Repository source precedence makes schemas authoritative. A convention in
  freshness notes or a generic evidence reference cannot require the Python
  writer and Go reader to agree on algorithm, digest syntax, or presence.
- The RPI/Validate digest mismatch survived passing isolated tests precisely
  because an identity invariant was implemented independently rather than
  bound by one executable contract.

### Failure mechanism if the underrated decision is omitted

Some verdicts include an epoch digest in a generic reference, others omit it,
and readers disagree about which reference denotes the proof contract.
Completion tooling groups the results as one epoch because every verdict is
schema-valid. A later schema change then makes cross-tranche comparisons
ambiguous, and the campaign recovers comparability through prose or
retrospective inference—the same semantic drift epochs were meant to prevent.

### Falsifier

If the existing `evidence_refs` schema already requires exactly one
algorithm-qualified proof-contract digest, distinguishes it from subject
evidence, and every Python/Go writer and reader rejects omission, duplication,
or malformed identity, a new field is unnecessary. The verified schema does
not provide those semantics.

### Smallest synthesis rule

> Add one required `proof_contract` identity object to the same atomic kernel
> schema/writer/reader revision that creates report v2; mint no epoch-1 verdict
> before every consumer enforces it.

### Final confidence

**97%.**

## The one disagreement the operator must decide

The remaining operator decision is the fail-closed default for an ambiguous CLI
proof-chain trace:

- **SOL:** implementation may continue, but the affected tranche cannot claim
  PASS until the crossing is proven sound or unused.
- **FABLE:** proceed in parallel and repair only proven crossings, even when the
  trace remains contested.

This is no longer “all CLI work first” versus “all CLI work separate.” It is
whether uncertainty about the proof runner permits a durable completion claim.

## Decision procedure and deciding evidence

T0 should generate an executable proof-chain ledger:

1. Enumerate the exact commands for every tranche acceptance criterion.
2. Expand shell/Python wrappers, Go call sites, spawned children, and output
   consumers to a transitive command/process graph.
3. Record for each edge: package/script owner, filesystem and external effects,
   dry-run semantics, output parser, buffering policy, timeout/cancellation,
   cleanup owner, and platform coverage.
4. Cross-reference every edge with the frozen CLI audit finding ledger.
5. Seed one negative fixture for every matched finding and one witness showing
   that each purportedly unused path is not reached.
6. Classify each edge `USED_SOUND`, `USED_UNSOUND`, `PROVEN_UNUSED`, or
   `UNKNOWN`.

Both positions agree that `USED_UNSOUND` blocks its consumer and
`PROVEN_UNUSED` does not. The operator must choose the policy for `UNKNOWN`.
The operating contract’s existing evidence rule supports SOL’s recommendation:
allow parallel implementation, but require `NOT_PROVEN` rather than PASS until
the edge is resolved.

## Point withdrawn completely

SOL completely withdraws the original claim that the entire Go CLI audit must
land before any skill-contract or behavioral migration. Only a verified
acceptance-path crossing is a skill-migration prerequisite. The CLI audit
remains an expedited independent release program, with eval containment urgent
on its own security merits.
