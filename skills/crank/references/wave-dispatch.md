### Step 3b: SPEC WAVE (--test-first only)

**Skip if `--test-first` is NOT set or if no spec-eligible issues exist.**

For each spec-eligible issue (feature/bug/task):
1. **TaskCreate** with subject `SPEC: <issue-title>`
2. Worker receives: issue description, plan boundaries, contract template (`skills/crank/references/contract-template.md`), codebase access (read-only)
3. Worker generates: `.agents/specs/contract-<issue-id>.md`
4. **Validation:** files_exist + content_check for `## Invariants` AND `## Test Cases`
5. **Wave 1 spec consistency checklist (MANDATORY):** run `skills/crank/references/wave1-spec-consistency-checklist.md` across all contracts in this wave. If any item fails, preserve the failed-item and affected-issue evidence, return it to the RPI orchestrator, and do NOT proceed to TEST WAVE. Any later SPEC work requires a new canonical orchestrator disposition and durable RPI admission before worker dispatch.
6. Lead commits all specs after validation

For BLOCKED recovery and full worker prompt, read `skills/crank/references/test-first-mode.md`.

### Step 3c: TEST WAVE (--test-first only)

**Skip if `--test-first` is NOT set or if no spec-eligible issues exist.**

**Lifecycle integration:** If `--no-lifecycle` is NOT set, delegate test generation to `/test`:

For each spec-eligible issue:
1. **TaskCreate** with subject `TEST: <issue-title>`
2. Worker receives: contract-<issue-id>.md + codebase types (NOT implementation code)
3. Worker generates failing tests via:
   ```
   Skill(skill="test", args="tdd <issue-description> --levels <test_levels>")
   ```
   If `/test` is unavailable or `--no-lifecycle` is set, workers generate tests inline (original behavior).
   - Workers classify generated tests by pyramid level: L0 (contract), L1 (unit), L2 (integration), L3 (component)
   - If `test_levels` metadata exists on the issue, workers MUST generate tests at each required level
4. **RED Gate:** Lead runs test suite — ALL new tests must FAIL
5. Lead commits test harness after RED Gate passes

For RED Gate enforcement and retry logic, read `skills/crank/references/test-first-mode.md`.

**Summary:** SPEC WAVE generates contracts from issues → TEST WAVE generates failing tests from contracts → RED Gate verifies all new tests fail before proceeding. Docs/chore/ci issues bypass both waves.

### Step 3b.1: Build Context Briefing (Before Worker Dispatch)

```bash
if command -v ao &>/dev/null; then
    ao context assemble --task='<epic title>: admission $RPI_ADMISSION_ID'
fi
```

This produces a 5-section briefing (GOALS, HISTORY, INTEL, TASK, PROTOCOL) at `.agents/rpi/briefing-current.md` with secrets redacted. Include the briefing path in each worker's TaskCreate description so workers start with full project context.

Worker prompt signpost:
- Claude workers should include: `Knowledge artifacts are in .agents/. See .agents/AGENTS.md for navigation. Use \`ao lookup --query "topic"\` for learnings.`
- Codex workers cannot rely on `.agents/` file access in sandbox. The lead should search `.agents/learnings/` for relevant material and inline the top 3 results directly in the worker prompt body.

### Step 3b.2: Load Shared Task Notes (Before Worker Dispatch)

Read `.agents/crank/SHARED_TASK_NOTES.md` and inject its contents into every worker's TaskCreate description (after the issue body). Include a `DISCOVERY REPORTING` instruction so workers report new findings for the orchestrator to harvest. See [shared-task-notes.md](shared-task-notes.md) for the injection template, size management rules, and discovery reporting format.

### Step 3b.3: Parallel-Wave Isolation (wave size ≥ 2)

**Skip if wave has only 1 worker.** Parallel workers in a shared clone can clobber each other's staged work when sibling workers run `git checkout` mid-task. Three-tier protection (prompt rule → conditional ephemeral worktrees → disposition gate) prevents this without re-introducing worktree sprawl. Read [parallel-wave-isolation.md](parallel-wave-isolation.md) for the full tier definitions, the worker prompt template, the `preflight-swarm.sh` escalation criterion, and the `check-worktree-disposition.sh` cleanup gate.

