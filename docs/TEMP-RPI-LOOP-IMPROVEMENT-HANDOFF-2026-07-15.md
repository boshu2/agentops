# Sanitized RPI Loop Improvement Handoff

> Temporary cross-machine technical context, synthesized 2026-07-15. All
> proprietary subject matter, local identities, absolute paths, internal
> document names, and organization-specific details have been removed. This is
> advisory evidence, not a verdict, assignment, implementation Plan, or
> continuation decision.

## Goal

Combine independently observed AgentOps RPI runs, audit disagreements against
durable evidence, deconflict overlapping recommendations, and preserve one
self-contained context document for an agent improving the RPI loop.

## Repository state at synthesis

- Repository: `boshu2/agentops`
- Branch: `main`
- HEAD: `baaa9e22dadc423d7b7bb73c043112ecfca425cb`
- Worktree: dirty, containing multiple intentional candidate sets.
- `ao packet` exists in the dirty source checkout but not in the pinned HEAD.
- Installed executable: `v3.2.0-229-g4b3188438`; it rejects `ao packet`.
- Dirty source executable observed through `go run`:
  `v3.2.0-725-gbaaa9e22d-dirty`; it contains `ao packet`.

Inspect current state before editing. Do not clean, reset, or overwrite the
mixed worktree while consuming this handoff.

## Consolidated diagnosis

The RPI semantic model is strong. Exact Plan and acceptance identity, bounded
subject scope, one fresh validator, durable verdicts, and report-and-stop
behavior prevented false success and preserved trustworthy outcomes.

The runtime path is not yet coherent. Across the mined sessions, agents used a
hybrid environment:

- live skills from a modified checkout;
- an older installed `ao` without the packet family;
- an uncommitted packet implementation invoked through `go run`;
- manual JSON and packet assembly;
- network-sensitive schema resolution;
- unstructured shell evidence and repeated orchestration polling.

This exercised an integration-in-progress rather than one released AgentOps
product. The highest-value improvements are release coherence and deterministic
packet tooling—not weaker validation.

## Evidence sample

The synthesis covers five durable verdicts over four distinct objectives:

| Objective class | Verdicts | Evidence status |
|---|---|---|
| Packet/fresh-validator migration | first `FAIL`, revision `PASS` | Both inspected |
| Scope and external-state objective | `NOT_PROVEN` | Supplied digest references only |
| Pre-existing-gate objective | `NOT_PROVEN` | Supplied digest references only |
| Non-repository documentation objective | `PASS` | Packets, transcript, and verdict inspected |

This sample supports prioritizing experiments. It does not establish universal
latency targets because multiple runs shared the same host, dirty checkout, and
installed/source version skew.

`verdict.v2` currently contains finding IDs but no finding digests. Where a
finding-level selector is needed, this handoff uses
`verdict artifact digest#finding-id`.

## Objective A: packet and fresh-validator migration

### Durable identities

- Plan digest:
  `c9ed05947a81d6c600706a6d495e4a27a71b5c5262f50d04cfda1085c0efce34`
- First candidate subject-manifest digest:
  `29faaebd4c3bdb9f010cfb7f3bad8257191d5c34ffb54aee417a40ebed006b68`
- First verdict digest:
  `30b2b0e9de11edc166b7372304015c93b796d08d18bc9decebc2863ba780cf4e`
  (`FAIL`)
- Finding selector:
  `30b2b0e9de11edc166b7372304015c93b796d08d18bc9decebc2863ba780cf4e#validate.cli-surface-smoke-stale-counts`
- Revision delta-manifest digest:
  `d9c0edd20c1743e31c8ad6e0934953db87a5b272bded1492ae5442877050c44c`
- Final candidate digest:
  `2c322947f57c302fa3a6711545e5cf6606dcf942ca070827cb58d25dade0be47`
- Final verdict digest:
  `afa2cda534ac4dc66adbfb1c9c871ff89a5801352a124441b1aecfc5f8b26d50`
  (`PASS`, no findings or unchecked items)

### Timeline and finding

Total elapsed time was 32m42s:

| Boundary | Elapsed |
|---|---:|
| Start to PlanPacket | 3m14s |
| PlanPacket to first manifest | 14m18s |
| Manifest to first CandidatePacket | 1m51s |
| First fresh validation | 5m00s |
| `FAIL` to RevisionPacket | 1m24s |
| Revision implementation and CandidatePacket | 1m08s |
| Second fresh validation | 5m07s |
| Final report and closeout | 40s |

