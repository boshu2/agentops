---
name: refactor
description: Execute safe refactors.
---
# Refactor Skill

Safe, incremental refactoring with verification at every step: one transformation, one focused test run, one commit. This skill changes structure without changing observable behavior.

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

## Constraints

- **Preserve behavior, not implementation.** Establish the observable contract and a green baseline before editing because a refactor that changes output, ordering, persistence, timing, or errors is feature work.
- **Keep transformations atomic.** Make one structural change, run its focused tests, and commit it alone because batched edits hide which transformation broke the contract.
- **Require an acceptance surface.** Add a characterization test before touching uncovered code and measure hot paths before refactoring them because untested or unmeasured preservation claims are guesses.
- **Respect ownership and public surfaces.** Coordinate around active code and sweep cross-language callsites before removing symbols because compile-local success misses scripts, workflows, docs, and external consumers.
- **Consult the pawl before raising the andon.** WARN, FAIL, or REFUTED results repair and rerun automatically because ordinary rejection is evidence about the transformation; only a breaker may enter HOLD or consume the helper lane.

## Breaker State Machine

- **Ordinary rejection — `WARN|FAIL|REFUTED -> AUTO-REDO`:** revert or narrow the transformation, repair the named defect, and rerun the focused plus full checks; plain rejection never enters HOLD and never consumes the helper lane.
- **Breaker — `BREAKER -> HOLD -> ONE-HELPER`:** freeze edits when behavior cannot be proved or the safe boundary is ambiguous, then route exactly one bounded helper consultation with the baseline, diff, and failing evidence.
- **Recovered — `HELPER-UNSTUCK -> AUTO-REDO`:** leave HOLD, apply the bounded recovery, and re-earn focused tests, full tests, and the pawl verdict.
- **Helper escalation — `HELPER-ESCALATE -> HUMAN`:** stop automation and send the helper-provided evidence packet to the operator.
- **Direct human lane — `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`:** skip the helper and route directly to the operator; these are the only direct-human states.

## Modes

- **Target (default):** `$refactor <file-or-function>` improves a named unit.
- **Sweep:** `$refactor --sweep <scope>` ranks hotspots, then handles the worst safe target first. Scope may be a path, package, or `all` with explicit caution.
- **Extract:** `$refactor --extract method:<name>|module:<file>|class:<name>` moves one cohesive responsibility behind a clear interface.
- **Simplify:** load [behavior-preserving-simplification.md](references/behavior-preserving-simplification.md) for de-slop, indirection removal, or readability work.

Sweep analysis uses `radon cc <path> -a -s` and `radon mi <path> -s` for Python or `gocyclo -over 10 <path>` for Go. With no scope, inspect recent Python/Go changes. Treat CC 1–5 as simple, 6–10 manageable, 11–20 a candidate, 21–30 urgent, and 31+ critical. Detailed transformation patterns and safety notes live in [behavior-preserving-simplification.md](references/behavior-preserving-simplification.md).

## Core Loop

```text
Baseline -> one transformation -> focused test
                               |-> PASS: commit -> next
                               `-> FAIL: revert -> narrow -> retry
