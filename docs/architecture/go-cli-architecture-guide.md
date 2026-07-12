# Understanding the AgentOps Go CLI

**Audience:** Developers who have completed Go 101 and want to reason about the code, not merely invoke it.
**Validated:** 2026-07-12 against `goal/go-cli-clean-architecture-repair` at `a07bc8f44675e74fad2b0703c66cd370baf26354`.

> **Current status:** The CLI is in a compatibility-first strangler migration. Ten command families are sealed as migrated, `goals` remains migrating, and the tested `cliapp.BuildRoot` target does not yet construct the production root. Revalidate the current-status and debt sections when either cutover lands.

Related references: [Codebase Overview](codebase-overview.md) · [Go CLI Production-Readiness Audit](../audits/2026-07-12-go-cli-production-readiness.md) · [Ports and Adapters](ports-and-adapters.md) · [Operating Loop](operating-loop.md) · [Generated CLI Reference](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md)

## What You Should Be Able to Do After This Guide

You should be able to:

1. Start at `main()` and trace an `ao` invocation to its effect and output.
2. Explain the difference between Cobra presentation, application policy, a port, and an adapter.
3. Read a command module and identify what belongs there and what does not.
4. Understand the Go idioms used for dependency injection, errors, contexts, I/O, interfaces, and deterministic behavior.
5. Distinguish the intended clean architecture from the still-live strangler scaffolding.
6. Add or review a command without copying transitional mistakes.

## 1. The Honest One-Paragraph Mental Model

`ao` is one Go executable built around Cobra. Today, the live executable is still assembled in the large `package main` under `cli/cmd/ao`: many files register commands through `init()`, a root pre-run negotiates global behavior, and build profiles prune the assembled tree. The refactor is strangling that shape one top-level command family at a time. A migrated family has an explicit module under `cli/internal/commands/<family>`, application policy in an internal package, concrete effects under `cli/internal/adapters`, a tiny composition owner in `cmd/ao`, an explicit machine contract, and compatibility tests that prove the public CLI did not change. Eleven families have modules; `gate` is fully cut over; `goals` is currently a dormant replacement being built beside one live legacy owner.

That paragraph contains two architectures:

- **Live transitional architecture:** package-wide Cobra globals + `init()` registration + build-tag pruning.
- **Target architecture:** deterministic root assembly from explicit modules, contracts, services, ports, and adapters.

Never confuse “the target types exist” with “production is using the target bootstrap.”

## 2. Scale and Shape

At the inspected commit:

| Measure | Current value |
|---|---:|
| Go files | 1,618 |
| Production Go files | 833 |
| Go test files | 785 |
| Go packages | 152 |
| Test/fuzz/benchmark functions | 7,675 |
| Explicit command-module packages | 11 |
| `cmd/ao` production lines | 66,430 |
| `internal` production lines | 106,877 |
| `cmd/ao` production `init()` functions | 186 |
| Direct subprocess sites remaining in `cmd/ao` | 72 |
| Default compiled command records | 173 |

The validated execution base had 77,691 `cmd/ao` lines, 205 production `init()` functions, and 89 direct subprocess sites. The direction is real, but this is not a finished small CLI.

## 3. Repository Map for the CLI

```text
cli/
├── cmd/ao/                    live executable package and composition boundary
│   ├── main.go                process entrypoint
│   ├── root.go                live root, global flags, pre-run, exit mapping
│   ├── *_module.go            composition for some migrated families
│   ├── *_composition.go       composition for other migrated families
│   └── hundreds of legacy command files still being strangled
├── internal/
│   ├── cliapp/                target profile/module/root assembly contract
│   ├── clicontract/           explicit machine-readable command metadata
│   ├── commands/<family>/     Cobra presentation for migrated families
│   ├── <application>/         use cases and policy, e.g. gate, done, claim
│   ├── adapters/<family>/     filesystem/process/tracker/runtime implementations
│   ├── ports/                 cross-context stable port types
│   ├── domain/                innermost domain aggregates and invariants
│   └── gates/                 BC2 declarative gate registry/orchestrator/report
├── embedded/                  files compiled into the binary
├── testdata/compatibility-baseline/
│                               immutable compiled behavior fixtures
└── docs/COMMANDS.md           generated command reference; never hand-edit
```

Go's `internal` directory is language-enforced encapsulation: packages outside the parent module tree cannot import these packages. It is a compiler boundary, not merely a naming convention.

## 4. The Live Process Entry

The real start is intentionally boring:

```go
func main() {
    Execute()
}
```

See `cli/cmd/ao/main.go:11`.

`Execute()` calls `rootCmd.ExecuteC()` and translates typed errors to process exit codes (`cli/cmd/ao/root.go:102`). The root is a package global declared at `root.go:29`. Global flags and command groups are registered in `root.go:210`.

Before any command handler runs, `PersistentPreRunE` does five host-wide jobs (`root.go:51`):

1. Resolve `--json` versus `--output`.
2. Mirror an explicit `--config` into `AGENTOPS_CONFIG`.
3. sanitize Git-related child-process environment;
4. repair shared worktree configuration;
5. build the transitional `App` and place it in `context.Context`.

The `App` is not the target dependency model. It is a broad transitional service bag (`cli/cmd/ao/app.go:12`) and only a few production files consume it. New command modules instead receive narrow constructor dependencies.

### Why `ExecuteC`, Not Just `Execute`?

Cobra's `ExecuteC()` returns both the command that ran and the error. This lets the root print context-sensitive hints and map errors after a subcommand has decided what happened.

