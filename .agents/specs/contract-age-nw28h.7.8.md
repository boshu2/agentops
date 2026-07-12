```yaml
issue: age-nw28h.7.8
framework: shell
category: refactor
```

# Contract: S1 Integrated Baseline / Origin-Main Integration

## Problem

The long-running Go CLI refactor has accepted migration evidence but is not based on current `origin/main`. Integration must retain both upstream and accepted behavior without rewriting history, mutating historical seals, or representing branch-local evidence as landed completion.

## Inputs

- `planning observations` (provenance only) — admission-time SHAs, refs, divergence counts, and path sets may be recorded to explain prior planning, but are stale observations and never acceptance constants or reusable integration evidence.
- `RED_CHECKPOINT_SHA` (40-character Git object ID) — the recorded checkpoint at which the exact right-reason RED harness has passed its admission criteria, including the exact command and exit evidence.
- `PRE_INTEGRATION_SHA` (40-character Git object ID) — derived from `HEAD` only after the right-reason RED checkpoint and the subsequent documentation checkpoint have both completed; the documentation checkpoint command, exit, and resulting exact SHA are recorded.
- `RESCUE_REF` (full Git ref) — derived only after both checkpoints as `refs/heads/rescue/age-nw28h-7-8-pre-integration-<shortsha>`, where `<shortsha>` is derived from `PRE_INTEGRATION_SHA`; it is created at exactly `PRE_INTEGRATION_SHA`, and the creation command and exit are recorded.
- `origin/main` (remote-tracking ref) — a moving input fetched immediately before integration and recorded by its exact live SHA.
- `merge-base` (Git object ID) — freshly recomputed from `PRE_INTEGRATION_SHA` and the just-fetched `origin/main`.
- `two-sided changed-path evidence` (two sorted path sets plus their intersection) — independently records paths changed on `merge-base..PRE_INTEGRATION_SHA`, paths changed on `merge-base..origin/main`, and their exact intersection.
- `actual-conflict ledger` (ordered entries) — distinct from the overlap ledger and exhaustive over every conflict actually produced by the merge.
- `accepted family seal` (three exact files per family) — `case.json`, `ownership.json`, and `lineage.json` for each of beads, capabilities, claim, close, config, council-gate, doctor, done, eval, and gate.
- `accepted_sha` (40-character Git object ID) — the immutable historical acceptance SHA already recorded in each accepted family's `lineage.json`.

## Outputs

- `INTEGRATED_SHA` (40-character Git object ID) — the non-rewriting merge descendant of both `PRE_INTEGRATION_SHA` and the freshly measured `origin/main`.
- `scripts/check-go-cli-integration-baseline.sh` (executable shell checker) — a fail-closed verifier for rescue equality, ancestry, exact overlap coverage, historical seal bytes, and ten descendant receipts.
- `tests/go_cli_integration_baseline.bats` (Bats fixture) — contains exactly one test named `integration baseline rejects stale ancestry`.
- `.agents/rpi/evidence/go-cli-integration-baseline.json` (JSON receipt) — records the admission and live SHAs, dynamic divergence and merge base, the complete dynamic overlap/disposition ledger, rescue verification, immutable-seal blob evidence, and integrated result.
- `.agents/evidence/go-cli-production-hardening/descendant-revalidation-<family>.json` (ten separate JSON receipts) — one receipt for each accepted family; no aggregate receipt substitutes for any family receipt.
- implementation wave result (`DONE` or `PARTIAL`) — `DONE` is permitted only after the authorized pawl, cockpit, and direct-main landing protocol proves the exact accepted SHA reachable from the freshly fetched `origin/main`; otherwise the result is `PARTIAL` and H8 remains open.

## Invariants

