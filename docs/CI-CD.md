# Repository CI and delivery

AgentOps produces a `PlanPacket`, one bounded implementation candidate, exact
content identity, one author-distinct Validate verdict, and a durable verdict
artifact. It does not own Git delivery, merge policy, retries, queues, work
ownership, or release transitions.

Repositories own delivery policy for local and cloud agents.

## Separation of responsibilities

```text
AgentOps: Plan -> Implement once -> Validate once -> verdict.v2 -> stop
Repository: deterministic checks -> repository-selected Git/CI/release policy
```

`ao gate check` is an ordinary deterministic repository test runner. Success
means only that its selected checks passed. It cannot create, strengthen, or
replace a semantic verdict.

AgentOps installs no push hook and does not choose how a repository invokes
these checks. A repository may call `ao gate check` from a local command, CI,
pull request, merge queue, or another delivery process.

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
python3 scripts/check-cathedral-cut-conformance.py
python3 scripts/generate-skill-mesh.py --check
bash scripts/ci-local-release.sh
```

The local release script validates this repository's release artifacts. That is
repository policy, not an AgentOps lifecycle transition.