### Typed Exit Errors

The root eventually recognizes any error satisfying:

```go
type commandExitError interface {
    error
    ExitCode() int
}
```

See `root.go:205`. This is an important dependency inversion: the root need not import every family's concrete error type. In Go, interface satisfaction is implicit. If a family's error has `Error() string` and `ExitCode() int`, it satisfies this interface automatically.

## 5. How the Live Cobra Tree Is Assembled

### Current Production Path

Most command files have an `init()` function that calls `rootCmd.AddCommand(...)`. Go runs package initialization before `main()`. After registrations, `zzz_default_spine.go:24` prunes commands that are not in the default product profile.

```text
Go loads package main
  → package variables initialize
  → init() functions register commands
  → final pruning init() removes non-default commands
  → main()
  → Execute()
```

This works, but it has architectural costs:

- initialization order becomes behavior;
- tests share mutable Cobra state;
- profile membership is removal after construction instead of selection before construction;
- command ownership is hard to inspect;
- package globals couple unrelated files.

### Target Path

`cli/internal/cliapp` defines the intended replacement:

```go
type Module interface {
    Contract() clicontract.CommandContract
    Command() *cobra.Command
}
```

`BuildRoot(profile, modules...)` validates contracts, filters modules by profile, rejects duplicate IDs/names/aliases, sorts deterministically, and builds a fresh Cobra root (`cli/internal/cliapp/root.go:19`).

At this commit, production has **no non-test call** to `cliapp.BuildRoot` or `ParseProfile`. The target assembly contract is proven by unit tests but is not yet the executable bootstrap.

That is the clearest example of strangler scaffolding versus live cutover.

## 6. The Target Command Pipeline

For a migrated effectful command, use this mental pipeline:

```text
argv / environment
       │
       ▼
Cobra command module
parse flags + args; build typed request
       │ driving interface
       ▼
application service / use case
validate intent; apply policy; orchestrate
       │ outbound port
       ▼
adapter
filesystem / process / tracker / Git / clock / network
       │
       ▼
typed result or typed error
       │
       ▼
command renderer
human / JSON / YAML / stderr
       │
       ▼
root exit-code mapping
```

The layers are about **reasons to change**:

| Layer | Changes when... | Must not own... |
|---|---|---|
| Command module | flags, aliases, help, output format change | business policy or direct effects |
| Application service | rules and orchestration change | Cobra details or concrete OS calls |
| Port | the application needs a stable capability | tool-specific flags/process mechanics |
| Adapter | the filesystem/tracker/process implementation changes | application policy |
| Composition | concrete implementations are selected | command behavior |

## 7. Four Representative Command Traces

### Trace A: `ao capabilities` — Pure Read/Render

This is the smallest clean example.

1. `cli/cmd/ao/capabilities.go:11` composes a service with two adapters: the live Cobra surface and runtime platform.
2. `internal/commands/capabilities/module.go:38` creates a fresh command.
3. Its `RunE` calls `module.render` (`module.go:51`).
4. The renderer asks the injected `Builder` for a `Document` (`module.go:58`).
5. `internal/capabilities/document.go:75` combines a surface snapshot, platform, stable exit dictionaries, environment documentation, and robot surfaces.
6. The command writes JSON or YAML through `command.OutOrStdout()` (`module.go:64-69`).

Why this is testable: the application depends on `SurfaceReader` and `PlatformReader`, while the command depends on `Builder`. Tests pass tiny recording fakes, not a real Cobra root or OS platform.

### Trace B: `ao gate run <name>` — One Effect Through a Port

1. `internal/commands/gate/module.go:315` parses exactly one positional argument.
2. The handler builds `gate.RunRequest{Name: args[0]}` and calls the `RunUseCases` interface (`module.go:337`).
3. `cmd/ao/gate_composition.go:70` resolves the project directory and constructs `gate.RunService` with `adapters/gate.Runner`.
4. `internal/gate/run_service.go:19` validates name and runner, then calls `ports.GateRunnerPort.Run`.
5. `internal/adapters/gate/runner.go:25` maps the gate name to `scripts/check-<name>.sh`, executes Bash, captures output, and maps exit codes:
   - 0 → PASS
   - 2 → WARN
   - 75 → SKIP
   - other → FAIL
6. The module JSON-encodes the typed `GateVerdict` (`module.go:341`).

The use case does not know Bash exists. The adapter does not decide whether an empty requested name is valid application intent. That is the seam.

### Trace C: `ao gate check` — Orchestration and Multiple Renderers

1. The command captures flags in closure-local variables (`commands/gate/module.go:349`).
2. It builds `gate.CheckRequest` and calls `CheckUseCases.Execute` (`module.go:370`).
3. `gate.CheckService.Execute` validates dependencies, interprets the scope, selects Fast or Full mode, and creates an orchestrator (`internal/gate/check_service.go:39`).
4. `gates.Orchestrator` selects checks by tier and changed-file globs (`internal/gates/orchestrator.go:133`).
5. Each check is either:
   - native Go through `Check.Run`, or
   - script-backed through `GateRunnerPort` (`orchestrator.go:183`).
6. Blocking checks fail closed: PASS/WARN/SKIP clear; FAIL/UNKNOWN/invalid status do not (`orchestrator.go:191`).
7. One `Report` supports human output, stable JSON, GitHub annotations, and exit code (`internal/gates/report.go:13`).
8. The module returns a typed `ExitError`; the root maps it to a process code without duplicating output.

Notice three distinct things named “run”:

