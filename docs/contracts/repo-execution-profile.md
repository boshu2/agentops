# Repo Execution Profile

> **Status:** Draft
> **Schema:** `repo-execution-profile.schema.json`
> **Consumers:** `/evolve`, `/rpi`, and future repo-native orchestration loaders

This contract defines the repo-local operating policy that autonomous orchestration should load before it starts selecting work. It is intentionally repo-scoped rather than skill-scoped: skill metadata in [docs/SKILL-API.md](../SKILL-API.md) describes how a skill behaves, while the repo execution profile describes how a specific repository wants that behavior parameterized.

## Purpose

The profile reduces giant repo-specific prompts by moving stable operating policy into a machine-readable contract:
- ordered startup reads
- canonical goals source and compatibility mirrors
- mandatory validation bundle
- tracker command wrappers and shell policy
- concrete definition_of_done predicates

`/evolve` uses the profile for repo bootstrap before queue or goal selection. When a repo-local `PROGRAM.md` contract exists, `/evolve` composes both: the execution profile governs bootstrap and session-level policy, while the program contract governs mutable scope and per-cycle keep/revert criteria. `/rpi` carries the relevant fields forward inside a normalized `execution_packet` so later phases do not fall back to loose prompt prose.

## Field Semantics

### `schema_version`
Contract version. Current value: `1`.

### `startup_reads`
Ordered repo paths to read before selecting work. This is the bootstrap layer that replaces repeated "read these five files first" prompt boilerplate.

### `goals_source`
Declares the canonical goals document plus any compatibility mirrors that must stay aligned.

### `validation_commands`
Ordered shell commands that define the repo's standard landing gate for substantive slices.

### `tracker_commands`
Repo-scoped command wrappers for issue tracking. This is where shell/runtime requirements such as `zsh -lc 'cd <repo> && bd ...'` live when a tracker needs a specific execution environment.

### `work_selection_order`
Optional source ladder for autonomous prioritization. When omitted, consumers should default to the repo's existing ladder.

### `definition_of_done`
Concrete predicates that determine when a cycle or full autonomous run may stop. The key design rule is: use explicit completion checks, not vague prose.

## Derived RPI Artifact: `execution_packet`

`/rpi` should derive a filesystem-backed `execution_packet` from:
- the user objective or selected epic
- the repo execution profile
- discovery artifacts
- the active epic id and pre-mortem verdict

Recommended packet fields for the first slice:
- `objective`
- `contract_surfaces`
- `validation_commands`
- `tracker_mode`
- `done_criteria`

When a repo-local `PROGRAM.md` contract exists, `/rpi` may also carry an additive `autodev_program` block derived from that file. This keeps runtime operating policy phase-stable without forcing `GOALS.md` to absorb mutable execution details.

This keeps repo policy additive and phase-stable without replacing the current goal/epic flow in one step.

## Minimal Example

```json
{
  "schema_version": 1,
  "startup_reads": [
    "docs/newcomer-guide.md",
    "docs/index.md",
    "docs/documentation-index.md"
  ],
  "goals_source": {
    "primary": "GOALS.md",
    "compatibility_mirrors": [
      "GOALS.yaml"
    ]
  },
  "validation_commands": [
    "scripts/ci-local-release.sh",
    "bash scripts/check-worktree-disposition.sh"
  ],
  "tracker_commands": {
    "shell_prefix": "zsh -lc 'cd <repo> && '",
    "ready": "bd ready --json",
    "show": "bd show <id> --json",
    "update": "bd update <id> --status in_progress --json",
    "close": "bd close <id> --reason \"Completed\" --json"
  },
  "work_selection_order": [
    "harvested",
    "beads",
    "goal",
    "directive",
    "testing",
    "validation",
    "bug-hunt",
    "drift",
    "feature"
  ],
  "definition_of_done": {
    "predicates": [
      "goals are green",
      "repo validation bundle is green",
      "ready queue is empty after generator passes"
    ],
    "required_validations": [
      "scripts/ci-local-release.sh"
    ],
    "require_clean_git": true
  }
}
```

## Compatibility Notes

- This contract is repo-local policy, not a replacement for skill frontmatter.
- The first slice is documentation and validation first. Consumers may warn and fall back when only the contract exists, but they must do so explicitly.
- Future runtime loaders may consume a checked-in profile instance directly. This contract establishes the field set and semantics first so that later enforcement does not invent another policy surface.
