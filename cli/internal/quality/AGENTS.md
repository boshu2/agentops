---
package: cli/internal/quality
status: active
owner: agentopsd
---

# cli/internal/quality

Doctor reporting, stale-command reference scanning, and installed Codex plugin
inspection. Read current package files before extending an old metrics or
knowledge interface; those implementations are no longer in this package.

## Ownership

- Owned by the agentopsd extraction track.
- Read-only against the repo and `~/.codex/`; any output writes stay in
  directories explicitly supplied by the caller.

## Read for the change

- **Doctor results:** [doctor.go](doctor.go) owns status aggregation and rendering;
  [the doctor command](../commands/doctor/module.go) owns presentation wiring.
- **Command renames:** [stale_refs.go](stale_refs.go) owns `DeprecatedCommands`
  and reference scanning. Update the rename map with the command and its docs.
- **Skill installations:** [skill_installs.go](skill_installs.go) inspects
  installed content across runtimes. Read
  [the Codex skill contract](../../../docs/contracts/codex-skill-api.md) before
  changing how the Codex plugin cache is located.

## Non-obvious rules

- **Status and exit code differ.** `pass`, `warn`, `fail`, and `info` are check
  statuses. Any `fail` makes the aggregate `UNHEALTHY`; warnings alone make it
  `DEGRADED`. `info` does not affect totals or health. `RunDoctor` returns an
  error for required failures in table mode; JSON mode emits a document and
  returns successfully, so inspect its checks rather than treating exit zero
  as healthy.
- **Source and installation:** `skills/` is canonical and is the tree every
  runtime loads; `~/.codex/plugins/` is installed state. Report overlap between
  installations; inspection must not repair it automatically.
- **Dependency direction:** shared evidence types may be consumed from
  `cli/internal/types`; keep `quality` out of that package's imports.

## Completion

Run the affected cases with `go test ./internal/quality` from `cli/`, then the
root contract's required checks. For doctor changes, cover required and optional
failures, `info`, and JSON output separately. For installation checks, retain
the distinction between the source tree and observed installed content;
neither establishes successful model behavior.
