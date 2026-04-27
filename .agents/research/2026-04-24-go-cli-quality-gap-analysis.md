---
id: research-2026-04-24-go-cli-quality-gap-analysis
type: research
date: 2026-04-24
backend: codex-sub-agents-plus-inline
ranked_packet: ".agents/rpi/ranked-packet-2026-04-24-go-cli-quality.md"
---

# Research: Go CLI Quality Gap Analysis

## Summary

The `ao` CLI is useful and unusually broad: it exposes the AgentOps control plane for initialization, retrieval, RPI orchestration, Codex lifecycle, overnight work, findings, plans, and quality gates. The quality baseline is stronger than a typical hobby CLI because it already has generated reference docs, command registration tests, JSON validity tests, completion tests, and `go vet` passing.

The gap to best-in-class Go CLIs is not "missing Cobra" or "missing tests." The gap is product-grade consistency at scale: stable automation contracts across all commands, complete shell completion coverage, live integration tests that degrade instead of panic, thinner command files, and a clearly classified command surface.

## Backend

- Selected backend: `codex-sub-agents`.
- A local explorer was dispatched as required by `$research`; the inline path continued with repo metrics and benchmark research while that explorer ran.

## Key Files

| File | Purpose |
|------|---------|
| `cli/cmd/ao/root.go` | Root Cobra command, command groups, global flags, context setup. |
| `cli/cmd/ao/completion.go` | Shell completion command. |
| `cli/docs/COMMANDS.md` | Generated CLI reference. |
| `scripts/generate-cli-reference.sh` | Source of generated command docs. |
| `cli/cmd/ao/json_validity_test.go` | Broad JSON output contract tests. |
| `cli/cmd/ao/flag_matrix_test.go` | Binary-level JSON/quiet/invalid-flag matrix. |
| `cli/cmd/ao/gc_bridge.go` | External `gc` bridge version/status checks. |
| `cli/cmd/ao/gc_bridge_test.go` | Live `gc` bridge tests; current full-suite failure. |
| `cli/internal/quality/doctor.go` | Example of better separation: reusable logic outside Cobra wiring. |
| `cli/cmd/ao/rpi_phased.go` | Long-running orchestration surface and timeout/runtime flag complexity. |

## Current Baseline

| Metric | Command | Result |
|--------|---------|--------|
| Go files under `cli/` | `find cli -name '*.go' -not -path '*/vendor/*' \| wc -l` | 762 |
| Production Go LOC under `cli/` | `find cli -name '*.go' -not -name '*_test.go' -print0 \| xargs -0 wc -l` | 100,180 |
| Test Go LOC under `cli/` | `find cli -name '*_test.go' -print0 \| xargs -0 wc -l` | 196,122 |
| Production Go LOC under `cli/cmd/ao` | `find cli/cmd/ao -name '*.go' -not -name '*_test.go' -print0 \| xargs -0 wc -l` | 57,765 |
| Test Go LOC under `cli/cmd/ao` | `find cli/cmd/ao -name '*_test.go' -print0 \| xargs -0 wc -l` | 140,955 |
| Documented top-level commands | `awk '/^### \`ao / {count++} END {print count}' cli/docs/COMMANDS.md` | 55 |
| Largest production command files | `find cli/cmd/ao -name '*.go' -not -name '*_test.go' -print0 \| xargs -0 wc -l \| sort -nr \| head` | `overnight.go` 1,587; `rpi_loop.go` 1,347; `codex.go` 1,344; `hooks.go` 1,180; `rpi_loop_supervisor.go` 1,140 |
| Production functions over CC 15 | `gocyclo -over 15 cli/cmd/ao cli/internal` | 38 production functions, max CC 19 |
| Build | `cd cli && make build` | PASS |
| Vet | `cd cli && go vet ./...` | PASS |
| CLI docs parity | `./scripts/generate-cli-reference.sh --check` | PASS |
| Focused JSON contract tests | `cd cli && go test ./cmd/ao -run 'TestJSONValidity|TestFlagMatrix_JSONOutput'` | PASS |
| Full Go suite | `cd cli && go test ./...` | FAIL: `TestGCBridgeVersion_Live` |

## Best-in-Class Comparison

### Machine-readable output

GitHub CLI sets a strong pattern: many commands expose `--json`, then let users reshape it with `--jq` or Go templates. See the official `gh help formatting` docs: https://cli.github.com/manual/gh_help_formatting.

Terraform treats human output as unstable for automation and provides stable JSON interfaces: `terraform output -json`, `terraform show -json`, machine-readable UI streams, and detailed exit codes for automation. Sources:
- https://developer.hashicorp.com/terraform/cli/commands/output
- https://developer.hashicorp.com/terraform/cli/commands/show
- https://developer.hashicorp.com/terraform/internals/machine-readable-ui
- https://developer.hashicorp.com/terraform/cli/commands/plan

`ao` has a global `--json` shorthand and many per-command JSON tests, but the surface is inconsistent: global `--output`, global `--json`, command-local JSON booleans, and separate `--format` flags coexist. The repo has already seen related findings around `--json` double-writes and output-mode side effects in `.agents/rpi/next-work.jsonl`.

### Generated docs and discoverability

Kubernetes publishes generated `kubectl` command docs from source and documents output format changes like `-o json`. Cobra also has first-class doc-generation and completion facilities. `ao` is already strong here: `scripts/generate-cli-reference.sh` builds `cli/docs/COMMANDS.md`, and parity passes.

The main gap is classification, not generation. `ao --help` exposes a very large command surface with 55 documented top-level commands. That breadth is useful for power users but creates discovery cost unless commands are intentionally grouped, hidden, deprecated, or promoted.

### Shell completion

