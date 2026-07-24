# WIZARD_SCORES_FABLE_ON_SOL — blind cross-score

- Scorer: FABLE (`claude-fable-5`)
- Subject: `raw/WIZARD_IDEAS_SOL.md` (read in full)
- Date: 2026-07-24
- Not read: any score of my own work; sealed per protocol.

## Verification stance

I did not score SOL's evidence on trust. Before scoring I re-verified their
load-bearing claims against live source. **Every claim I tested held**, including
two I had missed in my own sealed pass:

| SOL claim | My verification | Result |
|---|---|---|
| RPI/Validate digest-algorithm mismatch (`run_once.py:17-19,60` canonical-JSON mapping vs `validate.py` exact `intent_bytes`; test masks it) | Read both sources + `test_run_once.py` mock, which returns `MODULE.digest(_plan)` — rpi's own algorithm — so the incompatibility never surfaces; a real byte-digest verdict would raise at `run_once.py:77-78` | **CONFIRMED** (I missed this) |
| `verdict.v2` PASS forbids `not_checked` | `"not_checked": {"maxItems": 0}` in the PASS branch of the schema; writer raises `ContractError` "PASS cannot contain not_checked items" | **CONFIRMED** (I missed the plan-vs-schema tension) |
| Two live frontmatter authorities | `validate-skill-schema.sh:5,18` → v1; `validate-skill-frontmatter.sh` → v2 | **CONFIRMED** |
| Go catalog reader not a strict contract consumer | `cli/internal/skills/catalog.go:31-45`: no tier/disposition/capabilities/effects fields, no unknown-field rejection — and its comment falsely claims it mirrors the schema "exactly" | **CONFIRMED**, stronger than SOL stated |
| `regen-all.sh` records failure and continues | `step()` sets `fail=1` and proceeds to later generators | **CONFIRMED** (I did not know this script existed; it retires part of my own C19) |
| Strict frontmatter fails on one missing optional field | Reproduced: `49/49 ok … 1 missing context_rel … STRICT FAIL` | **CONFIRMED** |
| Orchestration boundary gate exits 2 on a deleted Go path | Independently found and executed in my own pass | **CONFIRMED** (convergent) |

This verification discipline shapes the scores: SOL's evidence quality is the
best thing about their artifact, and where I dissent it is on placement and
cost, not on facts.

---

## Finalist 1 — Executable RPI evidence transaction before campaign migration

**Breakdown:** Structural 235 / Coherence 200 / Proof 235 / Feasibility 215 —
**Total: 885**

**Strongest source-backed contribution.** The digest-mismatch discovery
(`run_once.py:60` vs `validate.py` byte-hashing, masked by the test mock) is the
single best finding either wizard produced in this duel. It proves the two
reference implementations of the core contract disagree on the *identity
algorithm the whole architecture hangs on*, while both test suites pass — the
exact "appears complete" failure class the overhaul exists to kill, already
alive in the kernel. The companion catch — PASS's `maxItems: 0` on
`not_checked` colliding with the plan's platform-gap disclosure duty — is
resolved correctly with `unchecked_required` vs `declared_exclusions`, and the
line "Do not merely loosen the current `not_checked` rule" shows exactly the
right fail-closed instinct.

**Strongest objection / hidden cost.** The earliest, least-proven tranche of
the whole program mutates the most safety-critical code (validate writer,
schema, Go reader, golden corpus, atomically), so the campaign's judge is
rebuilt before anything has been judged — the mitigation (pure Python, golden
corpus, one-tranche atomicity, v1 read-only decoder) is credible but the risk
concentration is real. Secondary gap: the proposed `rpi-report.v2` field list
has **no correlation facts**, yet
`docs/contracts/skill-ports-and-adapters.md:63-64` promises RPI "may carry
their identifiers as opaque correlation facts" — SOL's v2 report reproduces the
same unrepresentability defect the current `additionalProperties: false` v1 has.

**Ruling: ADOPT WITH PRECISE MODIFICATION.** (1) Add an optional opaque
`correlation` object to `rpi-report.v2` so the contract's promised carriage is
representable. (2) After this tranche lands, the repaired kernel
(validate.py + the three schemas) becomes an explicitly digest-frozen fixed
point for all later tranches; further changes require their own dedicated RPI
with re-baselining. (3) Keep their sequencing: this lands before any campaign
or portfolio migration.