1. **Checkpoint-ordered rescue and non-rewriting integration.** The worker admits the right-reason RED first, completes and records the documentation checkpoint second, then sets `PRE_INTEGRATION_SHA` to that docs-checkpoint `HEAD`. Only then does it derive `SHORTSHA`, create `RESCUE_REF=refs/heads/rescue/age-nw28h-7-8-pre-integration-$SHORTSHA` at exactly `PRE_INTEGRATION_SHA`, and record the exact creation command and exit. From integration start onward the rescue ref is never deleted or moved. `PRE_INTEGRATION_SHA` and the freshly fetched `origin/main` are both ancestors of `INTEGRATED_SHA`. Integration uses a merge commit; rebase, squash, cherry-pick replacement, force-push, and every other history rewrite are prohibited.
2. **Fresh baseline and separate exhaustive ledgers.** Immediately before the merge, the worker fetches `origin/main` and records the exact fetch command and exit, live `origin/main` SHA, merge base, divergence, both sorted changed-path sets, and their exact sorted intersection. The overlap ledger has exactly one disposition for every live intersection path and none outside it. A separate actual-conflict ledger has exactly one behavior-preserving disposition for every conflict actually produced by the merge and none invented from overlap alone. Set equality—not any frozen SHA, ref, count, or path set—is the gate.
3. **Immutable historical seal bytes.** For every accepted family in the exact ten-family set, all three protected paths `cli/testdata/compatibility-baseline/families/<family>/{case.json,ownership.json,lineage.json}` at `INTEGRATED_SHA` have the same Git blob object ID as at `PRE_INTEGRATION_SHA`. No whitespace, key ordering, digest, `accepted_sha`, or other byte may change. Formally, for every protected path `p`, `git rev-parse "$PRE_INTEGRATION_SHA:$p"` equals `git rev-parse "$INTEGRATED_SHA:$p"`. Descendant evidence must not be written into these files.
4. **Ten separate descendant revalidations.** Exactly ten family receipts exist, one each for beads, capabilities, claim, close, config, council-gate, doctor, done, eval, and gate. Each binds `family`, its unchanged historical `accepted_sha`, `PRE_INTEGRATION_SHA`, the live `origin_main_sha`, and the same `INTEGRATED_SHA`; proves `accepted_sha` is an ancestor of `INTEGRATED_SHA`; and records exit zero for both `scripts/check-go-cli-architecture.sh --family <family>` and `scripts/check-go-cli-compatibility.sh --verify-frozen --profiles default,flywheel,legacy,combined --family <family>` executed at that integrated tree. A combined or missing receipt fails.
5. **Right-reason first RED.** Before any integration or production-checker edit, `tests/go_cli_integration_baseline.bats` contains exactly one `@test "integration baseline rejects stale ancestry"`. Its body runs the Git ancestry behavior against the fetched stale branch and fails only because `git merge-base --is-ancestor origin/main HEAD` returns nonzero while `origin/main` is not an ancestor. A missing script, missing Git/Bats tool, syntax error, setup error, no-match filter, or any other harness failure is not RED.
6. **Behavior-preserving overlap and conflict resolution.** Every overlap disposition and every actual-conflict disposition retains upstream behavior and the accepted branch behavior applicable to that surface. Each ledger entry records the exact upstream SHA, exact accepted-side SHA, resolution, verification commands, and exit codes proving both upstream behavior and retained accepted behavior. Deleted legacy owners remain deleted unless the explicit architecture requires otherwise; generated files are regenerated from integrated sources; append-only provenance retains both unique histories. No H1-H7 behavior implementation, broad refactor, or unrelated cleanup may be smuggled into a resolution.
7. **Fail-closed evidence binding.** The checker and all eleven receipts bind the exact `INTEGRATED_SHA`; missing fields, malformed JSON, duplicate/missing family names, a changed live HEAD, rescue mismatch, overlap set mismatch, nonzero family proof, or historical blob drift fails the checker. Evidence created for another descendant cannot be reused.
8. **Conditional landed-only completion.** Branch-local ancestry and descendant checks prove H8 integration but do not alone prove landing. The authorized crank runner may report `DONE` and close `age-nw28h.7.8` only after the complete canonical pawl check, cockpit gate, and direct-main protocol all pass: record the exact accepted SHA; record every exact command and exit; land without changing that SHA; fetch `origin/main`; prove the accepted SHA is unchanged; and record exit zero from `git merge-base --is-ancestor "$ACCEPTED_SHA" origin/main`. If any landing step is unauthorized, absent, or nonzero, report `PARTIAL` and leave H8 open. H1-H7 remain outside this leaf's implementation scope but do not block an independently green H8 from landing or closing.

