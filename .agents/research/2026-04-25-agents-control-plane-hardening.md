---
title: Agents Control Plane Hardening Research
date: 2026-04-25
skill: agentops:discovery
release_range: v2.38.0..HEAD
---

# Research: Agents Control Plane Hardening

## Release Evidence

Baseline release: `v2.38.0`, published 2026-04-23.

Current range: `v2.38.0..HEAD` at `78a8e9df`.

Observed range metrics:

- 31 commits.
- 11 merged PRs into `main`.
- 345 changed files.
- 6,960 insertions and 3,013 deletions.

## PR Cluster

Merged PRs since release show a consistent theme:

- `.agents` contract and lint: `#139`, `#143`, `#144`, `#146`.
- CLI inspection and operator ergonomics: `#141`, `#143`, `#146`.
- temp-dir and find-root hardening: `#142`, `#145`.
- harvest recursion and real-rig metadata: `#138`, `#140`.
- nightly runtime and validation hardening: `#134`, `#135`.

## Local Command Findings

The current `ao agents` surface works from the repo root but has path-resolution
gaps from subdirectories.

From repo root:

```text
scripts/check-agents-write-surfaces.sh --json
{"contract":"docs/contracts/agents-write-surfaces.md","allowlist_size":40,"referenced":49,"undocumented":[],"status":"ok"}
```

From `cli/`:

```text
go run ./cmd/ao agents inspect
Error: reading contract docs/contracts/agents-write-surfaces.md: open docs/contracts/agents-write-surfaces.md: no such file or directory
```

```text
go run ./cmd/ao agents lint
Error: lint script not found at scripts/check-agents-write-surfaces.sh: stat scripts/check-agents-write-surfaces.sh: no such file or directory
```

With an explicit contract from `cli/`, JSON inspection finds the contract but
still reports zero active skills because skill discovery uses `skills` relative
to the current directory instead of the repo root.

## Code Evidence

Relevant implementation surfaces:

- `cli/cmd/ao/agents.go` registers the command, reads the default contract path
  directly, and calls `discoverActiveSkills("skills")`.
- `cli/cmd/ao/agents_lint.go` defaults the script path to
  `scripts/check-agents-write-surfaces.sh` and checks it relative to the
  current working directory.
- `cli/cmd/ao/projectdir.go` currently returns `testProjectDir` or `os.Getwd()`
  and does not locate the repository root.
- `cli/internal/plans/plans.go` contains a more capable `FindAgentsDir(startDir)`
  walk-up helper with temp-directory skip behavior.
- `scripts/check-agents-write-surfaces.sh` reports unknown subdir names but not
  source locations.
- `scripts/pre-push-gate.sh` captures and diffs shared `.agents` hash state,
  which can be noisy or slow during concurrent local agent activity.

## Test Evidence

Existing coverage gives a useful base:

- `cli/cmd/ao/agents_test.go` covers allowlist parsing, skill discovery,
  text/JSON inspect output, and missing contracts.
- `cli/cmd/ao/agents_lint_test.go` covers command registration, missing script,
  passthrough exit codes, and JSON forwarding.
- `tests/scripts/check-agents-write-surfaces.bats` has broad shell coverage for
  documented, undocumented, skill-owned, malformed, JSON, and help cases.

The next packet should extend these tests rather than replacing them.

## Retrieved Knowledge Applied

The plan applies three existing lessons and one pattern:

- `.agents/findings/f-2026-04-14-001.md`: command refactors need paired tests.
- `.agents/findings/f-2026-04-14-002.md`: planning and closure evidence should
  cite committed paths, not ephemeral discovery seed paths.
- `.agents/learnings/2026-04-07-v2.35.0-release-postmortem.md`: local and
  remote CI have different failure surfaces, so both focused and broad gates
  matter.
- `.agents/patterns/warn-then-fail-ratchet.md`: local hash-gate tuning can use
  bounded warnings, but CI must stay strict.

## Conclusion

The best next work is not a new subsystem. It is a consolidation pass that
turns the newly shipped `.agents` contract, CLI commands, scripts, and docs into
a dependable control plane.