- Cobra `RunE`: presentation handler;
- `CheckService.Execute`: application orchestration;
- `Orchestrator.runOne`: dispatch of an individual registered check.

### Trace D: `ao goals` — A Safe Mid-Migration Family

There are deliberately two trees in source but only one live owner.

- The legacy tree is the global `goalsCmd`, registered at `cli/cmd/ao/goals.go:53`.
- The replacement tree is built by `newGoalsCommand()` in `goals_composition.go:15`, but is not added to `rootCmd`.

The replacement module already owns:

- simple read/delegate commands;
- management mutations;
- manual steer commands;
- exact aliases, groups, flags, and help;
- placeholder command shapes for future slices.

`TestGoalsCompositionBuildsDormantExecutableModule` executes the replacement's `validate` command and proves root ownership remains exactly one (`goals_composition_test.go:10`).

This is the strangler pattern:

```text
freeze old observable behavior
  → build executable replacement off the live path
  → move vertical use cases one group at a time
  → test replacement and old compatibility together
  → atomically register new owner and delete old owner
```

The placeholder `futureCommand` returns `use case not configured` (`commands/goals/module.go:428`). It is scaffolding, not delivered behavior. Do not register the replacement root until those placeholders are replaced.

## 8. The Gate Subsystem as a Miniature Architecture

The gate subsystem is worth learning separately because it repeats the whole architecture internally.

### Core Types

| Type | Location | Meaning |
|---|---|---|
| `gates.Check` | `internal/gates/gates.go:57` | declarative definition of one gate |
| `gates.Registry` | `internal/gates/registry.go:12` | concurrency-safe map of checks, returned in sorted order |
| `gates.RunOptions` | `internal/gates/orchestrator.go:13` | mode, scope, fail-fast policy |
| `ports.GateVerdict` | `internal/ports/gate_runner.go:16` | typed result of one check |
| `gates.CheckResult` | `internal/gates/orchestrator.go:24` | check definition + verdict + timing + evaluation error |
| `gates.Report` | `internal/gates/report.go:15` | complete run projected to several consumers |

### Declarative Registration

A check is flat data with exactly one implementation path: `Backing` script or native `Run` function (`gates/gates.go:53`). `Registry.Add` rejects malformed or duplicate checks (`registry.go:24`).

Native example: `internal/gates/checks/go_build.go:16` registers `go.build` with path globs and `runGoBuild`.

Script example: rows in `internal/gates/checks/seed.go:273` declare IDs, tiers, match globs, blocking posture, backing script, arguments, and repair hints.

### Why the Registry Sorts

Go deliberately randomizes map iteration. `Registry.All` copies map values and sorts by ID (`registry.go:39`). Deterministic order is essential for stable CLI output, test fixtures, and reproducible diagnosis.

### Why Checks Run Serially

The orchestrator explicitly runs checks serially because scripts can share and mutate generated state (`orchestrator.go:45`). Parallelism would be faster but unsafe without stronger isolation.

## 9. Command Contracts and Profiles

`clicontract.CommandContract` declares:

- stable ID;
- profile membership bitmask;
- named positional-argument policy;
- output policy;
- effect set;
- exit-code classes.

See `internal/clicontract/metadata.go:79`.

### Why a Named Args Policy?

A function pointer such as `cobra.ExactArgs(1)` enforces behavior but is not serializable. `ArgsPolicy` pairs a stable machine name with the validator (`metadata.go:36`). The name can appear in capabilities/docs while the function enforces runtime behavior.

### Why Bitmasks?

Profiles and effects use powers of two created with `iota`:

```go
const (
    ProfileDefault ProfileSet = 1 << iota
    ProfileFlywheel
    ProfileLegacy
    ProfileCombined
)
```

Bitwise OR combines membership; bitwise AND tests membership. This is compact and type-safe enough for a small closed set.

### Current Contract Caveat

The explicit contract system is not fully wired into the live machine projection:

- eight live composition files call `clicontract.Attach`;
- current `gate` and `eval` composition do not attach their module contracts;
- dormant `goals` is not attached because it is not live;
- `clicontract.Inspect` currently infers metadata and does not call `ContractFor`/`ProjectContract`;
- live `ao capabilities` therefore reported `gate` and `goals` as `effects: pure` and `args: subcommands-only`, even though `gate.Module.Contract()` declares stateful effects.

Treat explicit contracts as partially active foundation, not yet the single source of truth promised by the target architecture.

## 10. Tracker and Workspace Resolution

`internal/trackerresolve` is a good example of isolating complicated environment policy.

`Resolution` carries the selected tracker, binary, ledger, selection source, repo/worktree facts, working directory, and child environment (`resolve.go:30`).

Selection precedence (`resolve.go:56`):

1. `AGENTOPS_TRACKER` environment override;
2. `.agentops/config.yaml` tracker key;
3. natural `_beads` ledger;
4. natural `.beads` ledger;
5. `br` found on PATH;
6. `bd` found on PATH;
7. actionable error.

Important nuance:

- `BEADS_DIR` tells an already-selected `br` process where its ledger lives.
- It does **not** select `br` over a repository's natural `.beads` ledger.
- `br` children receive one canonical `BEADS_DIR`.
- `bd` children have `BEADS_DIR` removed so native `.beads` discovery wins.

Worktrees complicate discovery. `resolveWorkspace` asks Git for `--git-common-dir`; in a linked worktree this points back toward the canonical checkout, where this repo's private `_beads` lives (`resolve.go:258`).