**Evidence that would move the score ≥150.** Down: a live composed integration
path (workflow, `ao`, or harness) that already reconciles the two digest
algorithms end-to-end — I searched and found none; its existence would collapse
the 98%-confidence premise and the finalist's urgency. Also down: proof that a
real `rpi-report.v1` consumer (e.g., anything in `statusapp` or provenance)
breaks under v2 in a way the read-only v1 decoder cannot absorb.

---

## Finalist 2 — One typed skill-contract compiler (`skill-contract.v3`)

**Breakdown:** Structural 205 / Coherence 235 / Proof 225 / Feasibility 190 —
**Total: 855**

**Strongest source-backed contribution.** The `authority` verb enum
(`advise | read_evidence | refine_intent | mutate_subject | write_verdict |
select_experiment | transport`) is the best structural idea in their proposal:
it converts catalog acceptance #3/#4 from prose review into compiler
rejections, and it is grounded in a verified defect chain (v1/v2 dual
authorities; `catalog.go` modeling neither tier nor effects while claiming
schema parity; strict-vs-loose validators disagreeing on the same 49 files
today). Generating bounded-context *membership* from contract metadata while
keeping hand-written responsibilities kills a second-inventory drift surface I
had only partially addressed.

**Strongest objection / hidden cost.** The enum itself contains an authority
error of the same class it exists to prevent: `select_experiment` is granted to
"`craft-goal` or a future explicit campaign engine," but SOL's **own appendix
row for craft-goal** says it must never own "running Goal, mutating graph" —
craft-goal compiles and lints campaign policy; the selecting agent is the Goal
*runtime* (a caller-side product surface), which is not a skill. No skill
should hold `select_experiment`; encoding it as a grantable skill authority
re-opens the door the invariant closed. Hidden cost: step 5 puts the 49-file
metadata population *inside* the compiler tranche, making it the heaviest
single candidate in either proposal; and v3 is silent on the fate of the
existing overlapping axes (`tier`, `disposition`, `hexagonal_role`) — without a
retirement rule the compiler adds the seventh and eighth classification axes
while pruning none.

**Ruling: ADOPT WITH PRECISE MODIFICATION.** (1) Delete `select_experiment`
from the skill-grantable enum entirely; the compiler invariant becomes "no
skill may declare it," which matches both wizards' architecture. (2) Add an
explicit axis-retirement decision (tier derived-or-deleted, disposition
constrained to curation) to the same tranche. (3) Move 49-file population out
to each skill's owning tranche, keeping the compiler tranche to
grammar + parser + fixtures + strict Go consumers, with their frozen-ledger
migration mode as the transition rail.

