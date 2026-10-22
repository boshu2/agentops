---
status: historical
---

# Per-skill scorecard, 2026-10 snapshot

Snapshot date 2026-10-21. Scores were assigned on the tree before
PRs #1217-#1221 and were not re-scored. Every status, line number and count below
was re-checked on `main` at a193b6bdf unless a row says otherwise. Overview and
verdict: [README.md](README.md). Debate detail: [decisions.md](decisions.md).
Each skill lives at `skills/<name>/SKILL.md`.

## 1. Answer

- 29 skills (`skills/catalog.json` `skill_count`, as of a193b6bdf). Lane
  verdicts: keep 8, revise 20, merge 1 (navigate into orchestrate). Overall
  scores run 4-8, median 6, mean 5.7.
- Measured value sits in skills that carry facts a base model lacks:
  claude-exec, memory, using-gc, skill-builder. General-judgment skills sit at
  the no-plugin ceiling: 8 of 29 behavior cases met every criterion without the
  plugin.
- Loading is the bottleneck, not content: 9 of 25 model-invocable skills loaded
  0/2 on blind requests. implement, review and council never loaded on their
  own behavior requests, so no eval has exercised their bodies.
- Top-edit status on a193b6bdf: applied 1 (plan), applied in part 4
  (agent-native, doc, domain, using-gc), rejected by debate 5 (idea-genie,
  implement, orchestrate, security, validate), deferred 7 (memory, navigate,
  premortem, reality-check, research, rpi, test), open 12. Only #1220
  (933218a89) edited skill files among the five PRs.
- No skill was removed, merged or had its description re-keyed. The debate
  deferred the navigate fold behind five exit criteria and rejected the
  implement and review cuts and the security re-key (§9).

## 2. Evidence classes

| Class | What | n / method | Caveat |
|---|---|---|---|
| JUDGMENT | Per-skill axis scores (§3), defects (§4), cross-skill problems (§5), overlaps (§6) | 4 repo-reading lanes (judgment, core, specialist, meta-adapters); each read SKILL.md, references, validators, tests and the eval scorecard | Same model family as the skill authors, the eval grader and the debaters; agreement is correlated, not independent. Not re-scored after #1220 |
| PROXY | Plugin eval behavior cases | 29 cases × 3 runs per arm (3.10, 3.9.0, no plugin) | Criteria written from each skill's own rules; measures rule uptake, not outcomes (§8) |
| PROXY | Blind routing | 25 held-out requests × 2 runs per release | Measures whether the Skill tool was called, nothing else |
| PROXY | Deterministic routing goldens | 13 queries over `ao skills find` | Lexical catalog ranking; the grader says it does not show that a model would route there |
| MEASURED | Line counts, grep results, file existence | Re-measured on a193b6bdf for this file | Drift with every commit |
| UNMEASURED | Outcome effect per skill; loading with host built-in skills present; whether Claude Code injects frontmatter beyond name and description; the shipped text of 12 skills edited after the eval (§8) | none | none |

Axis names are the scoring lanes' own. No written rubric beyond the names
survives; read them as: trigger (would the description route the right
requests), economy (tokens per useful instruction), actionability (can an agent
act from the text alone), verifiability (can a check or eval show it worked),
clarity (defined terms, no contradictions), necessity (what a no-skill model
lacks, informed by the no-plugin arm). Scores are 1-10.

## 3. Scorecard

Lines: SKILL.md line count as of a193b6bdf. Status: re-checked on a193b6bdf.
"Optional note" means the debate advocate listed the edit as having no defect
and no measured gain behind it, and the arbiter did not take it up.

