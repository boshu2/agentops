# Proposed bead manifest — AgentOps-native skill adoption

Status: draft until independent review passes. No tracker write precedes that
review. Every behavior child below owns exactly one frozen Given/When/Then and
one executable acceptance test; no child may close on another child's evidence.

## Epic

**Title:** Own valuable external-skill behaviors and mesh portable factory adapters

**Acceptance:** B1.1–B10.2 and the clean-room closure test pass unchanged; the
100 official and 25 companion packages have one audited disposition each;
generated projections agree; a commit-bound membrane verdict passes; the arc is
landed and pushed to `main`.

**Non-goals:** no copied external bodies, `repo-recon`, second operating loop,
automatic GC/NTM fallback, Agent Mail single-writer tax, second graph store, or
runtime city/session/mail mutation.

## One-scenario vertical slices

Each row is the complete acceptance nucleus copied into its eventual child
description. `Test` names the unchanged red-first test. `Scope` is exclusive
write ownership; rows sharing a scope are sequential.

| ID | Given / When / Then | Test | Scope | Depends |
|---|---|---|---|---|
| C01 B1.1 portfolio | Given an open-ended question and readable repo truth; when `idea-genie` explores; then it separates evidence/assumptions/mechanisms, reconciles overlap, saturates adaptively, and writes `idea-portfolio.v1` without BDD persistence. | `idea-genie produces an evidence-grounded idea-portfolio artifact` | `skills/idea-genie/**`, schema/fixture if added | C20 |
| C02 B1.2 no work | Given every candidate overlaps or lacks support; when `idea-genie` evaluates; then it returns evidenced no-new-work and creates no fixed-count padding. | `idea-genie can return no-new-work without manufacturing candidates` | `skills/idea-genie/**` | C01 |
| C03 B2.1 challenge | Given a contested one-way door; when `dueling-idea-genies` runs; then sealed contexts cross-review by dimension, preserve dissent/refutations, and hand `idea-challenge.v1` to plan-pawl without deciding. | `dueling-idea-genies emits a sealed challenge packet for plan-pawl` | `skills/dueling-idea-genies/**`, plan/discovery route refs | C01,C20 |
| C04 B2.2 reversible | Given a cheap reversible choice; when the door is classified; then it routes to one context/`idea-genie` without mandatory NTM, Agent Mail, or panel ceremony. | `dueling-idea-genies routes reversible choices without NTM ceremony` | `skills/dueling-idea-genies/**` | C03 |
| C05 B3.1 recon | Given a repository and source precedence; when `codebase-recon` runs; then it traces entry/domain/integration/test flows and emits evidence-bounded fact/inference/unknown claims and coverage. | `codebase-recon validates evidence-bounded fact inference unknown claims` | `skills/codebase-recon/**` | C20 |
| C06 B3.2 delta | Given a prior recon pack; when recon repeats; then it verifies the baseline and emits a delta while preserving valid evidence. | `codebase-recon requires a verified delta when a prior pack exists` | `skills/codebase-recon/**`, recon route refs | C05 |
| C07 B4.1 promote | Given three independent exemplars; when `pattern-mining` evaluates; then it separates invariants/variation/incidental details, passes all examples plus holdout/back-application, and routes through `operationalize`. | `pattern-mining promotes only a three-exemplar holdout-proven pattern` | `skills/pattern-mining/**`, `skills/operationalize/**` | C20 |
| C08 B4.2 hypothesis | Given fewer than three exemplars or failed holdout; when evaluated; then it records a bounded hypothesis and cannot route to reusable packaging. | `pattern-mining keeps weak evidence as a hypothesis` | `skills/pattern-mining/**` | C07 |
| C09 B5.1 NTM worker | Given a role/workspace/restart contract where attachable panes help; when the NTM `AgentWorker` starts; then readiness precedes delivery, engagement is observed, robot state/evidence are available, Agent Mail supplies scoped identity/reservation/ACK handoff only for multiple lanes, and retirement is clean. | `NTM AgentWorker executes the real robot spawn send observe lifecycle` | `cli/internal/adapters/{agentworker_ntm,agentmail_cli}/**`, `cli/internal/ports/agent_mail.go`, `skills/{ntm,agent-mail,agent-native}/**` | C20 |
| C10 B5.2 supervisor | Given a worker stops showing engagement; when supervised; then it becomes suspect, receives one bounded nudge, and is replaced only after the next failed observation. | `agent lifecycle uses suspect then bounded nudge then replacement` | `cli/internal/agentworker/**`, `skills/agent-native/**` | C09 |
| C11 B6.1 review lane | Given ao pawl needs independent evidence; when `pawl-review` sends immutable `ReviewRequestV1`; then a fresh read-only NTM lane echoes its nonce and returns `ReviewLaneResultV1`, while ao pawl alone decides/binds. | `pawl-review returns a fresh read-only nonce-bound NTM lane result` | `cli/internal/ports/review_lane.go`, `cli/internal/adapters/reviewlane_worker/**`, `skills/pawl-review/**` | C09,C20 |
| C12 B6.2 transport | Given the required reviewer evidence is unavailable; when a deadline/breaker trips; then transport failure remains nonsemantic and cannot create CONFIRMED, REFUTED, or a lowered tier. | `review transport loss cannot become semantic REFUTED or CONFIRMED` | review-lane port/adapter and `skills/pawl-review/**` | C11 |
| C13 B7.1 GC composition | Given the operator chose GC and a bounded quest role benefits from panes; when GC delegates through portable worker/review ports; then AgentOps owners receive the same contract, optional AM coordinates, and GC remains close door with no fallback. | `using-gc exposes optional worker and review-lane composition` | `skills/using-gc/**`, `skills/gc-membrane/**`, pack docs | C10,C12,C20 |
| C14 B7.2 GC verdict | Given a GC reviewer panel completed; when the real pack finalizer/close gate runs; then terminal output validates as `pawl-verdict.v1` with contained nonempty evidence, degradation writes only a nonsemantic attempt artifact, and docs admit native GC may consume that failed check attempt. | `GC real finalizer emits canonical verdicts with contained nonempty evidence` | `packs/agentops-membrane/membrane/{finalize,close-gate}*`, pack tests/docs | C12 |
| C15 B7.3 independence | Given only GC or only NTM exists; when the selected factory runs; then it remains operable alone and loop/membrane contracts do not vary by substrate. | `GC and NTM remain independently selectable adapters` | composition metadata/docs and mesh checker | C13,C14 |
| C16 B8.1 ATM migration | Given ATM-era wrappers overlap live owners; when migrated; then `using-atm` merges into `agent-native`, `pre-land-refuters` aliases `pawl-review`, callers/twins route to live owners, and every old behavior retains one owner. | `ATM-era callers migrate to agent-native and pawl-review` | disposition ledger, affected skill/router/profile sources, removed source/twins | C10,C12,C20 |
| C17 B8.2 boundaries | Given NTM and Agent Mail are external executables; when skills drive them; then live capability discovery precedes writes, executable contracts outrank remembered syntax, and AgentOps claims no binary ownership. | `canonical skills keep NTM and Agent Mail as external adapters` | `skills/{ntm,agent-mail,agent-native,pawl-review}/**`, boundary checker | C16 |
| C18 B9.1 reachability | Given each new skill is admitted; when maps rebuild; then an existing entry point names its trigger, the leaf declares dependencies/handoff, direction is acyclic, and users need not know leaf names. | `every admitted new capability is reachable from an existing entry point` | consumer skill frontmatter/body, mesh checker | C02,C04,C06,C08,C12,C15,C17,C20 |
| C19 B9.2 delegation | Given multiple entry points reach a leaf; when wired; then callers state only decision boundary/artifact handoff and do not copy the leaf workflow; maps/ledger agree. | `entry points delegate without copying leaf workflows` | clean-room/mesh checker, consumer route refs | C18 |
| C20 B10.1 generated graph | Given every live skill has frontmatter; when regeneration runs; then catalog and existing context-map contain every node once with source-derived typed dependency/context edges and diagnostics; `ao skills graph` reads that projection, and `user-invocable` cannot turn an orphan into a root. | `existing catalog context-map and ao graph regenerate every live skill` | catalog/schema/generators, `cli/internal/skills/**`, context-map projection | none |
| C21 B10.2 topology | Given a skill is added/retired/rewired; when check mode runs; then stale output names regeneration and duplicate/dangling-context-or-dependency/cycle/unreachable-non-root topology fails while only explicit graph roots remain zero-inbound. | `graph topology rejects duplicates dangling cycles and unreachable non-roots` | graph validator/tests, regen/check wiring | C20 |
| C22 arc clean room | Given captured manifests and original drafts; when the clean-room gate runs; then planted shared expression is rejected, source/mesh duplication is bounded, and a receipt records local comparison limits. | `clean-room gate rejects planted copied text and validates captured manifests` | similarity checker, provenance receipt, audit manifests | C19,C21 |

