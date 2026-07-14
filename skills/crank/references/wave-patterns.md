# Wave Patterns

## The FIRE Loop

Crank follows FIRE for each wave:

| Phase | Beads Mode | TaskList Mode |
|-------|-----------|--------------|
| **FIND** | Resolve the accepted leaf and selected wave | Resolve the accepted task and selected wave |
| **IGNITE** | One direct `/implement` worker by default | One direct implementer by default |
| **REAP** | Collect the direct result; Swarm only for proven disjoint lanes | Same |
| **CHECK** | Deterministic wave acceptance → PASS/FAIL evidence | Same |
| **ESCALATE** | Return blocked evidence to the orchestrator | Same |

**With `--test-first` flag, FIRE extends with two pre-implementation phases:**

| Phase | Description |
|-------|-------------|
| **SPEC** | Generate contracts per issue → `.agents/specs/contract-<id>.md` |
| **TEST** | Generate failing tests from contracts → RED gate (all must fail) |

## Routine single-writer model

```text
Wave N: accepted leaf + exact next failing proof
        → one direct implementer
        → targeted deterministic acceptance
        → canonical checkpoint + remaining-plan facts
        → orchestrator pulls the next unchanged wave or freezes the tranche
```

This is the default. Do not spawn an agent merely to satisfy a workflow shape;
the current implementer may execute the wave when it owns the leaf. Use the
parallel model only when the plan explicitly proves two or more disjoint lanes.

## Explicit parallel wave model

```text
Wave N in one leaf → [disjoint lane A, disjoint lane B]
        → explicitly selected isolated writers
        → lead integrates in declared order
        → targeted deterministic acceptance
        → canonical checkpoint + remaining-plan facts
        → materially changed: Discovery → Premortem
        → unchanged and below boundary: pull Wave N+1
        → leaf complete: freeze → Validate → Learn
        → soft boundary while incomplete: PARTIAL resume evidence, stop
```

Crank never directs Wave N to Wave N+1. Every wave terminates at its evidence
handoff. The orchestrator may start the next selected wave without per-wave
Validate or Learn. Semantic validation and Learn run once only after the leaf is
complete and the bounded tranche freezes. An incomplete soft boundary does not
authorize them.

## Spec-First Wave Model (--test-first)

When `--test-first` is enabled, crank runs 4 wave types instead of 1:

```
SPEC WAVE (conditional on --test-first)
  Workers: 1 per spec-eligible issue
  Input: issue description + plan boundaries + codebase (read-only)
  Output: .agents/specs/contract-{issue-id}.md
  Gate: Lead validates completeness (all issues have contracts)
                    ↓
TEST WAVE (conditional on --test-first)
  Workers: 1 per spec-eligible issue
  Input: contract-{issue-id}.md + codebase types (NOT implementation code)
  Output: test files committed to repo
  Gate: RED confirmation — ALL new tests must FAIL
                    ↓
IMPL WAVE (standard, enhanced with GREEN mode)
  Workers: 1 per issue (full access)
  Input: failing tests + contract + issue description
  Output: implementation code
  Gate: GREEN confirmation — ALL tests must PASS + wave acceptance check
                    ↓
REFACTOR WAVE (refactor-under-green — the load-bearing quality move, not optional)
  Workers: 1 per changed file group
  Input: passing tests + implementation
  Output: diff-only cleanup, as its own commit
  Gate: All tests still PASS *and the diff changed NO test file* — a refactor
        that edits a test changed behavior; that is a new slice, not a refactor.
```