## Dynamic Remeasurement and Overlap Dispositions

Admission-era measurements, including previously observed SHAs, divergence counts, rescue names, and path sets, are historical planning evidence only. The implementation worker must not copy them into acceptance evidence as though they were live.

The dynamically recomputed overlap set is authoritative. The following ten paths were observed during planning and are retained only as historical clues for likely resolution work. They do not define the live set, impose dispositions on a different live set, or permit mechanical reuse. Every freshly observed overlap and every actual merge conflict must be analyzed from its live upstream and accepted changes; if no safe in-scope disposition exists, the merge is aborted and the work replans.

| Path | Required disposition |
|---|---|
| `cli/cmd/ao/capabilities.go` | `reconcile-explicit-owner` — retain the branch's explicit capabilities owner while incorporating applicable upstream behavior; do not resurrect displaced package-main ownership. |
| `cli/cmd/ao/capabilities_test.go` | `reconcile-tests` — retain branch architecture/compatibility coverage and upstream regressions; no assertion is silently dropped. |
| `cli/cmd/ao/codex.go` | `reconcile-runtime` — combine upstream Codex behavior with the branch's current composition boundary; no unrelated redesign. |
| `cli/cmd/ao/doctor_test.go` | `retain-legacy-deletion` — keep the branch's deleted legacy owner; prove the accepted doctor architecture/compatibility checks still cover behavior. If an upstream assertion lacks a current owner, stop and replan rather than restoring the monolith. |
| `cli/cmd/ao/gate_check_test.go` | `retain-legacy-deletion` — keep the branch's deleted legacy owner and prove accepted gate coverage; an unowned upstream assertion is a replan signal. |
| `cli/cmd/ao/root.go` | `reconcile-root` — preserve both upstream root fixes and branch composition behavior without implementing the later H3/H7 repairs in B1. |
| `cli/cmd/ao/tick.go` | `reconcile-thin-owner` — retain the branch's migrated/thin tick ownership and incorporate applicable upstream behavior without restoring superseded implementation. |
| `cli/cmd/ao/tick_test.go` | `reconcile-tests` — retain both branch migration coverage and upstream regressions, with no duplicate or silently discarded test behavior. |
| `cli/docs/COMMANDS.md` | `regenerate` — resolve from authoritative integrated command sources using the repository generator; never hand-merge generated prose. |
| `docs/provenance/ledger.jsonl` | `append-only-union` — preserve every unique record from both histories in valid JSONL order; never rewrite or drop historical events. |

For every overlap-ledger entry, required fields include `path`, `disposition`, `accepted_sha`, `upstream_sha`, `branch_change`, `upstream_change`, `resolution`, `accepted_behavior_verification_command`, `accepted_behavior_verification_exit`, `upstream_behavior_verification_command`, and `upstream_behavior_verification_exit`. The distinct actual-conflict ledger uses the same proof fields and additionally records the conflict type. An empty explanation or command, a missing exit, a nonzero proof, SHA ambiguity, overlap/conflict conflation, or ledger set mismatch is a failure.

## First-RED Fixture Contract

The exact collector/execution command is:

```bash
test "$(rg -c '^@test "integration baseline rejects stale ancestry"$' tests/go_cli_integration_baseline.bats)" -eq 1 && bats --filter 'integration baseline rejects stale ancestry' tests/go_cli_integration_baseline.bats
```

The fixture's behavioral core is:

```bash
run git -C "$REPO_ROOT" merge-base --is-ancestor origin/main HEAD
[ "$status" -eq 0 ]
```

