# ADR-0016: State Tiers — One Authority per Claim; Projections Never Authoritative

- **Status:** Accepted (2026-07-18), amended for external reviewed memory (2026-09-06)
- **Author:** AgentOps maintainers
- **Builds on:** [ADR-0004](ADR-0004-corpus-moat-unproven-position-on-the-system.md) (corpus moat unproven — position on the verification system), [ADR-0011](ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md) (escape-corpus compounding demoted to hypothesis)
- **Origin:** `.agents/brainstorm/2026-06-19-agentops-memory-state-substrate.md` (the governed-lakehouse brainstorm that first stated the one-authority invariant), `.agents/audits/2026-07-18-agents-writer-matrix.md` (the writer-matrix audit that exposed the junk drawer)
- **Tracking:** epic `age-state-tiers-operationalize-5mzlm` (this ADR is `.1`), tracker epic `age-tracker-bd-dolt-return-jyg2g`

## Active amendment — 2026-09-06 external reviewed memory

This amendment supersedes the earlier location-based tiers, automatic citation
promotion and wholesale scratch-expiry prescriptions where they conflict with
the selected CDLC contract. One authority per claim, federated sources, private
evidence and the Go implementation-language division remain intact. CDLC means
Context Delivery Lifecycle: maintained external context/environment around
disposable agents, without weight training or deterministic-inference promises.

### Authorities and routing

| Material | Selected authority and placement |
|---|---|
| Work/status/dependencies/allowances/closure | Native BD/Dolt, never a second AO or memory work store. |
| Revisions and delivery history | Native Git and caller repository policy; commits do not imply acceptance. |
| Raw episodes | Native session/source systems; CASS locates sessions, authorized bounded raw reads establish coverage. |
| Maintained claims | Caller-selected external reviewed Markdown/OKF bundle in ordinary Git, initially without a remote; evidence, not automatic policy. |
| Policy | Existing caller/repository policy owners; knowledge promotion is a separate authorized change. |
| New CDLC drafts/proof/projections | Owner-selected protected external non-Git storage; projections inherit source access restrictions. |
| Existing requested proof and unique research | Preserve exact existing files, citations and source ownership, including legacy `.agents/`. |

Use the latest stable upstream BD chosen by the caller. Verify native
`bd context --json` routing from root and relevant subdirectories against the
expected backend, resolved destination, database and project identity before
mutation. A directory basename or local config hint does not prove the store
identity or Go enforcement. Root AGENTS and cli/AGENTS retain tracker-routing
ownership; T00 owns migration. `bd` and `br` are distinct implementations; no
new writes through retired BR, no removed `ao beads dir`, no empty replacement
store on missing routing. Preserve legacy `_beads`, old `.beads` and private
history. BV is optional read-only analysis over a fresh explicit BD export;
recheck native readiness and goal constraints. No second work/status index.

Caller/home configuration maps native project/source identity **plus owner
scope** to permitted bundle, non-Git staging, evidence and access-policy
locators. Native maintenance work records permitted recovery references. Actual
private locators/identities stay there, not in public product docs. Inspect
existing caller content before reuse; never replace it. Missing or ambiguous
routing fails without silently initializing memory or selecting another owner.
Same-named directories, clones and worktrees do not merge personal, employer
or customer scopes. No consumer docs/wiki/root memory, tracked `.agents/`,
submodule, symlink, hook or committed config is created by default. The old
in-project wiki-init proposal is historical input, not the adopted recipe.

Search the small Markdown bundle with metadata and `rg` first; a derived index
needs a representative query comparison and a named consumer. CASS/CM/ee/MS
remain optional compatible sources and are not silently copied into AO state.
Preserve searchable supported references even when they do not justify method
promotion; never copy a transcript lake.

### Confidentiality order: review before every Git surface

The mandatory flow is **authorized source access → protected non-Git draft →
exact factual-support review and destination-disclosure review → constrained
Git ingestion → native atomic active-ref update**. Both reviews precede ANY
Git object, index, stash or candidate commit, including unreachable/alternate
objects. Keeping rejected bytes off an active ref is insufficient. Filenames,
locators, receipts, commit text, scanner output, diagnostics and indexes are
also disclosure surfaces. Opaque identifiers do not confer clearance.

