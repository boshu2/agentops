# WIZARD_BLINDSPOTS_FABLE — blind-spot probe

- Author: FABLE (`claude-fable-5`)
- Date: 2026-07-24
- Read for this phase: both steelmen and every prior duel artifact, completely.
- Novelty discipline: each item below was checked against both original
  proposals *and* all later artifacts; where a later artifact grazes the topic,
  the graze is named and the genuinely new part is isolated. Nothing here
  renames an existing proposal.

The shared framing that produced these blind spots: both wizards treated the
repository worktree as the universe (the evidence packet was repo-internal),
treated deterministic fixtures as the only admissible proof (the repo's own
doctrine, reflected back), and optimized a completion end-state rather than
the life the system leads during and after the campaign.

---

## BS1 — The installed-estate version-skew cliff (strict readers × stale copies)

**Why both wizards missed it.** The evidence packet was repo-internal, and
`AGENTS.md`'s "consumer repositories keep their own policy" exiled
distribution from both minds. SOL's steelman objection-2 grazes "external
consumers get a compatibility window" for *catalog fields*; nobody examined
the actual delivery channels: npx/skills.sh **copies** (frozen at install
time, never updated), Claude/Codex **plugin bundles** (release cadence),
Homebrew `ao` binaries (independent cadence), and `ao skills link` symlink
estates (live). The repo's own memory has the lesson — "Landed ≠ installed" —
and neither wizard surfaced it.

**Affected surface.** CLI skill consumers (`cli/internal/skills*` after the
strict-reader change), `registry.json`/marketplace manifests, every image
manifest, release engineering.

**Failure mechanism and earliest signal.** The consensus architecture makes
Go readers *strict*: reject unknown fields, stale schema versions, count
mismatches. Strictness is correct in-repo and a cliff off-repo: a user
upgrades Homebrew `ao` to the v3-strict release while their skills tree is an
npx copy made months earlier (v2 catalog) → `ao skills list` hard-fails where
yesterday it worked. The mirror case — old binary, new catalog — fails soft
today (Go ignores unknown fields) but silently drops the new semantics, so
effect/authority honesty is invisible exactly where users run it. Earliest
signal: post-release issue reports of `ao skills` erroring on machines that
never touched the repo.

**Falsifier.** A channel audit proving catalog and binary always ship
atomically to every estate (they demonstrably do not — three channels, three
cadences), or telemetry showing the copied-estate population is empty.

**Smallest non-churning mitigation.** Version negotiation in the one strict
reader: full strictness for `schema_version == 3`; legacy-tolerant read with a
one-line deprecation warning for ≤2; a documented support window in the
release notes of the cutover release. One `switch` on a field that already
exists, plus one test per branch.

**Classification.** **Release blocker** for whichever release carries the v3
cutover; invisible to every in-repo gate.

---

## BS2 — Intent-byte transport: exact identity needs a single-mint rule *(mandated: exact identity/evidence comparability)*

**Why both wizards missed it.** Duel dynamics. Exact-byte identity became the
consensus hero after the digest-mismatch discovery, and united positions get
no adversarial pass: SOL's acceptance criterion even celebrates that
whitespace-differing bytes yield different digests. Both probed the
*algorithm*; neither probed the *transport* of the bytes between contexts.

**Affected surface.** The kernel tranche (`snapshot-intent`, `store-verdict`),
cross-model validator dispatch (`codex-exec`, `agy-native`, NTM panes),
campaign-level cross-experiment comparison.

**Failure mechanism and earliest signal.** The exact-byte doctrine is only as
stable as the path the bytes travel. Within one invocation on one machine the
snapshot file is the reference and all is well. But the architecture
explicitly invites *fresh validators in other runtimes and machines* — and a
fresh context that re-derives intent from the living tracker (re-export, API
fetch, copy-paste into a pane prompt) instead of receiving the snapshot file
picks up CRLF conversion, Unicode NFC/NFD normalization (macOS), trailing-
newline conventions, or tracker re-serialization. Result: digest mismatch →
NOT_PROVEN with no semantic drift — false-alarm storms that train operators to
distrust exactly the signal the kernel exists to provide. At campaign level,
two snapshots of the "same" acceptance differing only in line endings read as
different acceptance, corrupting cross-experiment reasoning. Earliest signal:
a NOT_PROVEN whose intent diff is whitespace-only.

