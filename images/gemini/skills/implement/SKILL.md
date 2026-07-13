---
name: implement
spine: true
description: 'Implement one tracked issue. Triggers: "implement", "implement one tracked issue.", "implement skill".'
practices:
- tdd
- refactoring
- code-complete
hexagonal_role: driving-adapter
consumes:
- domain
produces:
- git-changes
context_rel:
- kind: customer-of
  with: domain
skill_api_version: 1
metadata:
  graph_root: true
  tier: execution
  dependencies:
  - beads-br
  - standards
  - rch
  - refactor
  - test
context:
  window: isolated
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
output_contract: code changes, test results, bead status update, behavioral spec (optional)
---
# Implement Skill

> **Quick Ref:** Execute single issue end-to-end. Output: code changes + commit + closed issue.

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

## Constraints

- Freeze the claimed issue's acceptance criteria, non-goals, and write scope before editing, because every changed line must trace to the single vertical slice; route unrelated work to a follow-up.
- For behavior changes, capture a right-reason failing test before implementation and keep GREEN-mode tests immutable, because the failing proof is the slice contract rather than ceremony.
- Route a plain `REFUTED` validation result back through automatic repair and revalidation; only a circuit-breaker trip enters `HOLD` and one bounded helper pass, while helper `ESCALATE` or refusal/judgment/exhausted-budget classes reach a human.

## Loop position

Move **4 (TDD per slice)** of the [operating loop](../../docs/architecture/operating-loop.md). Consumes one vertical slice from the [slice validation plan](../../docs/templates/slice-validation.md); produces failing test → passing implementation → refactor-under-green. Discipline: (1) first failing test must fail for the right reason (missing behavior, not syntax); (2) smallest change to flip green; (3) refactor as its own commit. Slices that mix refactor + feature are two slices, not one. Code without a failing test has no contract; the slice is not done.

Execute a single issue from start to finish.

**CLI dependencies:** ao (issue tracking via `ao beads exec <cmd>` — it resolves the bead tracker, bd or br, and its ledger; plus ratchet gates). Optional — see `skills/shared/SKILL.md` for fallback table. If no tracker is available, use the issue description directly and track progress via TaskList instead of beads.

## When to use

**Triggers:** "implement", "implement one tracked issue", or "implement skill".

- Use `/implement <issue-id>` to implement a specific tracked issue.
- Use `/implement` (no argument) to pick up next ready work via `ao beads exec ready`.
- Use `/implement <description>` to implement an ad-hoc task without a tracked issue.

### Folded triggers (ag-s43tg wave 1): `pr-implement` routes here

- **`pr-implement` → OSS contribution mode.** Use when you need to implement a scoped OSS PR —
  fork-based implementation of an open source contribution with mandatory isolation checks.
  Same single-issue TDD discipline as internal work, plus the fork lane: ensure the fork exists
  and is current, create an isolated worktree, run an isolation pre-check (BLOCK on mixed
  concerns) and post-check (BLOCK on scope creep), check for competing PRs before starting, and
  hand off to `/pr-prep` for commit/PR shaping. Input is the plan artifact from `/pr-prep` +
  `/plan` (run those first if no plan exists).

## Examples

### Implement Specific Issue

**User says:** `/implement ag-5k2`

**What happens:**
1. Agent reads issue from beads: "Add JWT token validation middleware"
2. Explore agent finds relevant auth code and middleware patterns
3. Agent edits `middleware/auth.go` to add token validation
4. Runs `go test ./middleware/...` — all tests pass
5. Commits with message "Add JWT token validation middleware\n\nImplements: ag-5k2"
6. Closes only via `scripts/close-with-implementation-receipt.sh --issue ag-5k2 --receipt <canonical-receipt>`

**Result:** Issue implemented, verified, committed, and closed. Ratchet recorded.

### Pick Up Next Available Work

**User says:** `/implement`

**What happens:**
1. Agent runs `ao beads exec ready` — finds `ag-3b7` (first unblocked issue)
2. Claims issue via `ao beads exec update ag-3b7 --status in_progress`
3. Implements and verifies
4. Closes through the receipt wrapper after canonical pawl confirmation

**Result:** Autonomous work pickup and completion from ready queue.

### GREEN Mode (Test-First)

**User says:** `/implement ag-8h3` (invoked by `/crank --test-first`)

**What happens:**
1. Agent receives failing tests (immutable) and contract
2. Reads tests to understand expected behavior
3. Implements ONLY enough to make tests pass
4. Does NOT modify test files
5. Verification: all tests pass with fresh output

**Result:** Minimal implementation driven by tests, no over-engineering.

## Lifecycle Integration Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--no-lifecycle` | off | Skip ALL lifecycle skill auto-invocations (test gen, review, refactor) |
| `--lifecycle=<tier>` | matches complexity | Controls which lifecycle skills fire: `minimal` (test only), `standard` (+review), `full` (+refactor dry-run) |

Lifecycle tier defaults to matching the current complexity level. Explicit `--lifecycle=<tier>` overrides.

## Execution

Read [references/workflow.md](references/workflow.md) for the full procedure (Steps 0 through 11), including durable RED/GREEN evidence, the implementation receipt, independent pawl routing, confirmed close, and ratchet record.

GREEN mode rules live in [references/green-mode.md](references/green-mode.md). The pre-commit autonomous quality loop lives in [references/quality-loop.md](references/quality-loop.md). The behavioral spec format lives in [references/behavioral-spec.md](references/behavioral-spec.md).

## Key Rules