This package embodies a useful rule: resolve ambiguous environment once into a typed record, then pass the record downstream instead of re-reading globals everywhere.

## 11. Configuration

The documented configuration cascade is:

1. command-line flags;
2. `AGENTOPS_*` environment variables;
3. project `.agentops/config.yaml`;
4. home `~/.agentops/config.yaml`;
5. defaults.

The migrated config command illustrates the separation:

- Cobra module parses `--show`, `--set-tier`, and `--set-skill` and renders (`commands/config/module.go`).
- `config.CommandService` validates model-tier policy and operates through `CommandGateway` (`internal/config/command_service.go:48`).
- the adapter gateway owns actual environment/filesystem/config loading.

The service accepts `context.Context` for interface consistency, but some methods do not yet use it. `_ context.Context` makes that explicit and avoids a compiler error for an unused parameter.

## 12. Go Idioms Used Here

### 12.1 Small Interfaces Near the Consumer

Examples:

- `claimcommands.UseCases` (`commands/claim/module.go:20`)
- `donecommands.UseCases` (`commands/done/module.go:16`)
- gate's `ReviewUseCases`, `RunUseCases`, `CheckUseCases` (`commands/gate/module.go:19`)

The consumer declares only what it needs. Concrete services satisfy the interface implicitly. This makes command tests trivial to fake and avoids a giant shared “service interface.”

### 12.2 Constructor Injection

`NewModule(...)`, `NewService(...)`, and adapter constructors make dependencies visible. Go has no built-in dependency-injection framework; ordinary constructors and interfaces are usually clearer.

### 12.3 Compile-Time Interface Assertions

Adapters often include:

```go
var _ ports.GateRunnerPort = (*Runner)(nil)
```

See `adapters/gate/runner.go:93`. It creates no runtime value. The assignment asks the compiler to prove `*Runner` satisfies the interface.

### 12.4 `context.Context` First

Effectful use cases accept context as the first argument. Adapters check `ctx.Err()` or use `exec.CommandContext`, allowing cancellation to cross layers. Do not store a context in a struct; pass it per operation.

### 12.5 Error Wrapping and Inspection

`fmt.Errorf("gate run: %w", err)` adds context while preserving the original error. Callers can use `errors.Is` or `errors.As` through the chain. Custom errors may implement `Unwrap`, as `close.Failure` does (`internal/close/service.go:91`).

### 12.6 Errors Versus Negative Domain Results

An absent gate script returns `GateVerdict{Status: UNKNOWN}`, not a Go error. The adapter ran and produced a domain outcome. A failure to launch the subprocess machinery may be an error or UNKNOWN depending on the adapter contract. In the orchestrator, a blocking UNKNOWN fails closed.

This distinction matters:

- **error:** computation could not be evaluated;
- **FAIL verdict:** computation was evaluated and found a defect;
- **UNKNOWN verdict:** adapter could not decide, and caller policy chooses what that means.

### 12.7 Closure-Local Flag State

Migrated commands declare variables inside `Command()` or a child constructor, then bind flags to those variables. Each call gets fresh state. Examples: `done.options`, `gate.newCheckCommand`, `config.Command`.

Legacy commands often bind flags to package globals, so tests can contaminate one another.

### 12.8 Injected I/O

Handlers use:

- `command.InOrStdin()`
- `command.OutOrStdout()`
- `command.ErrOrStderr()`

This follows Go's `io.Reader`/`io.Writer` composition model and lets tests use `bytes.Buffer`. Direct `fmt.Println` or `os.Stdout` in a command handler is harder to isolate.

### 12.9 Request and Result Structs

Once an operation has several fields, a typed request prevents positional-parameter confusion and makes evolution easier. Examples: `done.Request`, `gate.CheckRequest`, `claim.BindRequest`, `trackerresolve.Resolution`.

### 12.10 Value Versus Pointer Receivers

Small immutable-ish services/modules often use value receivers. Stateful adapters or services with identity use pointers. Neither is automatically better:

- value receiver communicates “calling this does not replace the receiver”;
- pointer receiver avoids copying, permits mutation, and is required for some interface method sets.

### 12.11 Defensive Copying

Code uses `append([]string(nil), source...)` and explicit map cloning before storing or returning caller-owned data. This prevents later caller mutation from changing an attached contract or request after validation.

### 12.12 Deterministic Sorting

Map iteration is not stable. Commands, flags, registry checks, completions, and model overrides are copied and sorted before output. Determinism is part of a CLI's public contract.

### 12.13 Build Tags

Files beginning with `//go:build flywheel`, `legacy`, `windows`, `linux`, or negations are conditionally compiled. Profiles are presently a mixture of build tags and default-spine pruning. Platform-specific files provide separate implementations with identical package APIs.

### 12.14 `init()` Is Legal Go, but Transitional Here

`init()` is appropriate for small, undeniable package registration, but 186 production `init()` functions make the command graph implicit. The gate registry still intentionally uses decentralized `init()` registration for checks; the CLI root is moving away from it. Context matters.

## 13. Extracted Patterns Across Concrete Families

### Pattern 1: Fresh Command Module

**Instances:** capabilities, claim, config, done, gate, goals.

**Invariant:**

- `Module` stores injected collaborators;
- `Contract()` describes the top-level command;
- `Command()` returns a fresh Cobra tree;
- handlers parse, delegate, render.

**Variance:**

- pure commands inject a builder;
- simple effectful commands inject one use-case interface;
- large families group several narrow use-case interfaces;
- host-global concerns arrive as tiny functions in `HostOptions`.