**Falsifier.** A contract clause plus fixture demonstrating that every
validator entry path receives the snapshot *by digest reference* and that
re-derivation is structurally impossible (not merely discouraged) — that
closes the gap by construction.

**Smallest non-churning mitigation.** One contract sentence and one negative
fixture in the kernel tranche: *intent bytes are minted exactly once by
`snapshot-intent`; every later consumer receives the snapshot by digest
reference; re-derivation from the living source is a transport violation and
must be reported as such, not as acceptance drift.* No normalization layer —
single-mint makes normalization unnecessary, which is the cheapest possible
fix.

**Classification.** **Tranche blocker** for the kernel tranche (it is one
sentence plus one fixture; landing it later means re-opening epoch 1's
contract).

---

## BS3 — No live routing baseline: the overhaul rewrites every trigger with no behavioral before/after *(mandated: portfolio usability/routing after the metadata overhaul)*

**Why both wizards missed it.** Both wizards inherited the repo's
deterministic-proof aesthetic and reduced "trigger separation" to lexical
fixtures (`trigger_probes`, three-polarity route tables,
`scan_descriptions.py`). Model-behavioral measurement felt off-doctrine — and
the in-repo eval substrate had just been tainted by the audit, so neither
leaned on it. Yet the actual routing consumer is a stochastic model reading a
flat description list, and the repo's own memory already proved the gap:
"SKILL.md tool-choice instructions INERT — ship a behavioral probe."

**Affected surface.** All 49 descriptions/triggers (every tranche rewrites
some), `ms` search, harness-level skill selection, the codex 1024-character
description truncation.

**Failure mechanism and earliest signal.** Descriptions optimized to win
lexical probes are not automatically better routing inputs for a model —
keyword-dense trigger lists can read as noise, negative-boundary prose can
suppress legitimate invocation, and truncation can cut the polarity clauses
the fixtures verified. Post-overhaul, every probe is green while live agents
mis-route (wrong skill, or none) — and because nobody captured a *before*
baseline, the regression is undetectable and unattributable: the one
measurement that cannot be reconstructed after the fact is how routing behaved
before the rewrite. Earliest signal: sessions invoking `research` where
`codebase-recon` was intended (or vice versa), rising "which skill covers X"
questions, `ms outcome --failure` entries.

**Falsifier.** A pre/post routing evaluation on a frozen scenario set showing
selection accuracy unchanged or improved — which requires the *pre* half to
exist before the first description edit.

**Smallest non-churning mitigation.** Freeze ~30 routing scenarios now
(task-utterance → expected-skill, drawn from real session history via `cass`),
run them once against the current catalog (any cheap harness — even the
lexical probe *plus* one live-model pass), store results as the baseline
artifact; rerun at T7 and diff. One file, two runs, no new machinery.

**Classification.** **Tranche blocker** for the first description-touching
tranche (the baseline must precede the first rewrite); the rerun is a T7
convergence item.

---

## BS4 — Retirement by calendar, not by measured use: nothing instruments the aliases *(mandated: proof that retirement/rename does not erase real use)*

**Why both wizards missed it.** Both proved retirement safety with *static
reference graphs* (consumers repointed, generated checks green) — repo-
internal, snapshot-in-time. Usage lives elsewhere: session history, operator
configuration (the operator's global `CLAUDE.md` routes work to skills *by
name* in its registry tables), saved prompts, muscle memory. Source precedence
ranks session history last, so both wizards' evidence instincts skipped it.
The sharpest miss: SOL's `goals` alias expires after "one declared window" — a
**calendar** gate, in a repository whose own memory codifies the opposite
lesson ("never let a gate flip itself on a calendar; gate flips on measured
rate") and which already ships the needed pattern (the cc-hooks guardrail
telemetry: one hashed JSONL line per fire).

**Affected surface.** `goals`→`fitness` alias, `shared` retirement, deleted
trigger phrases (`research-plan-implement`), any future rename; operator
config surfaces outside the repo.