Use existing exact-intent and verdict.v2 mechanisms with distinct immutable
expected acceptance supplied independently by the caller. Factual support asks
whether the exact claim follows evidence. Disclosure binds the exact payload,
paths/metadata, owner scope, destination and policy to permission to store it
there. Later usefulness needs task evidence. One fresh authorized reviewer may
judge support and disclosure unless stronger separation is requested; neither
substitutes for the other, and the author cannot approve its own knowledge.
The candidate cannot choose a permissive policy. Changed content/metadata or a
new destination, recipient, public surface or policy requires applicable fresh
review before import. Rejected/malformed candidates remain outside Git.

Knowledge/disclosure intents, manifests, drafts, verdicts and runtime receipts
resolve to the explicit protected external evidence root before storage, with
no fallback to consumer-workspace proof. Existing standalone product-proof
placement remains caller-owned. Current helper defaults are not the complete
selected routing: [Validate mechanics](../../skills/validate/references/mechanics.md)
discloses that limit. Later Go owners must implement and prove it before those
entrypoints process restricted CDLC material.

Before reading a page, private citation, CASS hit or BD comment, the selected
runtime verifies task, owner, source, model/provider and destination authority.
Read permission does not grant model transmission, Git storage or exposure in
implementation outputs. Never retrieve denied material and redact afterward.
Restricted, unavailable, no-match and insufficient evidence are distinct; unread
ranges remain in protected coverage accounting. Withheld necessary evidence can
leave validation NOT_PROVEN.

The native runtime/OS must enforce miner/writer separation: miners can read
only authorized sources and write protected non-Git staging, without wiki/Git
writes or unauthorized egress. Writers receive only immutable approved payloads
and permitted review evidence, cannot access raw sources/unrelated drafts or
change destinations. Reviewers must be authorized for their inputs/provider.
A prompt, worktree, chmod or same-user unrestricted subprocess is not isolation.
Synthetic canary denials and bypass trials precede real restricted inputs;
unsupported protection is unavailable and prevents a full privacy release.
Synthetic or already-cleared substitutes establish only the narrower mechanism.

Secret scanning and semantic review supply limited evidence, not perfect
redaction or declassification. Report known false approvals/exclusions and
canary coverage; model agreement is no zero-leak proof. Uncertain sensitive
material stays source-only under owner controls. An approved private page
remains private. This is no claim of protection against a compromised host,
a malicious authorized recipient or arbitrary leakage after authorized delivery.

### Legacy preservation and recovery

Preserve requested `.agents/ao/` judgments/evidence exactly. Before cleanup,
inventory ownership, citations, unique copies, reproducible sources, real
consumers and actual Git exposure. Citation makes material relevant evidence;
it does not automatically authorize relocation, disclosure or policy promotion.
Mine only selected authorized sources incrementally, never whole directories.
Unknown ownership and inaccessible originals remain protected inventory gaps.

T38 owns bounded legacy inventory and recoverable routing, with no move/delete
authority or claim that all historical content was read. T19 alone owns later
explicitly selected migration, with backup/restore proof, reference readback
and original-hash preservation. Scratch expiry uses protected quarantine only
after unique evidence and owner retention are checked; there is no blanket TTL
delete or promise to keep sensitive mistakes forever. Existing requested thread
evidence stays where requested. Existing Git/public exposure is an incident
for separate owner handling; consolidation or rollback cannot retract disclosure.

Use the caller's supported access/encryption and backup systems for restricted
sources, drafts, evidence and indexes; keep keys outside Git and model context.
Prove restore before moving unique evidence. A private remote, `.gitignore` or
path guard is not encryption or semantic clearance. No remote sync is enabled
by selecting the bundle. Encrypted Git is a separate deferred optional profile,
not this baseline; no backend or custom crypto is selected. It would need
reviewed plaintext bound to ciphertext/revision, protected metadata, external
keys/recipient policy, pre-object encryption and tested recovery/revocation
limits. Ordinary Git search over ciphertext is not Recall.

### Measurement and implementation limits