**Do not over-extract:** a universal `Dependencies` bag would recreate the monolith. Keep constructors family-specific.

### Pattern 2: Consumer Interface → Application Service → Adapter

**Instances:** claim, done, gate review/run/check, close, config.

**Invariant:**

- command owns the driving interface;
- application owns policy;
- application declares or consumes outbound ports;
- composition chooses concrete adapters.

**Variance:**

- some ports live in the application package because only that context needs them;
- cross-context ports live under `internal/ports`;
- some services return typed domain results, while writer-injected legacy owners still return only `error`.

### Pattern 3: Typed Exit Result

**Instances:** gate `ExitError`, close `Failure`/command `exitError`, beads `ExitError`, root's family-neutral `commandExitError`.

**Invariant:**

- the command prints the explanation once;
- an error carries the desired process code;
- the root maps the code without importing family details.

**Variance:**

- some errors carry messages;
- some deliberately return an empty `Error()` because output already happened;
- some wrap a cause for `errors.As`/`Unwrap`.

### Pattern 4: Multi-Renderer Result

**Instances:** gate report, capabilities document, config results, claim proof report.

**Invariant:** application returns data; command or report owns human/JSON/YAML projection; writers are injectable.

**Variance:** highly structured subsystems centralize renderers on the result type; smaller commands keep renderer functions in the command package.

### Pattern 5: Compatibility-First Strangler Slice

**Instances:** beads, capabilities, claim, close, config, council-gate, doctor, done, eval, gate, goals.

**Invariant:**

1. freeze compiled behavior and owner scope;
2. build module/service/adapter slices;
3. keep exactly one live owner;
4. cut over atomically;
5. run family plus cumulative compatibility;
6. seal lineage at the accepted SHA.

**Variance:** small families cut over in one slice; large families use multiple vertical child slices; goals uses a dormant executable module before cutover.

## 14. Testing Strategy

### L0/L1: Pure Policy and Module Delegation

- fake use-case interfaces;
- call a fresh `Command()`;
- set args and `bytes.Buffer` writers;
- assert the typed request passed to the fake;
- assert output and errors.

Example: `commands/goals/module_test.go:58` proves manual-steer arguments are resolved before delegation.

### L2: Adapters Against Real-ish Resources

- `t.TempDir()` filesystem;
- temporary Git repository;
- executable test script;
- real Cobra tree with fake application service.

Example: `adapters/gate/runner_test.go` exercises exit mapping and output tail behavior.

### Compiled Black Box

`cli/testdata/compatibility-baseline/families/<family>` freezes help, flags, output, exit behavior, tracker selection, effects, ownership, and lineage across default/flywheel/legacy/combined profiles.

This catches what unit tests miss: exact user-visible behavior of the built executable.

### Architecture Fitness

`scripts/check-go-cli-architecture.sh` and `internal/archcheck` assert package ownership, allowed paths, adapter effects, legacy-symbol deletion, and one owner.

### Why All Four Layers Matter

- A service unit can be correct while Cobra wires the wrong flag.
- A module test can pass while the real binary omits the command under a build tag.
- A black-box test can preserve behavior while the code remains architecturally tangled.
- An architecture scanner can be green while output bytes regress.

## 15. Known Transitional Debt: What Not to Cargo-Cult

1. **Live root still uses global `init()` assembly.** `cliapp.BuildRoot` is not in production.
2. **Explicit contracts are not the live projection source.** `Inspect` still infers; some migrated families do not attach contracts.
3. **`App` is a broad context service bag.** It was an intermediate improvement, not the end-state dependency model.
4. **`goalsapp` services currently delegate to effectful `internal/goals.Run*` functions.** This is an intermediate seam; final effect ownership still needs adjudication.
5. **Dormant goals placeholders deliberately error.** They are command-shape scaffolding, not completed behavior.
6. **Some family contracts describe only the root container.** For example, `beads` declares `EffectPure` despite effectful children; do not assume top-level metadata fully represents descendants.
7. **Narrative text contains historical tracker wording.** Executable `trackerresolve` supports both `br` and `bd`; this repo itself tracks with `br`.
8. **Direct effects remain in `cmd/ao`.** The strangler is incomplete by measurement, not merely by style preference.
9. **`capabilities` YAML behavior and help text disagree.** The command help says output is always JSON, but `ao --output yaml capabilities` takes a live YAML branch. Because `Document` fields currently carry only `json` tags, that YAML uses keys such as `schemaversion` rather than the JSON contract's `schema_version`. Treat JSON as the stable wire contract until this is deliberately reconciled.
10. **The `beads` command help is still one-sided despite a dual-tracker implementation.** The family description says it complements `bd`, while `beads dir` says it prints the live `br` ledger; the actual resolver and directory service deliberately support either `br` or `bd`. Treat `beads tracker --json` and the typed `trackerresolve` policy as authoritative over those stale help strings.
11. **The default compiled binary prunes its own hidden `buildtags` introspection command.** `buildtags.go` and its test describe a default result of `spine (...)`, but package test binaries skip `pruneToDefaultSpine`, while the real untagged binary removes `buildtags` because it is absent from `defaultSpineCommands`. A `flywheel legacy` build exposes the command and reports both tags; the default binary currently returns `unknown command`. This is a compiled-black-box coverage gap, not merely stale prose.
12. **The current goals migration is intentionally red at the architecture boundary.** At source `a07bc8f4`, `scripts/check-go-cli-architecture.sh` reports 373 `family.legacy-symbol` violations from the still-live `cmd/ao/goals*` implementation. The goals lineage says `migration_state: migrating`, yet its frozen ownership record labels `cli/internal/commands/goals` as `live_owner`; runtime registration proves the legacy `goalsCmd` is still the sole executable owner. Read that field as the declared destination owner during this migration, not proof of production reachability. A green architecture checker remains a required cutover condition.
13. **The migrated `claim` adapter contradicts the shared `bd` child-context policy.** `trackerresolve` returns a canonical repository `WorkDir` and a `ChildEnv` with `BEADS_DIR` stripped for `bd`, and `beads exec` honors them. `internal/adapters/claim.Tracker` instead runs from the caller cwd and reconstructs the inherited environment, while its test explicitly expects a foreign `BEADS_DIR` to survive. Thus “migrated” proves ownership/boundary conformance, not cross-family semantic correctness; the `claim` path currently needs policy reconciliation with the canonical resolver.