## Exactly-one-test commands and coverage receipt

Each child sets `name='<exact test name>'` and runs:

```bash
test "$(bats --count --filter "^${name}$" tests/scripts/agentops-native-skills.bats)" -eq 1 \
  && bats --filter "^${name}$" tests/scripts/agentops-native-skills.bats
```

The count guard prevents Bats' zero-match `1..0` success from becoming a false
green. The exact names, in C01–C22 order,
are:

```text
idea-genie produces an evidence-grounded idea-portfolio artifact
idea-genie can return no-new-work without manufacturing candidates
dueling-idea-genies emits a sealed challenge packet for plan-pawl
dueling-idea-genies routes reversible choices without NTM ceremony
codebase-recon validates evidence-bounded fact inference unknown claims
codebase-recon requires a verified delta when a prior pack exists
pattern-mining promotes only a three-exemplar holdout-proven pattern
pattern-mining keeps weak evidence as a hypothesis
NTM AgentWorker executes the real robot spawn send observe lifecycle
agent lifecycle uses suspect then bounded nudge then replacement
pawl-review returns a fresh read-only nonce-bound NTM lane result
review transport loss cannot become semantic REFUTED or CONFIRMED
using-gc exposes optional worker and review-lane composition
GC real finalizer emits canonical verdicts with contained nonempty evidence
GC and NTM remain independently selectable adapters
ATM-era callers migrate to agent-native and pawl-review
canonical skills keep NTM and Agent Mail as external adapters
every admitted new capability is reachable from an existing entry point
entry points delegate without copying leaf workflows
existing catalog context-map and ao graph regenerate every live skill
graph topology rejects duplicates dangling cycles and unreachable non-roots
clean-room gate rejects planted copied text and validates captured manifests
```