The first validator caught a real generated-surface defect: the command matrix
expected 34 top-level, 49 subcommand, and 83 total headings, while an executable
smoke fixture still asserted 33/44/77. Several generation and parity checks had
passed, but their fragmented ownership did not close the executable fixture.
The revision used the owning count generator and passed.

Other observed friction included manual packet assembly, schema-resolution
hangs, incomplete write-scope companion discovery, stale built-binary test
execution, dirty-worktree path subtraction, and roughly five-minute fresh
validation phases.

## Objectives B and C: supplied `NOT_PROVEN` evidence

A separate mining brief reported two distinct objectives and two
`NOT_PROVEN` verdicts:

- `3a7d47a7dbc7e5d9ee97bdbb22854fe08c55a8023242d577b5800048f0d872b8#validate.scope-not-provable`
- `3a7d47a7dbc7e5d9ee97bdbb22854fe08c55a8023242d577b5800048f0d872b8#acceptance.s2-sbh-link-state`
- `c64248b27c00749f2bbc0646e8220933087452a64a37b799beb4a3244e33e886#preexisting-shellcheck-redness-blocks-exit-0`
- `c64248b27c00749f2bbc0646e8220933087452a64a37b799beb4a3244e33e886#overall-not-proven-rationale`

The strongest cross-objective claim was that Plans reached Implement with
evidence contracts that were schema-valid but mechanically invalid or
unprovable at baseline:

- one Plan used absolute scope paths that later failed the scope helper;
- another required a green gate that was already red before implementation.

The corresponding artifacts were unavailable during synthesis. Treat these as
externally supplied claims pinned by digest, not independently revalidated
facts.

## Objective D: non-repository documentation lifecycle

### Durable identities

- Plan digest:
  `bf450f0a9431c161c9a65c1ceff7100b1b45d1b75f063c7e8df716408a3b5a6f`
- Subject-manifest digest:
  `30e2a7166220d27296fe4ad12f995380d4babf04a26ef13a00f33cf11841fba2`
- Verdict digest:
  `a027a08480ec333d2bada0f3671c17b744076b545834df48a6bb54ec904ff250`
  (`PASS`)

The source artifacts and subject-specific filenames are intentionally omitted
from this sanitized handoff. The digest identities are retained so an
authorized holder of the original evidence can correlate them.

### Audited outcome

The `PASS` was supported. Independent audit confirmed:

- PlanPacket, CandidatePacket, subject manifest, and verdict validate offline
  against a local schema registry.
- The verdict's canonical content digest matches its artifact digest.
- The Plan digest recomputes to the CandidatePacket's bound digest.
- The frozen ten-file subject still matched its manifest when audited.
- Plan-to-Candidate scope comparison returned `PASS` with no out-of-scope
  paths.
- The verdict recorded complete link and inventory checks, all subject files
  inspected, `findings: []`, and `not_checked: []`.
- The transcript showed exactly one fresh validator with no inherited turns, a
  distinct validator identity, no subject repair after candidate freeze, no
  Git mutation, and one final durable verdict.

The Plan covered one coherent behavior across ten documentation paths. Its
normal and edge scenarios covered stale, ambiguous, and inactive material. Its
non-goals prevented repository, runtime-system, historical-state, publication,
and authority scope expansion. Implementation stayed within all ten paths and
retained one malformed diagnostic as evidence.

### Timeline

Six control artifacts totaled 30,978 bytes and spanned approximately 17m55s:

| Boundary | Relative time |
|---|---:|
| Intent materialized | 00m00s |
| PlanPacket frozen | 01m20s |
| CandidatePacket produced | 09m28s |
| Verdict persisted | 17m55s |

Candidate-to-verdict elapsed time was 8m27s.

### Confirmed runtime friction

1. The installed `ao` rejected all packet commands. The session discovered the
   untracked source implementation, first invoked `go run` incorrectly,
   corrected its syntax and working directory, and then used dirty source for
   all packet operations.
2. CandidatePacket schema checking hung once in the author context and once in
   the validator context because the Candidate schema combines an HTTPS `$id`
   with a relative subject-manifest `$ref`. The Plan schema has no external
   reference and validated quickly. The validator subprocess required external
   interruption.
3. A composite inventory diagnostic exited 24 because it required an index to
   enumerate itself. A corrected read-only coverage diagnostic then proved the
   intended payload set without another subject edit.
4. Packet construction duplicated intent, acceptance, scenarios, hashes, and
   prose evidence across six manually authored control artifacts.
5. The exact RPI report correctly exposed semantic `PASS` but did not expose
   phase timing, schema hangs, executable skew, or the malformed observation.

## Audit classification of the third perspective

### Confirmed

