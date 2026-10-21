---
status: historical
---

# Refreshing this snapshot

This snapshot is evidence as of a193b6bdf (2026-10-21), not current truth. A
refresh re-derives every claim from the tree in front of you and carries
nothing forward because this directory says it. Use the cheapest path that
answers the decision you face. [AGENTS.md](../../../AGENTS.md), the
[ADRs](../../adr/) and the gates outrank everything here.

| Path | Use when | Runs in | Agents | Output |
|---|---|---|---|---|
| [1. Quick refresh](#1-quick-refresh) | Default. Before acting on any gap, number or owner action in [README.md](README.md) | One Claude Code or Codex session | 1 | Each item marked still-true, fixed, changed or unverifiable, with path evidence |
| [2. Full re-score](#2-full-re-score) | A decision depends on the dimension scores (a parity claim against vanilla, or picking the next dimension to work on) and the caller has asked for delegation or a full re-score ([AGENTS.md](../../../AGENTS.md) "Concurrency"); otherwise run path 1 and report that the scores need a re-score | Claude Code Workflow tool; workflows are a Claude-only adapter ([workflows/README.md](../../../workflows/README.md)) | Up to 24: 12 finders, 12 skeptics | Skeptic-checked findings per dimension and a finder-stated score |
| [3. External refresh](#3-external-refresh) | A dimension bar rests on a source that may be superseded, or six months have passed | Any session with web access | 1 | New or superseding rows for the next snapshot's practices file |

Paths 1 and 2 are JUDGMENT. Neither turns an UNMEASURED item into MEASURED;
only a new outcome measurement does (README §2, owner action 4 in README §6).

## 1. Quick refresh

One session, no workflow. Skills are optional
([SKILL-ROUTER](../../SKILL-ROUTER.md): "no skill is mandatory"):

| Skill | Fits | Not for |
|---|---|---|
| [reality-check](../../../skills/reality-check/SKILL.md) | The whole job: each README row is a claim. Its confirmed / gap / unverifiable dispositions map onto still-true / fixed or changed / unverifiable | Choosing what to fix next |
| [research](../../../skills/research/SKILL.md) | One cited question the tree cannot settle, e.g. whether default Claude Code now loads nested `CLAUDE.md` files (gap 4) | A whole re-score |
| [council](../../../skills/council/SKILL.md) | Only when the caller asks for a second opinion on a disputed disposition | Default use; same-family opinions are correlated |

Paste-ready prompt, run from the repository root:

```text
Re-verify the dated research snapshot in docs/research/agentops-practice-ranking-2026-10/
against the current tree. Treat it as a list of claims, not as evidence.

1. Read README.md and decisions.md in that directory. Open practices.md or skills.md
   only when a claim cites them.
2. Record HEAD (git rev-parse --short HEAD) and the distance from the snapshot
   (git rev-list --count a193b6bdf..HEAD).
3. For every row of README.md section 5 "Open gaps on main" and section 6 "Owner actions
   remaining", re-run the check in its "How re-verified" column (or an equivalent) and
   open each cited path. Cited line numbers may have moved; locate by symbol or text.
4. Re-measure every number marked "as of a193b6bdf" or "measured", using the
   "Re-measure commands" block in refresh.md section 1 where it applies.
5. Classify each item: still-true | fixed | changed | unverifiable.
   - Evidence is path:line, a commit, or literal command output. No evidence, no verdict.
   - fixed: name the commit (git log -S'<text>' or git log -- <path>).
   - unverifiable: name the evidence that would settle it and why you lack it.
6. Read-only. Do not edit the snapshot, fix anything, or create beads or files.
7. Do not re-propose anything decisions.md rejected or deferred unless you cite the
   reopen evidence it names for that item, or evidence dated after 2026-10-21 that its
   rationale did not consider.
8. Report: (a) one table, one row per item; (b) the items whose disposition changed;
   (c) whether any change moves a dimension bar enough to justify the full re-score in
   refresh.md section 2, with the reason; (d) checked and not_checked.
If you share a model family with the snapshot's authors (the family of the default
`--judge-model` in evals/plugin-eval/grade.py), say so: agreement with the
snapshot is then correlated, not independent confirmation.
```

**Re-measure commands.** Each comment holds the value as of a193b6bdf. R1-R9
were re-run on that tree for this file, and R10 and R11 against GitHub on
2026-10-21; R12 comes from README §2.

```bash
# R1 AGENTS.md: 14,206 B and 233 lines; byte cap 14,336
wc -c AGENTS.md; wc -l AGENTS.md
grep -n BYTE_LIMIT scripts/validate-agents-split.sh
# R2 rules path-scoped: go.md paths "cli/**/*", python.md paths "**/*.py"
head -5 .claude/rules/*.md
# R3 skills: 29 SKILL.md files, 4,502 lines
ls skills/*/SKILL.md | wc -l; cat skills/*/SKILL.md | wc -l
# R4 registered checks: 65, of which 55 blocking (needs a binary built at HEAD)
ao gate check --full --dry-run --json
# R5 grandfathered blocking gates with no negative witness: 21
grep -v '^\s*#' scripts/.gate-negative-witness-grandfather | grep -c .
# R6 nested instruction files under cli/: 5 AGENTS.md, no CLAUDE.md
git ls-files '*AGENTS.md' '*CLAUDE.md' | grep '^cli/'
# R7 removed-verb references in Go: 2 (stale_refs.go, constraintindex.go)
git grep -n -e "constraint activate" -e "quality constraint" -- cli/internal
# R8 verdict identity is caller-attested: author, validator and freshness are required flags
grep -n 'AuthorContextID\|ValidatorContextID\|FreshnessSource' cli/internal/commands/provenance/evidence.go
# R9 no Validate run on push to main: on: lists v* tags, workflow_dispatch, pull_request, merge_group
sed -n '/^on:/,/^concurrency:/p' .github/workflows/validate.yml
# R10 admin bypass: required check "summary", enforcement_level non_admins (needs repo read access)
gh api repos/boshu2/agentops/branches/main --jq .protection.required_status_checks
# R11 latest release: v3.10.0 (2026-10-05), which predates #1221 (REST; works where GraphQL is blocked)
gh api repos/boshu2/agentops/releases/latest --jq '.tag_name + " " + .published_at'
# R12 always-on load: 11,371 vs 2,902 tokens, probed BEFORE #1219, 1 run per arm.
# Bytes are a PROXY at 2.4-2.9 B per token; a real token probe is a paid run (owner decision).
wc -c AGENTS.md .claude/rules/*.md
```

## 2. Full re-score

Run this only when the caller has asked for delegation or a full re-score
([AGENTS.md](../../../AGENTS.md) "Concurrency"). Reuse the existing conveyor;
do not write a new workflow.
`audit-dimensions` is a thin conveyor: the subject, charters and bar arrive in
`args` and the script adds only shape (a finder per dimension, then a skeptic
that re-opens every cited piece of evidence). Args, checked against
[workflows/audit-dimensions.js](../../../workflows/audit-dimensions.js) and the
[README example](../../../workflows/README.md#audit-dimensions):
`{ subject: string, dimensions: [{ key, charter }], bar?: string, root?: string, maxFindingsPerDimension?: positive number }`.
It returns `{ dimensions: [{ key, summary, findings, refuted }] }`; a dimension
whose finder died carries an `error` field.

**Runtime setup**

| Situation | What to do |
|---|---|
| Workflow not listed | Run `ao workflows link` from the canonical checkout. It links `workflows/*.js` into the project-local, gitignored `.claude/workflows/`. Claude Code snapshots the workflow registry at session start, so start a new session afterwards |
| AgentOps plugin loaded | The plugin copy may be listed as `agentops:audit-dimensions`. `plugin/workflows/audit-dimensions.js` was byte-identical to the source as of a193b6bdf. Use the name the runtime lists |
| Codex | No Workflow tool. Run path 1, or work through the 12 charters below one at a time in one session |

The `subject` keeps finders away from README.md, decisions.md and skills.md,
which hold the prior scores and would anchor them; the caller compares
afterwards. Run it from the repository root (no `root` arg).

```js
Workflow({ name: 'audit-dimensions', args: {
  subject: 'AgentOps skills and operating loop on the current tree at the repository root, judged against 2026 agentic-engineering practice. The external bar is docs/research/agentops-practice-ranking-2026-10/practices.md; cite its rows by ID (VG-, E, AN-, PF- or se-practice slug) and open the row before relying on it. Do not open README.md, decisions.md or skills.md in that directory: they hold prior scores and would anchor you. Evidence comes from the tree, CI config and command output, not from what docs intend.',
  bar: 'Severity. blocker: on the current tree a must-hold rule is unenforced, or a shipped instruction (CLI text, entry doc, skill, plugin metadata) is false, so that it misleads a user, breaks install or the CLI, or removes protection for the product. major: a gap against the dimension bar, citing a practice ID and a repo path. minor: drift, wording or a missing measurement with no user impact. A finding about something absent names the search and its empty output. Score: begin the summary with "score N.N/10; HEAD <short sha>". Anchor 5.0 = stock Claude Code or Codex with a short CLAUDE.md on this repository; 10 = every point of the bar met with outcome evidence. Support that is only same-family judgment or rule-uptake evals does not lift a point above 5.0. Label each claim MEASURED (with n), PROXY or UNMEASURED. Report a gap even when its obvious fix is one the 2026-10 debate rejected (a new gate, ratchet, classifier or detector, a numeric stall threshold, worktree lanes, skill folds or cuts, a shorter AGENTS.md rewrite, a cross-family reviewer default, a reversed verdict precedence, or a paid eval programme), but propose that fix only with evidence dated after 2026-10-21.',
  maxFindingsPerDimension: 5,
  dimensions: [
    { key: 'measured-value', charter: 'Bar (VG-23, VG-18, AN-11, AN-22, E22, PF-16, PF-24): outcome evals on 20-50 real tasks with a no-harness arm, negative controls, eval noise below the smallest effect acted on, a held-out split, a judge from another model family, cost and time per arm, and component ablation per model release. Check evals/, docs/evals/, PRODUCT.md "Evidence and claim limits", evals/_stats/power.py, and evals/plugin-eval/ (grade.py, behavior cases). Name the newest AgentOps-versus-vanilla outcome result, its n, its date and its judge family. Rule-uptake and routing results are PROXY.' },
    { key: 'loop-design', charter: 'Bar (VG-06, VG-07, AN-09, AN-21, PF-13, VG-20): a short loop stated in the always-loaded file; the plan is skipped when the diff fits one sentence; done means a runnable check passed with its literal output shown; review rounds are capped; exit and stall conditions are explicit and observable; each loop component has a retirement condition. Check AGENTS.md "Native execution and optional Lean RPI operating charter", docs/agent-workflow-reference.md, skills/rpi/SKILL.md, the stall-report fields in skills/agent-native/SKILL.md, and docs/adr/ADR-0017-loop-as-control-flow-not-knowledge.md with its amendments. Flag any surface that still teaches a fresh read on every change.' },
    { key: 'verification', charter: 'Bar (VG-06, VG-10, AN-10, AN-13, AN-20, E22, PF-02, protect-gates-from-agents): executable checks decide done; must-hold rules sit in CI or hooks that fail closed; tests and gates are protected from the author during a fix; fresh review is spent by a stated risk rule, not by preference; an approval binds to one commit and the author cannot issue it; the reviewer is not the author model family; harness changes are regression-tested. Check the summary job in .github/workflows/validate.yml, cli/internal/gates/checks/seed.go, scripts/.gate-negative-witness-grandfather, hooks/guards/policies/policies.json, skills/validate/SKILL.md, skills/validate/references/mechanics.md, cli/internal/commands/provenance/evidence.go and cli/internal/evidence/store.go.' },
    { key: 'context-engineering', charter: 'Bar (VG-08, AN-05, E12, E13, E14, PF-25, VG-17): the always-loaded file holds only what an agent cannot infer; language and area rules are path-scoped; each rule has one owner, files do not contradict each other, and coined terms are defined; guidance is tuned on held-out probes; always-on cost is measured against a no-file baseline. Check AGENTS.md (CLAUDE.md is a symlink to it) and its byte cap in scripts/validate-agents-split.sh, .claude/rules/*.md frontmatter, the nested AGENTS.md files under cli/ and whether default Claude Code loads them, skill description lengths, and plugin/ (what ships to users versus what loads only for contributors). File size versus adherence is contested (McMillan, arXiv 2605.10039); weigh cost and non-inferability, not line count.' },
    { key: 'skill-quality', charter: 'Bar (VG-17, VG-18, AN-06, AN-22, PF-21, PF-15): each skill fixes a failure that a no-skill baseline showed, carries minimal instructions, and is retired once the base model passes without it; triggering is measured on held-out prompts with negative controls; effect is measured on outcomes with noise discipline and a non-author judge; descriptions are third-person what-and-when under 1,024 characters; bodies are under 500 lines with references one level deep; must-hold policy sits in hooks or gates, not skill prose; process skills are not mandated. Check skills/*/SKILL.md, skills/catalog.json, docs/SKILL-ROUTER.md, scripts/check-routing-probe-goldens.sh with its goldens, and evals/plugin-eval/routing. If no blind-routing result is newer than the snapshot, say which skills are unmeasured.' },
    { key: 'sdlc-coverage', charter: 'Bar (AN-01..AN-04, AN-06, AN-07, AN-10, AN-11, AN-13, AN-15, AN-17, AN-24): each stage leaves an artifact the next stage reads (a tracker item linked to a commit counts); human gates are named by role and risk; must-hold rules have fail-closed backstops; authors cannot approve their own change; configuration changes trigger evals; incidents become regression checks or evals; rollback is rehearsed; each stage has a metric with a denominator and a countermetric. Check docs/agent-workflow-reference.md, docs/CI-CD.md, .github/workflows/release.yml, scripts/verify-release-ci.sh, scripts/retag-release.sh, the status lines of docs/adr/*.md against AGENTS.md "Constraint floor", and skills/memory/SKILL.md. AGENTS.md "Product boundary" excludes consumer landing and release by design; judge this repository, not consumers.' },
    { key: 'delivery', charter: 'Bar (small-batches, ci-as-gate, dora-five-metrics, goodhart-countermetrics, AN-13, AN-15, AN-20): small revertible batches on trunk; CI is the authoritative gate for every change reaching main, with no admin bypass; a release publishes only from a green exact-SHA verdict; rollback is rehearsed; throughput is reported beside rework or revert rate. Check the triggers and summary job of .github/workflows/validate.yml, .github/workflows/release.yml, scripts/retag-release.sh, .github/PULL_REQUEST_TEMPLATE.md, git log --first-parent --stat on main for batch size, and gh api repos/boshu2/agentops/branches/main (read-only; mark unverifiable if access is denied).' },
    { key: 'governance-safety', charter: 'Bar (AN-06, AN-07, AN-14, AN-19, VG-10, VG-24, protect-gates-from-agents): every must-hold rule is enforced outside the model and fails closed; hooks that fail open say so; boundaries follow access (credentials, push rights, other agents); the agent cannot waive or loosen its own checks unseen; a gate loosening carries a reason that survives a squash merge. Check hooks/guards/ (policies/policies.json and the fail-open paths in hooks/policy-dispatch.sh), .github/CODEOWNERS, scripts/check-gate-tightening-ratchet.sh, the trailer guidance in .github/PULL_REQUEST_TEMPLATE.md, and the main branch enforcement level.' },
    { key: 'adoption-cognitive-load', charter: 'Bar (AN-05, VG-07, VG-08, VG-17, E13, VG-20): one first-value path, identical in every doc and exercised end to end in CI; process scales to task size; doctrine is stable across releases; a short always-on file; newcomers reach value faster than with vanilla, measured. Check README.md, PRODUCT.md, GOALS.md, plugin/README.md, .codex-plugin/plugin.json, .claude-plugin/marketplace.json, the root help and the quick-start, demo and robot-docs briefs (cli/cmd/ao/root.go, cli/internal/commands/quickstart, demo and robotdocs module.go), and the newest docs/releases/ notes. Flag text that contradicts the AGENTS.md review rule, and say whether the latest released binary predates that rule.' },
    { key: 'multi-agent', charter: 'Bar (VG-04, VG-05, E16, E18, AN-08, AN-19, PF-13): one agent by default with a checkable rule for when to fan out; each concurrent writer is mechanically isolated, with scope checked before and after; concurrency is capped by review capacity; cost per lane and a single-agent baseline at equal tokens; least-privilege agent identities; deterministic stall detection. Check AGENTS.md "Concurrency", workflows/*.js and workflows/README.md (implement-wave lane order and dead-lane handling), skills/orchestrate/SKILL.md and skills/agent-native/SKILL.md. Separate claims a test pins (tests/fixtures/implement-wave-workflow.mjs) from instructions only.' },
    { key: 'memory-learning', charter: 'Bar (AN-05, AN-11, AN-17, PF-09, PF-10, E14, VG-20): a rule is added only after a repeated mistake and its effect is measured; a recurring correction becomes a deterministic check before it becomes prose; incidents become regression evals; configuration is pruned on a schedule and re-tested per model; reuse shows a measured benefit. Check skills/memory/SKILL.md, .context/README.md, docs/adr/ADR-0016-state-tiers.md, AGENTS.md "Repository work tracker" (bd remember), and git log --stat -- AGENTS.md for additions versus removals.' },
    { key: 'self-maintainability', charter: 'Bar (VG-10, VG-20, PF-09, AN-06, AN-07): machinery stays small relative to delivered value; every gate, skill, workflow and projection has a named consumer and a retirement condition; every blocking gate has a negative witness and none passes by construction; narrative docs derive from one source; process overhead is measured. Check cli/internal/gates/checks/seed.go and its tests, scripts/.gate-negative-witness-grandfather, scripts/regen-all.sh --check, the tombstones in workflows/, and the count of generated projections.' },
  ],
}})
```

**After the run** (caller, in the same session):

1. Take each score from its summary. The skeptic judged findings, not the
   score, so the score is one finder's JUDGMENT.
2. Compute the weighted total with the README §3 weights: measured-value 15;
   loop-design, verification, context-engineering, skill-quality 12 each;
   sdlc-coverage 10; delivery, governance-safety, adoption-cognitive-load 8
   each; multi-agent 7; memory-learning 6; self-maintainability 0; sum 110.
3. Compare with README §3 (4.71 → 4.26 pre-debate) and the debate-corrected
   range 4.6-4.9. Explain every move of 0.5 or more with a surviving finding.
4. Peers are not re-scored. Do not claim a rank change against a peer without
   re-scoring that peer on the same rubric.
5. Re-run only dimensions that came back with `error`, passing just those
   entries in `dimensions`.
6. Filter findings whose fix re-proposes a [decisions.md](decisions.md)
   rejection without new evidence (§4).

## 3. External refresh

Re-check sources in this order: those that set a bar where AgentOps scored
lowest or where the evidence is contested, newest first. IDs resolve in
[practices.md](practices.md).

| Order | IDs or sources | Bars that depend on them | Supersession signal |
|---|---|---|---|
| 1 | E12/PF-08 (Gloaguen, arXiv 2602.11988, v3 Sep 2026); McMillan, arXiv 2605.10039; Khatri, arXiv 2607.27250; E13; E14 | context-engineering, adoption-cognitive-load, memory-learning | A new version, a replication, or a study finding a size-adherence effect or a gain from long context files |
| 2 | AN-20, AN-21 (Claude Code team code review, 2026-10-15); AN-13; the playbook behind AN-01..AN-25 (web 2026-08-21 vs PDF 2026-09-23) | verification, delivery, sdlc-coverage; the conditional-review rule | A newer playbook version; a changed round cap or approval tiering |
| 3 | AN-22 (2026-09-28); VG-18 (2026-03-03); VG-23; E22 (arXiv 2609.17857); PF-02 (arXiv 2607.25152) | measured-value, skill-quality; the same-family caveat | New judge-bias or eval-noise results |
| 4 | PF-15 (Böckeler, n=2 per condition); PF-16 (Jain, 2026-10-13); E18 (arXiv 2604.02460); VG-20/PF-17 | loop-design, multi-agent | Larger-n TDD-in-loop or harness-cost results |
| 5 | VG-24 (2026-03-25); VG-25; AN-07; AN-14 | governance-safety | A change to hook failure defaults or managed-settings semantics |
| 6 | E02/PF-24 (METR 2026) | measured-value framing | A new controlled productivity study |
| Unverified | VG-09 (OpenAI "Harness engineering", primary returned 403); the just-in-time planning attribution (practices.md, Contested points, "Planning artifacts"); E11 Faros figures; the 40%→3% figure attributed to Hashimoto (PF-10) | Carried with caveats | Read the primary source, then verify the row or drop it |
| Pre-2026 baselines | protect-gates-from-agents (ImpossibleBench, 2025); E03, E04 (DORA 2024, 2025); E16; E21; VG-04 | Kept only for want of a 2026 source | Replace when a 2026 source covers the same claim |

**Recording a newer source**

- Record it in the next snapshot's practices file, never in
  [practices.md](practices.md) here.
- Same claim, newer source: keep the ID, put the new source, date, strength
  and URL in the row, and add `Supersedes: <old source, date>`. New claim: the
  next free ID in its lane (for example AN-26).
- Use the columns and vocabularies of [practices.md](practices.md) "How to
  read this file": strength `rct | benchmark | observational | vendor-guidance
  | practitioner-consensus | opinion`; Ver. `— | 403 | 2nd | conflict | drift`.
  Mark `2nd` if you read only a secondary summary.
- If the source changes a dimension bar, rewrite that charter in the next
  snapshot's copy of §2 and name the ID that moved it.
- Prefer 2026 or later. Keep an older row only as a labelled baseline.

## 4. Rules for the refresher

| Rule | Why |
|---|---|
| Same-family judges are correlated. Agreement between your run and this snapshot is not independent confirmation. Cross-family judgment is opt-in under [model-dispatch](../../../skills/agent-native/references/model-dispatch.md) and actual host authorization | E21, E22. The snapshot's scorers, critics, debaters, arbiter, implementers, fresh readers and the repository's eval grader share one model family (README §2) |
| Do not re-propose a rejected or deferred item without new evidence. [decisions.md](decisions.md) lists them: new gates and ratchets (banned-phrase, path classifier, red-test-first), a numeric stall threshold, push-to-main Validate, worktree lanes, skill folds and cuts, a 120-line or 8 KB AGENTS.md, reversed verdict precedence, a cross-family reviewer default, a paid outcome-eval programme. New evidence is the reopen evidence decisions.md names for that item, or evidence dated after 2026-10-21 that its rationale did not consider; it permits a proposal, not a bypass of a criterion decisions.md names | Re-litigation without new facts costs a debate and moves nothing |
| Write a new dated snapshot (`docs/research/agentops-practice-ranking-YYYY-MM/`) and link back here. Do not edit this directory; the README §1 retirement condition says when it is superseded | Dated evidence ranks last in AGENTS.md "Source precedence"; a silently edited snapshot loses its baseline |
| Apply the anti-ceremony test (AGENTS.md "Honest work and anti-ceremony") before creating any artifact, including the new snapshot: name its consumer, the decision it gates, the observed defect behind it and its retirement condition. If any is missing, answer in the session instead | No new workflow, skill, hook, gate or schema is needed to refresh: `audit-dimensions`, the optional skills and `gh`/`git` cover it |
| Keep the refresh read-only. Fix what it finds through native execution: checks and CI gate an ordinary change; one fresh read only on the AGENTS.md triggers | AGENTS.md "Native execution and optional Lean RPI operating charter" |
| Mark every number that can drift with the commit it was measured on, and state n and method for anything called MEASURED | #1217-#1221 moved several numbers and line citations in this directory |
| Cite repo-relative paths and public URLs only | A reader of the snapshot has the repository, not your session |