| Skill | Lines | Overall | Trig | Econ | Act | Verif | Clar | Need | Verdict | Top edit | Status after #1220 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---|
| plan | 146 | 8 | 8 | 7 | 8 | 7 | 7 | 8 | keep | Write to the tracker only with write authority; otherwise return a proposed amendment | **Applied** (#1220): Output write is conditional on authority; Workflow step 4 follows the same authority. #1220 also added `metadata.triggers: ["write scope", "scope boundaries"]`; description line byte-identical |
| claude-exec | 96 | 8 | 8 | 8 | 9 | 7 | 7 | 9 | keep | Derive `SECS` from a fixed deadline in both commands so the non-renewing budget lives in code | **Open**, not debated. Both commands still use a static `"$SECS"` (`skills/claude-exec/SKILL.md:61,66`) |
| codex-exec | 131 | 7 | 8 | 7 | 9 | 8 | 6 | 5 | keep | Show a caller retry that reuses the same `CODEX_EXEC_DEADLINE_EPOCH` and stops at remaining ≤ 0 | **Open**, not debated |
| interview | 114 | 7 | 6 | 8 | 9 | 5 | 7 | 6 | keep | Add `Settled: <x>. Open: <n>.` to the per-turn template | **Open**, not debated. Template at `skills/interview/SKILL.md:45-49` has no state line |
| memory | 122 | 7 | 8 | 6 | 7 | 5 | 4 | 7 | revise | Move "One authority per fact" and "Access, storage and honest limits" (`SKILL.md:82-122`) to a curation reference, about 40% fewer tokens | **Deferred**: kept out of the batch because `skills/memory/scripts/validate.sh` pins that text and it conflicts with active constraints |
| reverse-engineer | 170 | 7 | 8 | 6 | 8 | 7 | 6 | 6 | keep | Script paths relative to the skill directory; public licensed repos need no written authorization | **Open**, not debated. `SKILL.md:44,77,138` unchanged |
| using-gc | 342 | 7 | 7 | 4 | 8 | 6 | 4 | 8 | revise | Move "Preferred pack and registries" and "Upgrade an existing city to 1.4" to a reference; add a 6-term glossary; example sends `push=false open_pr=false` | **In part** (#1220): template sends `push=false open_pr=false` and publishing needs caller and repository authorization (`SKILL.md:159,162`). Size cut and glossary not done (optional note) |
| domain | 105 | 6 | 6 | 6 | 8 | 5 | 7 | 4 | revise | Move the standards lookup out of Domain; strip AgentOps-only rules from `references/standards/go.md` | **In part**: #1220 removed the AgentOps-internal identifiers from go.md; #1219 fixed the stale `skills/standards` pointers in `.claude/rules/python.md`. Standards still live under Domain; not debated |
| idea-genie | 153 | 6 | 6 | 6 | 6 | 6 | 6 | 5 | revise | Drop duel mode (collides with Council's duel); add a candidate title field | **Rejected by debate**: no observed defect; the two duel modes already point at each other |
| orchestrate | 169 | 6 | 7 | 5 | 6 | 4 | 5 | 6 | revise | Replace the restated charter with a pointer to Validate; add one worked status block | **Rejected** as a required edit: the copy matches Validate today, and orchestrate's author-distinct-review criterion is 2/3 with the plugin vs 1/3 without. #1220 added `last activity <time, kind> \| unobserved` to the assignments line (`SKILL.md:163`) |
| refactor | 137 | 6 | 5 | 7 | 9 | 6 | 9 | 3 | keep | Delete the eval-harness sentence; describe the skill by outcome | **Open** (optional note). Sentence still at `SKILL.md:63` |
| security | 199 | 6 | 5 | 5 | 8 | 6 | 7 | 7 | revise | Trigger on "find vulnerabilities / exploitable"; say the ship call belongs to the caller | **Rejected by debate**: loaded 3/3 on its behavior prompt and c5 failed after loading; blind-prompt words would contaminate the held-out set |
| skill-builder | 156 | 6 | 6 | 6 | 5 | 6 | 4 | 7 | revise | Add a consolidate-or-fix-a-description procedure; move Conversion to a reference | **Open** (optional note). Only the description mentions consolidation; Conversion still at `SKILL.md:119` |
| skill-eval | 196 | 6 | 8 | 5 | 7 | 6 | 4 | 4 | revise | Cut to rules, decision table and output template (about 80 lines) | **Open** (optional note) |
| test | 159 | 6 | 6 | 7 | 8 | 6 | 8 | 3 | keep | Add coverage and regression wording to the description (blind 1/2) | **Deferred**: description re-keys wait for new blind cases written without sight of descriptions (owner, paid) |
| validate | 213 | 6 | 6 | 5 | 8 | 6 | 5 | 3 | revise | A cited proven failure returns FAIL even when identity is unattested | **Rejected by debate**: the identity-first order is deliberate, published and fails safe; changing it is a contract reversal, not a skill edit |
| agy-native | 81 | 5 | 6 | 8 | 4 | 4 | 6 | 3 | revise | Add one bounded `timeout ... agy --print-timeout` command template | **Open** (optional note) |
| council | 223 | 5 | 6 | 4 | 6 | 5 | 5 | 3 | revise | Merge the four restated rules; add one valid `council-report.v1` example; about 120 lines | **Open** (optional note). No JSON example in SKILL.md |
| craft-goal | 205 | 5 | 6 | 4 | 7 | 5 | 3 | 6 | revise | Replace the Mayor/membrane/andon/ratchet metaphors with plain definitions; add one worked UNSAFE_GOAL example | **Open** (optional note) |
| doc | 137 | 5 | 5 | 6 | 6 | 4 | 5 | 4 | revise | Strip the mandatory interview and `--mode` flags from `references/readme-craft.md`; extend the link check to `references/` | **In part** (#1220): readme-craft.md fixed. Link check still reads SKILL.md only; other references still name `--mode` or an interview (§4) |
| postmortem | 137 | 5 | 6 | 6 | 8 | 4 | 6 | 3 | keep | Collapse "Critical Constraints" (`SKILL.md:71-85`) into one boundary line; drop dead `HISTORY` frontmatter | **Open**, not debated |
| premortem | 150 | 5 | 7 | 6 | 7 | 6 | 6 | 3 | revise | Rescope Check 1 to the charter's costly cases; order findings most consequential first | **Deferred** (owner, paid): Check 1 is premortem's only measured gain; rerun its behavior case (about $0.39) first |
| reality-check | 103 | 5 | 5 | 7 | 8 | 5 | 6 | 3 | revise | Put `ao goals` and `ao status` in the description, or move them out | **Deferred** with the other description re-keys |
| research | 132 | 5 | 4 | 8 | 7 | 6 | 6 | 3 | revise | Re-key the description to pattern-rule tests and recon packs; restore "not for teardowns"; delete the stale pre-approval sentence | **Deferred** (re-key). Stale sentence still at `SKILL.md:66`; not debated |
| rpi | 151 | 5 | 7 | 4 | 5 | 5 | 3 | 4 | revise | Replace charter prose with a 6-step run sequence; move stall doctrine to references | **Deferred**: conflicts with validator phrase pins and AGENTS.md's statement that RPI packages the charter. #1220 changed only the stall closeout line (`SKILL.md:140`) |
| agent-native | 144 | 4 | 5 | 5 | 6 | 5 | 3 | 3 | revise | Narrow the body to the dispatch checklist; add cross-model and bulk-read routes to the description; fix `agents/bulk-reader.toml` | **In part** (#1220): bulk-reader.toml sentence restored. Body and description unchanged. #1220 added `last_activity:` to the per-packet block (`SKILL.md:140`) |
| implement | 165 | 4 | 2 | 4 | 6 | 3 | 4 | 5 | revise; cut if still 0/5 | Delete the triggers copied from routing golden rq-07; re-key the description; fold into test if it still loads 0/5 | **Rejected by debate**: deleting the triggers turns the CI routing pin red; a cut conflicts with blocking pins and the GC maintainer (§9). Triggers still at `SKILL.md:26` |
| navigate | 154 | 4 | 4 | 6 | 7 | 3 | 3 | 3 | merge → orchestrate | Fold the pick rules and checkpoint block into Orchestrate | **Deferred** with five exit criteria (§9) |
| review | 112 | 4 | 3 | 8 | 7 | 5 | 9 | 2 | revise; cut if still 0/5 | Replace ask-and-wait (`SKILL.md:57-61`) with "advise, and say it is not acceptance"; cut if still 0/5 | **Open**: ask-and-wait text unchanged, not debated. The cut was rejected for now (§9) |

Two debate corrections to the table:

- Plan is not the cleanest gain. claude-exec's is larger and less confounded:
  14/15 vs 3/15 on a case written without sight of the skill. Plan's case was
  one of the 28 used to tune the 3.10 descriptions.
- The verdicts do not apply the report's own retirement rule (VG-18 in
  [practices.md](practices.md): retire a skill once the base model passes
  without it). 5 of the 8 "keep" skills (test, postmortem, refactor +0.08,
  codex-exec +0.17, reverse-engineer +0.20) sit below the 0.33 placebo null in
  §8. Only plan and claude-exec clearly beat it.

## 4. Defects still present on a193b6bdf

One to three per skill, most consequential first. Applied fixes are not listed.

| Skill | Defect | Evidence |
|---|---|---|
| plan | AgentOps-only provenance mechanics sit inside a general planning skill; the 15/15 gain is rule uptake on criteria that mirror rules 1, 2, 5, 3 and 6 | `skills/plan/SKILL.md:82-86` (`ao provenance evidence-orphans`); `evals/plugin-eval/behavior/plan/case.yaml` |
| claude-exec | Static time bound; flags pinned to an older CLI; `--bare` (skips settings, hooks and CLAUDE.md discovery) not mentioned | `skills/claude-exec/SKILL.md:31,61,66` |
| codex-exec | Retry-deadline rule landed 2/3; codex-cli version disagrees with the agent-native reference | `skills/codex-exec/SKILL.md:79` (0.156.1) vs `skills/agent-native/references/context-budget-delegation.md:64` (0.154.0) |
| interview | Per-turn template omits the state line the body requires; stop checklist restates craft-goal admission; eval covers the first turn only | `skills/interview/SKILL.md:45-49`; `evals/plugin-eval/behavior/interview/case.yaml` |
| memory | About a third of the body is AgentOps storage policy in undefined terms, loaded for every "remember this"; overlaps Doc on instruction files; validator pins prose | `skills/memory/SKILL.md:82-122`; `skills/memory/scripts/validate.sh` |
| reverse-engineer | Script paths resolve only inside an AgentOps checkout; the authorization rule also blocks public OSS teardowns; caps-and-emoji header; 8 grandfathered Python files | `skills/reverse-engineer/SKILL.md:42,44,77,138`; `scripts/.skill-python-grandfather` |
| using-gc | Largest body: one-time upgrade and registry procedures load on every stall question; command facts self-declared unverified for the current pin; unglossed Gas City terms | `skills/using-gc/SKILL.md:115-149,180-221`, `:67` |
| domain | Two unrelated jobs (terms; language standards); `common-standards.md` is 447 lines of generic advice; AgentOps skill-packaging guide sits under Domain | `skills/domain/SKILL.md:3,29-31`; `skills/domain/references/standards/common-standards.md`, `skill-structure.md` |
| idea-genie | Duel packet requires fields SKILL.md never names, so an agent working from SKILL.md alone fails validation; portfolio candidates have no title | `skills/idea-genie/scripts/validate-challenge.sh:12,56` (`requires_ntm`; 0 matches in SKILL.md) |
| orchestrate | Restated charter; opening is a negation list; no worked input-to-output example; no validator | `skills/orchestrate/SKILL.md:24-29,104-120` |
| refactor | Eval-harness instruction in a user step; "simplify" verb collides with the host's built-in simplify skill; no `scripts/` | `skills/refactor/SKILL.md:63`; `skills/refactor/` holds only SKILL.md and references |
| security | Description invites the ship verdict the body forbids; the gate validator requires `gate_status=="PASS"`, so a completed scan with findings reads as incomplete; maintainer-only text loads on every trigger | `skills/security/SKILL.md:3,153` |
| skill-builder | Description promises "consolidate"; the body has no procedure; `audit-checks.md` contradicts itself on `--strict`; 5 grandfathered `.py` scripts | `skills/skill-builder/SKILL.md:3`; `references/audit-checks.md:33-34` vs `:77` |
| skill-eval | "No replacement runner" rule stated more than once; name collides with `scripts/skill-eval.sh` and `tests/scripts/skill-eval.bats`, an unrelated lint | `skills/skill-eval/SKILL.md:84,146` |
| test | Description omits "coverage", one of its four modes; cross-skill link breaks when installed alone; feature file allowlisted as documentation-only | `skills/test/SKILL.md:3,130`; `scripts/.scenario-linkage-allow:35` |
| validate | Generic readiness requests must get a question and a wait, no findings; about a third of the body is dispatcher protocol | `skills/validate/SKILL.md:73-77` |
| agy-native | No runnable command template; the "Gemini" trigger may catch Gemini CLI requests | `skills/agy-native/SKILL.md:3,41-45` |
| council | Three rules restated about four times each; no `council-report.v1` example; `subject_digest` mandatory even for brainstorms | `skills/council/SKILL.md`; `skills/council/scripts/validate-output.sh:14` |
| craft-goal | Undefined metaphor layer (Mayor, membrane, andon, ratchet); rules stated three times; no worked example | `skills/craft-goal/SKILL.md` (6 lines match `Mayor\|membrane\|andon`) |
| doc | Residual `--mode` and interview text in other references; link check reads SKILL.md only; `architecture-report.md` (547 lines) cites absent skills and subagents | `skills/doc/references/de-slopify.md:7,10,138`; `oss-pack.md:1,167`; `readme.feature:6,20`; `skills/doc/scripts/validate.sh:25`; `architecture-report.md:12,164` |
| postmortem | No headroom (12/12 without plugin); about 20% of the body is negation; dead `context.sections.exclude: [HISTORY]`; a registry test asserts removed strings and is not run | `skills/postmortem/SKILL.md:28,71-85`; `tests/skills/test-finding-registry-flow.sh` (absent from `tests/skills/run-all.sh`) |
| premortem | Check 1 flags every author-verified unit, against the charter; "outrank any single technical risk" conflicts with the output template's ordering; stale live test | `skills/premortem/SKILL.md:36-45`; `tests/claude-code/test-premortem-skill.sh` |
| reality-check | Description routes only the claim audit, which the model does at ceiling; `ao goals` and `ao status` modes unreachable by routing; defaults to in-repo `.agents/scratch/` | `skills/reality-check/SKILL.md:3,66` |
| research | Stale "pre-approves only local tools" after the grant was removed; negative clause against reverse-engineer gone while the rq-08 rationale still relies on it | `skills/research/SKILL.md:66`; `evals/routing-probes/goldens/rq-08-competitor-teardown.json` |
| rpi | Near-verbatim copy of the AGENTS.md charter; validator pins 10 phrases including `grep -Fq 'empty'`; "RPI" never expanded | `skills/rpi/scripts/validate.sh:15` |
| agent-native | Description covers parallel dispatch only; the body also owns Reader/Writer delegation and cross-model judgment; a test fake ships in the skill | `skills/agent-native/SKILL.md:3` vs `:67-78`; `skills/agent-native/scripts/fake_model_runner.py` |
| implement | Never loads; hidden triggers copy rq-07; "the runtime derives identity and receipts" is false for both runtimes; `@covered-by` points at a bash comment | `skills/implement/SKILL.md:26,33,89`; `skills/implement/scripts/validate.sh` |
| navigate | Highest coined-term density (HOLD, breaker, ratchet, churn, `bv`); assumes each bead runs through RPI, which AGENTS.md says native execution does not need | `skills/navigate/SKILL.md:26-27,115` |
| review | Ask-and-wait rule contradicts its own example; never loads; no validator or feature file | `skills/review/SKILL.md:42-44` vs `:57-61` |

## 5. Cross-skill problems

| # | Problem | Evidence (as of a193b6bdf) | Status |
|---|---|---|---|
| 1 | Loading, not content, is the bottleneck | 9 of 25 skills 0/2 blind; implement, review, council 0/3 on behavior (§8) | Open; owner must write new blind cases first |
| 2 | Ceiling: 8 behavior cases met every criterion with no plugin | council, navigate, postmortem, reality-check, research, review, test, validate; skill-probe rows for the judgment group are INERT, saturated or unmeasured (`evals/skill-probes/LEDGER.md`) | Open |
| 3 | Necessity splits by kind of knowledge | Runtime or product facts: claude-exec 3→14/15, memory 6→15/15, using-gc 6→12/12, skill-builder 1→8/12. General judgment barely moves: agent-native 13/15, skill-eval 12/15, codex-exec 9/12 without the plugin | Finding, no action |
| 4 | The operating charter is copied into skills | "deleting a check that" in `AGENTS.md:174` and `skills/{orchestrate,rpi,validate}/SKILL.md`; "protected external non-Git" in 6 SKILL.md files (council, doc, memory, postmortem, rpi, skill-eval) | Open; orchestrate pointer rejected as no defect |
| 5 | Validators pin prose, not behavior | `skills/rpi/scripts/validate.sh` has 10 `grep -Fq` checks; implement.feature's `@covered-by` names a comment label; test and refactor feature files allowlisted (`scripts/.scenario-linkage-allow:27,35`) | Open; the pins also block the rpi and memory cuts |
| 6 | Routing metadata tuned to the test | implement's `metadata.triggers` copy rq-07's query (`skills/implement/SKILL.md:26`); rq-09 passed on a name-order tie | rq-09 fixed in #1220; rq-07 kept by decision |
| 7 | Frontmatter is large and mostly unread by hosts | 777 of 4,502 SKILL.md lines are frontmatter including delimiters (measured). Hosts read name and description; `ao skills find` also reads `metadata.triggers`; `docs/SKILL-API.md` calls the context object declaration-only | Maintenance cost; token cost unmeasured (frontmatter injection not checked) |
| 8 | Undefined repository vocabulary | RPI, bead, HOLD, verdict.v2, CDLC, "protected external non-Git storage"; CDLC defined only in `docs/adr/ADR-0016-state-tiers.md`, which consumer repos do not have | Open |
| 9 | Cross-skill links break "one level deep" | `../rpi/references/boundaries.md` from implement, plan, review, validate; `advice-or-acceptance.md` from reality-check and validate; test → `../implement/references/difficult-failures.md`; premortem → plan challenge → agent-native model-dispatch | Open |
| 10 | Durable schemas hold less than the inline output | premortem-plan-review.v1 has no verifier or one-way fields; reality-check-report.v1 no checked/not_checked; idea-challenge.v1 two-way packet must be empty; council-report.v1 cannot label single-method agreement | Open |
| 11 | Default artifact destinations disagree | reality-check (`SKILL.md:66`) and idea-genie default to in-repo `.agents/scratch/`; council and postmortem write no file without a destination | Open |
| 12 | AgentOps-internal content ships to consumer repos | security's AgentOps gate text, memory's `ao config context`, doc's CDLC routing | domain go.md fixed in #1220; rest open |
| 13 | Script paths assume the AgentOps checkout | reverse-engineer `:77,138`; research; security | Open |
| 14 | Headless adapters drifted apart | codex-exec derives time from a deadline; claude-exec uses static `$SECS`; agy-native has no command. The retry-budget criterion is 2/3 in both claude-exec and codex-exec | Open |
| 15 | Blocking clarification on the most common request | review and validate must ask and wait on a generic "is it ready?" (`skills/review/SKILL.md:57-61`, `skills/validate/SKILL.md:73-77`) | Open |
| 16 | Rotted tests | `tests/skills/test-finding-registry-flow.sh` asserts `finding-compiler.sh` (3 matches) and other removed strings and is not in `tests/skills/run-all.sh`; `tests/claude-code/test-premortem-skill.sh` and `test-research-skill.sh` assert retired behavior | Open |
| 17 | Python on the skill path | Grandfathered: reverse-engineer 8, skill-builder 5, security 2, agent-native 1 (`scripts/.skill-python-grandfather`); agent-native's is a test fake | Ratchet holds; no new files |

## 6. Overlaps

| Overlap | Evidence | Lane fix | Debate ruling / status |
|---|---|---|---|
| navigate / orchestrate / agent-native | Shared readiness and disjoint-scope rules; orchestrate and agent-native both own restart and recovery | Merge navigate; narrow agent-native | Navigate fold deferred (§9). Only two passages are verbatim duplicates: the empty-ready-list sentence and the restated fresh-read charter |
| validate / review / reality-check | One shared `advice-or-acceptance.md`; all three at the no-plugin ceiling | Make validate the single owner | Rejected: already the doctrine (`docs/MIGRATION.md`; orchestrate calls Validate the sole owner); moving the file changes 3 links and no behavior |
| plan / review | rq-09 tied at 0.3333 each; plan won on name order (`cli/internal/skills/find.go:114-118`) | Break ties by margin; drop "review" from plan's description | Accept-modified, applied in #1220: plan metadata triggers plus a tie-aware grader. The literal fix would have made review win (0.3333 vs 0.25) |
| idea-genie duel / council duel / plan challenge | Same name, different behavior (`skills/idea-genie/SKILL.md:37`; `skills/council/references/duel.md`) | Route challenges to council or plan | Rejected: no observed defect |
| premortem / review / idea-genie | "What's wrong with this plan" matches all three; premortem vs plan reversibility is split by query wording (rq-10) | none proposed | Not debated |
| implement / test | Both require RED first, forbid weakening the oracle, report only what ran; implement's behavior case is a test request | Fold implement into test if still 0/5 | Implement cut rejected (§9) |
| implement / refactor / agent-native / orchestrate | Implement absorbed refactor-while-green, delegation and a one-shot adapter | none beyond the cut | Not debated |
| doc / memory | Both claim edits to AGENTS.md and CLAUDE.md | none proposed | Not debated |
| research / reverse-engineer | Research's "not for teardowns" clause removed; reverse-engineer still calls itself research's specialization | Restore the clause | Deferred with description re-keys |
| research / postmortem / memory / skill-builder | Research and postmortem both claim "why"; rule promotion is split across research, memory and skill-builder | none proposed | Not debated |
| security / validate / review | security's "safe to ship" is a go/no-go request Validate owns and security forbids; an informal security look-over matches review | Re-key security | Re-key rejected (§9) |
| domain / interview / test / skill-builder | interview declares `settle_domain_terms`; test links domain's `test-pyramid.md`; skill-structure guide sits under Domain | Move standards out | Not debated |
| craft-goal / navigate / interview / plan | craft-goal restates wave and ratchet rules navigate owns; interview's stop checklist restates craft-goal admission | none proposed | Not debated |
| codex-exec / claude-exec / agy-native | Same rule set, drifted (§5 #14) | Per-skill templates | Not debated |
| AgentOps vs host built-ins | refactor vs simplify; review vs code-review and security-review; skill-builder and skill-eval vs skill-creator | Measure loading with built-ins present | Deferred: needs a paid live plugin eval |

## 7. What #1220 changed in skill files

| File | Change | Check |
|---|---|---|
| `skills/plan/SKILL.md` | Output write conditional on authority; `metadata.triggers: ["write scope", "scope boundaries"]` | `bash skills/plan/scripts/validate.sh`; description line byte-identical |
| `skills/using-gc/SKILL.md` | Template `push=false open_pr=false`; publishing needs caller and repository authorization | `grep -n 'push=true' skills/using-gc/SKILL.md` matches only the conditional sentence |
| `skills/doc/references/readme-craft.md` | Mandatory banner, `--mode`/`--rewrite`/`--validate`, AskUserQuestion interview and the doc-validator-as-README-evidence step removed; three request shapes; asks once only when ambiguous | `grep -nE 'YOU MUST EXECUTE\|AskUserQuestion\|--mode=' skills/doc/references/readme-craft.md` prints nothing |
| `skills/domain/references/standards/go.md` | AgentOps-internal identifiers removed; the three rules kept | `grep -nE 'ScrubGitDiscoveryEnv\|cli/cmd/ao\|next-work\.jsonl\|ag-mjlg\|ek8v'` prints nothing |
| `skills/agent-native/agents/bulk-reader.toml` | Split sentence restored | `tests/scripts/codex-context-agents.bats` |
| `skills/agent-native/SKILL.md`, `skills/orchestrate/SKILL.md`, `skills/rpi/SKILL.md` | Stall and activity reports name the category and the observed fact; no numeric threshold | Template text only; no deterministic check covers the behavior |
| `evals/routing-probes/goldens/rq-09-scope-to-plan.json`, `scripts/check-routing-probe-goldens.sh` | Rationale corrected; a forbidden id tied with rank 1 is a MISROUTE, ties read from top_k+1 results | `tests/scripts/check-routing-probe-goldens.bats` tie cases |

The #1220 fresh read returned PASS with no defects. Its optional notes: in
readme-craft, a check-only request with no README falls into "generate from
scratch"; the hidden-tie bats witness uses a score order `ao skills find` cannot
emit; the tie window stops at top_k+1.

## 8. Routing and loading evidence (2026-10-05 plugin eval)

Source: the 2026-10-05 plugin eval report (`docs/evals/2026-10-05-plugin-eval-*.md`)
and its per-criterion scorecard under `evals/plugin-eval/scorecards/`.

**These are rule-uptake proxies, not outcomes.** The criteria were written from
each skill's own rules, by the same audit that proposed the edits, before the
edits were made (eval report, "What this shows, and what it does not"). A pass
shows that a rule landed on one request. It does not show a better result on a
real task. The only end-to-end coding pilot,
[PR #1125](https://github.com/boshu2/agentops/pull/1125), found no paired
difference.

Further caveats from the eval report:

- 3 runs per case; one criterion-run moves a five-criterion case by 0.07; no
  significance claimed.
- The 28 behavior requests also tuned the 3.10 descriptions, so behavior-case
  loading flatters 3.10. The 25 blind requests were written without sight of
  the descriptions.
- Each run had Read, Glob, Grep and Skill only, an empty working directory and
  at most 12 turns. The four runtime adapters were graded on advice text, never
  on a real run.
- Grader and subject are the same model family. The grader agreed with the
  evaluator's own three-vote judge, also same family, on 95.9% of 458 criteria.
- craft-goal, interview, postmortem and rpi are user-only and were invoked by
  slash command; the no-plugin arm saw an unknown command, so their deltas are
  confounded.
- Two criteria fail by design against the current rules: agy-native c2 and rpi
  c3. On rpi's valid criteria the result is 12/12 vs 9/12.

**Aggregates (MEASURED on the proxy):** 363/387 criteria (94%) with 3.10 vs
272/387 (70%) without the plugin. A case-clustered bootstrap run during the
ranking gave +0.23 [0.14, 0.33] over all 29 cases and +0.21 [0.13, 0.31]
without the 4 slash-invoked cases. Blind loading 29/48 (3.10) vs 11/48 (3.9.0), over the 24 blind requests whose
skill exists in both versions (claude-exec is new in 3.10).

**Noise floor.** `evals/plugin-eval/README.md` treats a delta under about 0.15
as noise. The scorecard has an empirical null: on the 16 cases where 3.9.0
never loaded its skill, delta vs no plugin ranges from −0.33 to +0.17. Against
that null, 7 model-invocable skills clearly gain: claude-exec, plan, memory,
skill-builder, using-gc, idea-genie, security. Of the 14 cases that "moved" by
0.15 or more, about 7 are expected by chance.

| Skill | With 3.10 | No plugin | Delta | Loaded, behavior (of 3) | Blind 3.10 | Blind 3.9.0 | Above 0.33 null | Text changed since the eval |
|---|---:|---:|---:|---:|---:|---:|---|---|
| craft-goal | 12/12 | 3/12 | +0.75 | by name | n/a | n/a | confounded | |
| claude-exec | 14/15 | 3/15 | +0.73 | 3 | 2/2 | new | yes | |
| plan | 15/15 | 5/15 | +0.67 | 3 | 2/2 | 0/2 | yes | #1220 |
| memory | 15/15 | 6/15 | +0.60 | 3 | 2/2 | 0/2 | yes | |
| skill-builder | 8/12 | 1/12 | +0.58 | 3 | 2/2 | 2/2 | yes | |
| interview | 12/12 | 5/12 | +0.58 | by name | n/a | n/a | confounded | |
| using-gc | 12/12 | 6/12 | +0.50 | 3 | 2/2 | 2/2 | yes | #1220 |
| idea-genie | 15/15 | 9/15 | +0.40 | 3 | 2/2 | 0/2 | yes | |
| security | 13/15 | 7/15 | +0.40 | 3 | 0/2 | 0/2 | yes | a7732fd |
| orchestrate | 10/12 | 7/12 | +0.25 | 3 | 2/2 | 0/2 | no | #1220 |
| domain | 15/15 | 12/15 | +0.20 | 3 | 2/2 | 2/2 | no | #1220 (go.md) |
| reverse-engineer | 15/15 | 12/15 | +0.20 | 3 | 2/2 | 1/2 | no | |
| skill-eval | 15/15 | 12/15 | +0.20 | 3 | 2/2 | 0/2 | no | a7732fd |
| codex-exec | 11/12 | 9/12 | +0.17 | 3 | 2/2 | 0/2 | no | |
| agent-native | 15/15 | 13/15 | +0.13 | 2 | 2/2 | 0/2 | no | #1220 |
| doc | 10/12 | 9/12 | +0.08 | 3 | 0/2 | 0/2 | no | a7732fd, #1220 (readme-craft) |
| premortem | 12/12 | 11/12 | +0.08 | 3 | 2/2 | 0/2 | no | |
| refactor | 12/12 | 11/12 | +0.08 | 2 | 0/2 | 0/2 | no | |
| agy-native | 12/15 | 11/15 | +0.07 | 3 | 2/2 | 2/2 | no | |
| council | 12/12 | 12/12 | 0.00 | 0 | 2/2 | 2/2 | no | |
| implement | 6/12 | 6/12 | 0.00 | 0 | 0/2 | 0/2 | no | a7732fd |
| navigate | 15/15 | 15/15 | 0.00 | 3 | 0/2 | 0/2 | no | |
| postmortem | 12/12 | 12/12 | 0.00 | by name | n/a | n/a | no | |
| reality-check | 15/15 | 15/15 | 0.00 | 3 | 0/2 | 0/2 | no | |
| research | 12/12 | 12/12 | 0.00 | 2 | 0/2 | 0/2 | no | 6a62d53 |
| review | 12/12 | 12/12 | 0.00 | 0 | 0/2 | 0/2 | no | |
| rpi | 12/15 | 12/15 | 0.00 | by name | n/a | n/a | no | #1220 |
| test | 12/12 | 12/12 | 0.00 | 2 | 1/2 | 0/2 | no | a7732fd |
| validate | 12/12 | 12/12 | 0.00 | 3 | 0/2 | 0/2 | no | |

- 12 skills' shipped text differs from what the eval measured: a7732fd
  (2026-10-11) edited doc, implement, security, skill-eval and test; 6a62d53
  (2026-10-10) edited research's frontmatter; #1220 edited the rest marked.
  Their rows describe the old text.
- Seven of the nine blind 0/2 skills loaded at least 2 of 3 times on their
  tuned behavior request, so their descriptions fit the tuning requests better
  than requests in general.
- Not re-run after #1220: blind routing and behavior cases.

**Deterministic routing goldens.** 13 queries in
`evals/routing-probes/goldens/`, graded by
`scripts/check-routing-probe-goldens.sh` over `ao skills find`, which ranks
lexically on name, description and `metadata.triggers` and breaks ties by name
(`cli/internal/skills/find.go:114-118`). The CI pin is the bats test "the
repository's committed goldens grade with exactly the known finding" in
`tests/scripts/check-routing-probe-goldens.bats`. Since #1220 the grader treats
a forbidden id tied with rank 1 as a MISROUTE. The #1220 lane measured rq-09 at
plan 0.6667 vs review 0.3333 and 13/13 passing; not re-run for this file. rq-07
still passes only because implement's triggers copy its query. A green golden
says the catalog ranks the declared skill; it says nothing about whether a model
loads it.

## 9. Debate rulings on consolidation

Source: the arbiter's `skills-consolidation` and `skills-revisions` decisions,
both accept-modified. Full text in [decisions.md](decisions.md).

| Proposal | Ruling | Reasons |
|---|---|---|
| Fold navigate into orchestrate | **Deferred** with exit criteria | (1) Creates new name-order ties and misroutes in simulation: "is this epic done" becomes a 3-way tie (orchestrate = reality-check = rpi = 0.1667; navigate wins it strictly today); "pick the next wave on this epic" goes to implement (0.25 vs orchestrate 0.1667). (2) Breaks frozen goal prompts that call navigate by name (`skills/craft-goal/references/goal-prompt.md:26`); README presents navigate as step 3 of the goal loop. (3) Third home for that content in about three weeks: navigate is new in 3.9.0 (2026-10-03) with content moved from craft-goal (`docs/MIGRATION.md:273-274`). (4) Skips the `REMOVED_SKILLS` tombstone that the blocking gate `contract.cathedral-cut` enforces (`scripts/check-cathedral-cut-conformance.py:54`). (5) The evidence is one ceiling case (15/15 both arms) that tests a one-pass pick and never exercised the multi-wave loop; a zero delta at ceiling is not equivalence. (6) It pushes the multi-agent frame onto a single-agent question |
| Cut implement if it still loads 0/5 | **Rejected** for now | Implement is in the CORE set of `scripts/check-skill-mesh.py:20`, in rpi's dependencies, and in `requiredSkills` in `cli/internal/gcmaintainer/gcmaintainer.go:34`. [ADR-0017](../../adr/ADR-0017-loop-as-control-flow-not-knowledge.md) names Implement as an on-demand phase. Not loading on an ordinary bug fix is the intended native default. The 0/5 precondition needs a paid live eval. A future cut needs an ADR-0017 amendment and comparative evidence |
| Delete implement's rq-07 triggers | **Rejected** | Turns the CI routing pin red (12/13, rq-07 MISS), so keeping CI green would mean editing a test pin. The grader disclaims live routing, and the triggers serve `ao skills find` discoverability for real "next wave" queries |
| Cut review if it still loads 0/5 | **Rejected** for now | `tests/scripts/check-skill-probe-coverage.bats` pins the gated denominator at 9 with review in it, "so denominator loss cannot earn coverage". Review is on the primary menu (`scripts/generate-skill-mesh.py:135`). The precondition (0/5 with host code-review and security-review present) needs paid live runs |
| Re-key security's description on "find vulnerabilities / exploitable" | **Rejected** | Security loaded 3/3 on its behavior prompt and c5 ("no ship decision") failed after loading, so the description is not the cause. The body already forbids the ship call. Words from the blind prompt would contaminate the held-out set; the behavior requests were already used for tuning |
| Make validate the single owner of acceptance (move `advice-or-acceptance.md`) | **Rejected** | Already the doctrine; the move changes 3 links and no behavior, and only prepares a review cut that is not authorized |
| Route idea-genie duel to council | **Rejected** | No observed defect; the duel modes already cross-reference; `idea-challenge.v1` has consumers; idea-genie gained +0.40 and loaded 2/2 blind |
| Fix the plan/review tie (rq-09) | **Accepted, modified; applied in #1220** | [ADR-0018](../../adr/ADR-0018-retire-goals-shared-scope.md) requires plan to carry scope's former triggers. Metadata triggers alone give plan 0.6667 vs review 0.3333 and leave the host-visible description byte-identical. The tie-aware grader turns the lesson into a check with negative witnesses. The report's literal fix (drop "review" from plan's description) would have made review win |
| Measure loading with host built-ins present | **Deferred** | Needs a paid live plugin eval |
| Re-key descriptions for blind loading (research, test, reality-check, review, implement) | **Deferred** (owner) | Only after new blind cases written without sight of the descriptions, and a paid re-run |

**Navigate fold exit criteria (all required):**

1. A non-ceiling eval of the multi-wave loop that shows no gain, reported with
   its uncertainty.
2. A live routing re-measure.
3. Strict-margin goldens for "is this epic done" and "pick the next wave on
   this epic".
4. A `REMOVED_SKILLS` entry for navigate.
5. A `docs/MIGRATION.md` deprecation one release before deletion.

As of a193b6bdf none is met: `skills/navigate/SKILL.md` exists and
`skills/catalog.json` `skill_count` is 29.

## 10. Debate rulings on other top edits

| Edit | Ruling | Reason |
|---|---|---|
| validate: proven failure returns FAIL even with an identity gap | Rejected | Order is deliberate and published (`docs/releases/2026-10-05-v3.10.0-notes.md:76-78`; `skills/validate/references/mechanics.md:55-59`); the CLI reaches the case only with explicit empty or colliding identities; both outcomes block |
| premortem: rescope Check 1 | Deferred (owner, paid) | Changes the only criterion where premortem beats no plugin (c1 3/3 vs 2/3); no deterministic check covers it; rerun the case (about $0.39) first |
| orchestrate: pointer instead of the restated charter | Optional, not done | Copy is consistent with Validate today; no defect |
| rpi 6-step rewrite; memory storage-policy move | Deferred | Conflict with validator phrase pins and active constraints |
| skill-eval, council, craft-goal, using-gc size cuts; idea-genie, skill-builder, agy-native, refactor, reality-check edits | Not taken up | Advocate classed them optional notes with no defect or measured gain |
| plan, using-gc, readme-craft, domain go.md, bulk-reader.toml | Accepted; applied in #1220 | Each contradicted its own skill body or AGENTS.md on authority or publishing, or shipped garbled or AgentOps-internal text |

The debate's acceptance test for a skill edit was a verified defect: text that
fails accepted behavior, misleads a consumer, or contradicts the skill's own
body or AGENTS.md. Token-economy cuts with no measured benefit did not pass it.
