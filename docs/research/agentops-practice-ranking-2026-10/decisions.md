---
status: historical
---

# Debate decision ledger, 2026-10 snapshot

Snapshot date 2026-10-21. Tree: `main` at a193b6bdf, after PRs #1217-#1221.
Companion to [README.md](README.md) (verdict, scorecard, open gaps) and
[skills.md](skills.md) (per-skill table).

**Answer.** The arbiter ruled **accept-modified** on all 12 recommendation
clusters. Each was cut down to the part with a verified defect and a cheap
deterministic check, and all 12 narrowed changes landed in five squash-merged
PRs. Rejected: new gates (banned-phrase, path classifier, removal and
bats-count ratchets, red-test-first), a numeric stall threshold and detector,
a blocking tightening ratchet, push-to-main Validate, worktree lanes, skill
folds and cuts, a 120-line or 8 KB AGENTS.md, reversing verdict precedence, a
cross-family reviewer default, and a paid outcome-eval programme. Do not
re-propose a rejected or deferred item without the reopen evidence named here
or evidence dated after 2026-10-21 that its rationale did not consider, and cite
that evidence when you do. Such evidence permits a proposal only; it does not
bypass a criterion named here, such as an owner-approved ADR-0017 amendment.

## 0. How to read this file

| Term | Meaning |
|---|---|
| accept-modified | The defect was kept and the proposed fix was replaced with a narrower one |
| Rejected | No observed defect, a conflict with an active constraint, or a failed anti-ceremony test |
| Deferred | Plausible, but blocked on named evidence |
| Reopen (arbiter) | Criterion stated by the debate arbiter |
| Reopen (implied) | The arbiter stated none. This is the minimum bar the cited constraint already sets, written for this file |

| Evidence class | What it covers here |
|---|---|
| MEASURED, this file | File contents, line numbers, byte and entry counts marked "as of a193b6bdf"; GitHub REST reads on 2026-10-21 (release list; `branches/main` required-check settings) |
| MEASURED, debate time | Numbers marked "(debate)": taken from the repository or GitHub during the debate, before #1217. Not re-measured |
| JUDGMENT | Rulings, and calls on what is a defect versus an optional note |
| UNMEASURED | Whether any landed change improves outcomes. No eval ran on these changes |

**Same-family caveat.** The advocates, skeptics, arbiter, implementers and
fresh readers all came from one model family, which is also the family that
wrote much of this repository. Their agreement is correlated, not independent
evidence (AGENTS.md, "Honest work and anti-ceremony").

**Constraint shorthand used in the "Why" columns**

| Short | Source on a193b6bdf |
|---|---|
| anti-ceremony | [AGENTS.md](../../../AGENTS.md):93-96: a new process artifact needs a named consumer, the decision it gates, an observed defect and a retirement condition |
| no weakened green | AGENTS.md:99-101: changes to tests, gates, goldens or pins must be justified against the original intent |
| conditional review | AGENTS.md:170-183 and the [ADR-0017](../../adr/ADR-0017-loop-as-control-flow-not-knowledge.md) "Conditional-review amendment — 2026-10-03 (#1189)" |
| product boundary | AGENTS.md:193-198: AgentOps owns no retry controller, queue, landing or release; consumer repositories keep their own PR, CI, merge and release policy |
| shipped-skill boundary | AGENTS.md:26: `skills/` is the shipped product, loaded directly by every runtime. Inferred by the debate from that line: it must not carry AgentOps-internal helpers or policy, and it must not authorize what AGENTS.md:84-85 withholds (publishing, external mutation) |
| pin | A test that asserts current text or counts. Changing one falls under "no weakened green" |
| concurrency | AGENTS.md:213-216: one writer by default; concurrent writers need disjoint scopes and separate isolation; shared paths serialize |

## 1. Ledger at a glance

