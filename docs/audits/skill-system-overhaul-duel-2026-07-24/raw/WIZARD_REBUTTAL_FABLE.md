# WIZARD_REBUTTAL_FABLE — formal rebuttal

- Author: FABLE (`claude-fable-5`)
- Date: 2026-07-24
- Read for this phase: both reaction files, completely.
- Scope discipline: consensus is not relitigated. After the reactions, the
  convergence list is long — kernel-first (atom withdrawn), verdict epochs,
  CLI-as-parallel-program with a T0 crossing matrix, model-dispatch split then
  `shared` retirement, vocabulary-pin-then-rename for `fitness`, security as
  evidence with separately authorized mutation, compiler+budget merge with
  `select_experiment` deleted, collapsed tranche table, line cap retained as
  interim, and SOL's all-49 cutover rule for v3 (which I accept as the missing
  guard on my distributed population). None of that is argued again here.

---

## Part A — the two claims in SOL's reaction that remain most wrong or under-specified

### A1. "Complete staged rendering is required before mass catalog edits" (SOL F3 reaction; cross-plan decision 4)

**Claim and exact disagreement.** SOL demotes locks and crash journals to a
concurrency-triggered ratchet (agreed) but holds that staged-complete
rendering — extract render-to-directory modes from every writer, validate in
staging, atomic-replace into the live tree — must land *before mass metadata
edits*, because "fail-fast direct writes plus a final manifest detect partial
publication but do not prevent it." My position: the property SOL wants
(no damaged live tree persists) is already purchasable at ~5% of the cost,
because every owned projection output is a git-tracked file; full staging
belongs behind the same trigger as the lock.

**Concrete source and migration evidence.** (1) Every surface in the owner-map
census is tracked: `skills/catalog.json`, `registry.json`,
`docs/SKILL-ROUTER.md`, `skills/SKILL-TIERS.md`, the doc maps/graphs,
`images/*/manifest.json`, `images/gemini/skills/**`, and `skills-codex/**` all
appear as `M`/tracked content in the current worktree — git *is* a staging
area with a one-command rollback (`git restore --source=HEAD -- <owned
paths>`). (2) The in-family validate-before-write property SOL wants already
exists where it matters most: `generate-skill-mesh.py` runs `load_entries` →
`validate_graph` before its `outputs()` writes. The real, verified hazard is
*cross-family*: `regen-all.sh`'s `step()` records `fail=1` and continues, so a
later hash pass can seal a mixed tree — and that dies with fail-fast +
manifest-last, which we both adopted. (3) The cost side is not symmetric:
extracting render modes from four heterogeneous writers — two Python
generators, `regen-codex-hashes.sh`, and the 686-line `codex-sync.sh` with its
bespoke/excluded twin policy — is a refactor of the projection machinery
itself, sitting on the critical path *ahead of* semantic work. SOL demoted the
lock with exactly this critical-path argument; the same argument applies one
rung down.

**Failure mechanism if SOL's ruling is adopted.** The staging refactor becomes
the new blocking substrate tranche. The likeliest defect source in the
publication pipeline is then the refactor itself: a render-mode extraction bug
in `codex-sync.sh` (the subtlest writer, with deletion semantics and bespoke
carve-outs) corrupts projections in a *novel* way, against which the migration
has no golden history — trading a bounded, git-recoverable residual window for
unbounded refactor risk, while every semantic tranche waits.

**Falsifier.** Any of: (a) one owner-mapped output that is untracked or
gitignored — git-as-staging has a hole there and staging wins for that surface
immediately; (b) a seeded kill-at-any-step run that owner-scoped
`git restore` cannot return to a clean pre-run tree; (c) a demonstrated
consumer that reads the live tree inside the failure window of a *serial*
migration despite drift gates being red. Any one of these and I fold to
staging-first.

**Smallest synthesis rule preserving SOL's valid concern.** *No owned live
projection may remain in a failed intermediate state after a publication
attempt: on any step failure the publisher restores every owner-mapped path
from HEAD and reports the failure; complete staged rendering becomes mandatory
the moment any owned output is not git-tracked or concurrent publication is
authorized.*

**Final confidence: 78%.**

### A2. The disagreement SOL nominates for the operator is stale, and the epoch mechanism SOL holds is unspecified at its two load-bearing details