**Evidence that would move the score ≥150.** Up: a working v3 compiler
prototype with the hostile fixture set landing small, proving the mega-tranche
fear wrong. Down: demonstration that real skills cannot be expressed in the
authority/effects grammar without per-skill exception lists (e.g., `test`'s
TDD-mode product edits, `sbh`'s host mutations), which would show the schema is
overfitted and would push the design back toward free text.

---

## Finalist 3 — Projection publication transaction

**Breakdown:** Structural 220 / Coherence 190 / Proof 225 / Feasibility 175 —
**Total: 810**

**Strongest source-backed contribution.** The verified `regen-all.sh`
record-failure-and-continue behavior (step wrapper sets `fail=1`, later
generators still run) plus the hash pass running as a separate step is a real
mixed-generation sealing hazard: a failed mesh write followed by a successful
hash regeneration can stamp the wrong mixture as current. Their "manifest-last
as commit marker, absence/mismatch is explicit drift, never success" is the
honest way to get transaction semantics over a multi-file tree, and
`--check`/write sharing one staged renderer closes a divergence I had not
named.

**Strongest objection / hidden cost.** The machinery is sized for a concurrency
problem the repository does not currently have: the repo's stated default is
one agent, one writer, and both wizards' tranche designs serialize regeneration
at freeze points. The lock, staging root, owner map, crash-kill test matrix,
and journal are real engineering on the critical path *before mass edits*,
while ~80% of the observed hazard (continue-after-failure, mixed sealing) is
closed by two cheap changes to the existing script: fail-fast on first step
failure, and a generation manifest written last. The full publisher earns its
cost only when parallel lanes actually run projection-adjacent work.

**Ruling: ADOPT MODIFIED (staged).** Now: owner map, fail-fast `regen-all`,
manifest-last commit marker, `--check`==render parity, byte-idempotence test.
Deferred until a tranche actually plans concurrent lanes: repository lock,
staged-complete replacement, crash-kill matrix. Worker-lanes-never-generate is
adopted as written.

**Evidence that would move the score ≥150.** Up: any reproduced or historical
mixed-generation incident (a sealed hash over a failed mesh) — that converts
the crash machinery from speculative to demonstrated; or a real plan to run
parallel skill lanes in T3/T6, which makes the lock load-bearing. Down: a
demonstration that fail-fast + manifest covers every failure path the crash
matrix would catch, making the publisher redundant at this scale.

---

## Finalist 4 — CLI audit as a prerequisite substrate stage

**Breakdown:** Structural 210 / Coherence 165 / Proof 220 / Feasibility 170 —
**Total: 765**

**Strongest source-backed contribution.** The three-tranche decomposition
(containment/owned-temp → command effect/output policy → subprocess lifecycle)
is a better cut of the audit's roadmap than the audit's own ordering, each with
genuinely executable acceptance (traversal/symlink fixtures; dry-run
zero-effect matrix per effectful leaf; JSON equivalence per read leaf;
orphan-grandchild kill test; isolation-root cleanup across
success/error/timeout/partial-setup). Preserving audit reproducers as RED tests
is exactly right. All cited defects re-verified.

**Strongest objection / hidden cost.** The load-bearing premise — that skill
migration acceptance "explicitly depends on" the defective CLI properties — is
asserted, not traced. The migration's proof chain runs through frontmatter
validators (Python), the mesh generator (Python), codex-sync (bash),
`validate.py` (pure Python), bats, and the `ao skills`/`gate check`/
`verdictcheck` paths — which the audit's own per-package ledger rates among the
CLI's *strongest* surfaces. The defects live in `eval`, `provenance`
`goals`-measure, and dry-run paths the migration never invokes: no tranche
proof runs `ao eval`, needs `--dry-run`, or streams unbounded child output
through a migration gate. Making three Go tranches a hard prerequisite
therefore queues the entire skill program behind work whose necessity for that
program is undemonstrated — while the one genuinely urgent item (eval
traversal, a security defect) needs no prerequisite framing to justify going
first. The cost is pure critical-path inflation.

**Ruling: ADOPT THE DECOMPOSITION, REJECT THE PLACEMENT.** Adopt the three
tranches as the structure of a *parallel* CLI program: containment expedited
immediately (release-risk on its own merits), policy and process tranches
proceeding alongside skill tranches. The only hard ordering constraints kept:
(a) the shared effect/output vocabulary lands before or with the contract
compiler; (b) any specific check a migration tranche's acceptance actually
invokes is repaired before that tranche relies on it.

**Evidence that would move the score ≥150.** Up: an enumerated trace of
migration acceptance commands showing ≥1 acceptance-relevant proof traversing a
defective path (a gate invocation whose subprocess capture or dry-run behavior
can corrupt a tranche verdict, or a skill probe that shells into `ao eval`) —
that would validate "prerequisite" and I would concede the placement. Down:
the migration completing with proof-clean verdicts on the current CLI, which
would confirm the crossings model.

---

## Finalist 5 — Portfolio and tranche recut around authority and proof shape

**Breakdown:** Structural 195 / Coherence 210 / Proof 200 / Feasibility 185 —
**Total: 790**

**Strongest source-backed contribution.** The `security` reclassification is
the best portfolio call in either artifact: its collection is read-only by
default and its output is validation evidence; parking it with
candidate-producing specialists "invites a validator to authorize security
policy edits under Implement without separate caller judgment" — a sharper
authority argument than the plan's row and than my own appendix (which had
followed the plan). The conditional-retention probe rule for runtime adapters
(keep `agy-native` only while a distinct unavailable/timeout/cleanup probe can
be maintained, else fold into `agent-native`) is a falsifiable retention policy
rather than a taste call. The per-skill "observable specialization proof"
column is the strongest appendix column in either artifact.