**Failure mechanism and earliest signal.** The alias window lapses on
schedule; residual users (including the operator's own global config) route
"measure goals" into a dead name; failure is silent because nothing counted
alias hits — the migration *cannot know* whether removal was safe, only that
it was scheduled. Same shape for retired triggers: a saved workflow invoking
the old phrase simply stops matching. Earliest signal: none, by construction —
that is the defect. With instrumentation: a nonzero alias-hit count in the
telemetry file.

**Falsifier.** An instrumented alias showing zero hits across N weeks of real
use before removal — then removal is evidence-backed and this blind spot is
closed for that rename.

**Smallest non-churning mitigation.** Three small things: (1) every alias and
retired-trigger shim appends one telemetry line per hit (the cc-hooks JSONL
pattern, already in-tree); (2) retirement flips on measured zero, not
calendar; (3) a one-shot pre-retirement use-scan: `cass` over recent sessions
plus a grep across operator config surfaces for the dying names, results
attached to the retirement tranche's evidence.

**Classification.** **Post-migration watch item**, with one cheap tranche
gate: no alias ships uninstrumented.

---

## BS5 — No safe-harbor design: the campaign assumes it finishes *(mandated: migration operations/rollback)*

**Why both wizards missed it.** The prompt asked for the strongest *complete*
plan, and both wizards optimized the end-state plus per-tranche *revert* — the
rollback question they answered is "how do we undo a bad tranche," never "what
if the campaign simply stops." Neither treated the operator's reality as an
engineering constraint: this is a solo operator with a day job; multi-month
campaigns here pause more often than they fail. (SOL's readiness-rail
incidentally makes one intermediate state safe — abandonment mid-population
leaves v2 authoritative — but that is one accidental harbor, not a designed
property of every boundary.)

**Affected surface.** Migration control: tranche closing criteria, the plan
document's own status field, the contract's interim placement note, every
"temporarily two things are true" window (vocabulary landed but compiler not;
model-dispatch moved but `shared` not yet retired; epoch 1 minted but old
gates still citing epoch-0 checks).

**Failure mechanism and earliest signal.** The campaign pauses after tranche
N. Months later a fresh session — or a fresh audit — reads `status: ready` on
the plan, a contract note still delegating placement to it, half-migrated
vocabulary in generated views, and resumes work against a frame that
misdescribes the tree. Work is duplicated, or worse, the paused campaign's
in-flight conventions are treated as landed doctrine. Earliest signal: any
post-pause session citing the overhaul plan as current intent (the #988
half-adoption at this duel's *entry* is precisely this failure mode from a
previous effort — the pattern has already happened once).

**Falsifier.** A pause drill: freeze work at an arbitrary tranche boundary,
hand the tree to a fresh context with no campaign memory, and have it
correctly state what is landed, what is in flight, and what is authoritative
— using only committed artifacts.

**Smallest non-churning mitigation.** Two lines per tranche, no new
machinery: (1) every tranche's acceptance ends with a *resting-state
assertion* — "if the campaign stops here, all gates are green, no document
delegates authority to unfinished work, and generated views describe only
landed state"; (2) the plan's frontmatter `status` is updated at every tranche
close (`in_progress/T<k>-complete`), so the document can never claim more than
the tree delivers. The T7 authority-expiry then becomes the final resting-
state assertion rather than a special case.

**Classification.** **Tranche blocker** as a closing invariant on every
tranche (cost: minutes each); the pause drill itself is a one-time T0-adjacent
check.

---

## The single unproven assumption the final plan still depends on

**That contract-conformant skill text produces contract-conformant live
behavior.** Every layer of the combined architecture — typed compiler,
authority verbs, three-polarity triggers, negative fixtures, epochs, ledgers —
proves properties of *text and scripts*. The consumer of that text is a
stochastic model, and the mapping from "the contract says stop" to "the agent
stops" is measured nowhere in either proposal, both steelmen, or the combined
ten decisions. The repository's own memory already recorded the counter-
example once ("doc instruction to use tool before grep is INERT"). Everything
else in this duel is conditionally sound *given* that mapping; BS3's routing
baseline is the smallest first instrument pointed at it, and until something
like it exists, the overhaul's deepest claim — that better contracts make
better agents — remains an article of faith the entire program is built on.

*End of blind-spot probe. FABLE.*
