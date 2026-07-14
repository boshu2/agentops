# Orchestration-as-Prompt Pattern

## What

Orchestration logic embedded in SKILL.md prompts rather than in Go/Python code. The LLM reads the orchestration rules and executes them as part of its reasoning. The prompt IS the program.

## Why

- **Runtime adaptability.** The LLM adapts to runtime context (different backends, different capabilities) without conditional compilation or feature flags.
- **Judgment calls.** Prompt-based rules handle decisions that code cannot anticipate — "is this research sufficient?", "should this wave retry or escalate?"
- **Iteration speed.** Changing a SKILL.md is a single file edit. No build, no deploy, no version matrix.
- **Cross-runtime portability.** The same orchestration works across Claude Code, Codex, and Cursor without platform-specific code paths.

## When to Use Code vs Prompt

| Use Code For | Use Prompt For |
|---|---|
| Hard constraints (three waves or 90 minutes per tranche) | Judgment calls ("did the plan materially change?") |
| File I/O, git operations, CLI wrappers | Workflow sequencing and phase transitions |
| Schema validation, JSON parsing | Quality assessment and retry decisions |
| Timeout enforcement, kill switches | Scope decisions and prioritization |
| Binary pass/fail gates (test suites) | Nuanced severity classification |
| Secrets management, credential handling | Work selection ladders and fallback cascades |

## Examples from This Codebase

### Completion Markers (crank)

The Sisyphus Rule in `skills/crank/SKILL.md` uses prompt-embedded markers to
describe one wave's evidence state: `<promise>DONE</promise>`,
`<promise>BLOCKED</promise>`, or `<promise>PARTIAL</promise>`. These markers do
not own retries or escalation. RPI's single run governor owns tranche admission,
the three-wave/90-minute boundary, and HOLD/helper transitions.

### Wave Orchestration (crank + swarm)

`skills/crank/SKILL.md` defines one wave — identify ready work, execute one
direct writer by default, and return targeted evidence. RPI may admit another
unchanged wave, but freezes after at most three waves or 90 minutes. `/swarm`
is an explicit optimization for two or more disjoint write scopes, not the
default cost of doing work.

### Work Selection Ladder (evolve)

`skills/evolve/SKILL.md` defines a 7-layer priority cascade: pinned queue, harvested work, open beads, failing goals, testing improvements, validation tightening, drift mining, feature suggestions. The LLM walks the ladder each cycle, making judgment calls at every layer. Code handles the kill switch check and cycle logging. The dormancy decision ("are all generator layers truly empty?") is a prompt-level judgment, not a boolean.

### Phase Routing (rpi)

`skills/rpi/SKILL.md` keeps four authority boundaries: Discovery/Premortem shape
and admit the plan, Crank produces a bounded tranche, Validate judges one frozen
candidate, and Learn records the minimal plan impact. The orchestrator chooses
REPAIR or REPLAN; a failed check never escalates merely because a counter moved.

### Backend Selection (swarm)

`skills/swarm/SKILL.md` instructs the LLM to detect multi-agent capabilities at runtime and select the native backend. Rather than a code-level `if/else` on runtime type, the prompt says "use runtime capability detection, not hardcoded tool names" and the LLM adapts to whatever tools are available in the current session.

## Anti-Patterns

- **Timing/timeout logic in prompts.** LLMs cannot reliably track wall-clock time. Use code for timeouts, kill switches, and stall detection.
- **Binary validation in prompts.** If the answer is strictly pass/fail (test suite, schema check, lint), run it in code. Prompts add ambiguity where none is needed.
- **Secrets or credentials in prompt-based orchestration.** Prompts are logged, cached, and visible in transcripts. Keep credentials in environment variables and code-level injection.
- **Unbounded loops.** Persist one run-level boundary and stop a routine tranche
  after three waves or 90 minutes. Return remaining work instead of multiplying
  private phase retries.
- **Complex arithmetic or counting.** LLMs make arithmetic errors. Use code for counters, SHA comparisons, and numeric thresholds.

## Origin

Pattern validated by Claude Code's internal `coordinatorMode.ts` (discovered via npm source map leak, March 2026). The coordinator uses prompt-embedded orchestration rules for sub-agent dispatch, phase transitions, and tool routing — the same approach codified in AgentOps skills.