### Step 4: Execute Wave via Swarm

**GREEN mode (--test-first only):** If `--test-first` is set and SPEC/TEST waves have completed, modify worker prompts for spec-eligible issues:
- Include in each worker's TaskCreate: `"Failing tests exist at <test-file-paths>. Make them pass. Do NOT modify test files. See GREEN Mode rules in /implement SKILL.md."`
- Workers receive: failing tests (immutable), contract, issue description
- Workers follow GREEN Mode rules from `/implement` SKILL.md
- Docs/chore/ci issues (skipped by SPEC/TEST waves) use standard worker prompts unchanged

**Issue typing + file manifests (REQUIRED):** Include `metadata.issue_type` plus a `metadata.files` array in every TaskCreate. `issue_type` feeds active constraint applicability and validation policy; `files` feed swarm's pre-spawn conflict detection. Two workers claiming the same file in the same wave get serialized or worktree-isolated automatically. Derive both from the issue description, plan, or codebase exploration during planning.
This is the shift-left edge of the prevention ratchet: compiled findings target issue type plus changed files, so missing `metadata.issue_type` weakens enforcement back into guesswork.

**Grep-for-existing-functions (REQUIRED for new function issues):** When an issue description says "create", "add", or "implement" a new function/utility, include `metadata.grep_check` with the function name pattern. Workers MUST grep the codebase for existing implementations before writing new code. This prevents utility duplication (e.g., `estimateTokens` was duplicated in context-orchestration-leverage because no grep check was specified).

**Validation metadata policy (REQUIRED):** For implementation tasks typed `feature|bug|task`, include `metadata.validation.tests` plus at least one structural check (`files_exist` or `content_check`). `docs|chore|ci` use an explicit test-exempt path and should still include applicable structural and/or command/lint checks. Do not omit `metadata.issue_type` and hope task-validation can infer it later. When `/plan` includes `test_levels` metadata in the issue, carry it forward into `metadata.validation.test_levels` so workers know which pyramid levels (L0–L3) to target. See the test pyramid standard (`test-pyramid.md` in the standards skill) for level definitions.

**Acceptance-criteria injection (REQUIRED, pre-spawn):** Before each wave, read the bead's `acceptance_criteria` fenced YAML block from `bd show <bead-id>` body and inject it into each worker's TaskCreate description as `metadata.validation.acceptance_criteria`. Workers consume this list to know which gates apply to their task. The criterion shape is defined in `schemas/execution-packet.schema.json` (`$defs/Criterion`) — `id`, `description`, `check_type` (closed enum: `test_pass | command_exit_zero | file_exists | grep_match | manual | council_judge | custom_rubric`), `check_command`, `evidence_path`, `evidence_required`, `weight`, `optional`, and `agent_judge` (required when `check_type == "custom_rubric"`). Do not paraphrase or filter the block; pass it through verbatim so worker-side verdicts map back to plan-side ids.

**Language Standards Injection (REQUIRED for code tasks):** Detect project language from repo root markers (`go.mod`, `pyproject.toml`, `Cargo.toml`, `package.json`) and load the matching standard from the standards skill. For `feature|bug|task` issues, include the Testing section verbatim in each worker's task description. For test-modifying issues, also inject file naming and assertion quality rules.

**Validation block extraction (beads mode):** Extract validation metadata from each issue's fenced `validation` block (written by `/plan`). If no block found, fall back to `files_exist` from mentioned file paths. Inject into `metadata.validation` of each TaskCreate.

**Display file-ownership table (from swarm Step 1.5):**

Before spawning, verify the ownership map has zero unresolved conflicts:

```
File Ownership Map (Wave $wave):
┌─────────────────────────────┬──────────┬──────────┐
│ File                        │ Owner    │ Conflict │
├─────────────────────────────┼──────────┼──────────┤
│ (populated by swarm)        │          │          │
└─────────────────────────────┴──────────┴──────────┘
Conflicts: 0
```

**If conflicts > 0:** Do NOT invoke `/swarm`. Resolve by serializing conflicting tasks into sub-waves or merging task scope before proceeding.