- The hybrid installed-skill/source-CLI diagnosis.
- The semantic `PASS`, exact ten-path scope, and complete validation surface.
- One Plan, one Implement candidate, and exactly one fresh validator.
- The 17m55s artifact span and 8m27s Candidate-to-verdict interval.
- Two Candidate-schema hangs across the author and validator contexts.
- The exit-24 malformed inventory observation and later isolated coverage
  proof without a subject edit.
- Manual packet volume of 30,978 bytes across six control artifacts.
- The current RPI skill check is grep-only and passes even though the installed
  `ao` lacks every required packet command.
- Author, validator, and freshness identities are self-declared packet strings
  rather than runtime-injected opaque identities.

### Qualified

- Two observed schema hangs means one author-side attempt and one
  validator-side attempt—not two validator subprocesses.
- Packet volume added handling cost but was not demonstrated as the dominant
  delay. The schema/tooling hang was the clearest observed cause.
- A `source: runtime` freshness attestation satisfies the current declared-trust
  contract. Runtime-injected identities would strengthen assurance and
  ergonomics; their absence does not retroactively invalidate the `PASS`.
- Operational health should be exposed separately. Adding durations and
  tooling events directly to the semantic report risks mixing completion truth
  with telemetry. A companion observation artifact is the cleaner default.

### Rejected

- The ten-file scope was not inherently too broad. Exact manifest, scope,
  links, inventories, and every criterion passed.
- Fresh validation was not waste. Independent recomputation made the result
  trustworthy.
- A clean subject verdict does not prove the runtime path is coherent or
  efficient. Verdict meaning and orchestration health are different claims.

### Incomplete evidence

- It is unknown whether observed validation latency generalizes beyond this
  host/runtime and dirty checkout.
- The two externally supplied `NOT_PROVEN` verdicts remain unavailable for
  direct artifact inspection.
- It is unknown how much packet sealing, evidence receipts, and offline schema
  validation will save until measured across multiple distinct runs.

## Invariants to preserve

- One bounded Plan -> Implement -> fresh Validate invocation.
- Exactly one fresh validation sub-agent after a candidate exists.
- No validator for `NOT_PLANNED` or `NOT_BUILT`.
- Exact Plan, acceptance, and subject identity across phases.
- A failed or malformed observation remains visible evidence.
- No semantic judgment inside `ao packet`.
- No Git requirement for semantic completion.
- No automatic repair, retry, continuation, delivery, or release.
- Durable `PASS | FAIL | NOT_PROVEN`, then report and stop.

## Consolidated improvement suggestions

### P0 — Release coherence and executable pinning

Before Plan materializes artifacts, resolve exactly one packet-capable
executable, record its version and content digest, verify every command required
by the live Plan/Implement/Validate/RPI skills, and use that same executable for
the whole invocation. Fail before packet creation if requirements are not met.
Never silently fall back to an uncommitted source checkout.

`ao capabilities --json` already exists; do not create a parallel discovery
surface. Bind skill requirements to stable command contract IDs and add a
release cross-product test that loads shipped skills against the shipped
binary.

Candidate deterministic check:

```text
Given the shipped core-loop skills
When the shipped ao capability document is inspected
Then every required packet command contract ID exists
And its output, effects, arguments, and exit classes match the skill contract
```

### P0 — Typed packet command contracts

The current packet family registers directly, mutates the default command
allowlist, and does not attach the typed command metadata used by normal
modules. The capability document consequently advertises inaccurate metadata
such as `ao.packet.digest` with `effects: mixed` and `output: none`.

Move packet commands through the standard module/profile boundary and declare
stable IDs, argument policies, outputs, effects, and exit classes. The
release-coherence gate depends on those facts being accurate.

### P0 — Offline `ao packet validate`

Add schema validation for PlanPacket, CandidatePacket, subject manifests,
RevisionPacket, and verdicts using embedded schemas and a bundled local
reference registry. Tests must pass with network access disabled and cover
relative cross-schema references.

This is mechanical packet validation, not semantic subject judgment.

### P1 — Explicit Plan and Candidate sealing

Avoid independent helpers that still require agents to compose JSON with ad
hoc shell and Python.

1. A Plan seal/check validates offline schema, normalized relative scope,
   include/exclude collisions, logical subject anchor, required CLI
   capabilities, and observed versus expected first-check polarity before
   emitting an immutable PlanPacket.
2. A Candidate seal consumes the Plan digest, before/after manifests, author
   identity, runtime subject locator, caller-authored acceptance results, and
   evidence receipts. It derives changed paths, verifies scope and receipt
   bindings, and emits one immutable CandidatePacket and digest.