**Claim and exact disagreement.** SOL's reaction tells the operator the top
remaining dispute is "FABLE's fixed-point language allows re-baselining; SOL
rejects any interpretation that replaces or upgrades earlier judgments." That
dispute no longer exists: my Phase-3 reaction withdrew re-baselining in those
words ("Rebaselining is dead; epochs are the law") and adopted SOL's epoch
model wholesale. Presenting a settled point as the operator's decision
misdirects the one decision slot the duel produces. Meanwhile the epoch
mechanism SOL retains is under-specified exactly where it can fail: **the
carrier** and **the bootstrap**. "Bind every verdict to the validator
implementation or contract digest and schema IDs" names no field. `verdict.v2`
is a closed shape — its required properties contain no validator-contract or
epoch field, and its PASS branch is strict — so as written, epoch identity
would live in prose. And since adding an epoch field is itself a proof-contract
change, epoch 1 must be mintable under the *current* schema or the mechanism
cannot start.

**Concrete source and migration evidence.** `schemas/verdict.v2.schema.json`
required set: `schema_version, acceptance_digest, subject_manifest_digest,
author_context_id, validator_context_id, freshness_attestation, verdict,
criteria, findings, evidence_refs, checked, not_checked, validated_at,
artifact_digest` — no epoch, no validator version. The one extensible,
required, machine-readable carrier available today is `evidence_refs`
(array of strings, `minItems 1`). My reaction file already flagged the carrier
gap ("epochs don't start as prose"); SOL's reaction, written in parallel,
could not have seen that — but its own ruling still ships without a carrier.

**Failure mechanism if SOL's ruling is adopted as written.** The kernel
tranche mints epoch-1 verdicts with epoch identity recorded nowhere
machine-readable; the scheduled report/verdict revision later opens epoch 2;
T7's completion matrix cannot mechanically partition verdicts by epoch; the
"declared compatibility relation" degrades to narrative; cross-epoch results
get silently pooled — which is precisely the evidence-integrity failure SOL's
epoch rule exists to prevent, reached via under-specification instead of
re-baselining.

**Falsifier.** An existing machine carrier I missed — e.g., a validator-version
field inside the freshness-attestation schema — or a carrier specification
anywhere in SOL's artifacts. I checked the schema and their three files; none
exists.

**Smallest synthesis rule preserving SOL's valid concern.** *Epoch identity is
machine-carried from the first verdict: every verdict's `evidence_refs`
includes exactly one `proof-contract:sha256:<digest>` entry — the digest taken
over the validator scripts plus the three schema files — and the scheduled
report/verdict revision promotes it to a named field; a verdict lacking the
ref is epoch 0 and is never pooled with epoch ≥ 1.*

**Final confidence: 95%** (that the dispute is settled and the carrier +
bootstrap are the genuinely open items).

---

## Part B — the two parts of my revised architecture that remain most underrated

### B1. Plan-matrix authority expiry with a conformance check (my C24/C25)

**Claim and what is underrated.** SOL's scores, reaction, and consensus lists
never engage the interim-authority problem: the ports-and-adapters contract
currently says "the canonical placement matrix lives in the active overhaul
plan" — an open-ended grant of placement authority to a dated, tier-4 document
— and the plan's completion evidence begins with a 49×4 *hand* comparison of
that matrix against generated views. My proposal makes T7 (a) run a one-shot
conformance script diffing the plan's matrix against the generated catalog,
and (b) expire the interim note, replacing it with a pointer to the generated
views, stamping the plan `superseded`.

**Concrete source and migration evidence.** The interim note is live contract
text today; repository source precedence ranks "dated plans, audits,
changelogs" *last* — the note inverts the repo's own precedence order for as
long as it survives. No check binds the plan table to the catalog (verified:
none exists in `scripts/` or the gate registry). SOL's own false-completion
mode #1 — "the catalog says 49/49 while authority remains prose" — is the
sibling disease; this is the same disease between *documents*: catalog true,
plan stale, contract still pointing at the plan.

**Failure mechanism if dropped.** The migration completes; the note survives
(open-ended arrangements outlive their purpose); the first post-T7 skill
change updates frontmatter but not the plan's table; a future reader — or the
next audit — resolves placement against the stale matrix the contract still
blesses. Drift is silent because nothing diffs them; the overhaul's central
artifact becomes its first post-completion incoherence.

**Falsifier.** The interim note already carrying an expiry clause (it does
not), or an existing plan↔catalog conformance check (none), or evidence the
plan doc is deleted at completion (the plan specifies no such step).

**Smallest synthesis rule.** *T7 acceptance includes the plan↔catalog
conformance diff and the contract edit that retires the interim note; after
T7, placement questions resolve only against generated views, and the plan
carries an explicit `superseded-by` pointer.*

**Final confidence: 90%.**

### B2. The #988 reconciliation ledger (my C29) — sharpened by fresh evidence