## 16. How to Read Any Command Without Getting Lost

Use this seven-question worksheet:

1. **Registration:** where does this top-level command enter `rootCmd`?
2. **Presentation:** which package owns `Use`, aliases, flags, `RunE`, and rendering?
3. **Request:** what typed value crosses out of Cobra?
4. **Policy:** which service validates and orchestrates it?
5. **Effects:** which port names the needed capability?
6. **Adapter:** which concrete implementation touches the world?
7. **Proof:** which unit, adapter, composition, and black-box tests pin it?

For migrated code, start with `cmd/ao/<family>_module.go` or `_composition.go`, jump to `internal/commands/<family>`, then follow the injected interface inward. Do not start by reading a 700-line module top to bottom.

## 17. Ideal Recipe for a New Command Family After Cutover

1. Define the behavior and compatibility surface first.
2. Put application request/result types and policy in the owning bounded-context package.
3. Define the smallest outbound ports needed by that policy.
4. Implement real effects under `internal/adapters/<family>` with compile-time interface assertions.
5. Create `internal/commands/<family>/module.go`:
   - family-specific constructor;
   - explicit `Contract()`;
   - fresh `Command()`;
   - closure-local flags;
   - parse/delegate/render only.
6. Compose the module at the executable boundary.
7. Test policy, adapter, module delegation, composition ownership, and compiled behavior.
8. Generate command docs from the source.

Do not add a global service locator, a shared mega-`Dependencies` struct, or another concrete effect in `cmd/ao`.

## 18. Exercises

### Exercise 1: Trace Without Running

For `ao gate run compile-health`, write the ordered function/type path from Cobra handler to Bash process and back to JSON.

### Exercise 2: Predict the Result

What should `ao gate run definitely-not-a-real-gate` print? Is the missing script a Go error, FAIL, or UNKNOWN? What would happen if the same UNKNOWN were returned for a blocking registry check?

### Exercise 3: Layer Classification

Classify each responsibility:

- parse `--scope range:main..HEAD`;
- validate the range syntax;
- set `AGENTOPS_GATE_RANGE`;
- select fast versus full checks;
- emit GitHub annotations.

Choose among command, application, adapter, orchestrator/domain policy, renderer.

### Exercise 4: Interface Reading

Why can `gateadapter.Runner` be passed where `ports.GateRunnerPort` is required even though no `implements` keyword appears?

### Exercise 5: Find the Scaffolding

Prove from code searches that `cliapp.BuildRoot` is not the live production bootstrap. Then identify which test proves its intended behavior.

### Exercise 6: Spot the Contract Gap

Compare `gate.Module.Contract()` with live `ao capabilities` output for `ao gate`. Explain why they differ at this commit.

### Exercise 7: Design a Thin Handler

Re-sketch `ao claim check --changed --json` as a thin Cobra handler. It should build a typed request, call one interface, and render through the command writer. Do not include filesystem or subprocess calls.

## 19. Answer Key

### Answer 1

`commands/gate.Module.newRunCommand.RunE` → `RunUseCases.Execute` → `main.gateRunUseCases.Execute` → `gate.RunService.Execute` → `ports.GateRunnerPort.Run` → `adapters/gate.Runner.Run` → `exec.CommandContext("bash", script)` → `ports.GateVerdict` → JSON encoder on `command.OutOrStdout()`.

### Answer 2

The adapter returns a JSON `GateVerdict` with `Status: UNKNOWN` and a reason naming the absent script. It is a domain result, not a Go error. If a blocking registry check returns UNKNOWN, `isBlockingFail` treats it as failure because only PASS/WARN/SKIP clear a blocking check.

### Answer 3

- parse the scope string: command request construction;
- validate range syntax: application/runtime policy (`gates.ValidateRangeSpec`);
- set environment: adapter (`CheckRuntime`);
- select fast/full checks: application/orchestrator policy;
- GitHub annotations: renderer on `Report`.

### Answer 4

Go interfaces are satisfied implicitly by method set. `*Runner` has `Run(context.Context, ports.GateRunRequest) (ports.GateVerdict, error)`. The compile-time assertion at the bottom of `runner.go` proves it.

### Answer 5

`rg 'BuildRoot(' cli --glob '!**/*_test.go'` finds only the function definition. `internal/cliapp/root_test.go` proves profile selection, fresh trees, conflict rejection, and deterministic ordering.

### Answer 6

The module contract declares stateful effects and `no-args`. `gate_composition.go` does not attach it, and `clicontract.Inspect` currently infers metadata instead of projecting attached contracts. The live output therefore reports the fallback `pure`/`subcommands-only` shape.

### Answer 7