**Strongest objection / hidden cost.** Three costs. (1) Stage explosion: 18
stages ≈ 18 integration verdicts versus the plan's 8; their own R40 rejects
per-skill ceremony, yet T3C is a single skill (`postmortem`) holding its own
integration tranche — the marginal verdict there buys nothing a T3B membership
would not. (2) The `shared` retirement rests on an incomplete consumer search:
they checked frontmatter artifact-flow (finding only `bootstrap`) but not prose
references — `agent-native/references/model-dispatch.md` is normatively cited
by six skills including core `validate`, i.e., a cross-skill contract class
exists and currently lives under one optional adapter; retiring `shared`
without homing that class leaves a core→optional reference intact. (3)
Replacing the 250-line cap with a measured always-loaded context budget swaps a
cheap, enforceable-today gate for measurement machinery that is unspecified
(measured how, at which trigger, enforced where) — for a catalog where exactly
two kernels exceed the cap and both by <40 lines.

**Ruling: ADOPT PARTIALLY / SUBSUME.** Adopt: security move; router
re-layering of `automation-shape-routing` (convergent with my own ruling);
conditional-runtime-probe retention; the specialization-proof column as a
required matrix field; alias-window mechanics for any rename. Modify: merge
T3C into T3B; merge T2B into T2A or T3; cap total stages near the plan's
original count. Defer: `goals`→`fitness` executes only after the campaign
vocabulary is pinned in the ubiquitous-language contract (rename mechanics
theirs). Reject for now: `shared` retirement *as stated* — first decide the
home for the shared cross-skill contract class (populated `shared/` or a
non-skill home under `docs/contracts/`); retirement is acceptable only with
that home named. Reject: swapping out the line cap before a specified,
implemented context-budget measure exists; keep the cap as the proxy until
then.

**Evidence that would move the score ≥150.** Up: RED fixtures demonstrating
that two skills grouped in my coarser tranches genuinely require separate
integration verdicts (their falsifier, and a fair one) — sustained across T3/T6
that vindicates the finer table. Down: post-hoc evidence that the extra stages
caught no defect the coarser cut would have missed, or discovery of a real
`shared`-payload consumer that makes retirement flatly wrong.

---

## Score summary

| Finalist | Structural | Coherence | Proof | Feasibility | Total |
|---|---|---|---|---|---|
| F1 evidence transaction | 235 | 200 | 235 | 215 | **885** |
| F2 contract compiler | 205 | 235 | 225 | 190 | **855** |
| F3 publication transaction | 220 | 190 | 225 | 175 | **810** |
| F5 portfolio recut | 195 | 210 | 200 | 185 | **790** |
| F4 CLI prerequisite | 210 | 165 | 220 | 170 | **765** |

Spread 120 points; ranking F1 > F2 > F3 > F5 > F4.

## Opponent's strongest overall architecture

The through-line "make the membrane executable before migrating policy around
it." F1's typed evidence transaction plus F2's authority verbs turn the two
weakest words in the current plan — "behavioral proof" — into compiler
rejections and state-machine fixtures, and the whole stance is earned by a
real discovery: the kernel's two reference implementations disagree on intent
identity today while all their tests pass. That is the correct foundation
insight, it is verified, and my own proposal was weaker for building its
harness plan on `run_once.py` without noticing the disagreement.

## Opponent's weakest overall assumption

That the skill migration's proof chain materially traverses the defective CLI
paths — the premise elevating F4 from "urgent parallel program" to "hard
prerequisite stage." The audit's own per-package assessment rates the
proof-relevant packages (skills catalog/graph, gates orchestration,
verdictcheck, statusapp) as strong; the verified defects cluster in
eval/provenance/goals-measure and in flags (`--dry-run`, `--json`
fragmentation) no migration proof uses. A secondary weak assumption:
that `shared`'s emptiness proves the absence of a shared-contract need —
their consumer search covered frontmatter flow but not the six prose
references to `model-dispatch.md`, including one from core `validate`.

## Semantic matches between their finalists and mine

- **SOL F1 ↔ my F1 (validator fixed point) + my F5b (rpi-report rev).** Same
  concern — judge integrity across the campaign — with complementary
  mechanisms: they repair the kernel first; I froze it and gated changes. Their
  ordering is stronger because you cannot legitimately freeze a kernel whose
  two reference implementations disagree; my freeze becomes the *sequel* to
  their repair. My correlation-field schema fix is additive to their v2 report,
  which lacks it.
- **SOL F2 ↔ my F2 (seam vocabulary) + my F3 (taxonomy budget).** Their
  compiler subsumes my schema-tightening items (effects enum, typed
  output_contract, consumes/produces hygiene) with more mechanism; my
  contributions they lack: the axis-retirement rule (tier/disposition fate is
  unaddressed in v3) and the seam-name authority hygiene — notable because
  their authority enum reproduces the same `select_experiment` class of error I
  flagged in the plan's seam enum.
