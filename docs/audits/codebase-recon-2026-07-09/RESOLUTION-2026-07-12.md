# Resolution — F-04 through F-15 (2026-07-12)

> **Current resolution record.** The sibling recon reports are a historical,
> immutable snapshot of `fbba8af5ace635104775ef18f34fef362ba368ce` from
> 2026-07-09. This page records the later current-tree reproduction and repair;
> it does not rewrite the original findings.

## Baselines and scope

- Historical audit: GPT-5 / Codex native fan-out, pinned at `fbba8af5…`; durable
  source of the 15 original findings and their raw evidence.
- Resolution base: fetched `origin/main` at
  `0f49341eb6920c555e253d3552dc00e6c4471bba` on 2026-07-12.
- Latest landing rebase: fetched and rebased onto `origin/main` at
  `3f0e38870d38` after queued landings repeatedly advanced upstream during
  validation and the mixed-range replay.
- Resolution tracker: epic `age-ghk3i`, 15 behavior-sized children for the 12
  still-open findings F-04 through F-15.
- Bead IDs and repair labels below are the stable identifiers for the repair
  arc. After every substantive commit passed the mixed-range landing replay,
  the validated arc was compacted into one atomic epic commit to remove the
  long replay race; final hashes live in the verdict/provenance ledger.
- F-01 through F-03 were already remediated on the resolution base and were not
  reopened. The table below covers only the reproduced open set.

## Finding-to-fix matrix

| Finding | Resolved behavior | Stable fix identifier(s) | Deterministic proof |
|---|---|---|---|
| F-04 | LAW 0 scans every tracked executable plus production Go, including leading-whitespace command lines; `bin/ralph` uses Codex. | `age-ghk3i.1`; `fix(age-ghk3i): close Door 9 whitespace escape` | five-case `check-door9-no-claude-p.bats`, including an indented tracked executable, and the live checker |
| F-05 | Legacy dispatch compares filesystem-real paths and rejects existing or missing-leaf symlink escapes before auth/output. | `age-ghk3i.2` | `go test -tags legacy ./cmd/ao -run 'TestResolveCodexDispatchPathRejectsSymlinkEscape\|TestResolveCodexDispatchPathBounds'` |
| F-06 | Required evidence uses explicit repository-local argv; shell grammar, path escape, nonzero exit, and excess output reject dispatch. Read-only packets with executable required commands fail closed before worker/evidence execution because arbitrary child writes cannot be portably confined. | `age-ghk3i.3`; `fix(age-ghk3i): reject unconfined read-only evidence commands` | required-command policy/receipt/lifecycle tests plus `TestCodexDispatchReadOnlyRejectsRequiredCommandsBeforeExecution`, which proves an outside marker remains absent |
| F-07 | Applicable changelog read failures are blocking failures, never SKIP. | `age-ghk3i.4` | focused `TestRunChangelogSync_ReadFailureFailsClosed` and `TestGateCheckChangelogSyncApplicableEvidenceErrorsFail` |
| F-08 | Supported scripts no longer call retired `ao hooks` or `ao daemon`; active waivers are forbidden. | `age-ghk3i.5` | `bats tests/scripts/check-scripts-ao-invocations.bats` and live checker |
| F-09 | Worker stdout/stderr are independently capped at 16 MiB; required-command streams and receipt diagnostics remain 500 bytes; Darwin/Linux kill the complete process group on limit/deadline. | `age-ghk3i.6` | six named subprocess cases in `TestCodexDispatchBoundsOutputAndKillsProcessGroup`; broader legacy dispatch suite |
| F-10 | Chaos fixtures carry independent judge context IDs and smoke accepts the hookless default posture. | `age-ghk3i.7` | `TestEvalChaosFixturesCarryIndependentContextIDs`, hookless smoke regression, compiled `ao eval chaos` |
| F-11 | Malformed or incorrectly typed committed verification policy is explicitly invalid, renders `strict=true`/`autobind=false`, and produces HOLD 5 before verification or hook writes. | `age-ghk3i.8`; `fix(age-ghk3i): hold on typed committed policy errors` | typed `strict`, `autobind`, and `review_timeout` table plus exact verify-HOLD/no-review and verify-init/no-hook tests |
| F-12 | Capabilities enumerate default executable environment inputs and publish executable HOLD exit 5 for `ao verify` and `ao pawl review`; schema and authorizer agree that only CONFIRMED or fully validated REBOUND authorizes. | `age-ghk3i.9`, `age-ghk3i.12`; `fix(age-ghk3i): publish executable HOLD exits` | environment-owner test; compiled recursive exit-code oracle; 11-case pawl verdict contract Bats suite; embedded-schema parity |
| F-13 | First-read scale and build-profile claims are measured from tracked files and compiled default/combined command trees; tagged-only commands are not presented as default. | `age-ghk3i.10`; `docs(architecture): refresh Bats inventory count` | four-case architecture drift Bats suite and live `check-architecture-doc-drift.sh` |
| F-14 | MCP uses the running filesystem-real regular executable or an injected runner; it never falls back to ambient PATH. | `age-ghk3i.11` | trusted-binary, resolver-failure, and live-transport package tests |
| F-15 | `make docs-check` invokes only present executable validators and fails when an exact recipe validator is missing. | `age-ghk3i.14` | two-case target-contract Bats suite, `make docs-check`, 3,571-link release-doc check |

