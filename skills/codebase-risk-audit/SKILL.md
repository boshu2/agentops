---
name: codebase-risk-audit
description: |-
  Use when auditing codebase risks with evidence and prioritized remediation.
  Triggers:
skill_api_version: 1
user-invocable: false
hexagonal_role: supporting
practices:
- code-review
- threat-modeling
- operational-readiness
consumes:
- repository
- test-results
- runtime-configuration
produces:
- codebase-risk-audit-report
context_rel:
- kind: supplier-to
  with: plan
- kind: supplier-to
  with: validate
context:
  window: inherit
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: full
metadata:
  tier: execution
  stability: experimental
  external_dependencies:
  - shell
  - git
  - rg
output_contract: markdown report with evidence-backed findings, severity, likelihood, blast radius, and prioritized remediation
---

# Codebase Risk Audit

Use this skill to produce a focused risk audit of a repository. The audit looks
for problems that could make the system hard to change, hard to operate, hard
to test, unsafe around sensitive surfaces, or brittle under realistic failure.

The output is a decision aid, not a general critique. Findings must be tied to
specific repository evidence and ranked by the risk they create.

## Inputs

Collect only the context needed for the target repository and scope:

- User goal, explicit exclusions, and any risk areas they care about most.
- Repository structure, language toolchain, dependency manifests, and entry
  points.
- Existing tests, CI workflows, deployment scripts, runtime configuration, and
  operational documentation.
- Recent local diffs when the audit is about unmerged work.

If the user asks for a full-repository audit and gives no extra scope, sample
the codebase by ownership boundary and runtime path instead of exhaustively
reading every file.

## Workflow

1. Establish the system map.
   Identify the main bounded contexts, executables, libraries, data stores,
   external integrations, background jobs, and operator-facing scripts.

2. Trace critical paths.
   Follow startup, request handling, data mutation, authn/authz boundaries,
   persistence, scheduled work, and failure recovery. Prefer paths that cross
   module or process boundaries.

3. Inspect risk surfaces.
   Check architecture, operations, testing, security-adjacent behavior, and
   maintainability as separate lenses. Record both concrete defects and design
   conditions that make future defects likely.

4. Verify with evidence.
   Tie each finding to file paths, line references when useful, commands run,
   config values, test gaps, or directly observed behavior. Avoid claims based
   only on naming, style preference, or speculation.

5. Prioritize remediation.
   Rank by severity, likelihood, blast radius, reversibility, and cost to fix.
   Prefer small fixes that reduce high-impact uncertainty before broad rewrites.

6. Report residual risk.
   State what was not inspected, what evidence was unavailable, and what would
   change the confidence level.

## Risk Lenses

Use these lenses as prompts, not as a checklist that must be fully exhausted.

### Architecture

- Critical flows depend on hidden ordering, global state, or ambient process
  configuration.
- Module boundaries do not match runtime ownership or data ownership.
- Shared abstractions hide materially different behavior across callers.
- Error handling loses domain context or makes recovery ambiguous.
- Migrations, compatibility paths, or version boundaries lack an explicit
  owner.

### Operations

- Startup, shutdown, retry, timeout, backoff, and cancellation behavior is
  missing or inconsistent.
- Logging, metrics, traces, health checks, or runbooks do not cover critical
  failure modes.
- Deployment and rollback steps depend on manual state not captured in code.
- Configuration defaults are unsafe for local, CI, staging, or production use.
- Background jobs can duplicate work, drop work, or block shutdown without a
  clear recovery path.

### Testing

- Important behavior has only happy-path coverage.
- Tests assert implementation details while missing externally visible
  contracts.
- Fixtures or mocks conceal integration, ordering, concurrency, permission, or
  serialization risks.
- CI skips slow or environment-sensitive tests without a compensating gate.
- The project has no fast way to reproduce the riskiest local failure mode.

### Security-Adjacent

This is not a formal security audit. Flag risks that are visible from normal
engineering review:

- Authentication, authorization, tenant isolation, or secret handling is
  unclear at call sites.
- Inputs cross trust boundaries without validation, normalization, escaping, or
  size limits.
- Sensitive data can enter logs, errors, caches, telemetry, test fixtures, or
  generated artifacts.
- Dependencies, generated code, or external commands run with more privilege
  than the workflow needs.
- Failure behavior reveals information or bypasses an intended guard.

### Maintainability

- Complex code has no local explanation, test harness, or safe extension point.
- Similar behavior is implemented in multiple places with meaningful drift.
- Public contracts are implicit, undocumented, or contradicted by tests.
- Dead paths, compatibility shims, or TODO-driven behavior obscure current
  intent.
- Build, generation, or validation steps are hard to discover or easy to run
  incorrectly.

## Evidence Standard

Each finding must include:

- A concise title that states the risk, not only the symptom.
- Evidence from the repository: file path, line, command output, config, test
  result, or observed behavior.
- Impact: what can go wrong and who or what is affected.
- Likelihood: why this is plausible in normal operation or development.
- Remediation: the smallest credible next step and the owner boundary it
  touches.
- Priority: `P0`, `P1`, `P2`, or `P3`.

Use `P0` only for risks that can plausibly cause immediate production harm,
data loss, credential exposure, or blocked delivery. Use `P1` for high-impact
risks likely to surface soon. Use `P2` for important risks that are bounded or
less likely. Use `P3` for cleanup that improves confidence but is not urgent.

## Output Format

Return a markdown report with these sections:

1. `Scope`
   State what was inspected, commands run, and what was intentionally skipped.

2. `Executive Risk Summary`
   Summarize the top risks in priority order, with one sentence per finding.

3. `Findings`
   For each finding include priority, category, evidence, impact, likelihood,
   and remediation.

4. `Remediation Plan`
   Group fixes into immediate, near-term, and later work. Keep steps concrete
   enough to become issues or PRs.

5. `Validation Gaps`
   List missing tests, missing operational signals, or unknowns that limit
   confidence.

6. `Residual Risk`
   Name risks that remain after the proposed remediation, especially when the
   right fix requires product, infrastructure, or organizational decisions.

## Quality Bar

- Lead with findings, not background.
- Do not report style nits unless they create operational or delivery risk.
- Do not claim a vulnerability without evidence of a reachable trust boundary
  or sensitive asset.
- Do not prescribe rewrites when a local guard, test, interface clarification,
  or runbook would reduce the risk enough.
- Prefer fewer, stronger findings over broad undifferentiated lists.
- Mark inferences clearly when evidence is indirect.