**Claim and what is underrated.** Neither SOL's proposal nor either of their
later artifacts mentions the worktree's relationship to commit `16d764b5a`
(#988) at all, and the plan gives it one sentence. Fresh verification this
phase shows the hazard is worse than my sealed proposal stated: **the worktree
holds exactly half of #988.** The commit changed *both* `skills/craft-goal/**`
(new skill + twins) *and* `skills/rpi/SKILL.md` (+20/−14: the two-surface
report split — machine `rpi-report.v1` plus a concise human summary — exactly
the change the plan's rpi row re-schedules), plus `skills/rpi/references/
rpi.feature`, rpi's validator, and the rpi codex/gemini twins. The worktree
contains the craft-goal half (untracked) and the shared projection manifests
(modified), while `git status skills/rpi/` is **empty** — the rpi half is
entirely absent. A reviewed, merged-elsewhere design has been partially
adopted into the migration's starting tree, and the missing half sits directly
under the kernel tranche's future footprint. Bonus collision: #988 rewrote
rpi's `output_contract` into free prose ("machine artifact plus a concise
human-readable interactive summary"), which the merged compiler's *typed*
output-contract direction must adjudicate, not inherit or clobber blindly.

**Concrete source and migration evidence.** `git merge-base --is-ancestor
16d764b5a HEAD` → not an ancestor; `git show --stat 16d764b5a` → the dual
craft-goal + rpi file list above; `git status --porcelain skills/rpi/` →
empty. All three run this phase.

**Failure mechanism if dropped.** SOL's own kernel tranche (T2A) implements
the report split fresh and diverges from the already-reviewed #988 wording; or
a later reconciliation lands #988's rpi half on top of the tranche's version —
double-apply or clobber, in the highest-stakes tranche of the campaign.
Separately, T0's own acceptance ("pre-existing dirty paths are recorded and
preserved") is unfalsifiable without provenance for the untracked craft-goal
content: nobody can adjudicate what "preserved" means for content whose source
commit is unrecorded.

**Falsifier.** #988 merging to main before T0 (the ledger then degrades to a
rebase check — cheap either way), or a byte-compare showing worktree
craft-goal ≠ 16d764b5a in ways that a plain snapshot would catch anyway —
which is the ledger doing its job, not its absence being safe.

**Smallest synthesis rule.** *T0 emits a three-state ledger over `16d764b5a`'s
complete file list — present-in-worktree (byte-compared), absent, or
re-scheduled-by-plan-row — and every kernel-tranche edit to an rpi surface
cites its ledger line.*

**Final confidence: 92%.**

---

## The one disagreement the operator must decide

**Staged-complete rendering before mass edits (SOL) versus owner-scoped
restore-on-failure until concurrency is authorized (FABLE).** This is now the
*only* live architectural disagreement: the two previously nominated disputes
are settled — the CLI default dissolved when SOL demoted the prerequisite to
my crossings model (their stated confidence in the original placement fell to
72%), and the re-baselining dispute dissolved when I adopted SOL's epochs. The
staging question is small in surface but real in consequence: it decides
whether days of projection-machinery refactor sit in front of every semantic
tranche.

## The decision procedure and evidence that should decide it

1. **Owner-map census (hours):** enumerate every generated output; verify each
   is git-tracked and none is gitignored. Any untracked output → staging wins
   for that surface immediately, no further argument.
2. **Build the restore arm (hours):** add owner-scoped
   `git restore --source=HEAD` on step failure to the fail-fast `regen-all`,
   plus the manifest-last marker we already agree on.
3. **Run SOL's own test against both mechanisms (a day):** the seeded
   kill-at-each-step matrix on a scratch clone — kill after each write step of
   each generator family; after every kill, ask: does the tree return to a
   clean pre-run state, and do the drift gates stay red until it does?
4. **Decide by results and cost:** if restore recovers every seeded failure,
   defer staging to the concurrency/untracked-output ratchet and spend the
   saved days on the kernel; any unrecoverable case → build staging first.

The experiment costs about a day; the staging build costs days and carries its
own refactor risk. Either outcome is fine; deciding without running it is not.

## Points I withdraw completely

1. **The five-skill authority-transfer atom** — withdrawn in Phase 3;
   reaffirmed. Kernel-first is correct; the caller is the standing owner of
   continuation.
2. **"Re-baselining" of prior verdicts** — withdrawn and replaced by SOL's
   epoch model (with the carrier/bootstrap specification in A2 above).
3. **Populating `shared` as the model-dispatch destination** — withdrawn;
   the split-home design (neutral invariant → `docs/contracts/`, mechanics →
   adapters, then retire `shared`) is better than my original.
4. **The claim that a single regeneration entrypoint was missing** —
   `scripts/regen-all.sh` exists; my contribution reduces to hardening it
   (fail-fast, manifest-last, restore-on-failure), not inventing it.
5. **My appendix's T4 tranche placement for `security`** — superseded in
   Phase 2 by SOL's evidence-placement argument, which I adopted and do not
   reopen.

*End of rebuttal. FABLE.*