Mechanism correctness and demonstrated net benefit are separate claims. Exact
factual support, disclosure permission and later observed utility are distinct.
The bounded pilot observations support only their declared scope: synthetic
native controls passed; a supported disclosure-approved page entered only the
external private Git bundle and malformed/rejected content stayed out. A
historical recorder mode gap remains recorded. Later native launches used
umask 077, with recorder permissions observed at specific points; those
observations prove neither continuous nor retroactive protection. This is no
general privacy proof.

The cold-reuse trial froze one synthetic episode, two fresh workers and nine
cases per worker: X passed 9/9; Y passed 4/9 and independently failed for an
unjustified mandatory declared_event_count assumption. Both retrieved source
references; neither requested the optional knowledge body. Whole-trial
acceptance preserves Y's FAIL and **no demonstrated memory benefit**. A new
independent cohort is required for any positive improvement/compounding claim.
No universal context percentage, numeric utility score, page quota or mandatory
lesson per session is introduced. Keep failed, null, missing and harmful results.

T04 adopts these contracts; Recall/Learn, protection productization and Go
manifest/snapshot/verdict-storage implementation have later owners. The first
selected runtime target is skills plus the Go `ao` binary for required evidence
operations, with shared conformance before replacement. The existing Python
ratchet, specialist grandfathering and development-generator exemption remain;
no shipped Python logic, lifecycle command family or schema is added here.
`learning.coherence`, delivered bytes, citations and closed work cannot prove
OKF correctness, semantic acceptance or usefulness.

## Historical context

AgentOps accumulated state the way a workshop accumulates benches: every tool
minted its own directory, and by 2026-07-18 the `.agents/` workspace held 114
top-level directories with no declared owner, lifetime, or authority. The
2026-06-19 memory-state-substrate brainstorm had already named the underlying
confusion — the repo was mixing work-graph state, raw operational memory,
proof/judgment records, curated knowledge, and analytics in one undifferentiated
pile — and had stated the governing invariant: *every durable claim has exactly
one authority, and every derived view names the sources used to build it.*

Two 2026-07-18 decisions made the model operational rather than aspirational:
the tracker returns to bd + Dolt native (epic `age-tracker-bd-dolt-return-jyg2g`;
Gas City is bead-native), and Bo fixed the implementation-language division for
everything that ships. ADR-0004 and ADR-0011 supply the posture this ADR
extends: position on the proven verification/control system, keep unproven
knowledge-accrual machinery out of the product, and never let a derived artifact
masquerade as a source of record.

## Decision

### 1. The four state tiers

| Tier | Location | Authority | Lifetime |
|---|---|---|---|
| **Work** | beads (bd + Dolt, local Mac service per the 2026-07-18 tracker decision) | Source of record for what work exists, its status, dependencies, and close reasons | Permanent, versioned by Dolt |
| **Proof** | `.agents/ao/` | Source of record for verdicts, receipts, evidence, and pinned config | Permanent, append-only |
| **Canon** | `docs/` + `docs/adr/` | Source of record for rules, architecture, and decisions that change future behavior | Permanent, edited deliberately |
| **Scratch + projections** | remainder of `.agents/` | No authority — TTL'd work pad plus generated, manifest-stamped projections | Ephemeral; promotion-or-death |

The work tier is **queried, never indexed**: bead questions go through SQL
(Dolt views) and `bv` graph analytics directly against the store. No stored
index of bead data is ever built or committed — see invariant 2 below.

**Target layout.** The scratch/projection tier is intended to collapse from 114
ad-hoc directories to three preferred top-level `.agents/` entries:

- `ao/` — the proof tier (permanent; pawl evidence, verdicts, pinned config),
- `scratch/` — all ephemeral work, convention `scratch/WRITER/DATE-SLUG/`, TTL'd wholesale,
- `projections/` — generated artifacts with manifests, deletable at will.

This is a target state, not a currently enforced closed set. The planned
`fm-ws-noncanonical-topdir` detector (bead
`age-state-tiers-operationalize-5mzlm.7`) was not implemented. Current Doctor
checks own narrower classes such as spelling drift, empty directories, and
stale queues. Compatibility and exact-path consumers also keep declared roots
outside the preferred three: legacy `.agents/handoff/` is preserved read-only;
`.agents/mto-handoff/` is a distinct live recurrence protocol; and explicitly
selected earlier output paths remain supported where their owning skill says
so. Those exceptions are migration contracts, not new authority tiers. Doctor
receipts still live under repo-root `.doctor/`, outside `.agents/` entirely.

