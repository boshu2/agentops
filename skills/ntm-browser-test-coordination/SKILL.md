---
name: ntm-browser-test-coordination
description: |-
  Use when coordinating browser or UI tests through NTM panes with screenshots and handoffs.
  Triggers:
skill_api_version: 1
user-invocable: false
practices:
- continuous-delivery
- bdd-gherkin
- team-topologies
hexagonal_role: supporting
consumes:
- ntm
- agent-mail
- test
produces:
- browser-test-coordination-packet
- browser-test-evidence-index
- browser-test-handoff
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
metadata:
  tier: orchestration
  stability: experimental
  dependencies:
  - ntm
  - agent-mail
  - test
output_contract: "A browser-test coordination packet with scenario map, pane assignment table, evidence index, failure log, and handoff verdict."
---

# NTM Browser Test Coordination

Use this skill when browser or UI validation needs more coordination than a single local test command: multiple scenarios, parallel panes, visible browser evidence, failure triage, or a handoff that another operator can resume. It does not replace the browser test runner — it defines how the lead agent scopes scenarios, assigns NTM panes, requires evidence, records failures, and returns one verifiable verdict.

## Coordination Rules

- **Scenario first.** Define the user-visible scenario before dispatching panes. A command without a scenario is only a test invocation, not a UI validation plan.
- **One pane owns one scenario slice.** Each pane receives a scenario id, fixture slice, command, evidence path, and completion contract. Avoid shared users, shared browser state, and overlapping writes.
- **Evidence beats narration.** A pane saying "passed" is not enough. Require screenshots, traces, videos, console logs, test reports, or a specific "no artifact because" explanation.
- **Failures become records.** Every failed or blocked scenario gets a failure record with reproduction command, artifact paths, expected behavior, observed behavior, and current owner.
- **Handoff is part of the run.** The run is not complete until the lead can state what passed, what failed, what was not run, where evidence lives, and what the next operator should do.

## Workflow

### 1. Define the run

Create a coordination packet before touching NTM:

```markdown
## Browser test run: <run-id>
- Target: <local URL, deployed URL, or app command>
- Scope: <features, routes, flows, browsers, devices>
- Test command: <runner command or manual browser task>
- Evidence root: <path, for example .agents/browser-tests/<run-id>/>
- Exit criteria: <pass/fail/block rules>
```

Turn vague requests into named scenarios:

```markdown
| Scenario | User flow | Browser/device | Fixture slice | Expected evidence |
|---|---|---|---|---|
| S1 | Sign in and open dashboard | chromium desktop | user-01 | screenshot + trace |
```

### 2. Prepare pane assignments

Use the NTM robot surfaces to discover the live contract and current panes before dispatch. Confirm the session, pane ids, working state, and whether any panes already own related work.

Each assignment must be self-contained:

```markdown
## Pane assignment
- Scenario: <S-id and title>
- Fixture: <test data, account, seed, port, browser profile>
- Command: <exact command or browser steps>
- Evidence path: <root>/<scenario-id>/
- Required artifacts: <screenshots, trace, video, report, console log>
- Failure record path: <root>/failures.md
- Done when: <artifacts exist and verdict is written>
```

Reserve scarce fixtures before panes start browsers. If Agent Mail or NTM locks are available, reserve users, ports, seeded records, and browser profiles explicitly.

### 3. Dispatch with a strict completion contract

Send each pane only its owned slice. Do not send a broad "run all browser tests" prompt to every pane.

The dispatch message should include:

- Scenario id, user-visible acceptance criteria, exact command, URL, environment variables, and fixture slice.
- Evidence directory and artifact names.
- Required verdict shape: `PASS`, `FAIL`, or `BLOCKED`.
- Failure record fields and where to write them.
- Instruction to stop after its slice and report only artifact-backed results.

### 4. Tend the run

Poll NTM state while panes work:

- Snapshot the session after dispatch and after each meaningful state change.
- Tail panes that are silent, repeating output, or missing artifacts.
- Treat browser hangs, blank screenshots, missing traces, and orphaned server processes as failures or blockers, not as inconclusive success.
- Re-dispatch only the affected scenario when a pane stalls; keep successful scenario evidence intact.

Do not merge results until every assigned pane is idle or explicitly blocked and every scenario has a verdict record.

### 5. Verify and merge

Build one lead-owned verdict from artifacts, not prose:

```markdown
## Verdict
- PASS: <scenario ids>
- FAIL: <scenario ids>
- BLOCKED: <scenario ids and blocker>
- NOT RUN: <scenario ids and reason>
- Evidence index: <path>
- Failure log: <path>
```

For automated suites, confirm test reports and runner exit codes. For manual browser scenarios, confirm screenshots or videos show the relevant UI state and any console/network errors are captured or explicitly ruled out.

## Evidence Packet

The evidence index should let a future operator reproduce the run without reading pane scrollback:

```markdown
## Evidence index
| Scenario | Pane | Verdict | Artifacts | Reproduction |
|---|---:|---|---|---|
| S1 | 2 | PASS | S1/final.png, S1/trace.zip | npm run e2e -- --grep @S1 |
```

Minimum artifact expectations:

- Passing visual flow: final-state screenshot or runner report.
- Failing visual flow: screenshot at failure point plus reproduction command.
- Interaction failure: screenshot or video plus console/network evidence when available.
- Accessibility or layout issue: viewport, browser, selector or route, expected behavior, observed behavior.
- Blocker: command attempted, error output, owner needed, and next unblock action.

## Failure Handling

Record each failure in a stable format:

```markdown
## Failure: <scenario-id> <short title>
- Pane: <id>
- Verdict: FAIL | BLOCKED
- Reproduction: <command or browser steps>
- Expected: <user-visible expected behavior>
- Observed: <actual behavior>
- Artifacts: <paths>
- Suspected layer: <test, fixture, frontend, backend, environment, unknown>
- Next action: <fix, rerun, narrow, assign, defer>
```

Use failure records to decide reruns:

- Rerun immediately when the failure is environmental and the rerun has changed setup; do not rerun repeatedly without changing evidence or hypothesis.
- Split a broad scenario when one pane cannot isolate the failing step.
- Escalate fixture conflicts by stopping affected panes, releasing reservations, and assigning new fixtures.

## Handoff

Close the run with a handoff that includes:

- Target, run id, command, NTM session, and scenario table with final verdicts.
- Evidence index path and failure log path.
- Panes that were restarted, interrupted, or left blocked.
- Any live servers, ports, fixtures, locks, or browser profiles that remain active.
- Exact next command for the next operator.

The final user-facing answer should summarize the verdict and cite the evidence paths. If the run is incomplete, say which scenarios remain and why.

## Quality Rubric

A coordinated browser test run is good when:

- Every pane has a named scenario and non-overlapping fixture ownership.
- Every scenario has `PASS`, `FAIL`, `BLOCKED`, or `NOT RUN`.
- Every pass or failure is backed by artifacts or an explicit artifact exception.
- The failure log contains enough detail to reproduce without pane scrollback.
- The handoff identifies residual state and next actions.

## See Also

- `ntm` for robot snapshots, sends, tails, waits, locks, and handoffs.
- `agent-mail` for fixture reservations and cross-pane coordination.
- `test` for choosing local test commands and regression checks.