- **SOL F3 ↔ my C19/C18 (single regen entrypoint, lane rule).** Same direction;
  they found `regen-all.sh` already exists (retiring my C19 as novel) and
  correctly relocated the problem from "missing entrypoint" to "missing
  transactionality." Their mechanism is heavier than the observed hazard;
  staged adoption reconciles.
- **SOL F4 ↔ my §4 (separate program, three crossings).** Direct disagreement
  on placement; near-agreement on content (their three-tranche cut is better
  than my loose "roadmap items 1–2 first"), and both expedite eval
  containment.
- **SOL F5 ↔ my F4 (tranche recut) + my appendix.** Convergent independent
  findings: split plan-T2, split plan-T6, re-layer `automation-shape-routing`
  to advisory/support, treat `craft-goal` as compiler-not-runtime. Divergent:
  kernel-first (them) vs authority-transfer-atomic (me); `shared` retire vs
  populate; rename-now-with-alias vs rename-after-vocabulary-pin; line cap
  replaced vs retained.

## Genuine disagreements that must not be averaged away

1. **CLI audit placement: prerequisite stage vs parallel program with named
   crossings.** A sequencing fork — one path or the other. Discriminator:
   enumerate every command the migration's acceptance actually executes and
   check whether any traverses a defective path. If yes, their placement wins;
   if none, mine does. Do the trace; do not split the difference.
2. **Plan-T2 composition: kernel-first (their T2A→T2C) vs atomic authority
   transfer (my 5-skill atom).** Their argument — campaign tests can pass
   against a mocked or incompatible kernel, as the live digest mismatch proves
   — beats my static-prose concern about a zero-owner interim, because during
   the gap no runtime holds campaign authority; the interim is documentary.
   I concede the ordering; what must not be lost from my side is the atomicity
   *within* the envelope-deletion step: rpi's continuation authority must be
   deleted in the same verdict that names craft-goal as the campaign policy
   owner, even if craft-goal's own migration lands later.
3. **`shared`: retire vs populate.** Not averageable — a root either exists or
   does not. The deciding evidence is the model-dispatch class: six consumers,
   one core. Acceptable synthesis is not a compromise but a third option
   neither of us proposed: home cross-skill runtime recipes as a
   `docs/contracts/` document (non-skill, non-adapter), then retire the empty
   root. Either wizard's position becomes correct once that home is named.
4. **Kernel-size gate: 250-line cap vs measured context budget.** Different
   theories of what the gate measures. Keep the cap until a specified
   measurement exists; but record their point that lines are a proxy and the
   real quantity is always-loaded tokens. Swapping proxies before the
   replacement is implemented is how a cheap working gate dies.
5. **Where 49-file metadata population happens: inside the compiler tranche
   (their step 5) vs distributed to owning tranches (mine).** Determines the
   size of the riskiest non-kernel tranche; pick one explicitly.

## Appendix items that change my own dispositions

1. **`security`** — I adopt their MOVE to evidence/validation-evidence with
   separate authorization fixtures for mutating modes. My appendix had kept it
   in T4 following the plan; their authority argument is better than the row I
   approved.
2. **`agy-native`** — I upgrade my unconditional "agree/keep" to their
   CONDITIONAL KEEP with the merge-into-`agent-native` trigger when no distinct
   probe can be maintained. The falsifiable retention rule is the right shape
   for every thin runtime adapter.
3. **`rpi`/harness plan** — their digest-mismatch finding amends my F4: the
   pure reference layer must be *repaired to one identity algorithm* before it
   can host the negative-authority state-transition fixtures I proposed; and my
   F1 fixed-point freeze applies to the repaired kernel, not the current one.
4. **`craft-goal`** — their "Never own: running Goal" row sharpens my T2
   treatment: the campaign skill compiles and lints; the campaign runtime (a
   caller-side product surface, not a skill) selects experiments. Consequence
   carried into their own F2: no skill — including craft-goal — should hold
   `select_experiment`.
5. **Regen tooling** — the existence of `regen-all.sh` (their F3 evidence)
   retires my C19 as a novelty claim; my proposal reduces to "make the existing
   entrypoint fail-fast and manifest-committed," which is their F3's cheap
   half.

*End of scores. FABLE.*
