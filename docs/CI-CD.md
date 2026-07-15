# Repository CI and delivery

AgentOps produces a `PlanPacket`, one bounded implementation candidate, exact
content identity, one author-distinct Validate verdict, and a durable verdict
artifact. It does not own Git delivery, merge policy, retries, queues, work
ownership, or release transitions.

## Separation of responsibilities

```text
AgentOps: Plan -> Implement once -> Validate once -> verdict.v2 -> stop
Repository: deterministic checks -> repository-selected Git/CI/release policy
```

`ao gate check` is an ordinary deterministic repository test runner. Success
means only that its selected checks passed. It cannot create, strengthen, or
replace a semantic verdict.

The installed pre-push hook runs ordinary build, race, schema, generated-drift,
and security checks. It performs no model review, admission decision, tracker
transition, delivery serialization, or provenance backstop. Repositories may
replace or omit the hook and may use direct push, pull requests, external CI,
or another delivery process.

## GitHub workflows

| Workflow | Purpose |
|---|---|
| `.github/workflows/validate.yml` | Optional hosted execution of repository deterministic checks |
| `.github/workflows/release.yml` | Repository-owned tagged release publication |
| `.github/workflows/nightly.yml` | Scheduled deterministic regression and security coverage |

No workflow is a semantic validator. No workflow writes a Validate verdict.

## Current CI jobs

<!-- BEGIN GENERATED CI JOBS -->
<!-- Generated from docs/contracts/ci-jobs.yaml by scripts/generate-ci-jobs-table.sh. -->
| Job | What it validates | Common failure |
|-----|-------------------|----------------|
| **go-gate-shadow** | Runs the ordinary Go gate registry with workflow-coverage reporting | A deterministic check failure or workflow/registry coverage mismatch |
| **correctness** | Builds `ao`; runs Go, schema, generated-surface, shell, and smoke tests | Compilation, test, schema, generated-drift, or portability failure |
| **security** | Runs secret, dependency, static-analysis, and dangerous-pattern checks | A blocking security finding |
<!-- END GENERATED CI JOBS -->

## Local verification

Use the smallest focused checks while editing, then run the ordinary full suite
once for the complete candidate:

```bash
cd cli && go test ./...
./scripts/check-cathedral-cut-conformance.py
./scripts/check-skill-mesh.py
./scripts/ci-local-release.sh
```

The local release script validates this repository's release artifacts. That is
repository policy, not an AgentOps lifecycle transition.

## Bootstrap bypass

The Cathedral Cut itself may use the audited one-time bypass:

```bash
AGENTOPS_GATE_DISABLED=1 git push
```

The hook logs that bypass. It does not grant semantic validity; this migration
still requires the full ordinary suite and a fresh `verdict.v2` for the exact
candidate before the push.