**BEFORE each wave, atomically admit it:**
```bash
: "${RPI_RUN_ID:?RPI run id is required}"
: "${RPI_GOVERNOR_STATE_DIR:?persistent governor state dir is required}"
: "${RPI_REVIEWER_TOKENS:?reviewer-token meter is required}"
: "${RPI_ELAPSED_SECONDS:?elapsed-time meter is required}"
: "${RPI_REVIEW_CONTEXTS:?review-context meter is required}"
: "${RPI_DETERMINISTIC_EXECUTIONS:?deterministic-execution meter is required}"

ADMISSION_JSON="$(python3 skills/rpi/scripts/run-governor.py admit \
  --state-dir "$RPI_GOVERNOR_STATE_DIR" \
  --run-id "$RPI_RUN_ID" \
  --action crank-wave \
  --reviewer-tokens "$RPI_REVIEWER_TOKENS" \
  --elapsed-seconds "$RPI_ELAPSED_SECONDS" \
  --review-contexts "$RPI_REVIEW_CONTEXTS" \
  --deterministic-executions "$RPI_DETERMINISTIC_EXECUTIONS")" || {
    printf '%s\n' "$ADMISSION_JSON"
    echo "<promise>BLOCKED</promise>"
    exit 1
  }

test "$(jq -r '.authorized' <<<"$ADMISSION_JSON")" = true || exit 1
RPI_ADMISSION_ID="$(jq -r '.admissions[-1].id' <<<"$ADMISSION_JSON")"
WAVE_START_SHA="$(git rev-parse HEAD)"
```

The governor fsyncs and atomically replaces the run state before returning the
receipt. Only then may Crank dispatch. Fresh invocations reuse the same run ID
and state directory, so the default three admissions cannot reset. Missing or
corrupt state, a missing meter, or any hard-ceiling refusal fails closed; Crank
must not create a local fallback counter or helper.

**Pre-Spawn: Spec Consistency Gate**

Prevents workers from implementing inconsistent or incomplete specs. Hard failures (missing frontmatter, bad structure, scope conflicts) block spawn; WARN-level issues (terminology, implementability) do not.

```bash
if [ -d .agents/specs ] && ls .agents/specs/contract-*.md &>/dev/null 2>&1; then
    bash scripts/spec-consistency-gate.sh .agents/specs/ || {
        echo "⚠️ Spec consistency check failed — fix contract files before spawning workers"
        exit 1
    }
fi
```

**Cross-cutting constraint injection (SDD):**

Before spawning workers, extract cross-cutting constraints from the plan's `## Boundaries` / `## Cross-Cutting Constraints` section and inject into every TaskCreate's `metadata.validation.cross_cutting` array. Each entry has `name`, `type` (e.g., `content_check`), `file`, and `pattern`. "Ask First" boundaries are annotation-only in auto mode.

**Backend dispatch ladder (NTM > runtime-native > beads floor):**

Dispatch the wave per the canonical ladder in `skills/shared/SKILL.md` ("Selection policy"): prefer **NTM** (capability-probed via `ntm --robot-capabilities`), then **runtime-native** via `/swarm` (Claude Native Teams / Codex sub-agents), with the `AGENTOPS_ORCHESTRATION=off` opt-out degrading to the beads floor. Output-contract parity is unchanged: workers write `.agents/swarm/results/*.json`, the lead verifies-then-trusts.

> **gc pool is NOT selected (DEPRECATION).** gc tier removed (soc-2rtm0); retained for historical reference only — NOT selected. The Gas City (`gc`) CLI bridge was removed and `runtime=gc` is rejected by the CLI (see `agentops/CLAUDE.md`). [gc-pool-dispatch.md](gc-pool-dispatch.md) documents the old gc pool dispatch shape for archival purposes only — the top tier is **NTM**.

**For wave execution details (beads sync, TaskList bridging, swarm invocation), read `skills/crank/references/team-coordination.md`.**

**Cross-cutting validation (SDD):**

After per-task validation passes, run cross-cutting checks across all files modified in the wave:

```bash
# Only if cross_cutting constraints were injected
if [[ -n "$CROSS_CUTTING_CHECKS" ]]; then
    WAVE_FILES=$(git diff --name-only "${WAVE_START_SHA}..HEAD")
    for check in $CROSS_CUTTING_CHECKS; do
        run_validation_check "$check" "$WAVE_FILES"
    done
fi
```