One acceptable shape:

```go
type CheckUseCase interface {
    Check(context.Context, CheckRequest) (CheckResult, error)
}

func (m Module) checkCommand() *cobra.Command {
    var changed bool
    var base string
    cmd := &cobra.Command{Use: "check", Args: cobra.NoArgs}
    cmd.Flags().BoolVar(&changed, "changed", false, "check changed claims")
    cmd.Flags().StringVar(&base, "base", "origin/main", "comparison base")
    cmd.RunE = func(cmd *cobra.Command, _ []string) error {
        result, err := m.check.Check(cmd.Context(), CheckRequest{Base: base, ChangedOnly: changed})
        if err != nil { return err }
        return renderCheck(cmd.OutOrStdout(), result, m.output() == "json")
    }
    return cmd
}
```

## 20. Glossary

| Term | Meaning here |
|---|---|
| Cobra | Go library that parses commands, flags, help, and dispatch |
| command family | one top-level command and all descendants, e.g. `gate` |
| command module | package that owns a family's Cobra presentation |
| composition root | outer boundary that chooses concrete implementations |
| use case/service | application policy for one user intent |
| port | interface naming a capability across a boundary |
| adapter | concrete implementation of a port using the outside world |
| driving adapter | outside-in entry, such as Cobra |
| driven adapter | inside-out integration, such as filesystem or tracker |
| contract | explicit machine declaration of command behavior |
| profile | default/flywheel/legacy/combined compiled command membership |
| strangler | replace a legacy system incrementally while behavior stays live |
| dormant module | executable replacement built and tested but not registered |
| typed error | error value whose methods carry machine meaning such as exit code |
| fixture | frozen test input/output representing public behavior |

## 21. Suggested Study Order

1. Read `main.go`, then `root.go:29-78` and `root.go:102-208`.
2. Trace `capabilities` end to end.
3. Trace `gate run`, then `gate check`.
4. Read `trackerresolve.ResolveWithLookPath` and its table tests.
5. Compare `done` command/service/adapters.
6. Study dormant `goals` and explain why it is not registered.
7. Read `cliapp.BuildRoot` and identify what must change before it can replace live bootstrap.
8. Complete the exercises without the answer key, then review your misses.

## 22. Primary Source Index

