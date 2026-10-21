---
status: historical
---

# AgentOps practice ranking, 2026-10 snapshot

Snapshot date 2026-10-21. Tree: `main` at a193b6bdf, after PRs #1217-#1221.
Every repository count in this file was measured on a193b6bdf unless the row
says otherwise. Counts drift; re-measure before acting on one. After this
snapshot, [#1222](https://github.com/boshu2/agentops/pull/1222) (b54da62a9)
added the `architecture`, `ergonomics` and `triage` skills and edited 19
others; none of that is scored here.

## 1. What this is

| Field | Value |
|---|---|
| Subject | AgentOps skills and operating loop, ranked against 2026 best practice for agentic software work |
| Reader | A local agent session or the maintainer, with this repository and no access to the session that produced it |
| Consumer decision | Which improvement to make next, or whether to refresh the verdict |
| Authority | Research evidence only. [AGENTS.md](../../../AGENTS.md), the ADRs and the gates outrank it; dated audits rank last in AGENTS.md "Source precedence" |
| Retirement | Superseded by a newer refresh, or when every open item in §5 and §6 is closed. Then repoint or remove this snapshot's entry in [.context/README.md](../../../.context/README.md). A repointing edit adds text to `.context/` and takes the fresh review in [Memory](../../../skills/memory/SKILL.md) rule 4; a pure removal does not |

## 2. Verdict

The pre-debate report scored AgentOps 4.3/10 (4.26 weighted after critics, 4.71
before), below a vanilla anchor defined as 5.0 (stock Claude Code or Codex with
a short CLAUDE.md). A 27-agent debate corrected this to **an unproven tie with
vanilla, roughly 4.6-4.9 on the same rubric**, for three reasons. First, the
critic pass was asymmetric: critics were told to be harder on the subject,
lowered AgentOps in 10 of 11 weighted dimensions, and left most thin-evidence
peer cells unchanged; before the critics AgentOps led spec-kit 4.71 to 4.58.
Second, the measured 8.5K-token always-on load (AGENTS.md through the
`CLAUDE.md` symlink, plus unscoped `.claude/rules`) is a cost to contributors
to this repository, not to plugin users: `plugin/` ships skills, agents, hooks
and workflows, no AGENTS.md or rules, so plugin users load about 2K tokens of
descriptions (PROXY). Third, no measurement the repository owns separates AgentOps from
vanilla in either direction. The prosecution points that survived re-check
moved individual cells down, not the headline: ADR-0017 and the CLI still
taught the retired every-change review; verdict identity is caller-attested;
`summary` went green on cancelled lanes; release published without a Validate
verdict; implement-wave ran parallel writers in one tree. The arbiter
prescribed about five small-batch repairs (they landed as 12 narrowed changes
in five PRs) and rejected new gates, classifiers, detectors and a paid eval
programme. Four of the five defects are fixed on main
(§4); caller-attested identity is still open (§5). Nobody has re-scored the
dimensions since the fixes.

**Evidence classes behind the verdict**

| Class | Instrument | n / method | Result |
|---|---|---|---|
| MEASURED, outcome | Coding pilot, [PR #1125](https://github.com/boshu2/agentops/pull/1125) | 24 starts, 8 comparable pairs, 6 task families | No paired endpoint difference (`PRODUCT.md`, "Evidence and claim limits") |
| MEASURED, outcome, legacy | Skills on/off and contract on/off sweeps, Codex (`evals/estate-ablation/`) | n=8 per arm, low effort; contract then 7,639 B | hidden_pass ±1/8 and 0 or −2/8; +8-12% tokens; the repo labels them legacy-unverified and non-promotable |
| MEASURED, cost | Always-on probe: `claude -p --tools Read`, Claude Code CLI 2.1.305, sentinel fixtures, 2026-10-21, **before #1219** | 1 run per arm | Real repo 11,371 vs empty repo 2,902 tokens (+8.5K); rules unscoped 6,097 vs with `paths:` 3,081 |
| PROXY | Plugin eval, rule-uptake criteria (2026-10-05 report under `docs/evals/`) | 29 cases × 3 runs; criteria written from each skill's own rules; same-family grader; empty working directory | 363/387 (94%) with plugin vs 272/387 (70%) without; case bootstrap +0.23 [0.14, 0.33] |
| PROXY | Blind routing | 48 trials: the 24 held-out requests whose skill exists in both versions × 2 runs (claude-exec, new in 3.10, excluded) | 29/48 (3.10) vs 11/48 (3.9.0); on 3.10, 9 of the 25 blind requests loaded 0/2 |
| PROXY | Placebo (never-loaded skill) | 16 cases | Per-case noise up to 0.33; 7 skills clearly above it |
| JUDGMENT | 12 dimension scores (§3) | 1 scorer + 1 adversarial critic per dimension | Same model family as the subject's authors |
| UNMEASURED | Outcome vs vanilla with the current loop in context; cost per accepted outcome; time to first value; loop adherence; stall frequency; change-fail and rework rates; always-on tokens after #1219 | none | none |

Same-family caveat: the scorers, critics, debaters, arbiter, implementers, fresh
readers and the repository's eval grader are all one model family. Their
agreement is correlated, not independent evidence.

## 3. Scorecard

Weights sum to 110. Scores are the pre-debate report's; the debate did not
re-score cells, and nobody re-scored after #1217-#1221. Practice IDs (VG-, E,
AN-, PF-) resolve in [practices.md](practices.md).

| Dimension | Wt | Scorer → critic | 2026 best-practice bar (short) | After #1217-#1221 (not re-scored) |
|---|---|---|---|---|
| Measured value | 15 | 5.0 → 4.5 | 20-50 real tasks, a no-harness arm, a noise floor, an other-family judge, cost per arm (VG-23, AN-22) | #1218 restores the Harbor 0.22.0 pin with a coupling test. No new measurement |
| Loop design | 12 | 4.5 → 4.2 | Short loop; skip the plan when the diff fits one sentence; capped review rounds; a stall signal (VG-07, AN-21, PF-13) | #1219 makes AGENTS.md:18 conditional; #1220 stall reports name the category and the observed fact (no numeric threshold, by decision); #1221 amends ADR-0017 |
| Verification | 12 | 5.5 → 5.0 | Checks decide done; tests protected from the author; classifier-set risk tier; approval pinned to a commit (AN-10, AN-20, AN-13) | #1217 `summary` fails on cancelled, timed-out, missing or full-run-skipped lanes; #1218 retires two cannot-fail gates. Identity still caller-attested |
| Context engineering | 12 | 3.5 → 3.0 | Always-on file holds only what the agent cannot infer; path-scoped rules; one owner per rule (VG-08, E12, PF-25) | #1219: AGENTS.md 15,143 → 14,206 B under a 14,336 B cap; `go.md` and `python.md` path-scoped; dead refs fixed. Nested cli/ files still not loaded. Tokens not re-probed |
| Skill quality | 12 | 5.5 → 5.0 | Measured against a no-skill baseline; trigger evals with negative controls; retire when the base model passes without it (VG-18, VG-17) | #1220 authority fixes (plan, using-gc, readme-craft, domain go.md); tie-aware routing grader; plan `metadata.triggers`. Blind loading not re-run |
| SDLC coverage | 10 | 4.5 → 4.0 | One artifact per stage; gates named by role; fail-closed backstops; every incident becomes an eval (AN-01, AN-06, AN-11) | #1217 publish waits on a green exact-SHA Validate run |
| Delivery | 8 | 5.0 → 4.5 | Small batches; CI with no admin bypass; rework rate as countermetric (DORA baseline, AN-15) | #1217 release gate; retag pushes main before deleting. Admin bypass unchanged |
| Governance & safety | 8 | 5.0 → 4.5 | Must-hold rules enforced outside the model, failing closed; boundaries drawn by access (AN-07, AN-19, VG-24) | #1218 `core.policy:self-waive` asks before agent writes to the waiver file. Admin bypass and fail-open hook unchanged |
| Adoption & cognitive load | 8 | 3.0 → 3.0 | One first-value path tested in CI; stable doctrine; short always-on file (AN-05) | #1221: `ao --help`, `ao quick-start`, `ao demo`, `ao robot-docs` and 9 entry docs teach checks + CI. Product-identity text and installed binaries unchanged |
| Multi-agent | 7 | 5.5 → 5.0 | Single agent by default; worktree isolation; compared with single-agent at equal compute (VG-04, AN-08, E18) | #1220 implement-wave lanes run sequentially; a dead lane is recorded, not dropped |
| Memory & learning | 6 | 4.5 → 4.0 | A rule only after a repeated mistake; lesson becomes a check first; prune per model (AN-05, PF-10, PF-09) | Not touched |
| Self-maintainability | 0 | 3.0 → 3.0 | Every component has a consumer and a retirement condition; no gate passes by construction (VG-20, VG-10) | #1218 Gate-Loosen-Reason survives on the squash commit; PR template asks for trailers; removed-verb hint test. #1219 byte cap replaces a line cap that was gamed |
| **Weighted** | 110 | **4.71 → 4.26** | | Debate-corrected range 4.6-4.9; not recomputed |

Peer weighted totals from the same rubric (thin evidence for spec-kit, BMAD and
Kiro): Anthropic playbook as written 6.57, Superpowers 5.29, vanilla 5.00, Kiro
4.89, spec-kit 4.53, BMAD 4.04.

## 4. What landed

| PR | Commit | What now works | Evidence (red on old code, green on new) | Fresh read |
|---|---|---|---|---|
| [#1217](https://github.com/boshu2/agentops/pull/1217) | a924dc363 | Validate `summary` fails unless every lane is success or skipped, and fails on any skip in a full run; `release.yml` publish needs a `validate-verdict` job (green exact-SHA Validate); `scripts/retag-release.sh` pushes main before it deletes the release or tag | `tests/scripts/validate-release-tag-full-ci.bats`, `tests/scripts/release-workflow.bats` ("publish waits on a green exact-SHA Validate verdict"), `tests/scripts/retag-release.bats` (rejected-push case), `tests/scripts/test-bats-path-filter-wiring.bats` | PASS |
| [#1218](https://github.com/boshu2/agentops/pull/1218) | 587ae44f0 | `always.mutation-route` and `always.file-manifest-overlap` retired with a Gate-Loosen-Reason trailer; constraints.enforce hint names no removed verb; PR template asks authors to repeat trailers; `core.policy:self-waive` policy in ask mode; Harbor pins coupled and excluded from Renovate | `cli/cmd/ao/removed_command_hint_test.go`, `cli/internal/gates/checks/negative_witness_test.go` stale-pin failure, `tests/scripts/policy-dispatch.bats`, `evals/skills-rpi/test_prepare.py` (pin disagreement) | DEFECTS: installer text overstated waiver coverage for a relocated `AGENTOPS_HOME`; repaired before merge |
| [#1219](https://github.com/boshu2/agentops/pull/1219) | 171baa2c0 | `scripts/validate-agents-split.sh` enforces a 14,336 B AGENTS.md budget beside the 250-line cap; runtime floor reads "the native agent runtime"; CM, ee, NTM dropped; `.claude/rules/go.md` scoped to `cli/**/*`, `python.md` to `**/*.py`, with live standards paths | `tests/scripts/validate-agents-split.bats` byte-budget case (red on the 15,143 B and 14,980 B files); `tests/scripts/agents-operating-contract.bats` rules case (dead paths, empty or non-matching globs) | PASS |
| [#1220](https://github.com/boshu2/agentops/pull/1220) | 933218a89 | plan writes the tracker only with authority, else returns an amendment; using-gc example sends `push=false open_pr=false`; readme-craft drops the mandatory interview and flags; domain go.md drops AgentOps-only rules; stall reports name category + observed fact; routing grader treats a forbidden id tied at rank 1 as a misroute; plan carries metadata triggers; implement-wave lanes run one at a time | `tests/scripts/check-routing-probe-goldens.bats` (tie cases), `tests/fixtures/implement-wave-workflow.mjs` (sequential, prompt-honesty, dead-lane) | PASS |
| [#1221](https://github.com/boshu2/agentops/pull/1221) | a193b6bdf | ADR-0017 carries the 2026-10-03 conditional-review amendment; `ao --help`, `ao quick-start`, `ao demo`, `ao robot-docs` state one rule (checks and CI gate an ordinary change; one fresh read only on named triggers); 9 entry docs and the workflow tombstones agree | Go tests in `cli/internal/commands/{quickstart,demo,robotdocs}/module_test.go` and `cli/cmd/ao/root_test.go` pin the new sentence and assert retired claims absent; tombstone import check; docs grep for 5 retired phrases | DEFECTS: `ao --help` still required a fresh read; fixed in `cli/cmd/ao/root.go` before merge |

The binding text after these PRs: [AGENTS.md](../../../AGENTS.md) "Native
execution and optional Lean RPI operating charter" and the
[ADR-0017 amendment](../../adr/ADR-0017-loop-as-control-flow-not-knowledge.md).

## 5. Open gaps on main

Re-verified on a193b6bdf on 2026-10-21. "Decision" means the debate chose not to
fix it; see [decisions.md](decisions.md).

| # | Gap | How re-verified | Evidence | Disposition |
|---|---|---|---|---|
| 1 | No outcome eval against vanilla with the current loop in context | No file under `.github/workflows/` references plugin-eval; no vanilla-arm result newer than the PR #1125 pilot | `PRODUCT.md` "Evidence and claim limits" | Decision: no eval build yet; owner headroom probe first (§6) |
| 2 | Fresh-read trigger is judged by the author; nothing records whether a read happened | Trigger list is prose only | AGENTS.md:170-175; ADR-0017 amendment | Decision: classifier rejected |
| 3 | Verdict identity is caller-attested, so "the agent cannot approve" is not enforced | Required caller flags still present | `cli/internal/commands/provenance/evidence.go:88-92`; `skills/validate/references/mechanics.md:59` | Owner (§6) |
| 4 | The 5 nested cli/ AGENTS.md files do not load in default Claude Code | `git ls-files`: 5 nested files, root `CLAUDE.md` symlink to AGENTS.md, no `cli/CLAUDE.md`. Load behavior from Claude Code docs and the 2.1.305 probe; not re-probed | `cli/AGENTS.md` | Owner: the Claude Code memory docs state the default `claude-md-or-agents-md` reads CLAUDE.md files only when one is present, and a per-user `claude-md-and-agents-md` setting loads both; the about $0.05 probe is optional before adding nested CLAUDE.md files |
| 5 | Admins bypass the required check | REST `branches/main`, 2026-10-21: required check `summary`, `enforcement_level: non_admins` | GitHub settings | Owner (§6) |
| 6 | No Validate run on push to main | validate.yml `on:` lists v* tags, workflow_dispatch, pull_request, merge_group | `.github/workflows/validate.yml`; `.github/workflows/nightly.yml` covers part | Decision: deferred |
| 7 | Product identity still sells always-fresh judgment | Text unchanged by #1221 | GOALS.md:5-18; PRODUCT.md:54-72; `.codex-plugin/plugin.json`:9,17-18; `.claude-plugin/marketplace.json`:8,14 | Owner |
| 8 | Installed binaries print the retired review rule | Latest release v3.10.0 (2026-10-05) predates #1221 | GitHub releases API | Owner: ship a release |
| 9 | The rpi behavior eval grades the retired rule | Criterion c3 still requires fresh sign-off | `evals/plugin-eval/behavior/rpi/case.yaml` | Owner, before the next paid run |
| 10 | Reviewer and eval grader default to the author's model family | Text unchanged | `skills/validate/SKILL.md:144`; `evals/plugin-eval/grade.py` | Open; cross-family is opt-in per [model-dispatch](../../../skills/agent-native/references/model-dispatch.md) |
| 11 | 21 grandfathered blocking script gates have no negative witness (unproven, not shown unable to fail) | 21 non-comment lines | `scripts/.gate-negative-witness-grandfather` | Open |
| 12 | No hook protects tests from the author; the policy hook fails open without jq or its registry | 6 policies, none on tests | `hooks/guards/policies/policies.json`; `hooks/guards/hooks/policy-dispatch.sh:48-49` | Decision: red-test-first check rejected; fail-open is documented design |
| 13 | Two removed-verb references remain in Go | grep | `cli/internal/quality/stale_refs.go:49`; `cli/internal/constraintindex/constraintindex.go:222` | Open follow-up from #1218 |
| 14 | 9 of 25 skills never load on blind requests | Not re-run after #1220 | Pre-PR blind routing (§2) | Owner: new blind cases written without sight of descriptions before any re-key |
| 15 | Retired wording outside #1220 and #1221 write scope | grep | `docs/UPGRADING.md:36`; `docs/contracts/bounded-contexts.yaml:14`; `skills/doc/references/de-slopify.md:7,10,138`; `oss-pack.md:1,167`; `readme.feature:6,20` | Open; fix in place, no gate ([decisions.md](decisions.md)) |

Dropped because they no longer hold on a193b6bdf: `summary` green on cancelled
lanes; publish without Validate; retag deleting before pushing; ADR-0017 and
CLI text requiring a fresh read on every change; parallel writers in
implement-wave; unscoped rules and dead python.md paths; "Default to native
Codex"; the AGENTS.md:18 presupposition; the two cannot-fail gates; Harbor pin
drift.

## 6. Owner actions remaining

| # | Action | Status on a193b6bdf |
|---|---|---|
| 1 | Read `branches/main/protection` for linear-history and required-review settings, then enable "Do not allow bypassing the above settings" or ruleset 11923893 with an explicit bypass list | Open; `non_admins` measured 2026-10-21. Prerequisites #1217 landed |
| 2 | Decide product-identity wording: GOALS.md:5-18, PRODUCT.md:54-72, `.codex-plugin/plugin.json` longDescription and default prompts, `.claude-plugin/marketplace.json` descriptions | Open |
| 3 | Ship a release so installed `ao` stops printing the retired rule | Open; latest v3.10.0 |
| 4 | Optional capped headroom probe before any outcome eval: at most $40; at most 8 calibrated post-2026-06-30 single-defect cli/ fix tasks with fix-pass, parent-fail and paraphrase-pass controls; stock Claude Code with a 30-line CLAUDE.md, k=2; freeze the selection rule in a bead first; proceed only if at least 5 of 8 show control below 0.75, and only through a bead naming the gated decision, an action for every branch including inconclusive, and a task count at or above `evals/_stats/power.py` n_required ([decisions.md §2.5](decisions.md#25-outcome-eval) is binding) | Open |
| 5 | Rewrite rpi eval criterion c3 before the next paid plugin-eval run | Open |
| 6 | Decide whether to derive author, validator and freshness identities at runtime for stored verdicts | Open |
| 7 | Decide whether the constraint floor (AGENTS.md:31 and :140-143) should count Accepted-status ADRs only | Open |
| 8 | If `harbor==0.22.0` does not install on CI's Python, declare a new evaluator cohort and bump all three pins together: `evals/skills-rpi/requirements.txt`, `prepare.py` `HARBOR_VERSION`, `evals/skills-rpi/README.md` | Not triggered: it installed during #1218. Revisit only if a later CI install fails |
| 9 | Optional paid reruns: premortem behavior case (about $0.39); new blind routing cases; nested-CLAUDE.md probe (about $0.05) | Optional |
| — | Approve the ADR-0017 amendment text; confirm the runtime-floor rewording | Done (merged in #1221 and #1219) |

## 7. Files in this directory

| File | Contents |
|---|---|
| [practices.md](practices.md) | The 125 sourced practices from 5 research lanes, with source, year, evidence strength and implication for this repo; peers, consensus and contested points |
| [skills.md](skills.md) | Per-skill scores, defects, top edits and overlaps for the 29 skills, and which edits #1220 applied or the debate rejected |
| [decisions.md](decisions.md) | The 12 debate decisions with rationale and acceptance, the report corrections, and what was rejected and why |
| [refresh.md](refresh.md) | How to re-verify this snapshot on a newer tree, refresh the verdict, and decide when to retire it |

## 8. Method and limits

| Stage | Agents | Output |
|---|---|---|
| Repo reading | 7 lanes (operating loop, evidence and measurement, machinery, 4 skill lanes) | Facts, strengths, weaknesses with path citations |
| Web research | 5 lanes: vendor guidance, empirical studies, SE practice, Anthropic AI-native SDLC, peer frameworks | 125 practices; 77 (62%) dated 2026; SE practice 4 of 24 (the rest are pre-2026 baselines) |
| Scoring | 12 scorers, each challenged by 1 adversarial critic that re-opened citations | §3 table |
| Completeness | 1 critic over the draft, then 4 follow-up lanes | 13 factual errors found and fixed before the final report |
| Debate | 27 agents: prosecution + defense of the verdict, advocate + skeptic for each of 12 recommendation clusters, 1 arbiter | Corrected verdict; 12 accept-modified decisions in 5 PR groups |
| Implementation | 5 lanes, each with one fresh read; 2 reads found defects, both repaired before merge | #1217-#1221 |

- Concurrency: at most 2 lanes at once in the ranking run. One run was stopped
  while active to add a lane and resumed; no lane timed out or retried. 38 of 38
  ranking results passed schema validation.
- Re-measured on a193b6bdf for this file: AGENTS.md 14,206 B and 233 lines
  (`wc -c AGENTS.md`); cap 14,336 B; both rules files carry `paths:`
  frontmatter; 29 skills, 4,502 SKILL.md lines and 4,463 description
  characters (sum over `skills/catalog.json`); 65 registered checks, 55
  blocking (`ao gate check --full --dry-run --json` from a binary built at
  a193b6bdf: 64 full-tier plus 1 fast-only).
- Token figures: probed where §2 says so. Elsewhere they are byte estimates at
  the measured 2.4-2.9 B per token. AGENTS.md at 14,206 B is about 4.9K-5.9K
  tokens by that estimate (PROXY).
- Not checked: linear-history and required-review settings on main; Codex
  handling of nested AGENTS.md; whether Claude Code injects skill frontmatter;
  CI wall-clock; the evidence behind each peer cell; full Git history (shallow
  clone, 4,230 reachable commits at ranking time); any dimension score after
  #1217-#1221; blind routing and always-on tokens after the PRs.
- Unverified external claims: OpenAI "Harness engineering" (primary page
  returned 403); attribution of just-in-time planning to a named Anthropic
  engineer; Faros percentages; a 40%→3% figure attributed to Hashimoto.
- Recency: pre-2026 sources stay only as baselines where no 2026 replacement
  exists (ImpossibleBench, DORA 2024/2025, Kim et al. 2025, the OpenAI agents
  guide 2025, Anthropic posts from 2024-25).

## 9. How a session reaches this snapshot

Pull-only: nothing loads this directory on every turn.

| Route | How |
|---|---|
| Caller names it | Primary route. Name this README in the prompt; [refresh.md](refresh.md) §1 has a paste-ready prompt |
| Context map | One entry in [.context/README.md](../../../.context/README.md). The AGENTS.md "Triggered sources" row "Prior context or memory" and the [Memory](../../../skills/memory/SKILL.md) recall step already lead there |

Whether a session that is not given the path follows that route is UNMEASURED.

Deliberately not integrated, as of a193b6bdf:

| Surface | Why not |
|---|---|
| [AGENTS.md](../../../AGENTS.md) and `CLAUDE.md` (a symlink to it) | Loaded on every contributor turn, with 130 B of headroom (14,206 of 14,336 B). Its "Prior context or memory" row already routes to the context map. E12, PF-08 and the contested "Instruction-file size" row in [practices.md](practices.md): extra context-file content raises cost, and file size showed no measured effect on adherence |
| `hooks/hooks.json`, `hooks/guards/` | A hook is always-on, the opposite of pull-only. `hooks/hooks.json` is byte-identical to `plugin/hooks/hooks.json`, so it ships to consumer repositories. [ADR-0002](../../adr/ADR-0002-agentops-3-hookless-cdlc-rearchitecture.md) (status Proposed, 2026-05-15) allows a hook only with no default prompt injection and evidence of positive value; the recorded hook A/B deltas in [the effectiveness evidence](../../evals/agentops-effectiveness-evidence.md) are 0 (one historical, one ceiling-saturated over 12 cases) |
| Repo-local `.claude/` (settings hook, rules, commands, skills) | `.claude/` is gitignored apart from two force-tracked rules. A rule scoped to `docs/research/**` loads only after the snapshot is open; an unscoped rule is always-on outside the AGENTS.md byte cap. `cli/internal/gates/routing.go` treats `.claude/skills/` as installed copies and drops it from every gate. The refresh.md §1 prompt already does a command's job |
| Shipped skills and agents (`skills/`, `agents/`) | They ship through `plugin/` to repositories that have no `docs/research/`. #1220 removed AgentOps-internal references from a shipped standard for this reason. Memory already reaches the context map in generic words |
| `workflows/` | Ships through `plugin/workflows/`. [refresh.md](refresh.md) §2 reuses `audit-dimensions` with arguments |
| `docs/SKILL-ROUTER.md`, `docs/documentation-index.md`, mkdocs nav | Generated from other owners, or `research/` is excluded on purpose in `mkdocs.yml`; a hand edit would be overwritten or contradict that choice |
| A `.context/maps/` page | It would duplicate §1. [ADR-0016](../../adr/ADR-0016-state-tiers.md) creates pages only for a concrete consumer, and one map entry gives the same discovery |
| A gate or guard policy for snapshot freshness or edits | The debate rejected new gates and detectors ([decisions.md](decisions.md)); no defect has been observed |
| BD or `bd remember` | BD owns work: the owner files §6 items natively. `bd prime` injects memories, which is not pull-only, and tracker data stays out of this public repository |
| `evals/` cases for the §2 UNMEASURED items | The paid outcome-eval programme was rejected; the capped headroom probe is an owner action in §6 |

The `status: historical` front matter on these files takes them out of the
live-doc scope of `scripts/check-docs-cli-snippets.sh` (through
`scripts/lib/docs-scope.sh`), so retiring an `ao` command later does not force
an edit here. `scripts/check-doc-skill-refs.sh` still scans them.