PRs: [#1217](https://github.com/boshu2/agentops/pull/1217) a924dc363,
[#1218](https://github.com/boshu2/agentops/pull/1218) 587ae44f0,
[#1219](https://github.com/boshu2/agentops/pull/1219) 171baa2c0,
[#1220](https://github.com/boshu2/agentops/pull/1220) 933218a89,
[#1221](https://github.com/boshu2/agentops/pull/1221) a193b6bdf.

| # | Key | Report source | Landed | Main thing rejected | Fresh read of the PR |
|---|---|---|---|---|---|
| 1 | rules-scoping | Rec #1 | #1219 | Requiring every rule to be path-scoped | PASS |
| 2 | entry-docs | Rec #2 | #1221 | Banned-phrase gate | DEFECTS (`ao --help`), repaired before merge |
| 3 | stuck-checkable | Rec #3 | #1220 | Numeric stall threshold, `repeats:` field, detector | PASS |
| 4 | ci-gate | Rec #4 + release item | #1217 | push:main Validate; retag pre-check | PASS |
| 5 | outcome-eval | Rec #5 | #1218 | Building the eval; paid CI trigger | DEFECTS (installer text, see #12), repaired |
| 6 | agents-md-cut | Rec #6 | #1219 | 120-line gate; 8 KB cut | PASS |
| 7 | review-classifier | Rec #7 | #1218 | Classifier, CODEOWNERS tiers, ratchets, red-test-first, blocking flip | as #5 |
| 8 | skills-revisions | §5 per-skill table | #1220 | Implement trigger deletion; security re-key | PASS |
| 9 | skills-consolidation | Next table + §5 overlaps | #1220 | Navigate fold; implement and review cuts | PASS |
| 10 | cannot-fail-gates | Next table | #1218 | Reading the 21 grandfathered gates as cannot-fail | as #5 |
| 11 | implement-wave-isolation | Next table | #1220 | Worktree lanes plus scope diff (deferred) | PASS |
| 12 | validate-and-defaults | Next table + §5 rows | #1218 (and #1220 for plan, using-gc) | Precedence reversal; cross-family default; fail-closed hook | as #5 |

"Fresh read" is the PR group's single author-distinct read. #1218 covered
rows #5, #7, #10 and #12 together.

## 2. Decisions

### 2.1 rules-scoping

| Field | Value |
|---|---|
| Recommendation | Add `paths:` frontmatter to `.claude/rules/go.md` and `python.md`, repoint python.md's dead references, make the AGENTS.md runtime line runtime-neutral, and update the corpus-delta harness and its bats |
| Ruling | accept-modified. Verified defects: python.md cited a nonexistent `skills/standards/` and an untracked `pyproject.toml`; neither rule file had `paths:`; "Default to native Codex" contradicted "other runtimes only on explicit request" on a Claude Code host |
| Landed, #1219 | `go.md` scoped to `cli/**/*` (mirrors seed.go `goPaths`); `python.md` scoped to `**/*.py` and pointed at `skills/domain/references/standards/{python,test-pyramid}.md`, false pyproject line removed; comment-only fix in `scripts/corpus-delta-harness.sh`; new test "every .claude/rules file cites live paths and declares live path globs" (`tests/scripts/agents-operating-contract.bats:53`); AGENTS.md:109-110 now reads "Default to the native agent runtime and the local shell; other runtimes only on explicit request." The owner confirmed the wording on merge |

| Rejected | Why | Constraint |
|---|---|---|
| Report wording "Use the host runtime that loaded this file" | Would coin new doctrine; the detail owner already says "Use the native agent" ([agent-workflow-reference](../../agent-workflow-reference.md):3-5) | Source precedence: reuse the owner's phrase |
| Requiring every rule file to be path-scoped | No defect behind it. The test enforces live paths and matching globs only | anti-ceremony |
| `globstar` or Python in the new test | Portability; git pathspec matching over tracked files is used instead | — |
| Editing `tests/scripts/corpus-delta-harness.bats` | Only the harness comment was false; the 19 cases stay | pin |

Deferred: none. UNMEASURED: always-on tokens after scoping (not re-probed).

### 2.2 entry-docs

| Field | Value |
|---|---|
| Recommendation | Rewrite the entry docs to match the conditional-review rule; build a banned-phrase gate from a classified grep of about 35 files, registered in seed.go with a negative witness; replace the 16 phrase pins with structural checks |
| Ruling | accept-modified. The drift was worse than the report said: ADR-0017 (Accepted) and Go-pinned CLI text still required a fresh read on every change, and no entry surface linked the workflow reference. The root cause was a sweep that skipped its owed read (62834b1). A gate does not fix that |
| Landed, #1221 | ADR-0017 "Conditional-review amendment — 2026-10-03 (#1189)" (owner-approved) supersedes "A fresh author-distinct reviewer judges the final change" and the final-validation/empty-`not_checked` clause. `ao --help` (`cli/cmd/ao/root.go`), `ao quick-start`, `ao demo` and `ao robot-docs` state one rule: checks and CI gate an ordinary change, and one fresh read happens only on the named triggers. Package tests pin the sentence and assert that "convergence law" and `repair_rounds` are gone. Nine entry and operational docs were rewritten: eight link [agent-workflow-reference](../../agent-workflow-reference.md) (as of a193b6bdf) and GLOSSARY drops the convergence law; tombstone messages in `workflows/operating-loop.js` and `workflows/bdd-foundry.js` were updated |
| Fresh-read defect | `ao --help` still said a fresh read was required. Fixed in `cli/cmd/ao/root.go` before merge |

| Rejected | Why | Constraint |
|---|---|---|
| Banned-phrase gate | Its patterns copy today's sentences, so it flags correct text (`cli/internal/commands/provenance/judgments.go`, `skills/validate/SKILL.md`, AGENTS.md), misses paraphrases, and has no enforced retirement condition | anti-ceremony |
| Replacing the AGENTS.md phrase pins with structural checks | Out of scope; no AGENTS.md or `agents-operating-contract.bats` change in this PR | pin |
| Editing [ADR-0008](../../adr/ADR-0008-evolve-intelligent-agile-operating-model.md) | It is scoped to a selected outer goal and does not contradict the rule | Constraint floor |
| Editing product-identity text | An owner decision (§4) | — |

| Deferred or open | Reopen or close when |
|---|---|
| Banned-phrase gate | Reopen (implied): retired-rule text comes back after a sweep whose owed fresh read was done, so the read demonstrably fails to catch it |
| Leftover retired wording outside #1221's scope, as of a193b6bdf | `docs/UPGRADING.md:36` (convergence law, `repair_rounds`); `docs/contracts/bounded-contexts.yaml:14` ("one fresh independent verdict"). Fix in place; no gate |

### 2.3 stuck-checkable

| Field | Value |
|---|---|
| Recommendation | Define "stuck" by numbers: the same check fails 3 times with no diff change, 2 corrections on one issue, a review round with no new defect, or no tool call within the caller's window. Add `last_progress` and `repeats` to status blocks; optionally add a read-only detector |
| Ruling | accept-modified; the skeptic prevailed. The advocate's ADR-0017 citations sit under "Historical context" or follow from the superseded convergence law. Keep the useful part: report the observed fact behind a stall claim |
| Landed, #1220 | `last_activity:` in the per-packet result block (`skills/agent-native/SKILL.md:140`); `last activity <time, kind> \| unobserved` on the orchestrate assignments line (`skills/orchestrate/SKILL.md:163`); rpi Result form `stopped: stalled: <category> - <observed fact>` (`skills/rpi/SKILL.md:140`); one sentence in [rpi-traversal](../../architecture/rpi-traversal.md):142-144: "no count, ledger or detector defines a stall" |

| Rejected | Why | Constraint |
|---|---|---|
| Fixed numeric threshold | Brings back a default numeric stop. The ADR-0017 "Lean-harness amendment — 2026-09-09" superseded "a default two-round native stop". It also contradicts `skills/rpi/SKILL.md:109-110` ("retry count alone is not a spent budget") | ADR-0017 |
| `repeats:` field, read-only detector | No consumer beyond the proposal itself | anti-ceremony |
| AGENTS.md edit | Not needed, and it would spend AGENTS.md headroom | AGENTS.md byte cap (§2.6) |
| Report evidence "1-3 retries per slice" | Those retries came from dispatching a nonexistent subagent type in the since-retired operating-loop | — |

| Deferred | Reopen when |
|---|---|
| Numeric threshold | Reopen (arbiter): only with an owner-approved ADR-0017 amendment. Reopen (implied): observed stalls that a fact-naming report failed to surface |
| `last_activity` field (retirement) | Drop it when the runtime's own status shows per-worker activity |

Unchanged on purpose: "Capture before restart" (`skills/agent-native/SKILL.md:47-50`); no restart permission was added.

### 2.4 ci-gate

| Field | Value |
|---|---|
| Recommendation | Make `summary` fail on cancelled and required-skipped lanes; in retag, push main and confirm green before deleting; add `push: branches: [main]` as a post-landing check (reversing 3d4fc1299); make release wait on Validate; the owner enables admin enforcement |
| Ruling | accept-modified. Verified: `summary` failed only on `failure`, so cancelled lanes passed (debate: three production cases); publish needed no Validate verdict and v3.1.0 and v3.2.0 shipped from red Validate SHAs; retag deleted the release and remote tag before pushing main |
| Landed, #1217 | `summary` reads `NEEDS_JSON`, exits 1 on any lane that is not success or skipped, and on any skip in a full run (`.github/workflows/validate.yml`, "Check results"); `workflow_dispatch` forces a full run; the bats path filter adds `workflows/**` and `tests/fixtures/**`; the stale skill-gates comment is gone. `release.yml` job `validate-verdict` runs `scripts/verify-release-ci.sh` and publish needs it. `scripts/retag-release.sh` pushes main (:96-97) before it deletes the release (:127) or remote tag (:130). 12 bats cases, each red on the old code |
| Residual (author-disclosed) | `scripts/verify-release-ci.sh` takes the newest Validate run for the SHA from any event. A path-filtered PR run could be picked only if the tagged commit is itself a PR head (non-squash landing) |

| Rejected or deferred | Why | Reopen (arbiter) |
|---|---|---|
| `push: branches: [main]` or a scheduled full run | 30 of 30 sampled squash commits had trees identical to a green PR head (debate), so "unvalidated landed SHAs" is a squash artifact. 3d4fc1299 also cited serialization and the 20-slot concurrency pool. One direct push lacked Validate since 2026-09-23 (debate), and admin enforcement covers that | A landed tree differs from a validated PR head, or direct pushes without a dispatch recur more than monthly |
| Validate pre-check inside retag | Its only consumer was code added in the same proposal | A retag ends with the release deleted and nothing republished |
| Idempotency guard on the remote tag | Not needed for the observed failure | None stated |

As of a193b6bdf, `validate.yml` `on:` lists v* tags, `workflow_dispatch`,
`pull_request` and `merge_group`, with no branch push.

### 2.5 outcome-eval

| Field | Value |
|---|---|
| Recommendation | 20-30 outcome tasks plus 12-20 loop-adherence tasks; stock Claude Code with a 30-line CLAUDE.md versus plugin plus contract; k≥5; cross-family judge; CI trigger with a cost cap; 3-5 days plus $300-600 |
| Ruling | accept-modified: build nothing now; fix the one cheap defect, Harbor pin drift (`prepare.py` 0.22.0 vs `requirements.txt` 0.24.0, from Renovate automerge 6e63d64ca and 1591bb843) |
| Landed, #1218 | `evals/skills-rpi/requirements.txt` back to `harbor==0.22.0` (it installed on Python 3.14); `test_requirements_readme_and_prepare_name_one_harbor_version` in `evals/skills-rpi/test_prepare.py`; a `renovate.json` packageRule that disables Harbor bumps for that file; taskbank README wording fixed |

| Rejected | Why | Constraint |
|---|---|---|
| Building the eval | 15-30 tasks cannot resolve the design's own 0.15 margin (`evals/_stats/power.py` n_required about 35-49, debate), so the likely result is "inconclusive" with no action attached. The arms would measure a setup nobody runs (Harbor installs skills only, the contract at user scope, a cli/-only tree); the loop suite's Skill graders are not scored under ablation; the repository's prior results are null | anti-ceremony; conditional review ("report NOT_PROVEN instead of chasing it") |
| Paid CI trigger | No gated decision; recurring cost | anti-ceremony |
| A proposed calibration edit | It would break the CI calibration step, which runs with `AO_RUNTIME_ROOT` and `AO_SKILL_BUILDER_BIN` unset | pin |

| Deferred | Reopen (arbiter) |
|---|---|
| Any outcome-eval build | Only after the owner-run capped headroom probe (§4): at most $40; at most 8 calibrated post-2026-06-30 single-defect cli/ fix tasks, each with fix-pass, parent-fail and paraphrase-pass controls; k=2; the selection rule frozen in a bead first. Proceed only if at least 5 of 8 show control below 0.75, and then only through a bead that names the gated decision, an action for every branch including inconclusive, and a task count at or above `power.py` n_required |

### 2.6 agents-md-cut

| Field | Value |
|---|---|
| Recommendation | Cut AGENTS.md to at most 120 lines and 8 KB; move BD anchors, the federated table, memory and factory doors to references; define or drop acronyms; switch to a byte budget; give cli/ a CLAUDE.md |
| Ruling | accept-modified. Real defects: the :18 leftover presupposition, the runtime line, undefined CM/ee/NTM, duplicated factory-door and memory text, and a line gate passed by joining lines (118f78746: 256 to 248 lines at the same byte count). A cut regrows (debate: 8,451 B on 2026-08-16, 14,065 B on 2026-09-07), so the durable fix is a byte cap |
| Landed, #1219 | In-place edits: AGENTS.md:18 "When a fresh read is owed, assign it once"; runtime floor (§2.1); CM, ee, NTM dropped; CASS and CDLC (Context Delivery Lifecycle) spelled out; factory doors cut to one sentence citing `skills/using-gc/SKILL.md`; memory paragraph replaced by a Triggered-sources row. `scripts/validate-agents-split.sh:30` sets `BYTE_LIMIT=14336` beside `LIMIT=250`, with a negative witness at `tests/scripts/validate-agents-split.bats:104`. `cli/AGENTS.md` says "validation" instead of "fresh-judgment". AGENTS.md went from 15,143 B / 248 lines to 14,206 B / 233 lines (as of a193b6bdf, `wc`) |

| Rejected | Why | Constraint |
|---|---|---|
| 120-line gate | A line target can be gamed by joining lines (118f78746) | anti-ceremony (the measure is not the property) |
| The 8 KB cut as drafted (7,566 B) | It dropped rules that no other file states ("other runtimes only on explicit request"; "A synthesis frozen without an active constraint is invalid") and the CDLC scope that [ADR-0016](../../adr/ADR-0016-state-tiers.md) defines (:70-71, :84). The pins cover only about 21 phrases, so they would not have caught this | ADR-0016; Constraint floor |

| Deferred | Reopen (arbiter) |
|---|---|
| Deep cut toward 8 KB | After the phrase pins are replaced and a fresh reader holds a sentence-level removal map (each removed sentence mapped to an owner path:line, or restored) |
| Nested `cli/` CLAUDE.md symlinks | After a `claude -p` probe (about $0.05) shows they load and help. As of a193b6bdf, 5 nested AGENTS.md files and no `cli/CLAUDE.md` |
| Constraint floor counts Accepted ADRs only (AGENTS.md:31, :140-143) | Owner decision |
| Byte cap (retirement) | Retire when no supported runtime loads AGENTS.md always-on, or when a total always-on check that includes `.claude/rules` replaces it |

Standing rule from the debate: every later AGENTS.md change must fit under
14,336 B, offsetting any growth with cuts. Disclosed limit:
`scripts/validate-agents-split.sh` is outside the tightening ratchet's
governed paths (`cli/internal/gates/**`, `scripts/check-*.sh`), so raising
`BYTE_LIMIT` triggers no ratchet warning.

### 2.7 review-classifier

| Field | Value |
|---|---|
| Recommendation | A changed-path classifier that prints "fresh read owed: yes/no + reason" with fixed always-review paths, cited in closeout; CODEOWNERS tiers; a per-fix-range red-test-first check; `gate.tightening-ratchet` made blocking for gate paths |
| Ruling | accept-modified: reject all four parts. One small defect survives: Gate-Loosen-Reason trailers never reach main, because squash merges use the PR body. All 5 gate deletions since 2026-08-28 carried trailers on PR branch commits ([#1168](https://github.com/boshu2/agentops/pull/1168), [#1100](https://github.com/boshu2/agentops/pull/1100), [#1179](https://github.com/boshu2/agentops/pull/1179)) |
| Landed, #1218 | `.github/PULL_REQUEST_TEMPLATE.md:12` asks authors to repeat `Gate-Loosen-Reason:` / `Test-Removal-Reason:` trailers in the description; the `gate.tightening-ratchet` RepairHint says the same (`cli/internal/gates/checks/seed.go:475`). It stays `Blocking: false` |

| Rejected | Why | Constraint |
|---|---|---|
| Path classifier | It would print "no" for its own motivating direct push (a7732fd36) and flag about 67% of commits (debate). It would freeze a review doctrine that had just changed into Go, a second owner for the rule. In consumer repositories it would print AgentOps' review policy. A closeout citation would make `ao` a session ritual (AGENTS.md:112) | anti-ceremony; product boundary; conditional review |
| Removal ratchet, bats-count ratchet | The observed defect was refuted: the deletions were trailered on branch commits, and only the squash merge dropped them | anti-ceremony |
| Blocking flip | The seed.go comment (:464-474) says "ADVISORY PERMANENTLY at this fidelity", pinned by `TestGateTighteningRatchetIsAdvisoryAndRoutedOnTheGateSurface` (`seed_test.go:259`); the heuristic false-positived 2 of 2 times in replay (debate) | pin; no weakened green |
| CODEOWNERS tiers | `.github/CODEOWNERS` is `* @boshu2`: there is no second human to route to | anti-ceremony |
| Red-test-first check | Not checkable under squash merges or single-commit PRs; TDD inside the agent loop is contested ([practices.md](practices.md), "Contested points") | anti-ceremony |

| Deferred | Reopen or retire when |
|---|---|
| Template line and hint (retirement) | Remove both when the repository's squash-merge message is no longer the PR body |
| Classifier | Reopen (implied): an observed costly change that shipped without its owed fresh read, plus a guard so the check never runs in consumer repositories |
| Blocking flip | Reopen (implied): a precision detector that replaces the text-diff heuristic, plus an observed miss that the advisory gate let through |
| CODEOWNERS tiers | Reopen (implied): a second human maintainer exists |

### 2.8 skills-revisions

| Field | Value |
|---|---|
| Recommendation | Report §5: one top edit for each of 20 skills marked "revise", plus cross-skill items (charter copied into 6+ files, validators pinning prose) |
| Ruling | accept-modified; the skeptic's batch. Verified defects: `bulk-reader.toml` split a sentence with an inserted block; the only using-gc dispatch template published (`push=true open_pr=true`), against its own body and AGENTS.md:84-85; readme-craft mandated an interview and an undefined `--mode`, and ran the doc skill's validator as README evidence; plan wrote the tracker unconditionally, against its own `output_contract`; domain `go.md` told consumer repositories to call AgentOps-internal helpers |
| Landed, #1220 | `skills/agent-native/agents/bulk-reader.toml` sentence rejoined; `skills/using-gc/SKILL.md:159` sends `push=false open_pr=false`, and :162 makes publishing conditional on authorization; `skills/doc/references/readme-craft.md` uses three request shapes and asks once only when ambiguous; `skills/plan/SKILL.md:55-57` writes only with authority, otherwise returns a proposed amendment; `skills/domain/references/standards/go.md` keeps three rules and drops the internal identifiers |

| Rejected | Why | Constraint |
|---|---|---|
| Deleting implement's triggers (`skills/implement/SKILL.md:26`) | Turns the CI-run honest pin red ("the repository's committed goldens grade with exactly the known finding", `tests/scripts/check-routing-probe-goldens.bats:306`, 13/13); getting green back would mean editing a pin. It also cuts CLI discoverability for no measured gain | pin; no weakened green |
| Re-keying the security description on "exploitable" | Security already loaded 3/3 on its behavior prompt and its c5 failure came after loading; using words from the blind prompt would contaminate the held-out set | pin (tuned descriptions); held-out integrity |
| Orchestrate pointer replacing the restated charter | Optional note, not a defect: the copy agrees with validate and AGENTS.md today | conditional review (defect definition, AGENTS.md:177-179) |
| The report's other per-skill top edits (token-economy cuts, rpi rewrite, memory move, description re-keys) | Not argued as defects. The advocate deferred them as having no measured benefit, conflicting with active constraints (rpi, memory) or contaminating held-out routing. Validate's precedence change is ruled under §2.12 | anti-ceremony |

| Deferred | Reopen (arbiter) |
|---|---|
| Premortem Check 1 rescope | After a paid rerun of `evals/plugin-eval/behavior/premortem` (about $0.39) shows criterion c1 still holds |
| Description re-keys for blind loading | After new blind routing cases exist, written without sight of the descriptions |

Open on a193b6bdf (same defect class, outside #1220's write scope):
`skills/doc/references/de-slopify.md:7,10,138`, `skills/doc/references/oss-pack.md:167`
and `skills/doc/references/readme.feature:6,20` still mention `--mode=readme` or an
interview. Fix in place.

### 2.9 skills-consolidation

| Field | Value |
|---|---|
| Recommendation | Merge navigate into orchestrate; cut implement and review if they still load 0/5; make validate the single owner across validate, review and reality-check; fix the plan/review name-order tie; route the idea-genie duel to council; measure loading with host built-ins present |
| Ruling | accept-modified: only the routing-tie fix. Query rq-09 passed on a name-order tie (plan = review = 0.3333). [ADR-0018](../../adr/ADR-0018-retire-goals-shared-scope.md):46 requires plan to carry scope's former triggers, and only one survived, in prose that `ao skills find` does not phrase-score |
| Landed, #1220 | `skills/plan/SKILL.md:23` `triggers: ["write scope", "scope boundaries"]`, with the description line byte-identical; `scripts/check-routing-probe-goldens.sh:313` reports a forbidden id tied with rank 1 as a MISROUTE (window top_k+1; unscored packs grade rank 1 only); tie witnesses in the bats (:168, :180, :193); rq-09 notes corrected. Plan now scores about 0.67 against review's 0.33 |

| Rejected | Why | Constraint |
|---|---|---|
| Navigate fold into orchestrate | A simulated fold created new name-order ties and misroutes ("is this epic done" became a 3-way tie; "pick the next wave" went to implement). It breaks frozen goal prompts that call `navigate` by name (`skills/craft-goal/references/goal-prompt.md:26`), would be the third home for that content in about 3 weeks, and skips the `REMOVED_SKILLS` tombstone (`scripts/check-cathedral-cut-conformance.py:54`, gate `contract.cathedral-cut`) | pin; shipped-skill boundary (users and goal prompts call it by name) |
| Cutting implement and review | `scripts/check-skill-mesh.py:20` CORE includes implement; `cli/internal/gcmaintainer/gcmaintainer.go:34` requires implement; the probe-coverage denominator is pinned | pin |
| Validate as single owner; idea-genie duel to council | No observed defect | anti-ceremony |
| Measuring loading with host built-ins present | Not ruled on; covered by the owner's optional blind-case rerun (§4) | — |

| Deferred | Reopen (arbiter): all of these are required |
|---|---|
| Navigate fold | A non-ceiling eval of the multi-wave loop showing no gain, with its uncertainty; a live routing re-measure; strict-margin goldens for "is this epic done" and "pick the next wave on this epic"; a `REMOVED_SKILLS` entry; and a MIGRATION deprecation one release before deletion |

As of a193b6bdf: `skills/catalog.json` `skill_count` 29; 13 routing goldens.

### 2.10 cannot-fail-gates

| Field | Value |
|---|---|
| Recommendation | Remove or fix blocking gates that pass by construction, and fix the `constraints.enforce` hint that names a removed command |
| Ruling | accept-modified. Exactly two gates could not fail as registered: `always.mutation-route` guarded `cli/internal/daemon`, deleted in 010ade393 and kept deleted by [ADR-0009](../../adr/ADR-0009-daemon-deletion-in-session-only.md); `always.file-manifest-overlap` had no Args or Stdin, so it printed SKIP and exited 0 |
| Landed, #1218 | Both entries removed from `seed.go`; their scripts and the orphaned `parallel-execution-smoke.sh` fixture deleted; both grandfather lists pruned; test IDs swapped; the `constraints.enforce` hint (`cli/internal/gates/checks/constraints.go:50`) names both index files and the active-wins rule; `TestGateRepairHintsNameNoRemovedVerbs` (`cli/cmd/ao/removed_command_hint_test.go:202`) fails on any repair hint that names a removed verb. Gate-Loosen-Reason trailer on the commit and in the PR body |

| Rejected | Why | Constraint |
|---|---|---|
| The advocate's hint wording ("the index that defines it ... or ...") | `mergeConstraintIndexes` is active-wins, so that hint would itself mislead | — |
| Keeping the scripts unregistered as a fallback | No consumer | anti-ceremony |
| Treating the other grandfathered gates as unable to fail | They are unproven, not shown unable to fail; out of scope | — |

| Open or deferred | Reopen when |
|---|---|
| 21 grandfathered blocking script gates without a negative witness (as of a193b6bdf, `scripts/.gate-negative-witness-grandfather`) | Reopen (implied): a specific gate is shown unable to fail as registered. Then retire it with a Gate-Loosen-Reason, as #1218 did |
| Removed-verb strings left in Go, as of a193b6bdf | `cli/internal/quality/stale_refs.go:49`, `cli/internal/constraintindex/constraintindex.go:222`. A separate follow-up named in #1218 |

Gate count as of a193b6bdf: 65 registered, 55 blocking (measured with
`ao gate check --full --dry-run --json`: 64 full-tier plus 1 fast-only). The
debate counted 66 and 56; removing two gives 64 and 54. The one-gate
difference was not traced.

### 2.11 implement-wave-isolation

| Field | Value |
|---|---|
| Recommendation | Give each implement-wave lane a worktree (`isolation: 'worktree'`) and run a post-run scope diff; add tests |
| Ruling | accept-modified: serialize instead. The workflow ran parallel writers in one tree, against the concurrency rule (AGENTS.md:213-216), whose own remedy is "shared paths serialize". `code-write` already did this (a3d6018f2) |
| Landed, #1220 | `workflows/implement-wave.js:90` runs a sequential for-loop with a try/catch per lane; the lane prompt (:93) says earlier lanes' uncommitted changes are "not yours to revert, reset or stash"; the verifier is told the run order (:149); the README and meta drop "strictly disjoint". New `tests/fixtures/implement-wave-workflow.mjs` and `tests/scripts/implement-wave-workflow.bats` (sequential, dead-lane, prompt-honesty, invalid-args). The CHANGELOG notes that wall-clock time is now the sum of lane times |

| Rejected or deferred | Why | Reopen (arbiter) |
|---|---|---|
| Worktree isolation plus a post-run scope diff | The design's precondition (a clean tree) cannot hold for a second wave that leaves changes uncommitted; shared regen outputs (`skills/catalog.json`, `docs/SKILL-ROUTER.md`, `cli/docs/COMMANDS.md`) would collide across lanes; committed worktrees are never swept; it labels child-reported git output as fact; it turns a thin conveyor into a patch lander (product boundary) | After an observed out-of-scope edit or lost work, or a measured latency need, with a design that handles shared regen outputs and worktree retention |

### 2.12 validate-and-defaults

| Field | Value |
|---|---|
| Recommendation | Seven items: return FAIL when a proven failure meets an identity gap; default to a reviewer from another model family; set using-gc to `push=false`; make plan's tracker write conditional; have memory try a check before a prose rule; make the policy hook fail closed without jq or its registry; stop agents self-waiving. Plus: no policy protects tests |
| Ruling | accept-modified: two items here (self-waive visibility; installer waiver text). The using-gc and plan items landed through §2.8 |
| Landed, #1218 | `core.policy:self-waive` in route (ask) mode (`hooks/guards/policies/policies.json:115-144`), matching Write/Edit and common Bash write shapes onto the waiver file; its rationale says "evadable visibility, not tamper-proofing". `hooks/guards/scripts/install-hooks.sh:83-87` now prints the real mechanics: `AOP_WAIVE` in the environment Claude is launched with, or a waiver-file line; a Bash-command prefix does not waive. FIRE and SILENT cases in `tests/scripts/policy-dispatch.bats` (:299-351) |
| Fresh-read defect | The installer claimed the ask covered a relocated `AGENTOPS_HOME`. The matchers anchor on the default `.agents/ao/policy-waivers` path, so the text was narrowed to "default path only" and pinned by a bats case before merge |

| Rejected | Why | Constraint |
|---|---|---|
| Deny mode for self-waive | The regex can be evaded many ways; ask makes the documented path visible without blocking a waiver the operator asked for | anti-ceremony (no false protection claim) |
| FAIL before NOT_PROVEN on an identity gap | The order is deliberate and published ([`docs/releases/2026-10-05-v3.10.0-notes.md:76-78`](https://github.com/boshu2/agentops/blob/7de636720/docs/releases/2026-10-05-v3.10.0-notes.md#L76-L78); `skills/validate/references/mechanics.md:52-59`); both outcomes block; the CLI reaches the case only with explicitly empty or colliding identities. Changing it reverses a contract | Released contract |
| Cross-family reviewer by default | ADR-0017 "Prior review-selection amendment — 2026-09-08" (:176-185) makes the author's family the default and cross-model opt-in (`skills/validate/SKILL.md:144-151`) | ADR-0017 |
| Fail-closed hook without jq or registry | Fail-open is documented, tested design (`hooks/guards/hooks/policy-dispatch.sh:45-49`; bats "missing registry fails OPEN") | pin |
| Hook that blocks test edits | The tightening ratchet is advisory by design (§2.7) | pin |
| Memory "check first" rule | It edits a skill that beats the placebo noise floor, on evidence the repository marks non-promotable | anti-ceremony |

| Deferred or retire | Reopen or retire when |
|---|---|
| `core.policy:self-waive` (retirement) | Retire on false-positive evidence, or when an owner-only waiver mechanism exists (`value_proof` in policies.json) |
| Precedence reversal | Reopen (implied): an owner-approved contract change with a release note, plus a case where NOT_PROVEN caused a wrong ship decision |
| Cross-family default | Reopen (implied): a caller-approved ADR-0017 amendment |
| Runtime-derived verdict identities | Owner (§4) |

## 3. Report corrections

The arbiter's corrections to the pre-debate report. Line numbers that the
five PRs moved are replaced by file or symbol names.

| Report claim | Correction | Status as of a193b6bdf |
|---|---|---|
| A long, self-contradicting always-on file (about 8.5K tokens) ships and loads on every task | AGENTS.md and `.claude/rules` are not in `plugin/`. The 8.5K load falls on sessions inside this repository; plugin users load about 2K tokens of skill and agent descriptions (PROXY). What shipped with the retired rule: the CLI briefs, the Codex manifest and the workflow tombstones | CLI and tombstones fixed (#1221); manifest is an owner item |
| About 35 files carry fresh-validation wording, all contradicting AGENTS.md | "All" overstates it: `skills/validate`, `skills/rpi` and the workflow reference agreed. The list missed the two most authoritative sources: ADR-0017 (Accepted) and the Go-pinned CLI text. Rec #2 could not land without amending ADR-0017 | Fixed (#1221) |
| Verdicts are bound to exact content; AN-13 commit-pinned approval is implemented in code | The digest binding is real. Author and validator identity, freshness and scope result are required caller flags (`cli/internal/commands/provenance/evidence.go:88-92`), which `mechanics.md:59` calls "attestations, not independently discovered runtime facts". Only an honest collision is caught | Open (owner) |
| 67 gates, 57 blocking; 9 native gates outside the witness test | 66 registered (57 in seed.go, 9 native), 56 blocking (debate). The native count of 9 is correct | 65 registered, 55 blocking (§2.10) |
| Script-backed blocking gates must prove they can fail; the exemptions cover cannot-fail gates | Only new gates must. The grandfathered gates (23 then) are unproven, not shown unable to fail. Exactly two could not fail as registered | Two retired (#1218); 21 grandfathered remain |
| Rec #3 evidence: a 167-agent run burned 1-3 retries per slice; ADR-0017 wants checkable stops | The retries came from dispatching a nonexistent subagent type in the since-retired operating-loop. The cited ADR-0017 passages sit under "Historical context" or follow from the convergence law, which the 2026-09-09 lean amendment superseded, including "a default two-round native stop" | Unchanged; §2.3 |
| Rec #4: the quota reason for dropping push-to-main is gone; landed main SHAs are unvalidated | 3d4fc1299 also cited serialization and the 20-slot concurrency pool. 30 of 30 sampled squash commits matched a green PR head tree (debate), so the gap is a squash artifact. One direct push lacked Validate since 2026-09-23 (debate) | push:main deferred (§2.4) |
| Release is not gated on tests | Partly true: policy required a local release CI run and tag Validate runs are full verdicts, but publish never waited for Validate. v3.1.0 and v3.2.0 published from SHAs whose Validate failed | Fixed (#1217) |
| Rec #7: make `gate.tightening-ratchet` blocking; deletions go unrecorded | The gate is "ADVISORY PERMANENTLY at this fidelity", pinned by `seed_test.go`; its heuristic false-positived 2 of 2 in replay (debate). All 5 gate deletions since 2026-08-28 carried trailers on branch commits; squash merge with the PR body dropped them | Template line and hint (#1218) |
| Validate edit: a cited proven failure returns FAIL even when identity is unattested | Identity-first order is deliberate and published (v3.10.0 notes :76-78; `mechanics.md`:52-59), restated in the rpi skill, and fails safe since both outcomes block. Changing it reverses a contract, not a skill edit | Rejected (§2.12) |
| Rec #6: cut AGENTS.md to at most 120 lines and 8 KB | A line target can be gamed (118f78746). A faithful 8 KB cut dropped rules with no other owner and the ADR-0016 CDLC scope. A byte cap plus in-place fixes captures the correctness value; the deep cut needs pins replaced and a removal map first | Byte cap 14,336 B (#1219) |
| "Assign final review once" (:18) contradicts "no fresh review is owed" | A leftover presupposition from e6a1b0f54 in delegation guidance. A one-clause conditional fixes it; it was not a contradiction in effect | Fixed (#1219, AGENTS.md:18) |
| Plan shows the clearest skill gain (15/15 vs 5/15) | claude-exec's gain is larger and less confounded: 14/15 vs 3/15 on a case written without sight of the skill. Plan's case was among the 28 used to tune the 3.10 descriptions (PROXY; same-family grader) | Unchanged |
| Delete implement's triggers copied from rq-07; re-key security on "exploitable" | Deleting the triggers turns the CI-run honest pin red (13/13). Security already loaded 3/3 on its behavior prompt, and c5 failed after loading. Keying on blind-prompt words would contaminate the held-out set | Rejected (§2.8) |
| Merge navigate into orchestrate (overlap on readiness and disjoint-scope rules) | Only two passages are verbatim duplicates; navigate's pick rules are its own. A simulated fold creates new ties and misroutes, breaks frozen goal prompts, and skips the `REMOVED_SKILLS` tombstone | Deferred (§2.9) |
| Hooks are installed only for Claude Code, not Codex | An opt-in Codex read-budget PreToolUse hook ships (`scripts/install-codex-read-budget-guard.sh`). What Codex lacks is the `policies.json` dispatcher rules | Unchanged |
| The outcome-eval substrate decayed silently because nothing runs it | The Harbor drift came from Renovate automerge (6e63d64ca, 1591bb843) against a documented pin. A coupling test fixes it; a funded run does not | Fixed (#1218) |

## 4. Owner actions

Status as of a193b6bdf and GitHub REST reads on 2026-10-21.

- [x] **Approve the ADR-0017 conditional-review amendment text.** Done: merged in #1221 (ADR-0017 :8-31).
- [ ] **Decide product-identity wording.** Open. GOALS.md "Product outcome" still lists "fresh independent judgment"; PRODUCT.md "What the product provides" still says "one fresh, author-distinct semantic judgment" and "Final judgment must come from a fresh context"; `.codex-plugin/plugin.json:9` (longDescription "have a fresh reviewer validate the exact change") and :17-18 (default prompts "Use AgentOps Implement ...", "Validate from fresh context"); `.claude-plugin/marketplace.json:8,14` ("independent validation"). Implementers can supply a proposed diff.
- [x] **Confirm the runtime-floor rewording.** Done: merged in #1219 (AGENTS.md:109-110).
- [ ] **Decide whether the constraint floor counts Accepted-status ADRs only** (AGENTS.md:31, :140-143). Open.
- [ ] **Enforce branch protection for admins.** Open. `branches/main` on 2026-10-21: required check `summary`, `enforcement_level: non_admins`. First read `branches/main/protection` (it returned 403 to the debate session) for linear-history and required-review settings, then enable "Do not allow bypassing the above settings" or ruleset 11923893 with an explicit bypass list. Prerequisite met: the retag reorder landed in #1217.
- [ ] **Run the capped headroom probe before any outcome-eval build** (optional). Open. Terms are in §2.5.
- [x] **If `harbor==0.22.0` fails to install on CI's Python 3.14, declare a new evaluator cohort.** Not triggered: it installed during #1218. Revisit only if a later CI install fails; then bump `evals/skills-rpi/requirements.txt`, `prepare.py` (`HARBOR_VERSION`) and `evals/skills-rpi/README.md` together (pinned by `test_requirements_readme_and_prepare_name_one_harbor_version` in `evals/skills-rpi/test_prepare.py`), and update the taskbank README mention.
- [ ] **Optional paid reruns.** Open: the premortem behavior case (about $0.39) for the Check 1 rescope; new blind routing cases written without sight of the descriptions, before any security, research, test, review or implement description re-key; a `claude -p` probe (about $0.05) before adding nested `cli/` CLAUDE.md files.
- [ ] **Rewrite rpi eval criterion c3 before the next paid plugin-eval run.** Open: `evals/plugin-eval/behavior/rpi/case.yaml:23-26` still requires "final sign-off must come from a fresh reviewer", which grades the retired rule.
- [ ] **Ship a release.** Open: the latest release is v3.10.0, published 2026-10-05, before #1221. Installed binaries still print the retired rule from `ao quick-start`, `ao demo` and `ao robot-docs`.
- [ ] **Decide on runtime-derived author, validator and freshness identities for stored verdicts.** Open: they are still caller-supplied flags (`cli/internal/commands/provenance/evidence.go:88-92`), so "the agent cannot approve" is not enforced.