- **Captured RED for behavior** - every behavior change closes only with a reproducible failing contract at a pre-implementation commit; GREEN input counts as captured RED, and no-framework work uses a minimal executable harness. `--no-tdd` cannot authorize behavior closure. Only mechanically derived docs-only and independently reviewed pure-refactor lanes may waive RED; pure refactor proves canonical acceptance green before and after with unchanged test drivers.
- **Refactor after every green — it is the load-bearing move.** Refactor under green as its own commit after each behavior, never deferred to one final pass (deferred-refactor workflows were the worst-performing cluster in the study above). **Never let a refactor step change a test** — a test change during refactor means behavior changed, which is a new slice, not a refactor.
- **One behavior per cycle (small batch).** Implement one behavior, keep the suite green, refactor, move to the next — small batches beat all-at-once.
- **Lifecycle skills fire automatically** - /test, /review, /refactor run at appropriate steps (disable with `--no-lifecycle`)
- **Explore first** - understand before changing
- **Edit, don't rewrite** - prefer Edit tool over Write tool
- **Follow patterns** - match existing code style
- **Verify changes** - run tests or sanity checks
- **Commit with context** - reference the issue ID
- **Close the issue** - update status when done, then run crank's [Close checkpoint](../crank/SKILL.md#close-checkpoint--a-closed-bead-is-a-sensor-reading-age-cysr): a closed bead is a sensor reading — if what it taught falsifies an assumption the remaining plan depends on, surface it for re-planning instead of silently proceeding (age-cysr)

## Without Beads

If br CLI not available:
1. Skip the claim/close status updates
2. Use the description as the task
3. Still commit with descriptive message
4. Report completion to user

## Output Specification

- **Path:** modify only issue-approved product/test paths; store evidence under `.agents/evidence/implement/<issue-id>/` and the final receipt under its `<full-sha>/` directory.
- **Filename:** product/tests use repository-native names; the receipt is exactly `<issue-id>-<full-sha>-receipt.json`.
- **Format:** product files use native formats; the receipt is JSON conforming to [schemas/implementation-receipt.schema.json](schemas/implementation-receipt.schema.json) and binds immutable RED plus fresh GREEN evidence to the full SHA.
- **Validation command:** run issue acceptance and relevant gates, `scripts/validate-workflow-contract.sh source`, then `scripts/verify-implementation-receipt.sh --issue <issue-id> --receipt <canonical-path>`; closure requires the verifier and canonical pawl check to pass.
- **Downstream handoff:** pass the receipt and exact SHA to `/validate`; `REFUTED` auto-repairs, breaker `HOLD` consults one helper, and only `CONFIRMED` authorizes closure.

## Quality Checklist

- Acceptance fidelity: every changed line maps to one acceptance example or necessary cleanup, with non-goals unchanged.
- Test fidelity: the first failing proof fails for missing behavior, final tests pass fresh, and refactor commits do not modify the behavioral contract.
- Scope fidelity: changed paths remain inside the issue write scope; unrelated findings become follow-ups instead of hitchhiking.
- Evidence fidelity: commit, tracker closure, changed files, and validation commands identify the same final implementation SHA.

## Completion Markers

```
<promise>DONE</promise>
```

If blocked or incomplete:
```
<promise>BLOCKED</promise>
Reason: <why blocked>
```

```
<promise>PARTIAL</promise>
Remaining: <what's left>
```

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Issue not found | Issue ID doesn't exist or local state looks stale | Run `ao beads exec show <id>` to verify; trust the tracker's source-of-truth ledger (for br, `_beads/issues.jsonl`) if the local cache looks stale |
| GREEN mode violation | Edited a file not related to the issue scope | Revert unrelated changes. GREEN mode restricts edits to files relevant to the issue |
| Verification gate fails | Tests fail or build breaks after implementation | Read the verification output, fix the specific failures, re-run verification |
| "BLOCKED" status | Contract contradicts tests or is incomplete in GREEN mode | Write BLOCKED with specific reason, do NOT modify tests |
| Fresh verification missing | Agent claims success without running verification command | MUST run verification command fresh with full output before claiming completion |
| Ratchet record failed | ao CLI unavailable or chain.jsonl corrupted | Implementation still closes via br, but ratchet chain needs manual repair |

## Reference Documents

- [references/behavioral-spec.md](references/behavioral-spec.md) — Behavioral spec format for Stage 4 validation
- [references/binary-deployment-gate.md](references/binary-deployment-gate.md) — CLI/hook binary-deployment gate spec
- [references/gate-checks.md](references/gate-checks.md) — Ratchet and pre-mortem gate checks
- [references/green-mode.md](references/green-mode.md) — GREEN mode test-first implementation rules
- [references/implement.feature](references/implement.feature) — Executable spec: the /implement done-state (first-failing-test → green → refactor → verified close) (soc-qk4b.2)
- [references/quality-loop.md](references/quality-loop.md) — Pre-commit autonomous quality loop
- [references/resume-protocol.md](references/resume-protocol.md) — Resume protocol for interrupted sessions
- [references/workflow.md](references/workflow.md) — Full execution workflow (Steps 0 through 11)
- [schemas/implementation-receipt.schema.json](schemas/implementation-receipt.schema.json) — Durable RED/GREEN and independent-validation receipt

## Local Resources

- `scripts/verify-implementation-receipt.sh` — fail-closed closure authority over Git, evidence digests, rerun commands, and the canonical pawl verdict
- `scripts/verify-implementation-receipt.bash` — non-executable verifier implementation entered only through the sanitized launcher
- `scripts/test-implementation-receipt.sh` — forged-dimension behavioral fixtures
- `scripts/close-with-implementation-receipt.sh` — snapshot, verify, recheck, and close without a verifier-to-close gap
- `scripts/close-with-implementation-receipt.bash` — non-executable close implementation entered only through the sanitized launcher
- `scripts/validate-workflow-contract.sh` — ordered workflow and schema drift gate

## See also

- [test](../test/SKILL.md) — Test generation, coverage analysis, and TDD workflow