All transformations -> full suite -> metrics -> summary
```

Never debug on top of a broken refactor. Revert the structural change first, reread the violated contract, and retry with a smaller transformation. A newly discovered behavior bug becomes a separate bug-fix task.

## Execution Workflow

### Step 0: Establish a green baseline

Run the full suite for the target scope before editing, for example `cd cli && go test ./...` or `pytest`. Record command, pass/fail/skip counts, duration, and the base SHA.

If an ambient test is red, reproduce it on the untouched base and exclude the failing area explicitly; otherwise stop this refactor. Never claim a pre-existing failure as caused or fixed by the structural change.

### Step 1: Analyze the target

Record:

- observable inputs, outputs, errors, ordering, persistence, and timing;
- complexity, function length, parameters, nesting, duplication, and naming;
- focused tests covering the target and any missing characterization tests;
- callers across source, shell, workflows, docs, skills, and tests;
- risk (`low|medium|high`) and active-owner collision risk.

For a CLI command, flag, exported symbol, or cross-language surface removal, run `scripts/check-removed-symbol-refs.sh -- <symbol>` and justify every deliberate exclusion.

### Step 2: Plan atomic transformations

Write an ordered list. Each item names exactly one structural change, its focused test, expected metric delta, risk, and dependency. If a step cannot be reviewed or reverted alone, split it again. Write characterization tests before the first uncovered transformation.

### Step 3: Execute one transformation

For each plan item:

1. Apply only that change.
2. Run the named focused tests immediately.
3. On red, revert the transformation and enter AUTO-REDO with a smaller approach.
4. On green, inspect the diff for behavioral changes and commit `refactor(<scope>): <description>`.

**Checkpoint:** after each transformation, compare the focused-test result with the recorded baseline before committing; after the final transformation, require the full suite and before/after metrics before writing the summary.

### Step 4: Verify the complete refactor

Run the full suite, the relevant static analysis, and complexity analysis on changed files. Compare baseline and final pass/fail/skip counts. Measure complexity, lines, functions, nesting, and any performance-sensitive benchmark. Tests added as characterization must pass against both old and new implementations.

### Step 5: Write and validate the summary

Write `.agents/refactor/YYYY-MM-DD-refactor-<scope>.md`. Record targets, before/after metrics, each transformation with its commit SHA, baseline/final tests, and learnings. Then run the validator from the Output Specification.

## Output Specification

- **Path:** `.agents/refactor/YYYY-MM-DD-refactor-<scope>.md` in the active repository.
- **Filename convention:** `YYYY-MM-DD-refactor-<scope>.md`, where `<scope>` contains only letters, digits, dots, underscores, or hyphens.
- **Serialization/schema format:** Markdown with one `# Refactor:` title and exact `## Targets`, `## Metrics`, `## Transformations Applied`, `## Tests`, and `## Learnings` sections; mode, file count, metric rows, and commit-bound transformations are required, and the unique baseline/final lines each use `<N> passing, <N> failing, <N> skipped`.
- **Validator command:** run `bash skills/refactor/scripts/validate-summary.sh ".agents/refactor/YYYY-MM-DD-refactor-<scope>.md"`.
- **Downstream handoff:** send the validated summary path, transformation commit SHAs, focused/full test commands, and before/after metric deltas to review or closeout; a breaker packet stays in HOLD and follows the state machine above.

## Quality Checklist

- Each committed transformation is behavior-preserving, independently reviewable, and protected by a focused test that passed before the commit.
- The final full suite is at least as green as the recorded baseline, with any ambient failures reproduced on the untouched base and explicitly excluded from the claim.
- Before/after metrics show the intended structural improvement without moving complexity into an unmeasured helper or abstraction.
- Ordinary rejection remains in AUTO-REDO; HOLD has exactly one helper, and operator escalation is limited to the declared human states.

## Guardrails

- Do not refactor code you do not understand, uncovered code without first adding tests, active code without coordination, or hot paths without benchmarks.
- Stop at diminishing returns, test instability, behavior change, scope creep, or the declared timebox.
- A growing diff, broad test rewrites, preference-only renames, or changed external output means the transformation is too large or not a refactor.
- For renames, search all references including strings and docs; exported API renames are breaking changes unless explicitly migrated.
- For extractions, make inputs, outputs, errors, state, and side effects explicit; avoid circular imports and hidden initialization-order changes.
- For dead code, rule out reflection, interfaces, build tags, generated callers, and external consumers before deletion.

## References

- [references/behavior-preserving-simplification.md](references/behavior-preserving-simplification.md) — simplification contract plus extract, rename, inline, conditional, parameter, and dead-code patterns with safety checks.
- [references/refactor.feature](references/refactor.feature) — executable scenarios for atomic transformations, revert-on-red, target, and sweep behavior.
- `$standards` — language conventions; `$validate` — post-refactor quality; `$implement` — switch here when behavior must change.