### 2. The invariants

1. **One authority per claim.** Every durable claim has exactly one source of
   record. If two surfaces disagree, one of them is by construction a stale
   projection, and the tier table above says which.
2. **A queryable SOR needs no stored index.** Dolt gives the bead store a full
   SQL engine and `bv` gives it graph analytics; building and storing an index
   over it (a JSONL digest, a wiki index, a cached matrix) creates a second
   surface that can drift. Stored indexes are caches for demonstrably slow
   queries only — never authorities, never committed as truth.
3. **SQL views cannot go stale; filesystem-input snapshots can.** A Dolt view
   is re-evaluated against live bead data on every query, so it needs no
   freshness apparatus. Any projection whose inputs are *files* (source trees,
   skill catalogs, audit scans) is a snapshot and MUST carry a manifest
   (`generated_by`, `inputs`, `generated_at`) so a reader can detect staleness
   mechanically.
4. **Promotion-or-death, via quarantine-rename — never delete.** Scratch
   content either promotes to a tier with authority or expires. Expiry is a
   quarantine-rename with a receipt, with actual disposal a separate operator
   decision — never a direct delete at TTL. The safety rationale is corrected
   from earlier drafts: `.agents/` is gitignored (only `ao/config.yaml` is
   tracked) and cass indexes session transcripts, not script-generated
   artifacts, so a generated file in scratch can be the *only copy in
   existence*. A TTL that deletes would destroy unrecoverable state.
5. **The three-question promotion rule.** At TTL triage, ask in order:
   1. *Is it cited by a verdict or bead close?* Then it is evidence — it
      belongs in `ao/` or the close reason. Citation IS promotion, automatic.
   2. *Would it change what a future agent does* (a rule, gotcha, constraint,
      decision)? Promote to docs/ADR if repo-wide, to the bead description or
      a comment if scoped to one work item.
   3. *No identifiable consumer?* That is hoarding. Let the TTL expire it to
      quarantine. Might-be-useful-someday is not a consumer.

### 3. The division rule (implementation language, fixed 2026-07-18)

- **Mechanism, trust, and receipts ship in the `ao` Go binary.** Anything that
  enforces, verifies, mutates user files, or writes receipts is Go. A single
  static binary IS the distribution story.
- **Know-how ships as skills with references.** Recipes, judgment guidance,
  and reusable method live in `skills/**/SKILL.md` plus reference files.
- **Glue is POSIX `sh` only**, thin argument-plumbing over declared tools.
- **Python never ships in skills.** Interpreter dependencies break determinism
  on user machines; a skill that needs logic beyond `sh` glue routes that logic
  into an `ao` subcommand instead.
- **Prototype anywhere, ship in Go.** The scratch tier accepts any language —
  prototypes die by TTL, so they carry no maintenance debt. What survives the
  promotion questions is rewritten into Go before it ships.
- **Promotion into `ao` has a usage bar.** A new subcommand requires either a
  gate that needs it or demonstrated repeated cross-session use, because every
  subcommand is permanent maintenance surface. The cautionary case is the
  memory-moat machinery: roughly 4,400 lines of Go accreted around an unproven
  claim and had to be removed wholesale (`age-7grl`, per ADR-0004's honesty
  posture). Worked example of the intended flow: the writer-matrix audit was
  prototyped as scratch Python, proved its value, and ships as an `ao doctor`
  detector in Go (bead `age-state-tiers-operationalize-5mzlm.2`).

#### Amendment 2026-07-25 (enforcement and the tests carve-out)

The language rule above was fixed on 2026-07-18 and named its own violation "a
gate failure" — but no gate existed. It was inert prose for seven days while the
tree kept accumulating interpreter dependencies, which is the identical defect
this ADR diagnoses elsewhere: an authority nobody executes. This amendment
records the enforcing check and the one scope decision it depends on, so neither
lives as an unstated exception.