Admission requires exactly one collector match and Bats status 1 with the failed `[ "$status" -eq 0 ]` ancestry assertion visible. After integration, the same filtered test must pass without weakening or replacing the assertion.

## Descendant Receipt Contract

Each of the ten receipt files is named exactly:

```text
.agents/evidence/go-cli-production-hardening/descendant-revalidation-beads.json
.agents/evidence/go-cli-production-hardening/descendant-revalidation-capabilities.json
.agents/evidence/go-cli-production-hardening/descendant-revalidation-claim.json
.agents/evidence/go-cli-production-hardening/descendant-revalidation-close.json
.agents/evidence/go-cli-production-hardening/descendant-revalidation-config.json
.agents/evidence/go-cli-production-hardening/descendant-revalidation-council-gate.json
.agents/evidence/go-cli-production-hardening/descendant-revalidation-doctor.json
.agents/evidence/go-cli-production-hardening/descendant-revalidation-done.json
.agents/evidence/go-cli-production-hardening/descendant-revalidation-eval.json
.agents/evidence/go-cli-production-hardening/descendant-revalidation-gate.json
```

Each JSON object contains at least:

- `schema_version: 1`
- `issue_id: "age-nw28h.7.8"`
- `family` equal to the filename family
- `historical_accepted_sha` copied from the unchanged lineage file
- `pre_integration_sha`, `origin_main_sha`, and `integrated_sha`
- `historical_seal_blobs` mapping `case.json`, `ownership.json`, and `lineage.json` to their unchanged Git blob IDs
- `accepted_sha_is_ancestor: true`
- `architecture_command`, `architecture_exit: 0`, and captured output tail
- `compatibility_command`, `compatibility_exit: 0`, and captured output tail

The integration checker rejects unknown, duplicated, absent, aggregate, stale-SHA, or nonzero receipts.

## Failure Modes

1. **Fresh `origin/main` moves or the overlap set differs from planning evidence** → record the new SHA/counts/set, recompute all dispositions, and continue only when every path remains in scope; otherwise stop and replan. Never force a frozen 126/115 or ten-path assumption.
2. **Either checkpoint is incomplete, rescue creation is early, or `RESCUE_REF` is missing, moves, or differs from `PRE_INTEGRATION_SHA`** → abort before merging; complete the checkpoints, derive the required short-SHA rescue name, and record its creation command/exit/equality. Do not silently select another rescue point. Once integration starts, any rescue movement is a failure.
3. **First RED is no-match, syntax, setup, missing-tool, or missing-script failure** → reject admission, repair only the fixture/harness, and rerun until the ancestry behavior assertion itself is RED.
4. **A merge conflict has no safe in-scope behavior-preserving disposition** → while the merge is active, run and record the exact `git merge --abort` command and exit; then record proof that `HEAD` returned exactly to `PRE_INTEGRATION_SHA` and `RESCUE_REF` still resolves exactly to that SHA. Preserve the RED fixture and blocker evidence, then replan. Do not rewrite history or widen into H1-H7.
5. **Any protected historical seal blob changes** → fail immediately and restore exact bytes from `PRE_INTEGRATION_SHA`; never edit lineage to bind the descendant.
6. **Any family proof fails or any one of the ten receipts is invalid/missing** → keep the merge branch-local, emit the exact failing receipt, and route `REFUTED -> AUTO-REDO`; do not manufacture an aggregate green receipt.
7. **Branch-local H8 proof passes but authorized landing proof is incomplete** → emit `PARTIAL`, preserve the green evidence, and leave `age-nw28h.7.8` open. If pawl, cockpit, and direct-main are authorized and all exact-SHA landing checks pass, emit `DONE` and close H8.

## Out of Scope