The child description substitutes its line verbatim into the command; anchors
make accidental multi-test matches fail the count check below. Before tracker
creation the packet produced this coverage receipt:

```text
frozen Scenario lines: 21
scenario-tagged Bats tests (B1.1–B10.2): 21
behavior rows C01–C21: 21
arc closure rows: 1
full Bats plan: 1..22
exact anchored child commands with count=1: 22/22
first red: 22 failed, 0 passed, exit 1
```

Recompute it with:

```bash
test "$(rg -c '^Scenario:' docs/plans/agentops-native-skill-adoption/behaviors.md)" -eq 21
test "$(rg -c '^# B[0-9]+\.[0-9]+$' tests/scripts/agentops-native-skills.bats)" -eq 21
test "$(rg -c '^\| C(0[1-9]|1[0-9]|2[01]) ' docs/plans/agentops-native-skill-adoption/beads-manifest.md)" -eq 21
test "$(rg -c '^\| C22 ' docs/plans/agentops-native-skill-adoption/beads-manifest.md)" -eq 1
while IFS= read -r name; do
  test "$(bats --count --filter "^${name}$" tests/scripts/agentops-native-skills.bats)" -eq 1
done < <(awk 'capture && /^```$/{exit} capture{print} /^```text$/{capture=1}' \
  docs/plans/agentops-native-skill-adoption/beads-manifest.md)
bats tests/scripts/agentops-native-skills.bats
```

## Per-child closure contract

Every child description also carries these fields verbatim:

- **Non-goals:** do not change another row's owned behavior; do not start live
  NTM/GC/Agent Mail state; do not edit generated files except via regeneration.
- **First red:** the named test failed in the `1..22` receipt dated 2026-07-09.
- **Evidence for done:** named test green plus focused unit/skill audit for its
  owned paths; attach command, exit code, and artifact path to the bead.
- **Rollback:** `git revert <this-slice-commit>`, run `make regen-all`, then rerun
  the named test and every dependent child test. If the row retires a skill,
  the revert restores its source and disposition together.
- **Next action:** implement the smallest vertical path that makes only this
  scenario green; never close on docs or compilation alone.

## Execution order

`C20 → C21` establishes the generated mesh contract. Independent leaf pairs may
then proceed only when their write scopes do not overlap; shared generator and
consumer routing changes stay sequential. `C22` is last. The epic closes only
after every child carries its own evidence and the independent membrane verdict
is bound to the final commit.