| Topic | Source |
|---|---|
| process entry and exit mapping | [`main.go`](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/main.go), [`root.go`](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/root.go) |
| transitional root/profile pruning | [`zzz_default_spine.go`](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/zzz_default_spine.go) |
| target module/root assembly | [`cliapp/module.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/cliapp/module.go), [`cliapp/root.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/cliapp/root.go) |
| explicit command metadata | [`clicontract/metadata.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/clicontract/metadata.go) |
| pure module example | [`commands/capabilities/module.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/commands/capabilities/module.go) |
| complex module example | [`commands/gate/module.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/commands/gate/module.go) |
| application orchestration | [`gate/check_service.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/gate/check_service.go) |
| effect adapters | [`adapters/gate/runner.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/adapters/gate/runner.go) |
| gate registry/orchestrator/report | [`registry.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/gates/registry.go), [`orchestrator.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/gates/orchestrator.go), [`report.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/gates/report.go) |
| tracker selection/worktree policy | [`trackerresolve/resolve.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/trackerresolve/resolve.go) |
| typed close policy | [`done/service.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/done/service.go), [`close/service.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/close/service.go) |
| active strangler example | [`goals.go`](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/goals.go), [`commands/goals/module.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/commands/goals/module.go) |
| compatibility fixtures | [`families/`](https://github.com/boshu2/agentops/tree/main/cli/testdata/compatibility-baseline/families) |
| architecture fitness | [`archcheck/checker.go`](https://github.com/boshu2/agentops/blob/main/cli/internal/archcheck/checker.go), [`check-go-cli-architecture.sh`](https://github.com/boshu2/agentops/blob/main/scripts/check-go-cli-architecture.sh) |

## 23. Capstone: Reconstruct `ao done` From Source

Do this exercise from source before reading the key. Start at
`cli/cmd/ao/done_composition.go`, then follow only the symbols called by the
selected path.

### Scenario

Assume:

- the user runs `ao done age-9 --sha abcdef012345 --reason Shipped --json`;
- the local provenance ledger contains a `wasDerivedFrom` edge from
  `age-9@abcdef0` (type `verdict`) to commit `abcdef012345` with
  `disposition=CONFIRMED`;
- the tracker closes successfully and prints `closed by br`.

Answer these before running anything:

1. Where is the command registered, and which concrete values satisfy its three
   service ports?
2. Which layer parses the bead ID, SHA, reason, and JSON flag? Write the exact
   application `Request` it creates.
3. Which layer decides whether the verdict is sufficient to close the bead?
4. Which repository and ledger operations run even though `--sha` was explicit?
5. What exact close-reason string reaches the tracker?
6. What fields appear in JSON, and why does `closed by br` not appear?
7. Predict the behavior if the only matching verdict is `REFUTED`.
8. Predict the behavior if there is no verdict but every changed file is under
   `docs/provenance/`.
9. Predict the behavior if there is no verdict and `--force-no-verdict` is set.
10. Identify at least three implementation risks or transitional seams that a
    green happy-path test would not expose.

### Capstone Key

1. `done_composition.go` constructs package-global `doneModule` and
   `doneCommand`, then its `init()` attaches the contract and adds the command to
   `rootCmd`. `doneadapter.Repository`, `doneadapter.Ledger`, and
   `doneadapter.Tracker` satisfy `RepositoryPort`, `LedgerPort`, and
   `TrackerPort` respectively. The tracker delegates through the host's shared
   beads command helper.
2. `internal/commands/done.Module` owns Cobra parsing. It creates:

   ```go
   done.Request{
       BeadID: "age-9",
       SHA: "abcdef012345",
       Reason: "Shipped",
       ForceNoVerdict: false,
   }
   ```

   The local `--json` flag is presentation state and is not placed in the
   application request.
3. `internal/done.Service.Execute` owns the close policy. Cobra does not inspect
   provenance, and adapters do not choose the disposition.
4. The service always calls `RepositoryPort.WorkingDir`, even with explicit
   `--sha`, and always calls `LedgerPort.Read`. It does not call `ResolveHead` in
   this scenario. With a locally confirmed verdict it does not need
   `CommitProvenanceOnly` or `OriginEdges`.
5. The tracker receives:

   ```text
   Shipped [verdict:abcdef0:CONFIRMED]
   ```

6. JSON contains `bead_id`, `commit_sha`, `disposition`, `stamp`,
   `close_reason`, and `closed`. `TrackerOutput` has `json:"-"`, so tracker
   chatter is kept out of stdout-as-data.
7. A local `REFUTED` disposition does not confirm the close. If origin does not
   provide a matching confirmed verdict and the force flag is false, the service
   returns `RefusalError` before calling the tracker. The message names `ao
   verify`, `ao pawl review`, the trivial waiver, and the explicit escape hatch.
8. With no disposition and `CommitProvenanceOnly(...) == true`, the service uses
   `waived-trivial`, stamps `[verdict:abcdef0:waived-trivial]`, and closes.
9. With no confirmed/trivial route and `ForceNoVerdict == true`, it uses
   `UNVERIFIED`, closes with `[verdict:abcdef0:UNVERIFIED]`, and human rendering
   adds a warning note. JSON reports the disposition without tracker chatter.
10. Valid findings include:

    - `doneModule` and `doneCommand` are eagerly constructed as package globals,
      so the service captures ambient ledger configuration during package
      initialization instead of through a fresh root assembler.
    - `Service.Execute` dereferences all three ports without explicit nil checks;
      an incorrectly composed service can panic rather than return a configuration
      error.
    - Human rendering slices `result.CommitSHA[:7]` and trusts the service result;
      a faulty alternative use-case implementation can panic on a short SHA.
    - The explicit contract declares `OutputNone` even though the command emits
      human and structured JSON output.
    - The command uses a local `--json` boolean instead of the shared output-mode
      callback used by newer modules, so `--output json` parity needs explicit
      compiled verification.
    - `WorkingDir` is required even with an explicit SHA, making a cwd failure
      block a request that might otherwise have enough explicit identity.
    - The tracker error text says `br close` even though the shared tracker helper
      may resolve a different supported backend.
    - A package/service test cannot prove default-profile reachability or the
      installed binary's exit behavior.

### Capstone Standard

You understand this family rather than merely using it when you can explain,
without reading help output:

- why JSON is presentation state rather than application policy;
- why a REFUTED result is a domain refusal rather than a subprocess failure;
- why the tracker is not invoked on refusal;
- where the trivial-waiver policy belongs;
- which effects each port hides;
- which tests would prove parsing, policy, adapter behavior, composition, and
  compiled reachability separately.

## 24. Teaching Objective Coverage Audit

| Requested outcome | Evidence in this guide |
|---|---|
| Explain the current architecture honestly | Sections 1-6 distinguish the live global/init/prune bootstrap from the tested-but-unused `cliapp.BuildRoot` target and trace the intended command pipeline. |
| Explain how commands work | Section 7 traces pure `capabilities`, effectful `gate run`, orchestrated `gate check`, and dormant `goals`; Sections 8-11 cover gate policy, contracts/profiles, tracker/worktree resolution, and config precedence. |
| Explain Go idioms at a Go-101 graduate level | Section 12 covers consumer-side interfaces, constructor injection, method assertions, context, error wrapping, result/error separation, closure flags, injected I/O, receiver choice, defensive copies, sorting, build tags, and `init()`. |
| Extract repository patterns rather than invent them | Section 13 derives five patterns from at least three concrete command families each: fresh modules, service/port/adapter chains, typed exits, multi-renderers, and compatibility-first strangler slices. |
| Teach verification and safe change-making | Sections 14-17 explain the test pyramid, non-cargo-cult debt, a seven-question reading workflow, and the ideal post-cutover command recipe. |
| Make the learner active rather than a passive user | Sections 16, 18-19, and 23 provide code-reading questions, implementation exercises, worked keys, and a complete `ao done` capstone with a stated mastery standard. |
| Apply codebase archaeology | The guide is pinned to source `a07bc8f4`, traces `main` to adapters, distinguishes executable truth from narrative, compares the target base, and records compiled/runtime discrepancies rather than hiding them. |
| Apply pattern extraction | Section 13 requires repeated instances and names applicability, structure, and tradeoffs rather than calling one example a pattern. |
| Produce a durable codebase report | This file is the report, with scale inventory, architecture maps, source index, current debt ledger, exercises, keys, and capstone. |

### Evidence Limits

The guide proves that the curriculum and verification exercises exist and are
grounded in the inspected source. It cannot observe a reader's private mental
state. The capstone standard is therefore the honest comprehension check: a
reader can demonstrate mastery by reconstructing `ao done` (or another family)
without relying on CLI help, then comparing the reasoning with the key.