Refactor-after-green is where the quality actually comes from (test-first
*ordering* alone contributed nothing measurable — [agentic-workflow-evidence.md](../../standards/references/agentic-workflow-evidence.md)); a pipeline that defers or skips it lands in the worst-performing cluster. See the canonical [narrow-waist micro-cycle](../../../docs/architecture/operating-loop.md#the-narrow-waist-micro-cycle-canonical--every-loop-skill-cites-this).

### Category-Based Skip

Issues categorized as docs, chore, or ci skip SPEC and TEST waves entirely:
- **feature / bugfix / refactor** → full pipeline (SPEC → TEST → IMPL)
- **docs / chore / ci** → standard implementation waves only

### RED Confirmation Gate

After TEST WAVE, the lead runs the test suite. ALL new tests must FAIL:
- If a new test passes → the test validates existing behavior, not new requirements
- Tests that pass are removed or flagged for rewrite
- Only proceed to IMPL when all new tests are confirmed RED

### RED Gate Failure Recovery

When the RED gate detects unexpected test passes:

1. **Identify cause:** Tests that pass against current code validate existing behavior, not new requirements from the contract
2. **Preserve evidence:** Record the unexpected-pass list and affected contract invariants
3. **Return:** Stop the TEST action and return `BLOCKED` evidence to RPI
4. **Re-enter explicitly:** Only an orchestrator decision may dispatch another TEST action

```bash
# RED gate failure tracking
if [[ ${#UNEXPECTED_PASSES[@]} -gt 0 ]]; then
    bd comments add <issue-id> "RED GATE: ${#UNEXPECTED_PASSES[@]} tests passed unexpectedly. Returned to RPI." 2>/dev/null
fi
```

### GREEN Confirmation Gate

After IMPL WAVE, the lead runs the test suite. ALL tests must PASS:
- New tests (from TEST WAVE) must now pass
- Existing tests must still pass (no regressions)
- Standard wave acceptance check also applies

### Contract Validation

SPEC WAVE workers explore the codebase before writing contracts (not fully isolated). This prevents generic, ungrounded specs. Workers read:
- Existing types, interfaces, and patterns
- Related test files for style reference
- Module structure and dependencies

But do NOT read implementation details of the specific feature being specified.

## Wave Acceptance Check (MANDATORY)

> **Principle:** Verify each wave deterministically before handoff. The
> orchestrator reads the actual wave diff itself (Step 3.5, the
> anti-green-washing check); a green promise plus evidence JSON is never
> sufficient. Independent judgment belongs to the downstream Validate umbrella,
> not duplicate inline judges inside Crank.

**After one implementation wave, before returning to RPI:**

**Note:** SPEC WAVE has its own validation (contract completeness check) and TEST WAVE has the RED gate. The Wave Acceptance Check applies only to IMPL and REFACTOR waves.

1. **Compute wave diff** (WAVE_START_SHA recorded in Step 4):
   ```bash
   git diff $WAVE_START_SHA HEAD --name-only
   WAVE_DIFF=$(git diff $WAVE_START_SHA HEAD)
   ```

2. **Load the current leaf's affected acceptance criteria:**
   ```bash
   ao beads exec show <leaf-id>  # extract affected acceptance criteria
   ```

3. **Validate worker result evidence (FAIL-CLOSED):**

   Read the direct worker result, or each selected parallel-lane result, and
   validate its declared schema.

   Required evidence policy for IMPL/REFACTOR acceptance:
   - targeted evidence for the affected acceptance is mandatory for every wave;
     `full_suite` is reserved for the final post-repair candidate.
   - `red_green` evidence is mandatory for issues that ran through TEST WAVE (`--test-first` path).
   - Every check listed in `evidence.required_checks` must exist in `evidence.checks` and have `verdict: PASS`.

   Any one of the following sets the wave verdict to **FAIL** immediately:
   - missing result file
   - schema validation failure
   - missing required evidence
   - required evidence check with `FAIL` verdict

3.5. **Orchestrator's own diff-read (MANDATORY — the anti-green-washing check):**

   > **The orchestrator itself reads `WAVE_DIFF` before counting any slice.** A green `<promise>DONE</promise>` plus a passing evidence JSON is a *claim*, not proof of scope: a worker can emit a clean evidence file while its diff touches files outside the slice's declared boundary. The roll-up of verdicts does not catch this; only reading the actual diff does. (This is the exact check that caught the Codex pawl-embed + `validate.yml` drift — `git status` showed 7 files where 3 were expected.)

   For each issue closed in the wave, the orchestrator (not a sub-judge) attributes the files THAT slice touched **from its evidence** — the per-issue result (`.agents/swarm/results/<issue-id>.json`, already validated in Step 3) records the slice's touched-file list, the authoritative attribution. **Do NOT use `git log --grep "<issue-id>"`** — a slice that omits the id from its commit message yields an empty changeset and passes vacuously. Then check each slice's claim + write-scope:
- **Scope match (per-slice, evidence-attributed):** every file in *this slice's recorded touched-file list* falls inside *its* declared write scope. This catches a slice writing OUTSIDE its boundary — including into **another** slice's file (it is in THIS slice's evidence but not its scope → FAIL; another slice owning that path does not excuse it). Do NOT compare the full `WAVE_DIFF` against one slice's scope — multi-slice waves touch disjoint files, which would false-flag every valid parallel slice.
- **Wave coverage (union):** every file in the full `WAVE_DIFF` appears in *some* slice's recorded touched-file list. A file in **no** slice's evidence is unclaimed drift — the case that caught the Codex pawl-embed + `validate.yml` drift (files touched that no slice owns). (A diff file absent from every slice's evidence also means the evidence is incomplete, which Step 3 already fails on.)
- **Claim match:** each slice's diff actually does what it claims (not an empty or unrelated change behind a green promise).

   If a slice's evidence-files fall outside *its* scope, OR any wave file is in *no* slice's evidence, OR a claim doesn't match, the slice/wave is **flagged** — set the wave verdict to **FAIL** (do not silently count it) and surface the offending file list to the operator. This is a hard deterministic gate.

4. **Aggregate deterministic verdict:**
   - If Step 3 fails evidence validation → **FAIL**
   - If Step 3.5 flags an out-of-scope or claim-mismatched diff → **FAIL**
   - Otherwise → **PASS**

5. **Return verdict and evidence to RPI:**

   | Verdict | Action |
   |---------|--------|
   | **PASS** | Record targeted facts and return them to RPI; it may pull another unchanged wave or freeze a completed leaf. |
   | **WARN** | Record nonblocking caveats and return them to RPI; do not create an inline fix wave. |
   | **FAIL** | Record blockers and return BLOCKED evidence. The orchestrator owns any helper, retry, or re-plan. |

   Validate and Learn do not run here. They run once after the leaf is complete
   and frozen; an incomplete soft boundary persists resume evidence only.

## CI-Policy Parity Gate

> **Principle:** When a wave touches GitHub Actions workflows, the AGENTS.md CI table and the workflow's `summary.needs:` / `summary.if:` fail-set MUST stay in three-way sync. A drift means the docs lie about which jobs are blocking — exactly the failure mode that produced commit `c587b361` (manual fix codex-team applied AFTER soc-lmww1 added `factory-claim-ledger-strict (advisory)` to validate.yml without updating AGENTS.md or `summary.needs:`).

The crank orchestrator runs `scripts/validate-ci-policy-parity.sh` as a conditional acceptance gate inside Step 5.5.

### Trigger detection

```bash
# Only trigger when wave actually touches a workflow YAML file.
# CODEOWNERS-only or markdown-only changes do NOT trigger this gate.
if git diff --name-only "$WAVE_START_SHA" HEAD -- | grep -qE '^\.github/workflows/.*\.ya?ml$'; then
    bash scripts/validate-ci-policy-parity.sh || exit 1
fi
```

The grep pattern is intentionally narrow:
- `^\.github/workflows/` anchors the trigger to the workflows directory (not `.github/CODEOWNERS`, not action templates elsewhere).
- `\.ya?ml$` matches `.yml` and `.yaml` only.

### Pre-fix state (simulating the c587b361 bug)

A worker adds `factory-claim-ledger-strict` as a new advisory job in `validate.yml`:

```yaml
factory-claim-ledger-strict:
  needs: [changes]
  if: needs.changes.outputs.docs_or_workflow == 'true'
  continue-on-error: true
  runs-on: ubuntu-latest
  steps:
    - run: bash scripts/check-factory-claim-ledger.sh
```

…but does NOT add it to `summary.needs:` and does NOT add a row to the `### CI Jobs and What They Check` table in AGENTS.md.

The orchestrator's gate fires:

```
$ bash scripts/validate-ci-policy-parity.sh
CI_POLICY_PARITY: Job list drift detected (AGENTS table vs validate.yml summary.needs).
--- AGENTS jobs
+++ Workflow summary.needs jobs
@@ ... @@
+factory-claim-ledger-strict
Action: align AGENTS CI table entries or summary.needs job list.
CI_POLICY_PARITY: FAILED (1 drift group(s) detected)
$ echo $?
1
```

Wave verdict → **FAIL**. The orchestrator surfaces the validator's drift report to the operator and refuses to advance.

### Post-fix state

The worker (or a follow-up wave) adds the job to `summary.needs:`:

```yaml
summary:
  needs: [..., factory-claim-ledger-strict, ...]
```

…and adds a corresponding row to AGENTS.md. Because the job is `continue-on-error: true`, AGENTS.md must mark it `(non-blocking)`:

```markdown
| **factory-claim-ledger-strict** | … | Non-blocking (`continue-on-error: true`); … |
```

The gate now passes:

```
$ bash scripts/validate-ci-policy-parity.sh
CI_POLICY_PARITY: PASS (47 jobs; 7 non-blocking)
$ echo $?
0
```

Wave verdict resumes normal aggregation (Steps 4–6 above).

### Fix-it message format

When the gate fails, the orchestrator surfaces a concise, actionable summary to the operator:

```
[wave-acceptance] CI-policy parity drift detected (validate-ci-policy-parity exit 1).
  Drift kind: <Job list | Non-blocking policy | Blocking policy>
  Required edits:
    - <align AGENTS.md ### CI Jobs and What They Check table>
    - <align .github/workflows/validate.yml summary.needs and/or summary.if fail set>
  Re-run after fix:
    bash scripts/validate-ci-policy-parity.sh
```

### Why this gate exists

- **Self-reference precedent:** Commit `c587b361 ci(reconcile): wire factory-claim-ledger-strict into summary + AGENTS parity` is the manual fix that motivated this gate. A previous wave (`soc-lmww1`) added the advisory job without keeping the three sources in sync; codex-team patched it after the fact. PR-F (this gate) is the formalization of the recurrence-prevention rule from `finding-2026-05-07-ci-parity-as-wave-acceptance`.
- **Existing surface:** The validator (`scripts/validate-ci-policy-parity.sh`, ~186 LOC) already exists in the Go gate registry as `ci.policy-parity` (and remains covered by the legacy bash gate as a fast diff-conditional check). Wave acceptance hits the same gate earlier — at wave-close time, when fix-it cost is lowest — so drift never escapes a wave.
- **Narrow trigger:** Bypasses the gate for waves that don't touch workflows. This keeps the gate cheap and prevents false positives from CODEOWNERS, README, or non-workflow YAML changes.
