# WIZARD_IDEAS_FABLE — Sealed architecture proposal, skill-system overhaul

- Wizard: FABLE (`claude-fable-5`)
- Date: 2026-07-24
- Phase: 1 (sealed; the other wizard's artifacts were not read)
- Baseline: worktree at HEAD `74795448a`, 49 `skills/*/SKILL.md` on disk (matches
  the plan ledger), `craft-goal` and its projections present as untracked/modified
  worktree content whose origin commit `16d764b5a` (#988) is **not** in HEAD lineage
  (verified with `git merge-base --is-ancestor`).
- Method: read the full frozen packet, then verified the plan's load-bearing claims
  against live sources — all 49 SKILL.md files, the frontmatter v2 schema, the mesh
  generator, the boundary gates, the evidence schemas, and the flagged Go CLI code
  paths. Executed checks where cheap (`check-orchestration-skill-boundaries.sh`,
  `validate.py --help`, schema loads). Source behavior outranked narrative
  throughout.

---

## 0. Verified load-bearing facts

These are the observations everything below rests on. Each was checked against the
live tree, not quoted from the plan.

**V1 — The plan's core premise is real.** `skills/rpi/SKILL.md:87-106` carries a
"Continuation envelope" (lane budgets, repair-revision counts per wave, two-stop
checkpoints — campaign state inside the experiment membrane) and its description
still advertises the retired trigger `"research-plan-implement"` (line 3).
`skills/implement/SKILL.md:68-71` grants "one repair revision of the intent" —
revision authority inside a leaf. `plan` and `implement` lack `output_contract`.
The campaign/experiment separation the plan wants is genuinely absent from today's
core skills.

**V2 — The proof substrate the plan names is partly dead.**
`scripts/check-orchestration-skill-boundaries.sh` — listed in the plan's "First
useful checks" — exits 2 at HEAD because it greps
`cli/internal/adapters/agentworker_ntm/ntm.go`, a file deleted in the Go CLI
cleanup (the adapters dir now holds only `capabilities/ config/ doctor/ eval/
gate/`). No bats test and no CI step invokes it (`grep -rln` over `tests/` is
empty; `.github/workflows/validate.yml:298` mentions it only inside a comment
about why ripgrep is installed). Even if resurrected, its phrase anchors have
drifted: it requires `single worker pays no coordination tax` while
`skills/agent-native/SKILL.md:80` now reads "single local agent pays no factory
coordination cost". A named gate is simultaneously broken, orphaned, and stale —
three failure classes in one script.

**V3 — The declared-contract tier advertises retired architecture.**
`schemas/plan-packet.v1.schema.json`, `candidate-packet.v1.schema.json`, and
`revision-packet.v1.schema.json` exist with **zero** consumers in `cli/` or
`scripts/` (the only references are tombstone guards in
`skills/plan/scripts/validate.sh:5-8`, `skills/implement/scripts/validate.sh:9-10`,
and `check-cathedral-cut-conformance.py`). Source precedence ranks
"declared contracts and schemas" at #2 — above narrative docs — so the retired
packet architecture currently outranks the operating-loop doc that retired it.

**V4 — The classification-axis count is already at six before the plan adds two.**
Live axes on every skill: `hexagonal_role` (5 values,
`skill-frontmatter.v2.schema.json:21-24`), `metadata.tier` (9 live values incl.
`orchestration`, held by exactly one skill, `codex-exec`, while its sibling
adapter `agy-native` sits in `cross-vendor`), `metadata.disposition` (5 values —
already placement-flavored: `keep_strategy`, `keep_optional_adapter`),
`context_rel` (7 DDD kinds), bounded contexts (6 in
`docs/contracts/bounded-contexts.yaml` — **no Campaign context, no Evolution
context**), and `consumes/produces`. D1 adds `system_layer` (9) +
`lifecycle_seams` (14) without retiring anything.

**V5 — The plan's seam enum and the contract's seam table have already diverged,
and one divergence grants forbidden authority by name.** The contract's seam
table (`skill-ports-and-adapters.md:96-111`) has "Option shaping"; the plan's D1
enum has `experiment_select` instead, and assigns it to `idea-genie`
(plan matrix row, line 202). But catalog acceptance #3 and the contract invariant
(`skill-ports-and-adapters.md:166`) reserve experiment selection exclusively for
the caller/Goal. A judgment strategy carrying a seam literally named
`experiment_select` is a vocabulary-level contradiction of the plan's own
acceptance criterion. The plan also adds `standalone` and `core_phase`, which the
contract's table does not define.

**V6 — The report schema cannot hold what the architecture promises.**
`skill-ports-and-adapters.md:63-64`: "RPI may carry their identifiers as opaque
correlation facts." `schemas/rpi-report.v1.schema.json` is
`additionalProperties: false` with no correlation field — the promised opaque
correlation is unrepresentable in the durable report today. The rpi target row
("retain opaque outer correlation") cannot be satisfied without a schema revision
the plan never schedules.

**V7 — Cross-skill contracts live inside one optional adapter.**
`agent-native/references/model-dispatch.md` is normatively referenced by six
skills — `validate` (core), `council`, `idea-genie`, `codex-exec`, `ntm`,
`agent-native` — while `skills/shared/` contains **only** its own SKILL.md
(verified with `find`), an empty library whose prose claims to hold "runtime
capabilities and evidence formats." Core-phase text depending on an optional
adapter's reference file is a prose-level violation of "optional advice … never
become hard core dependencies."

**V8 — The campaign vocabulary has no home.**
`docs/contracts/ubiquitous-language.md` contains zero occurrences of
`campaign`, `goal`, `fitness`, `ratchet`, `envelope`, or `wave` (grep verified).
The `domain` skill — the designated lookup path — can define none of the terms
the new architecture pivots on. Meanwhile the `goals` skill (project fitness) and
the Goal/Mayor concept collide on the same word with opposite meanings.

**V9 — Effect honesty is a 30-skill sweep, not a touch-up.** `doc`, `test`,
`refactor`, `scaffold`, `security`, `converter`, `bootstrap`, `cc-hooks`,
`toil-mining`, `postmortem` and ~20 more declare `effects: []` while their bodies
document writes (`.agents/doc/`, `.agents/tests/`, code changes, scaffolds, scan
artifacts, `settings.json` edits, host mutations). Only 15 skills declare any
effect. Acceptance #5 is currently false for the majority of the catalog.

**V10 — Artifact-flow fields already carry skill slugs.** `rpi` `consumes:
[plan, implement, validate]` and `bootstrap` `consumes: [goals, product, doc,
shared]` — skills, not artifact kinds — duplicating `metadata.dependencies`
(where rpi correctly lists the same three). The schema (`consumes` item
description: "artifact-kind or skill slug") blesses the ambiguity.

**V11 — Runtime leakage in canonical source.** `status` frontmatter carries
`allowed-tools: Read, Grep, Glob, Bash` and `model: haiku`; `research` carries
`allowed-tools` — Claude-runtime configuration inside runtime-neutral canonical
source, exactly the class the plan's `status` row names but treats as one skill's
defect rather than a schema-policy decision.

**V12 — The Go CLI audit's top finding is real.**
`cli/internal/eval/task_service.go:73-84`: `task.ID` from caller YAML is checked
only for non-emptiness, then `filepath.Join(root, "tasks", task.ID, "task.yaml")`
is written — `../../escape` escapes the eval store. Read paths repeat it. The
audit's severity ordering held up under direct source reading.

**V13 — Reference implementations for core semantics already exist and are pure.**
`skills/rpi/scripts/run_once.py`, `skills/swarm/scripts/dispatch_once.py`, and
`skills/validate/scripts/validate.py` (subcommands verified: `manifest`,
`verify-manifest`, `snapshot-intent`, `digest`, `store-verdict`) execute the
dispatch/stop, at-most-once, and identity/verdict semantics with no Git, tracker,
or network. The D4 "Core phase → behavioral state-transition tests" row has no
named harness, but its natural harness is already in the tree.

**V14 — Kernel ceiling status.** Only `cc-hooks` (289) and `cass` (263) exceed
250 lines; `test` (242) and `craft-goal` (226) are near it. Acceptance #12 is two
modularizations, not a campaign.

---

## 1. Candidate ledger (34 generated, winnowed to 5)

Dispositions: **F#** = folded into that finalist; **T** = tranche-critique
section; **A** = appendix row-level; **R** = rejected (reason given).

| # | Candidate | Disposition |
|---|---|---|
| C1 | Executable-check liveness inventory at T0: every check the plan names must be proven wired and runnable | **F1** |
| C2 | Pin `validate.py` + `verdict.v2` + `subject-manifest.v1` as a digest-frozen fixed point for the whole campaign | **F1** |
| C3 | Map each of the 12 catalog acceptance criteria to a named executable check (acceptance→check matrix) | **F1** |
| C4 | T0 machine snapshot gains exact path + per-skill content digests so tranches diff against a frozen baseline | **F1** |
| C5 | Unify seam vocabulary: contract owns the enum; rename `experiment_select` → `option_shaping`; define or drop `standalone`/`core_phase` in the contract table | **F2** |
| C6 | Ubiquitous-language pin for campaign vocabulary (campaign, Goal, ratchet, envelope, wave, fitness) before any skill renames | **F2** |
| C7 | Add Campaign and Evolution bounded contexts to `bounded-contexts.yaml` in T1 | **F2** |
| C8 | `goals`→fitness rename gated on the language pin; CLI keeps `ao goals` with a documented alias note | **F2** |
| C9 | Taxonomy budget: `tier` becomes generated/derived from `system_layer` (or is deleted with SKILL-TIERS.md re-keyed); one-in-one-out for classification axes | **F3** |
| C10 | Constrain `disposition` to curation-only meaning once seams exist; forbid placement semantics in it | **F3** |
| C11 | Schema-enforce `consumes`/`produces` as artifact kinds only (fixtures for skill-slug abuse) | **F3** |
| C12 | Effects vocabulary enum (write_repo_paths, write_artifact_dir, mutate_host_config, spawn_processes, network, credential_use…) so the 30-skill honesty sweep is mechanical | **F3** |
| C13 | Runtime-hint policy: schema-bless an optional `runtime_hints` block projected into images, or ban and move to overrides — one decision, not per-skill whack-a-mole | **F3** |
| C14 | `output_contract` becomes a typed object `{kind: schema\|artifact\|prose, ref}`; structured outputs must be schema-backed | **F3** |
| C15 | Re-cut T2 to the authority-transfer atom {craft-goal, rpi, plan, implement, validate}; move `product`+`goals` out | **F4** |
| C16 | Split T6 (18 skills) into T6a transports (8) and T6b host/support (10) | **F4** |
| C17 | Extend the pure-python reference layer (run_once.py, dispatch_once.py, validate.py) into the executable negative-authority harness for T2's eight scenarios | **F4** |
| C18 | Within-tranche parallel lanes by skill directory; regen serialized at freeze; at most one tranche's source set open at a time | **F4/T** |
| C19 | One regen entrypoint (single command running mesh → codex-sync → hashes → image manifests in fixed order) | **F4/T** |
| C20 | Delete/retire the dead packet schemas (plan-packet, candidate-packet, revision-packet; audit briefing/learning/memory-packet for the same class) with tombstone guards | **F5** |
| C21 | Add optional `correlation` field to `rpi-report.v1` (schema rev) to make the contract's promised opaque correlation representable | **F5** |
| C22 | Promote `model-dispatch.md` from `agent-native/references/` into `shared/` — populates `shared` with its six live consumers and removes the core→optional prose dependency | **F5** |
| C23 | Give `cc-hooks` and `dcg` output contracts (currently absent entirely) | **F5/A** |
| C24 | Plan-matrix conformance script at T7: machine-diff this plan's 49-row table against generated catalog placement; after T7 the plan's matrix authority retires into generated views | **F1/T** |
| C25 | Completion matrix rows must carry verdict digests (machine-checkable), not hand-written checkmarks | **F1** |
| C26 | Make acceptance #7 (trigger separation) executable via the existing `trigger_probes` + `scan_descriptions.py --probe` machinery | **F3** |
| C27 | `automation-shape-routing` re-layers from runtime to support (it routes, never transports); fix its "invoke the owner" vs "route only" internal tension | **A** |
| C28 | Router projection re-keyed by layer/seam instead of disposition (current SKILL-ROUTER.md groups by `keep_*`, not a routing key) | **A/T** |
| C29 | `#988` reconciliation must enumerate which target-row changes it already implements (rpi report humanization overlaps the rpi row) so T2 neither double-applies nor clobbers | **T** |
| C30 | Negative fixture: campaign skill renewing its own envelope (extends T1's RED list) | **F4** |
| C31 | Expedite eval path containment + dry-run fail-safe as release work outside tranches | **§5** |
| C32 | Unified subprocess runner across eval/gate/goals stays in the CLI program, not the skill migration | **§5** |
| C33 | Split `security` one-shot scan vs bounded investigation (per plan) | **A** (plan already right) |
| C34 | Retire `SKILL-TIERS.md` as a distinct projection page or regenerate it as the layer inventory | **F3** |

Rejected candidates worth recording: **R1** — merging `premortem` into `craft-goal`
lint (rejected: one challenges an experiment intent, the other compiles a campaign
contract; different seams, different consumers). **R2** — folding `scope` into
`plan` (rejected: scope review is deliberately a fresh-context advisory with its
own isolated context contract; folding re-creates the author-judges-self
problem). **R3** — making the CLI audit a prerequisite tranche T-1 (rejected in
§5: it would serialize a security fix behind a metadata migration and vice
versa). **R4** — renaming `goals` CLI command to `ao fitness` (rejected: outside
write scope, breaks consumers; the skill-side vocabulary pin suffices).

---

## 2. The five finalists

### F1 — Prove the proof substrate first: check-liveness inventory + frozen validator fixed point

1. **Claim.** T0 must produce (a) an executable-check liveness inventory — every
   check named anywhere in the plan is executed once, classified
   `green | red-for-cause | dead`, and every `dead` check is repaired or formally
   retired before any tranche opens — and (b) a digest-pinned validator fixed
   point: `skills/validate/scripts/validate.py`, `schemas/verdict.v2.schema.json`,
   `schemas/subject-manifest.v1.schema.json`, and `schemas/rpi-report.v1.schema.json`
   are frozen by content digest for the campaign; any change to them is its own
   dedicated RPI with explicit re-baselining of prior tranche verdicts. Each of
   the 12 catalog acceptance criteria gets a named executable check (an
   acceptance→check matrix), and the final completion matrix carries verdict
   digests per tranche, not prose checkmarks.
2. **Defect resolved.** The plan validates a semantic migration with a substrate
   it never proves is alive, and mutates the judge mid-trial. It gestures at the
   first problem ("the implementation must first repair any gate whose own source
   references deleted files") but makes it a side condition, not a deliverable
   with proof.
3. **Evidence.** V2 (a plan-named check is broken, orphaned, and phrase-stale at
   HEAD — reproduced exit 2); the plan's First-useful-checks list maps to none of
   the 12 acceptance items; T2's write scope includes `validate` and its "owned
   references, schemas, scripts" — i.e., the fixed point is inside the blast
   radius of the tranche it judges; V6 shows a schema revision is already needed,
   so schema change *will* happen mid-campaign unless explicitly sequenced.
4. **Target architecture / ownership.** The check inventory is a T0 artifact
   (machine JSON: `{check, invocation, wiring, status, cause}`) owned by the
   migration campaign; gate wiring itself is owned by CI config and
   `cli/internal/gates`. The fixed point is owned by a named freeze record in the
   T0 snapshot (digests), consumed by every tranche's Validate step.
5. **Migration steps.** (i) Enumerate every command in the plan + acceptance
   text; (ii) run each; (iii) repair or retire dead ones
   (`check-orchestration-skill-boundaries.sh` gets: path fix to live packages,
   phrase anchors replaced with structural assertions, and a bats wrapper — or a
   formal retirement note); (iv) emit the acceptance→check matrix into the plan;
   (v) record fixed-point digests; (vi) add the T7 conformance script (C24) that
   diffs plan matrix ↔ generated catalog.
6. **Proof / acceptance.** The inventory JSON exists with zero `dead` rows; each
   of the 12 criteria names ≥1 check that exits nonzero on a seeded violation
   (one seeded-red proof per criterion class); fixed-point digests recorded and
   re-verified by T7; any schema rev shows its own RPI verdict + re-baseline
   note.
7. **Failure modes / rollback.** Risk: inventory scope-creeps into fixing every
   historical script. Containment: only plan-named checks are in scope; others
   are recorded, not repaired. Risk: freezing schemas blocks a legitimately
   needed change (V6). Containment: the escape hatch is defined (dedicated RPI +
   re-baseline), so freezing is a speed bump with a documented gate, not a wall.
8. **Skills / CLI findings affected.** All 49 (every tranche's verdict inherits
   the fixed point); directly touches `validate`, `rpi`; CLI findings: none
   directly, but the gate-repair work touches `scripts/` and possibly
   `cli/internal/gates` registry wiring.
9. **Confidence: 92%.** Falsified if: the boundary gate turns out to be wired
   somewhere I did not search (I checked `tests/`, workflows, Makefile), or if
   the team can show tranche verdicts remain comparable across an unpinned
   validator change without re-baselining.

### F2 — One seam vocabulary, owned by the contract; no authority-implying seam names

1. **Claim.** Before T1 lands fields, the seam enum must be reconciled to a
   single list owned by `skill-ports-and-adapters.md`, with the plan and schema
   conforming. Specifically: rename `experiment_select` → `option_shaping`
   (matching the contract's existing table and stripping the authority
   implication), define `standalone` and `core_phase` in the contract's seam
   table or remove them from the enum, and pin the campaign vocabulary
   (campaign, Goal/Mayor, ratchet, envelope, wave, fitness-vs-goal) into
   `docs/contracts/ubiquitous-language.md` + a new Campaign bounded context (and
   an Evolution context) in `bounded-contexts.yaml`. The `goals`→"fitness"
   terminology change happens only after the language pin.
2. **Defect resolved.** The plan's D1 enum and the contract's seam table have
   already diverged before implementation starts, and one divergence
   (`idea-genie` ↔ `experiment_select`) names a seam after the exact authority
   that acceptance #3 and the contract invariant reserve for the Goal. Left as
   is, the generated placement views would teach every future reader that a
   strategy skill participates in experiment selection.
3. **Evidence.** V5 (enum vs table divergence, line-cited); V8 (zero campaign
   vocabulary in the ubiquitous-language contract; `domain` skill cannot define
   the architecture's own nouns); V4 (bounded contexts lack Campaign/Evolution);
   `craft-goal` untracked at HEAD means the campaign layer currently exists
   *only* as worktree content with no contract vocabulary behind it.
4. **Target architecture / ownership.** Enum source of truth:
   `skill-ports-and-adapters.md` (already declared canonical by the plan's
   `architecture_ref`). Schema (`skill-frontmatter` rev) validates against that
   list. Vocabulary: `ubiquitous-language.md` owns term definitions;
   `bounded-contexts.yaml` owns BC membership; the `domain` skill serves both
   unchanged.
5. **Migration steps.** (i) T0: reconcile enum in the contract doc (add
   `standalone` definition: "invocable outside any campaign/experiment, result
   consumed directly by the caller"; keep `core_phase` but define it as
   membership, not authority); (ii) T0: language-contract additions; (iii) T1:
   schema enum literal matches contract; fixture: unknown seam fails; fixture: a
   non-campaign skill declaring `goal_design`-class authority fails; (iv) T2:
   `goals` wording change rides the pinned vocabulary.
6. **Proof / acceptance.** `grep`-level: the schema enum and the contract table
   enumerate the identical set (a trivial conformance check added to the mesh
   generator's `--check`); `domain` lookup for "campaign", "ratchet", "Goal"
   returns definitions with source paths; no skill row in the generated
   placement matrix carries a seam absent from the contract table.
7. **Failure modes / rollback.** Risk: renaming seams mid-plan invalidates the
   49-row matrix. Containment: the rename is one mechanical substitution in the
   plan before T1; rows are otherwise untouched. Risk: vocabulary bikeshed.
   Containment: only terms the architecture doc already uses get entries; no new
   coinage.
8. **Skills / CLI findings affected.** `idea-genie` (seam rename), `goals`
   (gated rename), `craft-goal`, `domain` (serves new entries), all 49 indirectly
   via schema enum. CLI: none.
9. **Confidence: 88%.** Falsified if the plan's authors can show
   `experiment_select` was deliberately scoped as "supplies candidates *to*
   selection" and that no generated view or reader-facing surface would render
   the bare seam name — I judge that unlikely because the generated views print
   field values verbatim.

### F3 — A taxonomy budget: placement lands only with retirements, and the honesty sweeps become mechanical

1. **Claim.** T1 may add `system_layer` + `lifecycle_seams` only together with:
   (a) `tier` demoted to a derived value (generated from layer, or deleted; the
   `SKILL-TIERS.md` projection re-keyed to layers or retired); (b) `disposition`
   constrained to curation-only semantics (keep/retire lifecycle), with a
   fixture rejecting placement synonyms; (c) `hexagonal_role` and `context_rel`
   documented as DDD-only, unchanged; (d) `consumes`/`produces` schema-tightened
   to artifact kinds (V10 fixtures); (e) a closed effects vocabulary enum so
   acceptance #5 becomes a mechanical sweep over ~30 currently-empty
   declarations; (f) `output_contract` typed as `{kind, ref}`; (g) one explicit
   policy for runtime hints (`allowed-tools`, `model:`) — schema-blessed
   projection block or banned; (h) trigger separation (acceptance #7) made
   executable via the existing `trigger_probes` / `scan_descriptions.py --probe`
   machinery.
2. **Defect resolved.** The overhaul prunes skill sprawl while seeding metadata
   sprawl: six live classification axes grow to eight with no retirement rule,
   and three of the twelve acceptance criteria (#5 effect honesty, #6 output
   honesty, #7 trigger separation) are unenforceable as written because the
   fields they audit are free-text, absent, or unmeasured.
3. **Evidence.** V4 (axis census with the `orchestration`-tier-of-one
   incoherence); V9 (effects empty on ~30 mutating skills); V10 (artifact-flow
   abuse in `rpi`, `bootstrap`); V11 (runtime hints in `status`, `research`);
   C23 (`cc-hooks`/`dcg` have no `output_contract` at all); the schema's
   `additionalProperties: true` on metadata means unvalidated fields ride along
   silently today; `trigger_probes` already exists in the v2 schema with a probe
   script named in its description — the machinery for #7 is built and unused.
4. **Target architecture / ownership.** Frontmatter schema owns all field
   grammar; the mesh generator owns every derived view; the contract doc owns
   the layer/seam meaning. Rule of the budget: **every new axis names the axis
   it replaces or the reader question only it can answer.** Axis census after
   T1: `system_layer` (where it belongs), `lifecycle_seams` (where results are
   consumed), `dependencies` (what must execute), `context_rel` (DDD),
   `consumes/produces` (artifact flow), `effects` (mutation honesty),
   `disposition` (curation) — with `tier` gone/derived, net axis count is flat.
5. **Migration steps.** All inside T1 (the factory tranche already owns schema,
   generator, fixtures, and CLI catalog consumers): schema rev; generator
   derives tiers view from layers or drops it; RED fixtures for each rule
   (unknown effect token, skill slug in `consumes`, prose `output_contract` on a
   structured producer, `model:` outside the hints block); per-skill field
   population then happens in each skill's owning tranche (T2–T6) so T1 touches
   grammar, not 49 bodies.
6. **Proof / acceptance.** `generate-skill-mesh.py --check` fails on each seeded
   fixture class; catalog report shows effects coverage 49/49 with enum-valid
   tokens by end of T6; probe run shows every skill ranks #1 for its own
   trigger phrases; axis census in the T7 report shows no net growth.
7. **Failure modes / rollback.** Risk: T1 becomes a mega-tranche. Containment:
   T1 changes grammar + generator + fixtures only; population is distributed to
   owning tranches (this is also why the effects enum must land in T1 — without
   it every later tranche invents tokens). Risk: deleting `tier` breaks a CLI
   consumer. Containment: T1's scope already includes CLI catalog/graph
   consumers; grep shows `tier` in `skillsapp`/catalog paths — migrate in
   lockstep, and keep `tier` emitted-as-derived for one release if any external
   consumer exists.
8. **Skills / CLI findings affected.** All 49 (grammar); acutely: `status`,
   `research` (hints), `cc-hooks`, `dcg` (contracts), `rpi`, `bootstrap`
   (artifact flow), `codex-exec`, `agy-native` (tier incoherence). CLI findings:
   the audit's "capabilities contracts incomplete" improvement is the same
   disease on the CLI side; the effects enum should share vocabulary with
   `CommandContract` effect metadata when that lands (see §5 crossing 3).
9. **Confidence: 85%.** Falsified if a live external consumer of `tier` or
   `SKILL-TIERS.md` exists that cannot follow a derived view (I found none:
   the page is generated, and `catalog.json` consumers are in-repo), or if the
   team demonstrates the axes serve disjoint reader questions I collapsed
   wrongly.

### F4 — Re-cut the tranches around the authority-transfer atom; give the core scenarios an executable harness

1. **Claim.** (a) T2 shrinks to the authority-transfer atom
   `{craft-goal, rpi, plan, implement, validate}` — the five skills between
   which continuation/repair authority actually moves — executed as one RPI so
   authority is never owned twice or zero times. `product` and `goals` move to a
   small product-layer tranche (T2p) that can run any time after T1, in parallel
   with T3. (b) T6 splits into T6a transports
   (`agent-mail, agent-native, agy-native, codex-exec, ntm, swarm, using-gc`)
   and T6b host/support
   (`account-rotation, automation-shape-routing, bootstrap, cc-hooks, dcg,
   handoff, ms, rch, sbh, shared, status`) — two genuinely different contracts
   (packet/deadline/cleanup vs effects-honesty/capability-discovery);
   `automation-shape-routing` rides T6b because it re-layers to support (C27):
   it routes and never transports. (c) T2's eight behavioral scenarios
   are implemented as state-transition tests over the **existing pure reference
   layer** (`run_once.py`, `dispatch_once.py`, `validate.py`), extended with
   negative-authority cases: continuation-after-red refused, second dispatch of
   a phase refused, verdict write outside `store-verdict` refused, envelope
   renewal refused (C30). (d) Within any tranche, lanes parallelize per skill
   directory; all regen surfaces serialize at source freeze through **one**
   regen entrypoint command (C19); at most one tranche's source set is open at a
   time.
2. **Defect resolved.** D2 says one tranche = one architectural decision, but T2
   holds two decisions (define campaign layer; strip core authority) plus two
   bystanders (product, goals), and T6 holds eighteen skills across two
   contracts. Meanwhile the core-phase proof row in D4 names no harness, which
   is exactly how "behavioral scenario" degrades into prose review.
3. **Evidence.** V1 (the authority being moved is live in rpi/implement text);
   `product` and `goals` hold no continuation authority (their SKILL.md bodies
   are advisory/measurement only — verified), so they are not part of the atom;
   T6's own acceptance list already reads as two lists (transport
   deadline/cleanup items vs support effects/capability items); V13 (the pure
   reference scripts exist and already encode dispatch/stop and at-most-once);
   the plan's own D3 lists four regen families invoked as four separate commands
   in `skill-builder`'s procedure (steps 4–5), a partial-regen drift surface.
4. **Target architecture / ownership.** Campaign→experiment authority boundary
   is owned jointly by `craft-goal` (above) and `rpi/plan/implement/validate`
   (below); moving it atomically in one verdict is the only state with no
   interim dual/orphan ownership. The reference layer becomes the executable
   semantics of the core loop, owned by the owning skills' `scripts/`; the
   negative-authority tests are the "Core phase" row of D4 made real.
5. **Migration steps.** (i) T0 reconciles #988 content with an explicit ledger
   of which rpi-row changes it already implements (C29); (ii) T1 unchanged
   (factory); (iii) T2 = 5-skill atom, RED-first via the extended reference
   harness, one integration manifest, one fresh verdict; (iv) T2p
   ({product, goals}) after the F2 vocabulary pin; (v) T3–T5 as planned; (vi)
   T6a then T6b (or in parallel lanes with serialized regen — their source dirs
   are disjoint); (vii) T7 unchanged plus C24 conformance.
6. **Proof / acceptance.** The eight T2 scenarios each map to a named executable
   test that fails on the pre-migration text/behavior and passes after (the
   continuation-envelope deletion in rpi is provable: the harness test
   "second non-PASS on one intent triggers no further dispatch *and* rpi source
   contains no wave/budget grammar" goes red today); tranche count of open
   source sets never exceeds 1 (checkable from the tranche freeze records);
   regen runs exactly once per tranche via the single entrypoint (its log is
   part of the tranche manifest).
7. **Failure modes / rollback.** Risk: the 5-skill atom is still too big to
   review. Containment: the checklist stays per-skill; the *verdict* is atomic,
   the diffs remain per-directory. Risk: splitting T6 doubles integration
   verdicts. Accepted: two verdicts for eighteen skills is inside D2's economy
   argument; the alternative risks one giant NOT_PROVEN. Rollback: tranche
   boundaries are plan text until frozen; re-cutting costs a plan edit, no code.
8. **Skills / CLI findings affected.** T2 atom: craft-goal, rpi, plan,
   implement, validate. T2p: product, goals. T6a/T6b: the eighteen listed. CLI
   findings: none directly; the regen entrypoint may live in `Makefile` or
   `scripts/`.
9. **Confidence: 83%.** Falsified if the behavioral scenarios genuinely require
   `product`/`goals` participation in the same verdict (I found no scenario in
   the plan's T2 list that touches either — "multi-experiment terminal outcome
   routes through a bounded Goal" needs craft-goal, not goals), or if the
   reference-harness extension proves materially harder than a bounded T2 task
   (the scripts are small and pure; I rate that risk low).

### F5 — Make the declared-contract tier tell the truth: dead schemas out, promised fields in, shared contracts homed in `shared/`

1. **Claim.** One coherence tranche-item set, mostly riding T0/T1/T2/T6b: (a)
   delete `plan-packet.v1`, `candidate-packet.v1`, `revision-packet.v1` schemas
   (and audit `briefing.v1`, `learning.v1`, `memory-packet.v1` for the same
   dead-consumer class), keeping the existing tombstone guards; (b) revise
   `rpi-report.v1` with an optional opaque `correlation` object (V6) in the same
   T2 RPI that strips rpi's continuation envelope — the two changes are one
   architectural statement ("campaign state lives above; only an opaque
   correlation crosses"); (c) move `model-dispatch.md` from
   `agent-native/references/` to `shared/references/`, repoint its six
   consumers, and let that satisfy T6b's "`shared` has live consumers or is
   retired" with *populate*; (d) give `cc-hooks` and `dcg` output contracts.
2. **Defect resolved.** Source-precedence tier 2 ("declared contracts and
   schemas") currently outranks the narrative that retired half its contents; a
   core skill normatively cites an optional adapter's reference file; the
   architecture promises a report field the schema forbids; and an empty
   library skill advertises content that does not exist. These are the exact
   places a fresh validator looks first, and today each one misleads it.
3. **Evidence.** V3 (dead schemas, zero consumers, guard-only references); V6
   (`additionalProperties: false` vs the contract's correlation promise); V7
   (six consumers of model-dispatch incl. core `validate`; `shared/` contains
   only SKILL.md); C23 (missing output contracts, confirming the prior audit's
   2 FAIL rows).
4. **Target architecture / ownership.** `schemas/` holds only live contracts;
   retired shapes leave a tombstone in the guard scripts (already present) and,
   if history matters, a copy under `docs/retired/`. Cross-skill runtime
   recipes are owned by `shared` (a library skill whose *stated* purpose this
   finally matches); adapters own only adapter-specific text. `rpi-report.v1`
   owned by the rpi skill + schemas dir, revised once, under F1's fixed-point
   escape hatch (this is the one scheduled schema rev, proving the hatch
   works).
5. **Migration steps.** (i) T0: dead-schema audit (grep consumers; delete;
   guards stay); (ii) T2: schema rev + envelope strip in one diff; regenerate
   projections; (iii) T6b (or earlier, it's five file moves): relocate
   model-dispatch, repoint six SKILL.md references, update codex twins via the
   regen entrypoint; (iv) T6b: cc-hooks/dcg contracts.
6. **Proof / acceptance.** `grep -rln 'plan-packet\|candidate-packet\|revision-packet'
   cli/ scripts/ skills/` returns only tombstone guards; `rpi-report.v1`
   round-trips a correlation-bearing report and rejects unknown extra fields
   otherwise; `find skills/shared -type f` returns >1 file and every
   `model-dispatch` reference resolves inside `shared/`; frontmatter validation
   shows 49/49 `output_contract` present.
7. **Failure modes / rollback.** Risk: an out-of-repo consumer reads the packet
   schemas. Containment: they are repo-relative paths never published in the
   plugin images (checked `images/*/manifest.json` scope is skills, not
   schemas); restore from git if one surfaces. Risk: moving model-dispatch
   breaks codex-twin parity. Containment: the move flows through the regen
   entrypoint and parity gates that already exist
   (`validate-codex-generated-artifacts.sh`).
8. **Skills / CLI findings affected.** rpi, validate, council, idea-genie,
   codex-exec, ntm, agent-native, shared, cc-hooks, dcg. CLI findings: none.
9. **Confidence: 90%.** Falsified if a consumer of the packet schemas exists
   outside my grep surface (e.g., an external tool reading raw GitHub paths) or
   if `shared`'s emptiness is deliberate staging for content already authored
   elsewhere (I found no evidence of either).

---

## 3. Tranche ordering and generated-write serialization critique

**Ordering — what the plan gets right.** T0 freeze → T1 factory → content
tranches → T7 convergence is the correct spine: teach the factory the grammar
before populating it, converge once at the end. `skill-builder` in T1 (it owns
the migration factory) and `learn` deferred to T5 are both correctly reasoned.

**Ordering — what to change.**

1. **T0 is underweight.** It freezes documents but not the proof substrate. Add
   F1's check-liveness inventory and validator fixed point, F2's vocabulary
   reconciliation (the seam enum must be settled *before* T1 encodes it), and
   the #988/craft-goal reconciliation ledger (C29). A T0 that only snapshots
   inventory would let T1 encode a seam enum that contradicts the contract it
   cites as canon.
2. **T2 carries two decisions plus two bystanders.** Re-cut per F4: the
   authority-transfer atom is {craft-goal, rpi, plan, implement, validate};
   `product`/`goals` are advisory-layer work with no authority movement and
   should not share the riskiest verdict of the campaign. Keeping the atom
   unified is deliberate: splitting campaign-definition from core-stripping
   would create an interim state where continuation authority is owned twice
   (both craft-goal and rpi's envelope) or not at all — both incoherent.
3. **T3 (12) and T4 (7) are fine as single decisions** (advisory contracts;
   specialist write-honesty) with per-skill lanes. **T6 (18) is two decisions**
   — split per F4. Transports and host-support fail differently and their
   acceptance lists barely overlap.
4. **Self-hosting hazard is unaddressed.** The campaign uses RPI to rewrite RPI.
   The fixed point (F1) is the mitigation: the *executable* semantics
   (validate.py, verdict/manifest schemas, run_once.py) stay frozen while the
   *prose contracts* migrate; the one scheduled schema rev (F5b) exercises the
   documented escape hatch. Without this, tranche N's verdict and tranche N+2's
   verdict are minted under different judges and the completion matrix is
   comparing incommensurables.
5. **T7 needs a machine check for its first duty.** "Inspect the generated
   placement matrix against this plan" is a 49×4 hand-diff as written; C24
   makes it a script, after which the plan's matrix (a tier-4 dated doc in the
   repo's own precedence order) formally hands authority to the generated
   views. A dated plan that silently stays "canonical placement matrix" after
   completion — as the contract's own interim note currently makes it — is a
   drift bomb.

**Serialization — verdict on D3.** The list is correct and complete against the
generator's actual outputs (verified: catalog.json, root `registry.json`,
SKILL-ROUTER.md, SKILL-TIERS.md, domain map, graph, context map, three image
manifests, gemini skill sync; plus codex-sync + hash regen as a second family).
Two amendments:

- **One regen entrypoint (C19).** Today "regenerate once" is four commands in a
  documented order (`generate-skill-mesh.py`, `codex-sync.sh`,
  `regen-codex-hashes.sh`, image validation). Every place the order is manual is
  a partial-regen drift surface — the exact class the 2026-07-15 fold and the
  hot-main rebase incidents already demonstrated. Make it one command whose log
  is part of each tranche manifest.
- **State the lane rule the plan implies.** rpi's own text has the right rule
  ("lanes whose write scopes share a regen surface serialize") — the plan should
  state it as: within a tranche, per-skill-directory lanes may parallelize;
  regen runs exactly once at source freeze; at most one tranche's source set is
  open at any time. That last clause is the one D3 omits and the one that
  prevents cross-tranche regen interleaving.

Additionally, SKILL-ROUTER.md currently groups skills by **disposition**
(`keep_strategy`, `keep_optional_adapter` …) — a curation key, not a routing
key. After placement lands, the router should be re-keyed by layer/seam (C28);
otherwise the flagship generated view answers a question nobody routing work is
asking.

---

## 4. Where the CLI audit belongs

**Decision: a separate program, with three named crossings — not a migration
tranche and not a prerequisite substrate tranche.**

Reasoning from the write-scope boundary the plan itself draws ("unrelated Go CLI
product behavior" is excluded): the audit's substantive findings (eval path
containment, dry-run honesty, JSON negotiation, subprocess lifecycle, temp-dir
ownership) live in `eval`, `provenance`, `gate`, `goals` — none are consumed by
the skill migration's proof chain. Folding them into tranches would serialize a
**security fix behind a metadata migration** (eval containment, V12, is
release-risk today regardless of skills) and would bloat tranche verdicts with
unrelated Go diffs. Conversely, making the whole audit a prerequisite tranche
would block 49 skill contracts on subprocess-runner unification they do not
need.

The three crossings, made explicit so neither program silently assumes the
other:

1. **T1 lockstep (already in the plan):** placement fields crossing the CLI
   catalog/graph boundary change `cli/internal/skills*` consumers inside T1,
   with the focused Go test/vet/lint set the plan names. Keep.
2. **Gate-substrate repair (F1):** the *specific* checks the overhaul's
   acceptance relies on get repaired/retired in T0 — this is the only piece of
   CLI-adjacent work the migration owns, because a dead gate poisons the
   migration's own evidence (V2).
3. **Shared effect vocabulary (F3 ↔ audit roadmap #5):** when leaf
   `CommandContract`s gain effect/dry-run metadata, they should use the same
   effect token vocabulary the skill frontmatter adopts. One honesty language
   across `metadata.effects` and CLI capability contracts; two dialects would
   recreate the drift this overhaul exists to end.

Sequencing recommendation for the separate program: audit roadmap items 1–2
(containment, dry-run fail-safe, subprocess unification) before the next
release, independent of tranche progress; items 3–6 opportunistically.

---

## 5. Three most dangerous ways the plan could appear complete while the system stays incoherent

1. **Relabel-without-behavior.** All 49 skills gain valid `system_layer` +
   `lifecycle_seams`, projections regenerate drift-free, frontmatter gates go
   green — while the *bodies* keep their old authority. The plan's own check
   list is dominated by structural validators (`heal --check`, frontmatter,
   mesh `--check`, parity), which cannot see that rpi's continuation envelope
   or implement's repair-revision clause survived in prose. The completion
   matrix reads 49/49 covered; the semantics never moved. Tell-tale: a tranche
   whose diff touches only frontmatter and whose "behavioral proof" cites a
   phrase-grep. Countermeasure: F4's executable negative-authority harness plus
   acceptance #9's structural/behavioral distinction enforced per target row
   (every row's proof artifact named, and at least one test that goes red
   against today's text).
2. **Judge drift across tranches (self-hosting erosion).** T2 edits `validate`
   — whose owned scripts and schemas are inside its write scope — and later
   tranches are then judged under quietly different verdict semantics. Every
   individual verdict looks valid; the *collection* T7 inspects is
   incommensurable, and "all tranches PASS" no longer composes into "the
   catalog passes". Tell-tale: a mid-campaign diff to `validate.py` or a schema
   with no dedicated RPI around it. Countermeasure: F1's digest-pinned fixed
   point with the documented escape hatch, exercised exactly once (F5b's
   rpi-report rev) so the hatch itself is proven.
3. **Dead-check green.** Tranche reports cite named checks; some of those
   checks are orphaned scripts nothing executes (V2 proves the class is live
   today: a plan-named gate is broken, wired to nothing, and its phrase anchors
   drifted without any signal). "The gate passed" silently means "the gate I
   chose to run passed" — or worse, "the gate would have failed had anything
   run it." The same class swallows the plan-matrix ↔ catalog congruence duty:
   with no executable conformance check, the generated matrix can diverge from
   the plan's 49 rows and T7's hand-inspection blesses whichever one the
   inspector happens to open. Countermeasure: F1's liveness inventory with
   seeded-red proof per criterion class, plus C24's conformance script.

---

## 6. Rulings on the current plan

**Keep:**

- The target architecture and its two-level split (campaign above, RPI atom
  below); it is the correct recovery of authority from V1's evidence.
- D1's two orthogonal concepts (layer vs seams) and the refusal to collapse
  them into `tier`/`context_rel`/dependencies.
- D2's one-RPI-per-tranche economy and the refusal to pay 49 integration
  verdicts.
- D3's serialized-surface list (verified complete against the generator).
- D4's proof-per-shape table — the best single table in the plan.
- T0→T1→content→T7 spine; `skill-builder` in T1; `learn` in T5.
- The 12 catalog acceptance criteria as *goals* (their enforceability is
  repaired by F1/F3, not their content).
- The write-scope exclusion of unrelated Go CLI behavior (ratified in §4).
- The per-skill target rows for T3/T4/T5 skills — spot-checked against sources
  (e.g., `status`'s runtime hints, `cass`'s kernel size, `reverse-engineer`'s
  "steal-map decision" wording, `shared`'s populate-or-retire) and found
  accurate.

**Change:**

- T0: add the check-liveness inventory, validator fixed point, seam-enum
  reconciliation, vocabulary pin, and #988 reconciliation ledger (F1, F2, C29).
- D1 enum: `experiment_select` → `option_shaping`; define `standalone` and
  `core_phase` in the contract's seam table; contract owns the enum (F2).
- T1: add the taxonomy budget — tier derived/retired, disposition constrained,
  consumes/produces tightened, effects enum, typed `output_contract`,
  runtime-hints policy, probe-backed trigger acceptance (F3).
- T2: re-cut to the five-skill authority atom; move `product`+`goals` to a
  light parallel tranche (F4).
- T2 scenarios: bind to the extended pure-python reference harness with
  negative-authority state-transition tests (F4).
- T6: split into T6a transports / T6b host+support (F4).
- Regen: one entrypoint command; state the lane rule incl. "one open tranche at
  a time" (F4/C18/C19).
- Schedule the `rpi-report.v1` revision (correlation field) inside T2 as the
  fixed point's one sanctioned schema change; delete the dead packet schemas;
  home `model-dispatch.md` in `shared/` (F5).
- T7: add the plan-matrix conformance script and verdict-digest completion
  matrix; after T7, placement authority formally transfers from the plan's
  table to the generated views (C24/C25).
- SKILL-ROUTER regeneration key: disposition → layer/seam (C28).

**Reject:**

- Nothing in the plan requires outright rejection. The nearest candidates —
  T2's composition and the D1 enum — are repairable by the changes above rather
  than wrong in aim. One soft rejection: the interim rule that "the canonical
  placement matrix lives in the active overhaul plan" must be explicitly
  time-boxed to T7; as an open-ended arrangement it inverts the repo's own
  source-precedence order (a dated plan outranking generated projections) and I
  reject it as a *standing* state.

---

## 7. Appendix — one-row-per-skill disposition (all 49)

Legend: **layer/seams** = my ruling (∅ = agree with plan row verbatim);
**Δ** = amendment to the plan's target change; **Tr** = tranche under the F4
re-cut. "agree" means the plan's row survived verification against the skill's
current source.

| # | Skill | Plan layer | My ruling | Δ vs plan row | Tr |
|---|---|---|---|---|---|
| 1 | `account-rotation` | runtime | ∅ | agree; host-routing facts (claude-acct vs caam) stay boundary prose, add capability discovery | T6b |
| 2 | `agent-mail` | runtime | ∅ | agree; add reservation/ACK schemas + degraded-surface fixtures | T6a |
| 3 | `agent-native` | runtime | ∅ | agree + **give model-dispatch.md to `shared/`** (F5); keep intervention-ladder text | T6a |
| 4 | `agy-native` | runtime | ∅ | agree | T6a |
| 5 | `automation-shape-routing` | runtime | **support**; seams cross_cutting, standalone | it routes and never transports; also resolve "route only" vs "invoke the owner" internal tension (C27) | T6b |
| 6 | `bootstrap` | support | ∅ | agree; byte-idempotence proof; fix `consumes` skill-slugs (F3) | T6b |
| 7 | `cass` | evidence | ∅ | agree; kernel 263→<250; external-dependency facts → discovery | T3 |
| 8 | `cc-hooks` | support | ∅ | agree; kernel 289→<250; **add missing output_contract**; policy-dispatch subsystem docs → references | T6b |
| 9 | `codebase-recon` | evidence | ∅ | agree | T3 |
| 10 | `codex-exec` | runtime | ∅ | agree; `tier: orchestration` singleton dissolves with F3 | T6a |
| 11 | `converter` | implementation | ∅ | agree; keep clean-write + parity semantics; projection-edit rejection test | T4 |
| 12 | `council` | judgment | ∅ | agree; `council-report.v1` schema-backed under F3's typed output_contract | T3 |
| 13 | `craft-goal` | campaign | ∅ | agree — sole campaign compiler/linter; **reconcile #988 first (C29)**; structured outer-goal contract becomes a schema; joins the T2 authority atom | T2 |
| 14 | `dcg` | support | ∅ | agree; **add missing output_contract**; stale command facts → discovery | T6b |
| 15 | `doc` | implementation | ∅ | agree; delete shouted execution prose; declare `.agents/doc/` + README writes in effects | T4 |
| 16 | `domain` | evidence | ∅ | agree; it also serves the new campaign vocabulary once F2 pins it | T3 |
| 17 | `goals` | product | ∅ | agree, but the fitness rename is **gated on the F2 language pin**; CLI stays `ao goals` | T2p |
| 18 | `handoff` | support | ∅ | agree; naming/schema/digest rules; read-without-consume test | T6b |
| 19 | `idea-genie` | judgment | seams **option_shaping**, plan_input | seam renamed per F2 (V5); otherwise agree | T3 |
| 20 | `implement` | experiment | ∅ | agree — remove repair-revision authority (V1); binding output contract = manifest+receipt shape | T2 |
| 21 | `learn` | evolution | ∅ | agree; dedup-by-objective + decay already sketched in body | T5 |
| 22 | `ms` | support | ∅ | agree; split read vs write/admin; volatile corpus facts → discovery | T6b |
| 23 | `ntm` | runtime | ∅ | agree; truth-stack text is good, keep it | T6a |
| 24 | `operationalize` | evolution | ∅ | agree; three-instance floor + reapply proof already strong | T5 |
| 25 | `pattern-mining` | evolution | ∅ | agree; holdout/back-application discipline already strong | T5 |
| 26 | `plan` | experiment | ∅ | agree; add output contract (amendment shape); scope-class admission stays | T2 |
| 27 | `postmortem` | judgment | ∅ | agree; declare report-write effect (currently `effects: []`) | T3 |
| 28 | `premortem` | judgment | ∅ | agree; material-amendment-stops-RPI matches the contract's example | T3 |
| 29 | `product` | product | ∅ | agree; byte-preservation of user sections | T2p |
| 30 | `rch` | runtime | ∅ (seams implement_method, cross_cutting per plan) | agree — it is build-method support, not transport | T6b |
| 31 | `reality-check` | judgment | ∅ | agree; publish report schema | T3 |
| 32 | `refactor` | implementation | ∅ | agree; declare code-write effects; neutrality gates already strong | T4 |
| 33 | `research` | evidence | ∅ | agree; **runtime-hints policy** applies (`allowed-tools`) per F3 | T3 |
| 34 | `reverse-engineer` | evidence | ∅ | agree; "steal-map decision" → adoption recommendation; authorization guards stay | T3 |
| 35 | `rpi` | experiment | ∅ | agree — delete continuation envelope + stale trigger (V1); **schema rev adds opaque `correlation`** (F5/V6); keep proportionality guard reworded as within-experiment economy; fix `consumes` slug abuse | T2 |
| 36 | `sbh` | support | ∅ | agree; device/volume identity binding | T6b |
| 37 | `scaffold` | implementation | ∅ | agree; refusal/overwrite proofs; declare filesystem effects | T4 |
| 38 | `scope` | judgment | ∅ | agree; axiom-kernel + recovery ceremony are keepers; publish review schema | T3 |
| 39 | `security` | evidence | ∅ | agree; split one-shot scan vs bounded investigation; declare artifact effects | T4 |
| 40 | `shared` | support | ∅ | **populate, not retire**: receives model-dispatch.md + evidence-format refs, gaining six live consumers (F5/V7) | T6b |
| 41 | `skill-builder` | implementation | ∅ | agree; owns the T1 factory; placement support + semantic stocktake; regen entrypoint lands here | T1 |
| 42 | `standards` | evidence | ∅ | agree; repair invalid practice slugs; findings shape | T3 |
| 43 | `status` | support | ∅ | agree; **strip `model: haiku` + `allowed-tools` per F3 policy**; JSON output | T6b |
| 44 | `swarm` | runtime | ∅ | agree; resource/external-effect isolation beyond paths; `dispatch_once.py` joins the F4 harness | T6a |
| 45 | `test` | implementation | ∅ | agree; oracle hierarchy/mutation-kill are keepers; declare test/product write effects | T4 |
| 46 | `toil-mining` | evolution | ∅ | agree; measured-floor discipline already strong; declare artifact write | T5 |
| 47 | `using-gc` | runtime | ∅ | agree; quest-state leakage boundary already correct in body | T6a |
| 48 | `validate` | experiment | ∅ | agree; cross-model recipe reference repoints to `shared/` (F5); required-vs-preferred diversity fail-closed test | T2 |
| 49 | `workflow-builder` | implementation | ∅ | agree; framework-gravity guard is a keeper; declare generated-script writes | T4 |

Count check: 49 rows; T1×1, T2×5, T2p×2, T3×12, T4×7, T5×4, T6a×7, T6b×11 = 49.

---

*Sealed. FABLE.*
