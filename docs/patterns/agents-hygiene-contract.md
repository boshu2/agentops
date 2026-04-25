# Pattern: `.agents/` hygiene contract

> **Status:** Active
> **Captured from:** `/evolve` 2026-04-25 (PRs #139, #140, #141, #142, #143)
> **Applies to:** introducing structural `.agents/` tooling, codifying knowledge-surface ownership, extending agentops native control of `.agents/` end-to-end.

When taking native ownership of a structural surface inside `.agents/`,
layer the work as five concentric rings — contract doc, shell lint,
bats tests, CLI surface, pre-push wire-in. Each ring is one PR, each
PR is small enough to review in isolation, and the rings stack so the
later PRs can be authored without re-litigating the earlier decisions.

## Shape

### 1. Contract doc — `docs/contracts/<surface>.md`

- Narrative table: each entry has `Subdir | Owner | Lifecycle | Purpose`.
- Parseable allowlist between explicit BEGIN/END HTML comment markers
  so the lint can read the doc directly.
- "How to update" section listing the four edits a contributor must make.

Concrete example: `docs/contracts/agents-write-surfaces.md`.

### 2. Shell lint — `scripts/check-<surface>.sh`

- Reads the allowlist between markers from the contract doc directly.
- Auto-allows skill-owned subdirs by reading `skills/<name>/SKILL.md`
  so adding a new skill never requires a contract edit.
- `--json` for machine-readable output, `--help` for the user surface.
- Exit codes: `0` clean, `1` violation, `2` invocation error.
- shellcheck-clean.

Concrete example: `scripts/check-agents-write-surfaces.sh`.

### 3. Bats tests — `tests/scripts/check-<surface>.bats`

- Build a self-contained fake repo per test (cli/, scripts/, hooks/,
  lib/, skills/, docs/contracts/).
- One test per contract dimension: passes-on-documented,
  fails-on-undocumented, skill-owned auto-allow, test-files-ignored,
  missing-doc, empty-allowlist, malformed-entries, comment+blank-lines,
  `--json` shape, `--help`, bad-flags.
- Use `/bin/cp` and absolute paths to bypass shell aliases.

Concrete example: `tests/scripts/check-agents-write-surfaces.bats`.

### 4. CLI surface — `cli/cmd/ao/<namespace>.go` + `<namespace>_<verb>.go`

- Parent command (`ao agents`) plus subcommands. Start with `inspect`
  for read and `lint` for gate; layer in `doctor`, `migrate` later as
  concrete needs surface.
- `inspect` parses the contract directly; `lint` wraps the shell script
  and surfaces its exit code via a typed error returned from `RunE`,
  recognized by `Execute()`.
- Tests use `cmd.SetOut(&buf)` for stdout capture and override package
  globals with `t.Cleanup` restore.

Concrete example: `cli/cmd/ao/agents.go`, `cli/cmd/ao/agents_lint.go`.

### 5. Pre-push wire-in — `scripts/pre-push-gate.sh`

- Always runs (not behind `needs_check`) when the lint is sub-100ms.
- Use `pass` / `fail` / `indent_output` consistently with neighboring
  checks.

## Why this works

- The contract doc is **the** source of truth. The lint reads it; the
  CLI surface reads it; humans read it. No third location can drift.
- Shell lint over Go lint: zero-build operator workflow plus `grep`
  portability.
- Skill auto-allow keeps the contract from churning every time a new
  skill ships.
- Stacked PRs by cycle: each ring is one PR (≈50–200 LOC), reviewable
  separately, mergeable in the order that respects declared
  dependencies.
- Pre-push wire-in catches drift before `git push`; CI enforces the
  same script in `validate.yml`.

## When NOT to apply

- **Pure read-side mirrors.** Reads share the literal set with writes
  in production code, so a separate "read-surfaces" contract is
  redundant. Use end-to-end smoke tests instead — see PR #140's
  `TestRunHarvest_NestedSubdirsCaught_E2E` as the template for
  read-side invariant locks.
- **One-off helper scripts** that don't introduce a long-lived surface.
  The contract overhead is only justified when the surface will be
  edited by multiple contributors over time.

## Cycle ledger from the originating session

| Cycle | PR | Ring | Outcome |
|---|---|---|---|
| 1 | #139 | Contract doc + shell lint + bats tests + pre-push wire-in | 40 surfaces catalogued, 14 bats tests, lint runs ≈50ms |
| 2 | #140 | E2E walker test (read-side invariant lock) | Locks PR #138 walker patch at the cobra command level |
| 3 | #141 | CLI namespace + `inspect` subcommand | `ao agents` namespace established (mirrors `ao goals`/`ao ratchet`) |
| 4 | #142 | Pre-existing flake fix (orthogonal) | `findAgentsDir` skips `os.TempDir()` so cruft `/tmp/.agents/` does not shadow real rigs |
| 5 | #143 | `lint` subcommand wrapping the cycle 1 script | Operator-facing surface for the contract gate, exit-code passthrough |

## See also

- `docs/contracts/agents-write-surfaces.md` — the contract this pattern produced
- `docs/contracts/repo-execution-profile.md` — adjacent contract that bootstraps autonomous orchestration
- `docs/contracts/autodev-program.md` — repo-local operational contract
