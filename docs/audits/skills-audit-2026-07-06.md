# Skills Audit — full-corpus disposition pass (2026-07-06)

> **What this is:** the fresh per-skill disposition pass that `age-e3zk` ("execute the skill
> mass-retire? Needs a FRESH disposition pass first") names as its precondition, plus the
> idea-wizard-filtered recommendation set. Execution epic: `age-skills-audit-fable-*` (see §6).
> **Method:** deterministic sweep (sizes, 26-day usage, churn, ledgers, routers) → 7 parallel
> readers over all 66 skills with an evidence rubric (every claim carries a quoted fragment;
> each reader verified its own claims against live surfaces) → orchestrator synthesis →
> 30 recommendation candidates → adversarial winnow → **14 survivors, 16 rejected** (§5).
> **Prior art honored:** usage-evidence 2026-06-10; disposition-triage 2026-06-16 (keep 38 /
> update 27 / refactor 7, largely unexecuted); operator-leakage audit 2026-06-30 (clean,
> gate shipped); consolidation program in flight (169 → 77 → 66 on main → 62 on
> `feat/skills-consolidation-followups`, compile→curate merged there; landing = `age-p2c7`).

## 1. Deterministic findings

1. **Corpus:** 66 skills, 5.5MB; SKILL.md payload 39.7KB (evolve) → 1.0KB (red-team).
2. **Usage (26 days, this host):** live head ≈ 12 — discovery 24, agent-mail 20, rpi 18,
   crank 17, ntm 13, validate 11, research 8, council 8, using-atm 7, pre-mortem 6, plan 6,
   post-mortem 5. Everything else 0–2. (Undercounts Codex-side, bushido, read-not-invoked
   libraries — those are named per-skill below, not treated as dead.)
3. **Loop payload:** seven-move loop skills ≈ 182KB (~45k tokens/arc); top-4
   (plan/swarm/crank/rpi) ≈ 81KB with verbatim-duplicated lore. `implement` (8.3KB,
   references-led) is the model shape; `ntm` is the model for tool-wrapper shape.
4. **Always-on tax:** 11.3KB (~2.8k tokens) of `description:` lines injected into every
   session before any skill fires.
5. **Routing surfaces:** four overlap (docs/SKILLS.md 19.8KB; SKILL-TIERS.md decision tree;
   registry.json; domain map). SKILL-TIERS carries stale tokens: "Claude Opus 4.6",
   "GOALS.yaml", /compile prominent while experimental-demoted; count drift (says 63;
   dirs say 66; 06-30 audit said 77; branch says 62) — count is not gated.
6. **Tooling:** `ao skills retire` is a real 5-phase retire tool (validate → remove trees →
   flip ledger → regen → reference scan). Retirement is cheap; this audit supplies the data.

## 2. Master disposition table (66 skills)

Verdicts: **KEEP** (as-is / trivial nit) · **FIX** (surgical staleness) · **TRIM**
(progressive-disclosure or content excision) · **MERGE→x** · **RETIRE** (via
`ao skills retire`, gated on the `age-e3zk` decision) · **RESOLVE** (promote-or-retire
decision needed). Size = SKILL.md bytes; Use = 26-day invocations.

| Skill | Size | Use | Verdict | Why (one line) |
|---|---|---|---|---|
| discovery | 17.1K | 24 | KEEP | top-used spine entry; cross-link behavior-first-planning |
| agent-mail | 12.3K | 20 | KEEP | highest-used substrate skill; disciplined boundary |
| rpi | 18.1K | 18 | TRIM | demote framing to one-turn executor; dedupe pawl/replan prose vs crank+pawls.md |
| crank | 19.5K | 17 | FIX | retired PR-merge loop (`gh pr merge --squash --admin`) → push-to-main + pawl |
| ntm | 15.6K | 13 | KEEP | model progressive-disclosure; owns tending doctrine (see S11) |
| validate | 18.9K | 11 | FIX | strip `/rpi --auto`/PR-mode/dead judge-orchestration; align close path to pawl scripts; drop or finish its 6 "Replaces" claims |
| research | 18.1K | 8 | TRIM | workhorse; dedupe references vs shared/ twins |
| council | 2.8K | 8 | KEEP | pre-work decision forum the pawl does NOT replace; reconcile validate's replace-claim |
| using-atm | 27.3K | 7 | TRIM | 432 lines, 0 refs — 2nd-largest; extract meter-LIES/continuity/tmux to references/ |
| pre-mortem | 19.3K | 6 | FIX | "enforced by hook when /crank" — hookless 3.0; dedupe vs validate |
| plan | 23.2K | 6 | TRIM | largest loop skill; dedupe Gherkin contract (owner: behavior-first-planning) + wave lore |
| post-mortem | 17.1K | 5 | KEEP | move-7 anchor; on curate retire, absorb mining half (S3) |
| swarm | 21.1K | 1 | TRIM+FIX | dedupe wave-validity lore; excise "worker's PR is confirmed MERGED" reaping |
| implement | 8.3K | 0 | KEEP | model shape; Move-4 owner |
| test | 16.1K | 0 | KEEP | standalone value beyond implement (coverage/strategy/harnesses) |
| refactor | 13.8K | 0 | KEEP | fix `.agentscomplexity/` path typo |
| push | 4.9K | 0 | FIX | wire `ao gate check --fast --scope head` + pawl-land as THE ship path; kill "NEVER push to main" |
| scope | 6.3K | 0 | RESOLVE | enforcement premise inert: promised PreToolUse hook exists nowhere; `ao scope freeze` writes locks nothing reads |
| review | 15.3K | 0 | MERGE→validate | ~0 use; validate --mode=pr claims it; `bd list` stale |
| red-team | 1.0K | 0 | RETIRE | dead 1KB stub → mt-olympus; absorbed by validate --debate |
| converge | 3.4K | 0 | KEEP | clean memo over live `ao converge` |
| reality-check | 8.7K | 0 | RESOLVE | cleanest boundaries in cluster but user-invocable:false + 0 use — promote or fold into validate |
| pre-land-refuters | 16.0K | 0 | KEEP | THE live pawl-path memo; excise vestigial PR-binding §6 |
| security | 15.9K | 0 | KEEP | script-backed distinct domain |
| eval-outcomes | 2.4K | 0 | RETIRE | stub; validate --target=scenario + `ao eval scenario` cover it |
| curate | 12.7K | 0 | MERGE→post-mortem | declares itself superset of compile+flywheel; shim never ran — all three ship live ("worst outcome"); branch merged compile in |
| compile | 9.4K | 0 | RETIRE→`ao compile` | thin wrapper; SessionEnd hook dead; coordinate with p2c7 |
| flywheel | 12.7K | 0 | RETIRE→`ao flywheel status` | thin CLI wrapper, monitor not miner |
| operationalize | 14.1K | 0 | TRIM | distinct route-to-enforcement lane; cut demoted-flywheel prose |
| toil-mining | 9.7K | 0 | KEEP | cleanest of cluster; LAW-0 aware feeder |
| cass | 10.4K | 0 | KEEP | tight self-describing primitive |
| domain | 7.3K | 0 | FIX | bd→br in "How to use" |
| handoff | 12.0K | 0 | FIX | frontmatter `produces:` wrong; handoff/ vs handoffs/ path split |
| recover | 12.8K | 0* | MERGE→status | status+recover are one dashboard duplicated (keep recover's post-compaction trigger as a mode) |
| status | 12.0K | 0 | FIX+absorb | duplicate `/validate` QUICK-COMMANDS line (botched rename); becomes the one session surface |
| product | 17.4K | 0 | TRIM | collapse 9-framework name-drop table to a ref |
| goals | 16.8K | 0 | FIX | lead says "Maintain GOALS.yaml" — canonical is GOALS.md v4 |
| release | 11.3K | 0 | KEEP | well-factored references-led exemplar |
| doc | 9.8K | 0 | TRIM | default-mode steps are frontier-trivial; value is readme/oss refs |
| bootstrap | 10.1K | 0 | FIX | "Next: /rpi '...'" — command removed |
| scaffold | 12.9K | 0 | TRIM | ~85% generic templates; keep domain-slice binding |
| perf | 11.4K | 0 | RETIRE (or trim-to-nothing) | ~90% frontier-generic, ZERO repo bindings; jsm skills cover it |
| pr-prep | 7.9K | 0 | KEEP | scoped to external repos; unaffected by in-repo PR retirement |
| skill-builder | 13.9K | 0 | KEEP | core meta-skill; invokes converter+heal as real gates |
| heal-skill | 12.3K | 0 | KEEP | corpus-wide CI hygiene; not a builder mode |
| converter | 8.6K | 0 | KEEP | fix "vibe" comment; distinct format adapter |
| ms | 6.3K | 0 | KEEP | honest, current, documents its own footguns |
| standards | 8.0K | — | FIX | index routes to retired **vibe** (20×, dead paths); content cited 765 file-hits — gold content, stale wrapper |
| shared | 13.8K | — | FIX | fallback table says "install bd" — tracker is br |
| cc-hooks | 9.4K | 0 | TRIM | hookless-HONEST; dedupe doubled "Absorbed skills" block |
| dcg | 6.4K | 1 | KEEP | honest safety host-tool |
| sbh | 3.8K | 0 | KEEP | compact task-organized cheat-sheet |
| rch | 10.7K | 0 | KEEP | best-in-class wrapper ("Don't re-learn the command surface") |
| beads-br | 15.8K | — | KEEP | current post-gc carve-out |
| beads-bv | 5.7K | 0 | FIX | examples use stale `bd-123` ids + bare `br` without BEADS_DIR wrapper |
| account-rotation | 5.4K | 0 | FIX | false claim "there is intentionally no caam skill" |
| behavior-first-planning | 9.3K | 0 | KEEP | current; becomes the single OWNER of the Gherkin contract (S11) |
| reverse-engineer | 8.7K | 0 | KEEP | 47 refs = tested teardown ENGINE (depth-as-tooling, not sprawl) |
| evolve | 39.7K | 13* | TRIM hard | ~40% retired procedure (PR-merge, autodev-legacy, hooks/bash-gate); compounding framing outruns ADR-0004/0011; keep fitness ladder + pawl cadence |
| codex-exec | 20.0K | — | KEEP | load-bearing LAW-0 headless path; dense not bloated |
| using-gc | 7.8K | new | KEEP | fresh, clean four-jobs JIT split |
| gc-membrane | 8.2K | new | KEEP | current close-door reference |
| agy-native | 15.4K | 0 | KEEP | current (Gemini 3.5); nit: desc "scoped worktrees" vs body `--add-dir` |
| agent-native | 9.6K | 0 | FIX | falsely claims `ao agent bundle`/`ao mcp serve` "not yet in the live CLI" — both shipped; allowlist cleanup never ran |
| automation-shape-routing | 10.8K | 0 | KEEP | router distinct from builders; dedupe twice-stated spike numbers |
| workflow-builder | 5.2K | 1 | KEEP | thin, distinct Workflow scaffold |

\* evolve usage is 06-10 count; recover's 5 uses are 06-10 (0 recent). "—" = read-not-invoked by design.

**Tally:** KEEP 35 · FIX 14 · TRIM 9 · MERGE 3 · RETIRE 4 · RESOLVE 2. Executing every
RETIRE/MERGE lands the corpus at ~58–60 (converging with the branch's 62).

## 3. The five structural findings

1. **The validation cluster declared absorption and never executed it.** validate's Modes
   table claims to replace council, pre-mortem, red-team, eval-outcomes, review, and vibe —
   all still ship, several still call each other. Doubly-documented surface. Meanwhile the
   LIVE close gate is the pawl scripts; validate's Steps 3–7 judge-orchestration and
   council's spawn machinery describe the superseded in-skill path. Only pre-land-refuters
   fronts the real path.
2. **The flywheel-experimental family shipped the worst outcome.** curate declares itself
   the superset ("--mode=compile Replaces /compile"), the conversion never landed, so
   curate + compile + flywheel are all live. The in-flight branch already merged
   compile→curate; finish the collapse: post-mortem keeps the capture half, CLIs keep the
   mechanical half.
3. **Retired-surface debris is a CLASS, present in ~16 skills** — PR-flow machinery
   (crank, swarm, push, evolve, pre-land-refuters §6), dead hook mandates (pre-mortem,
   scope, compile, flywheel), bd→br (review, shared, domain, beads-bv), vibe→validate
   (standards ×20), /rpi (bootstrap, validate), GOALS.yaml lead (goals), plus point
   defects (status dup line, account-rotation caam claim, refactor path typo, agent-native
   false not-shipped caveat, converter vibe comment). One sweep, grep-manifest driven.
4. **Payload predicts obedience and cost together.** Inline-heavy skills carry the
   duplicated mandate lore (plan/swarm/crank/rpi ~81KB; using-atm 27KB/0 refs; evolve
   39.7KB); references-led skills (ntm, release, implement, rch) are both cheaper and
   cleaner. Inline-vs-references cleanly predicted reader dispositions across clusters.
5. **Good skills are unreachable while claimed-canonical skills go unused.**
   reality-check: cleanest boundaries in its cluster, user-invocable:false, 0 use.
   validate: claimed-canonical, 11 uses vs discovery's 24. The router (4 overlapping
   surfaces, stale entries) — not skill quality — is doing the selecting.

## 4. Idea-wizard gauntlet — 14 survivors (ranked)

| # | Recommendation | Routed to |
|---|---|---|
| S1 | **Debris sweep as a class** — one bead, grep-manifest over §3.3's list; surgical excisions only, no rewrites | new epic .2 |
| S2 | **Disposition-ledger flip** — record §2 verdicts in skill-dispositions.yaml rationale rows; this unblocks the e3zk decision | new epic .1 |
| S3 | **Retire/merge wave** — red-team, eval-outcomes, compile, flywheel, perf via `ao skills retire`; review→validate; curate→post-mortem (coordinate `age-p2c7` branch landing) | new bead, dep `age-e3zk` (Bo's call) |
| S4 | **Ship-path alignment** — push wires gate+pawl; validate close-path re-anchored on pawl scripts; kill "NEVER push to main" | new epic .3 |
| S5 | **evolve hard-trim** — excise ~40% retired procedure; soften compounding claims to ADR-0004/0011 posture | new epic .4 |
| S6 | **Payload program execution list** — using-atm extraction + plan/swarm/crank shared-lore dedupe into the refs they already cite | update `age-verification-economics-ebec.8` |
| S7 | **Skill-usage telemetry instrument** — transcript-counter script (the 06-10 method, automated); feeds dispositions + `age-e508` measured-tier | new epic .5 |
| S8 | **Router consolidation** — one curated router; SKILL-TIERS keeps taxonomy only; fix Opus-4.6/GOALS.yaml/compile-prominence; count gate vs dirs | new epic .6 |
| S9 | **Anti-regrowth criteria** — admission checklist gets: fresh-frontier test, repo-binding requirement, references-led shape, probe-or-demote; plus SKILL.md size warn-gate | feed `age-7d3r` |
| S10 | **Session-surface merge** — recover→status modes (keep post-compaction trigger); fix handoff frontmatter + path split | new epic .7 |
| S11 | **Single-owner doctrine moves** — swarm-tending → ntm (using-atm points); Gherkin contract → behavior-first-planning (plan/discovery cite) | new epic .8 |
| S12 | **Description diet** — normalize 66 `description:` fields (11.3KB always-on) to trigger-first ≤160 chars where routing allows | new epic .9 |
| S13 | **Generic-craft trims** — scaffold→domain-slice binding; doc→readme/oss modes; product framework-table→ref | new epic .10 |
| S14 | **Spine redefinition (decision)** — membrane spine list includes retire/resolve candidates (red-team, reality-check); re-anchor spine on the live path + pawl scripts; spine-integrity gate follows | new epic .11, decision-class |

## 5. Rejected by the gauntlet (kept honest)

1. "Add token-economy prose to skills" — measured inert in this repo; structural changes only.
2. Merge handoff→status/recover — write-side vs read-side, complementary (C-reader verified).
3. Retire beads-bv — contract drift is a FIX, not death.
4. Retire cc-hooks as doctrine contradiction — it is hookless-HONEST user-side teaching.
5. Merge automation-shape-routing + workflow-builder — front-door vs scaffold, clean split.
6. Merge using-gc + gc-membrane — fresh intentional operator/reference split.
7. Retire pr-prep because PR flow retired — its audience is external repos; explicitly scoped.
8. Ref-count ceiling gate — reverse-engineer's 47 refs are a tested engine; ref-count is not a defect signal (depth-as-tooling vs sprawl-as-debt).
9. Finish validate's absorb-everything design by prose — adoption already rejected it once; only merge with router + usage data (S3 does review/eval-outcomes only).
10. Retire ms/sbh/converter/standards/shared on zero usage — library/on-demand nature; usage metric doesn't apply (telemetry S7 will segment these).
11. Auto-generate the router purely from registry — loses curation; consolidation (S8) keeps one curated surface.
12. Retire operationalize+toil-mining with the flywheel family — they are the salvageable route-to-enforcement lane, distinct from demoted corpus-compounding.
13. Rewrite rpi as pure workflow — it is the one-turn executor per CLAUDE.md; TRIM framing, don't rebuild.
14. Immediate scope hook implementation — decision first (RESOLVE): agent-mail reservations may already own this lane.
15. Mass single-bead-per-skill fixes — 16 surgical fixes as N beads is tracker spam; one class-sweep bead (S1).
16. Spine expansion to include gc skills — premature; gc is opt-in coexisting substrate.

## 6. Bead routing

New epic `age-skills-audit-fable` (children .1–.11 per §4) + S3 as e3zk-gated bead +
notes appended to `age-verification-economics-ebec.8` (S6) and `age-7d3r` (S9) +
"fresh pass done" note on `age-e3zk`. Phase discipline: .1/.2/.3 ready now; S3 blocked
on Bo's e3zk decision; the rest P2/P3.

## 7. Decision asks (Bo)

1. **e3zk retire wave** (S3): execute the 4 retires + 2 merges? This audit is the fresh pass it demanded.
2. **Spine redefinition** (S14): re-anchor the 15-skill spine on the live pawl path?
3. **scope + reality-check resolves**: ship the missing enforcement / flip user-invocable, or retire each?