- **Enforced by `scripts/check-skill-python-ratchet.sh`** (gate ID
  `skill.python-ratchet`, blocking). It is a shrink-only ratchet: the 24
  execution-path files present at the 2026-07-25 cutoff are pinned in
  `scripts/.skill-python-grandfather` and exempt; every new one is a hard
  failure; a pinned file that is promoted into `ao` must be pruned from the
  snapshot; the growth guard rejects allowlist additions, so a change cannot
  exempt itself in its own diff. The surviving count prints on every run.
- **"Ships" means the user execution path.** The rule governs
  `skills/*/scripts/**` — what a skill actually invokes on a user's machine, and
  therefore what carries the interpreter dependency that breaks determinism
  there.
- **`skills/*/tests/**` is exempt as a class.** Test code never executes on a
  user's machine, so the determinism argument does not reach it; a skill's tests
  run in this repository, where Python is already a declared development
  dependency. This is a genuine refinement of the rule's scope, not a loophole —
  but it is recorded here precisely because an unwritten carve-out would decay
  into the same inert-prose failure. Anything that migrates from `tests/` into
  the execution path loses the exemption at that moment.
- **Generated projections are governed at their source.** `skills-codex/**` is
  regenerated from `skills/**`; it is never independently governed, per this
  ADR's own title.
- **Un-promotable code is an amendment, not an allowlist entry.** If a file
  genuinely cannot become an `ao` subcommand, that case is made per file, here,
  with its rationale. Widening the snapshot is rejected mechanically.

### Amendment 2026-08-07 (federated source authority)

The tier table names authorities inside this repository's own state; the
operations-layer alignment makes the external half explicit. AgentOps links
source identities in a federated integration graph; it does not absorb their
authority into `.agents/`:

- **The tracker (Beads) owns work** — what exists, status, dependencies, close
  reasons. Queried directly; never re-indexed into a second work store.
- **Git owns source content and delivery history.** AgentOps binds exact
  content identity when useful; a commit or merge never implies semantic PASS.
- **CASS and CM remain source systems** for past sessions and curated memory.
  Retrieval is on demand, cited with provenance and freshness; their output is
  evidence, never policy, and is not merged into a local knowledge lake.
- **`.agents/ao/` is requested proof, not a general knowledge lake.** It holds
  verdicts, receipts, intent snapshots, and pinned config that a caller or
  declared consumer asked for — nothing accumulates there by default.
- **`.agents/scratch/` is disposable work**, convention
  `scratch/WRITER/DATE-SLUG/`, expiring by quarantine-rename.
- **`.agents/projections/` holds named-consumer, manifest-stamped derived
  views.** A projection exists only while it has a named consumer and is
  cheaper than re-querying its sources; deleting it must not change semantic
  behavior.
- **No stored index duplicates a queryable source without measured need.**
  Invariant 2 generalizes beyond beads: Git, CASS, CM, and any other queryable
  source system get the same treatment — caches for demonstrably slow queries
  only, never authorities.

## Consequences

- The three-directory layout remains the preferred migration target, but no
  catch-all detector enforces it today. New default writers that mint an
  undeclared top-level directory are source bugs; declared legacy-read and
  exact-path consumer exceptions remain in place until their own migrations
  complete.
- No tool may build a stored index over bead data; bead reporting goes through
  Dolt SQL views and `bv` (the beads-views skill,
  `age-tracker-bd-dolt-return-jyg2g.8`). Filesystem-input projections without
  manifests are findings.
- TTL enforcement anywhere in the system quarantine-renames with a receipt;
  a deleting TTL is a defect against this ADR.
- Shipping Python inside a skill is a gate failure, not a style nit; the
  prototype path through scratch exists precisely so that rule has no cost. The
  gate is `skill.python-ratchet`
  (`scripts/check-skill-python-ratchet.sh`) — named here because a mechanical
  claim with no named check is indistinguishable from advisory prose.
- [docs/agents-dir-hygiene.md](../agents-dir-hygiene.md) remains the operating
  manual for the scratch tier (TTL mechanics, drift aliases, doctor detectors)
  and will be rewritten around this tier table (bead `.5`); where the two
  disagree, this ADR wins.