Cobra supports completion generation and custom flag completions. `ao` has a `completion` command and custom static flag completions, but only supports bash, zsh, and fish in `cli/cmd/ao/completion.go:9-28`. Cobra's shell completion guide covers PowerShell too: https://cobra.dev/docs/how-to-guides/shell-completion/.

The current command fails for PowerShell:

```text
go run ./cmd/ao completion powershell
exit=1
Error: invalid argument "powershell" for "ao completion"
```

### Thin command layers and testable internals

Cobra's large-app pattern is thin command wiring over testable business logic. This repo demonstrates the better pattern in `doctor`: `cli/cmd/ao/doctor.go:69-74` delegates rendering and result logic to `cli/internal/quality/doctor.go:39-58`.

The broader `cli/cmd/ao` package remains heavy: 57,765 production LOC and several 1,000+ LOC command files. The best next step is not a package-wide rewrite; it is a hot-file budget for the most changed command files and extraction of reusable internal packages where a command already has stable behavior.

### Live integration boundaries

Best-in-class CLIs separate unit tests, deterministic integration tests, and optional live checks. `ao` currently has at least one live check that fails hard in ordinary local conditions. `gcBridgeVersion` reads stdout from `gc version` at `cli/cmd/ao/gc_bridge.go:60-65`; `/usr/bin/gc version` exits 0 with empty stdout and `Can't open version` on stderr, so `TestGCBridgeVersion_Live` reports empty output and then panics indexing `v[0]` at `cli/cmd/ao/gc_bridge_test.go:752-764`.

That should become a deterministic skip or a structured compatibility failure, not a panic.

## Evidence From Source

### Strengths

- The root command uses Cobra groups for help organization at `cli/cmd/ao/root.go:78-87`.
- Global `--output` and `--json` are registered at `cli/cmd/ao/root.go:89-96`.
- `PersistentPreRunE` injects an `App` context with resolved flags and working directory at `cli/cmd/ao/root.go:41-68`.
- Generated docs are explicit: `scripts/generate-cli-reference.sh:4-13` documents generation/check mode and `scripts/generate-cli-reference.sh:254-258` enforces parity.
- JSON validity has direct test helpers and many command cases in `cli/cmd/ao/json_validity_test.go:49-64`.
- The binary-level flag matrix validates real `ao` subprocess JSON output for selected commands at `cli/cmd/ao/flag_matrix_test.go:33-76`.
- Command registration tests enumerate the Cobra tree and expected top-level commands at `cli/cmd/ao/cobra_commands_test.go:366-433`.
- The companion local explorer found the intended `App` pattern in `cli/cmd/ao/app.go` and Options-style examples in `cli/internal/goals` and `cli/internal/quality`; that supports using pilot migrations rather than inventing a new architecture.

### Concerns

- The CLI surface is large enough that discoverability is a product problem: `cli/docs/COMMANDS.md` has 55 top-level command headings, while registration tests enumerate a wider internal surface at `cli/cmd/ao/cobra_commands_test.go:386-398`.
- PowerShell completion is missing despite Cobra support and Windows smoke being a CI concern: `cli/cmd/ao/completion.go:9-28`.
- Full-suite failure is reproducible in the live `gc` bridge test: `cli/cmd/ao/gc_bridge_test.go:752-764`.
- Some output paths still write directly to process stdout with `fmt.Printf`, especially orchestration/status paths. This is sometimes intentional, but the plan needs a machine-output audit so JSON/SSE modes cannot be polluted.
- Global package variables (`output`, `jsonFlag`, command-local JSON booleans) are used heavily in tests and command execution. That makes parallelization and isolation harder; `cli/cmd/ao/json_validity_test.go:21-45` shows the current cleanup pattern.
- The local explorer found that the shell JSON consistency script can warn instead of fail on invalid non-empty JSON; the implementation plan should harden the gate after classifying unsupported/stateful commands.
- The local explorer also found a standards drift: the Go style guide aspires to lower complexity than the currently enforced CI thresholds. Treat that as policy reconciliation, not an immediate code rewrite.

## Quality Validation

Coverage was broad enough for a planning discovery:

- Explored root command wiring, completion, docs generation, JSON tests, command registration, doctor separation, RPI orchestration, and live `gc` bridge behavior.
- Ran mechanical build/test/vet/doc checks.
- Compared against current official docs for Cobra, GitHub CLI, Kubernetes/kubectl docs generation, and Terraform CLI automation output.

Depth ratings:

| Area | Depth | Notes |
|------|-------|-------|
| Command architecture | 3/4 | Root/Cobra wiring and hot-file metrics are clear. |
| Output contracts | 3/4 | JSON tests and output flags are mapped; full per-command schema inventory remains implementation work. |
| Completion/docs | 3/4 | Current docs and shell support are clear. |
| Live integration quality | 3/4 | One concrete failure reproduced; broader external bridge audit remains planned. |
| Product usefulness | 3/4 | CLI is clearly useful as the AgentOps control plane; command taxonomy needs product design. |

## Recommended Direction

Create one epic with five implementation tracks:

1. Harden the live `gc` bridge test and version parsing.
2. Create a command-surface inventory that classifies public, hidden/internal, deprecated, and docs-visible commands.
3. Normalize machine-readable output contracts with a generated matrix and focused tests.
4. Add PowerShell completion and broaden flag-completion coverage for enum-like flags.
5. Start a hot-file architecture pass for the largest command files, beginning with extraction targets that preserve behavior and include paired tests.
6. Reconcile the CLI complexity policy so docs, lint config, and CI thresholds tell contributors the same story.

Test levels for downstream planning: L0 + L1 + L2. L0 for command/output contract matrices, L1 for helper behavior, L2 for built-binary command execution and generated docs/completion parity. L3 is not needed unless the implementation changes the RPI runtime orchestration flow itself.