- Implementing any H1-H7 repair: semantic seals, recursive contracts, root purity, fail-closed tracker configuration, cancellation, shared tracker execution, or output truth.
- Broad Go CLI refactoring, package moves, presentation cleanup, performance work, or remaining-family migration.
- Editing, regenerating, rebinding, or replacing any historical accepted seal byte or `accepted_sha`.
- Rebasing, squashing, cherry-picking replacement history, force-pushing, or otherwise rewriting the branch.
- Implementing or closing H1-H7, or treating their status as part of H8's behavior acceptance.
- Treating the branch-local integrated SHA as landed without the complete pawl, cockpit, direct-main, fetch, unchanged-SHA, and accepted-SHA-on-`origin/main` proof.

## Test Cases

| # | Input | Expected | Validates Invariant |
|---|---|---|---|
| 1 | Error / first RED: freshly fetched `origin/main` is not an ancestor of pre-integration `HEAD`; run the exact collector/filter command. | Exactly one test is collected; Bats fails on the ancestry status assertion, not setup/tool/syntax/no-match. | #5 |
| 2 | Success / integration: rescue ref equals `PRE_INTEGRATION_SHA`; merge fresh `origin/main`; all dynamic overlaps have complete dispositions. | Both parents are ancestors of `INTEGRATED_SHA`; filtered Bats and the integration checker pass. | #1, #2, #6 |
| 3 | Edge / moving remote: `origin/main` changes after the planning measurement and adds or removes an overlapping path. | Live SHA, divergence, merge base, and exact overlap intersection are re-recorded; every live path gets one disposition or integration stops for replan. | #2 |
| 4 | Error / historical drift: modify one byte in any protected `case.json`, `ownership.json`, or `lineage.json`. | Checker fails on blob identity and rejects descendant evidence; the historical file is restored from `PRE_INTEGRATION_SHA`. | #3, #7 |
| 5 | Error / incomplete receipts: nine family receipts pass and the tenth is absent, duplicated, stale, aggregate-only, or nonzero. | Checker fails and rejects completion; no family or issue completion is inferred. | #4, #7 |
| 6 | Success / ten descendants: all exact ten family receipts bind one `INTEGRATED_SHA`, retain their historical `accepted_sha`, prove accepted ancestry, and record both family commands exit zero. | Descendant revalidation passes without changing any historical seal file. | #3, #4 |
| 7 | Edge / legacy deletion overlap: upstream changed `doctor_test.go` or `gate_check_test.go`, which the branch deleted. | Deletion stays only when every upstream assertion is dispositioned to accepted coverage; otherwise integration fails and replans instead of resurrecting the legacy owner. | #6 |
| 8 | Boundary / conditional landing: branch-local checker passes. | If authorized pawl, cockpit, direct-main, fetch, unchanged accepted SHA, and accepted-SHA ancestry proof all exit zero, report `DONE` and close H8; otherwise report `PARTIAL` and leave H8 open. H1-H7 status does not change this outcome. | #8 |
| 9 | Error / unsafe conflict: a live conflict has no behavior-preserving in-scope resolution. | Record `git merge --abort` and its exit; prove `HEAD == PRE_INTEGRATION_SHA` and `RESCUE_REF == PRE_INTEGRATION_SHA`; report for replan without history rewriting. | #1, #2, #6 |

## Acceptance and Handoff

Branch-local behavior acceptance is:

```bash
bash scripts/check-go-cli-integration-baseline.sh && git merge-base --is-ancestor origin/main HEAD
```

This command is necessary but not sufficient for landed completion. The implementation wave hands off its exact `INTEGRATED_SHA`, separate live overlap and actual-conflict ledgers, immutable-seal evidence, and ten family receipts. The crank runner then either:

1. records the exact accepted SHA and exact commands/exits for the authorized canonical pawl check, cockpit gate, direct-main landing, and post-push fetch; proves the accepted SHA did not change; records exit zero from `git merge-base --is-ancestor "$ACCEPTED_SHA" origin/main`; reports `DONE`; and closes H8; or
2. reports `PARTIAL` and leaves H8 open when that complete authorization or proof is unavailable.

H1-H7 remain out of implementation scope for this contract. Their incompleteness is not a blocker to landing or closing a fully proven H8.