## Adopted landing blockers

The integrated current base had checks that were outside the original finding
diff but blocked honest completion. They were not waived:

- `test(cli): scope archive checks to owning tags` made archive tests profile-specific so `legacy` is not mistaken
  for the combined `flywheel legacy` build.
- `refactor(cli): simplify archcheck lint paths` and
  `refactor(cli): simplify tick close path` removed four full-lint findings with
  behavior-preserving extraction under green tests; full lint reports zero.
- `fix(skills): restore self-contained Codex references` restored reference payloads exposed by the
  repaired docs gate.
- `fix(skills): link status recovery reference` linked the bespoke status recovery payload; strict skill healing
  then reported no findings.
- The local MemRL drain repaired 16 zero-sentinel citation feedback rows; the
  runtime health check then reported 0/16 residual rows. This is local runtime
  state, not a tracked source change.

## Integrated proof on the resolution branch

The following ran after all code changes were combined:

- Default `go build ./...`, `go vet ./...`, and `go test ./...`: PASS.
- `go test -tags legacy ./...`: PASS, including the complete dispatcher arc.
- `make regen-check` and `make docs-check`: PASS.
- `scripts/golangci-lint-v2.sh run ./...`: PASS, zero findings.
- Every F-04 through F-15 exact acceptance surface: PASS; the F-15 RED child
  retains its pre-implementation failing output as historical TDD evidence.
- `ao gate check --fast --scope upstream`: 76 PASS, 0 FAIL, one expected
  corpus-freshness SKIP across 77 selected checks.
- The first full upstream gate pass reached 109 PASS and exposed three closeout
  conditions: strict skill reference drift (repaired), MemRL local feedback
  state (repaired), and unrelated operator worktrees (external host state).

### Independent validation retry

Phase 3 attempt 1 correctly returned FAIL after fresh judges added adversarial
fixtures beyond the original focused tests. They reproduced four gaps: an
indented Door 9 invocation, a read-only required-command outside write, typed
verification-policy fallback, and omitted verify/pawl HOLD exits. No passing
validator evidence was minted and nothing landed.

The same epic was re-cranked test-first. Each differentiating fixture first
failed against the first integrated resolution record, then passed after the
four retry fixes listed above.
The retry tree subsequently passed the exact adversarial suite, default build,
vet and full tests, combined legacy build/vet/full tests, full Go lint with zero
issues, `make regen-check`, and `make docs-check` (3,566 links, zero broken).
Independent Phase 3 validation still owns the next verdict and the 15 declared
evidence files.

`full.worktree-disposition` remains an environment classification, not a source
defect: the five worktrees created for this resolution were removed and pruned;
the remaining attached worktrees predate or are independent of `age-ghk3i` and
are not deleted by this workflow.

### Landing pawl redo: truthful Ralph bounds

The landing pawl correctly REFUTED one surviving runtime claim: `bin/ralph`
advertised and printed `--max-budget` although its Codex subscription path
enforced only a wall-clock timeout. `fix(ralph): reject unenforceable dollar budgets`
removes the fake default,
help example, and banner. The legacy flag remains recognized but exits 2 before
state creation or Codex execution, explains that dollar budgets cannot be
enforced, and directs callers to `--phase-timeout`. Checkpoint syntax is
unchanged because Ralph never persisted `MAX_BUDGET`.

The three-case Bats contract was observed RED before implementation and GREEN
afterward: help exposes only the enforceable bound, a legacy invocation cannot
touch either the Codex marker or `.agents/ralph`, and a pre-removal checkpoint
resumes successfully. ShellCheck, the combined eight-case Ralph plus Door 9
Bats run, the live Door 9 checker, `make docs-check`, and `make regen-check` all
pass. `make local-ci` also exercised the changed-surface checks successfully;
known current-main markdown, retired-command, and skill-lint failures remain
outside this fix.

## State boundary

This page records the source resolution and its deterministic proof; it does not
infer trunk or tracker state from a local checkout. Final closure is authoritative
only when the pawl verdict is commit-bound, the landed commit is an ancestor of
remote `main`, and `age-ghk3i` is closed in the private tracker. Those mutable
facts live in the verdict/provenance ledger and tracker rather than being frozen
prematurely into this pre-land source record.
