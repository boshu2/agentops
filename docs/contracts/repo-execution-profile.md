# Repo Execution Profile

> **Status:** Draft
> **Schema:** `repo-execution-profile.schema.json`
> **Consumers:** `/evolve`, `/rpi`, and future repo-native orchestration loaders

This contract defines the repo-local operating policy that autonomous orchestration should load before it starts selecting work. It is intentionally repo-scoped rather than skill-scoped: skill metadata in [docs/SKILL-API.md](../SKILL-API.md) describes how a skill behaves, while the repo execution profile describes how a specific repository wants that behavior parameterized.

## Purpose

The profile reduces giant repo-specific prompts by moving stable operating policy into a machine-readable contract:
- ordered startup reads
- canonical goals source and compatibility mirrors
- mandatory validation bundle plus structured validation lane metadata
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

### `validation_lanes`
Structured metadata for validation commands. Each lane has a stable `name`, the exact `command`, and mutation policy fields:
- `read_only` — command should not mutate tracked repo files or persistent agent state
- `writes_artifacts` — command intentionally writes validation/release artifacts
- `artifact_paths` — known artifact paths or globs written by the lane
- `isolated_agents_home` — command should run with isolated `HOME`/`AGENTS_HOME` or equivalent no-citation state
- `release_only` — command is reserved for release readiness rather than fast everyday validation
- `mutation_escape_hatch` — named opt-in for intentional mutation, or `null` when mutation is not allowed
- `cost_class` — `cheap`, `standard`, or `expensive`
- `auto_select` — `default`, `changed-surface`, `explicit`, or `release-only`
- `timeout_seconds` — recommended wall-clock cap for routine agent execution
- `expensive_reason` — rationale for expensive or explicit-only lanes

Agents should select the smallest lane set that proves the slice:
- Fast local validation uses lanes where `read_only=true`, `writes_artifacts=false`, `release_only=false`, and `auto_select` is `default` or the lane matches the changed surface.
- `expensive`, `explicit`, and `release-only` lanes are not routine validation. Run them only when the operator asks for that lane, the plan acceptance criteria name it, or the objective is release readiness.
- Release readiness uses `release_only=true` lanes only when preparing a tag, release PR, or explicit release audit.
- If a lane has `isolated_agents_home=true`, create an isolated agent state directory before running it instead of writing into persistent local agent knowledge.
- If `writes_artifacts=true`, check `artifact_paths` before and after the run and report the generated files.
- If `mutation_escape_hatch` is non-null, do not run that lane as routine validation; name the escape hatch in the handoff or release evidence.
- If a command is not represented by a lane, treat `go test -race`, `-shuffle`, `-count=N` where `N > 1`, eval runners, retrieval bench, headless runtime smoke, and release gates as explicit-only.

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
- `validation_lanes`
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
  "validation_lanes": [
    {
      "name": "worktree-disposition-read-only",
      "command": "bash scripts/check-worktree-disposition.sh",
      "purpose": "Classify worktree cleanliness before landing.",
      "read_only": true,
      "writes_artifacts": false,
      "isolated_agents_home": false,
      "release_only": false,
      "mutation_escape_hatch": null,
      "cost_class": "cheap",
      "auto_select": "default",
      "timeout_seconds": 30
    },
    {
      "name": "local-ci-release",
      "command": "scripts/ci-local-release.sh",
      "purpose": "Run full local release readiness validation.",
      "read_only": false,
      "writes_artifacts": true,
      "artifact_paths": [
        ".agents/releases/local-ci/**"
      ],
      "isolated_agents_home": true,
      "release_only": true,
      "mutation_escape_hatch": "operator-run-release-validation",
      "cost_class": "expensive",
      "auto_select": "release-only",
      "timeout_seconds": 900,
      "expensive_reason": "Full release readiness gate writes release evidence and runs broad validation."
    }
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
