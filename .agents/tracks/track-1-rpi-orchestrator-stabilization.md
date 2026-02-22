# Track 1: RPI Orchestrator Stabilization

**Type:** Evolution cycle (RPI loop until done)
**Priority:** Primary — do this first
**Thesis:** Bring the orchestrator to hook-level trustworthiness.

## Context

The AgentOps RPI orchestrator (`ao rpi phased` + `ao rpi loop --supervisor`) is the core engine. Its philosophy is "agents are unreliable actors; intelligence should live in the system." The hooks deliver on this — they are shell scripts with kill switches, timeouts, and graceful degradation. The orchestrator aspires to the same trustworthiness but has not earned it yet.

Evidence: 5 of the last 9 releases contain RPI reliability fixes. The three most critical CLI subsystems have the lowest test coverage:

| Package | Coverage | Criticality |
|---------|----------|-------------|
| `cli/cmd/ao/` | ~46% | Highest — all user-facing commands |
| `cli/internal/rpi/` | ~33% | Highest — the core orchestrator |
| `cli/internal/goals/` | ~15% | High — goal evaluation drives /evolve |

The testing effort is inverted relative to criticality.

## Feature Freeze

**CRITICAL:** No new flags, modes, or options on these files until existing paths are stable:
- `cli/cmd/ao/rpi_phased.go`
- `cli/cmd/ao/rpi_loop_supervisor.go`
- `cli/internal/rpi/`

## Work Items

### 1. Integration Test Suite for Phased Engine

Create a `test.RpiHarness` that mocks the command runner and feeds scripted output. Table-driven tests covering:

- **Happy path**: research → plan → implement → validate completes
- **Discovery failure + retry**: research phase fails, retries, succeeds
- **Timeout**: phase exceeds time budget, handled gracefully
- **Stall detection**: supervisor detects stalled phase
- **Cleanup after crash**: incomplete run leaves no orphan worktrees or state

**Target:** `cli/internal/rpi/` from ~33% to 65%+ coverage
**Target:** `cli/cmd/ao/` from ~46% to 65%+ coverage

### 2. Supervisor Loop Tests

Test the supervisor lifecycle:

- **Lease acquisition/release**: supervisor claims lease, releases on exit
- **Detached branch healing**: supervisor detects and prunes detached branches
- **Gate enforcement**: test `off`, `best-effort`, and `required` gate modes
- **Failure policy**: test `stop` vs `continue` failure policies
- **Max cycles**: supervisor respects `--max-cycles` and exits cleanly

### 3. Goals Subsystem Tests

Create fixture GOALS.yaml files and test:

- `ao goals validate` with passing/failing goals
- `ao goals measure` drift detection
- `ao goals validate --json` structured output
- Edge cases: empty goals, malformed YAML, missing check scripts

**Target:** `cli/internal/goals/` from ~15% to 50%+ coverage

### 4. Extended Unattended Execution (Trust Run)

- Run `ao rpi loop --supervisor --max-cycles=10` on this repo
- Log every anomaly, fix every crash
- Repeat until 10 cycles complete without human intervention
- Create a goal: `rpi-cycle-completes` that runs `bash tests/e2e/test-rpi-cycle.sh`

### 5. Hook Latency Profiling

- Add `AO_HOOK_PROFILE=1` mode that times each hook
- Establish 5-second budget for SessionStart total
- If budget exceeded, identify which hooks are slow and optimize

## Success Criteria

- [ ] `cli/internal/rpi/` coverage >= 65%
- [ ] `cli/cmd/ao/` coverage >= 65%
- [ ] `cli/internal/goals/` coverage >= 50%
- [ ] `ao rpi loop --supervisor --max-cycles=10` completes on this repo
- [ ] All new tests pass in CI
- [ ] No regressions in existing test suite
- [ ] `rpi-cycle-completes` goal added and passing

## Approach

Run this as a `/evolve` cycle with GOALS.yaml. The evolution loop should:
1. Measure current coverage baselines
2. Write tests for the lowest-coverage, highest-criticality paths first
3. Fix any bugs discovered during test writing
4. Re-measure coverage after each wave
5. Continue until all success criteria met

## Key Files

```
cli/cmd/ao/rpi_phased.go          # Phased engine entry point
cli/cmd/ao/rpi_loop_supervisor.go # Supervisor loop
cli/internal/rpi/                  # RPI engine internals
cli/internal/goals/                # Goals subsystem
cli/cmd/ao/goals.go               # Goals CLI commands
tests/                             # Test infrastructure
```