The CLI assembles and verifies facts; it does not choose acceptance, write
scope, verdict, or continuation.

### P1 — Structured check and evidence receipts

Represent each check with:

- stable check ID;
- exact argv/command and working directory;
- expected pre-state and post-state;
- observed exit code;
- start/end timestamps;
- input subject-manifest digest;
- executable version and content digest;
- stdout/stderr artifact digests or content-addressed paths;
- runtime identity.

Large transcripts should live in content-addressed evidence artifacts rather
than duplicated prose inside CandidatePacket.

Define malformed check execution separately from semantic failure. A command
that cannot observe its intended condition may emit `OBSERVATION_INVALID` and
be superseded by a corrected read-only observation while the subject,
acceptance, and phase remain unchanged. This must not permit subject repair,
acceptance mutation, another Implement phase, or automatic retry toward green.

### P1 — Before/after manifests and build provenance

Capture a base manifest over declared roots before editing, a current manifest
after editing, and derive additions, changes, and deletions mechanically. This
removes Git cleanliness and author-written hash journals as implicit proof
requirements, including for non-Git workspaces.

For executable checks, bind the receipt to the actual binary digest or build a
fresh temporary binary from inspected source. This prevents stale binaries
from being confused with current code behavior.

### P1 — Generated-artifact closure receipts

Allow a repository to declare one deterministic closure command for a source
owner. For AgentOps CLI surfaces, one closure should cover the matrix,
generated docs, parity, count owner, and executable smoke test. Candidate
sealing may require this repository-declared receipt; generic packet tooling
must not infer or silently expand arbitrary repository write scope.

### P1 — Runtime-provided context identity when available

Runtime adapters should inject opaque author/validator context IDs and bind the
fresh-validator spawn receipt automatically. Retain an explicit declared-trust
fallback for runtimes that cannot provide identities; do not silently present
caller-authored labels as stronger isolation than they prove.

Any change to whether caller-attested identity can support `PASS` is a semantic
contract/version decision, not a mechanical CLI patch.

### P2 — Revision evidence invalidation and validator briefing

Replace free-form reusable-evidence strings with receipt bindings covering
command, working subject, input manifest, tool capability, executable, and
output digests. Generate a revision brief partitioning evidence into
`reusable`, `invalidated`, and `must_run_fresh`.

The fresh validator still judges every acceptance criterion. Evidence reuse
only avoids repeated mechanical reconstruction.

### P2 — Runtime progress and operational-health sidecar

Separate two concerns:

- optional adapter progress events reduce blind polling;
- a non-semantic `rpi-run-observation.v1` records phase durations, tooling
  events, schema/packet retries, executable skew, revision count,
  generated-closure failures, and evidence reuse.

Keep those observations out of verdict semantics. `rpi-report.v1` can remain
the minimal semantic boundary; a future report version may carry an optional
observation reference rather than inline operational diagnosis.

Collect at least ten distinct objectives before promoting latency targets or
new default timeouts.

## Dependency order

The improvements have a natural dependency order, without implying assignment
or authorization:

1. accurate typed packet capability contracts;
2. installed skill/binary release-coherence gate and executable pinning;
3. embedded offline packet schema validation;
4. Plan/Candidate sealing, receipts, and before/after manifests;
5. generated closure and build provenance;
6. runtime identity/spawn receipts;
7. revision evidence briefing and operational telemetry.

## Unresolved facts and risks

- Two supplied `NOT_PROVEN` artifacts were unavailable for direct inspection.
- Five verdicts over four objectives remain a small sample.
- Multiple observations share one host and one dirty checkout, so recurring
  skew and schema friction are corroboration but not independent release
  replication.
- Concurrent feedback implementation may be changing the checkout; re-inspect
  current source before applying any suggestion.
- It remains unknown whether five-to-eight-minute validation phases generalize.
- The direct packet/default-allowlist registration may be intended architecture
  or an in-scope workaround; current capability metadata is inaccurate either
  way.

## Sanitization statement

This handoff intentionally omits:

- organization, product, project, team, and person names;
- source-workspace and user-home paths;
- proprietary document names and subject content;
- infrastructure, repository-host, environment, and operational identifiers;
- secrets, credentials, tokens, hostnames, and network addresses.

Only generic AgentOps mechanics, public repository identity, packet/verdict
digests, timing measurements, and generalized evidence claims remain.

## Stop boundary

This document records audited context and candidate suggestions only. It
contains no selected continuation, assignment, retry instruction, tracker
mutation, delivery decision, publication authority, or new semantic verdict.
