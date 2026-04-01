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
| Hard constraints (`MAX_EPIC_WAVES = 50`) | Judgment calls ("is this research sufficient?") |
| File I/O, git operations, CLI wrappers | Workflow sequencing and phase transitions |
| Schema validation, JSON parsing | Quality assessment and retry decisions |
| Timeout enforcement, kill switches | Scope decisions and prioritization |
| Binary pass/fail gates (test suites) | Nuanced severity classification |
| Secrets management, credential handling | Work selection ladders and fallback cascades |

## Examples from This Codebase

### Completion Markers (crank)

The Sisyphus Rule in `skills/crank/SKILL.md` uses prompt-embedded markers to enforce completion semantics. After each wave, the LLM must emit one of `<promise>DONE</promise>`, `<promise>BLOCKED</promise>`, or `<promise>PARTIAL</promise>`. The retry logic (max 3 attempts, escalation on repeated BLOCKED) lives entirely in the prompt. Code only enforces the hard wave cap (`MAX_EPIC_WAVES = 50`).

### Wave Orchestration (crank + swarm)

`skills/crank/SKILL.md` defines the full wave loop — identify ready issues, bridge to TaskList, invoke `/swarm`, verify results, loop until epic closes. The LLM decides wave composition, conflict resolution strategy (serialize vs isolate), and when to stop. `skills/swarm/SKILL.md` defines the mayor-first architecture where the LLM auto-selects spawn backends (Claude teams, Codex sub-agents, inline fallback) based on runtime capability detection — no hardcoded tool names.

### Work Selection Ladder (evolve)

`skills/evolve/SKILL.md` defines a 7-layer priority cascade: pinned queue, harvested work, open beads, failing goals, testing improvements, validation tightening, drift mining, feature suggestions. The LLM walks the ladder each cycle, making judgment calls at every layer. Code handles the kill switch check and cycle logging. The dormancy decision ("are all generator layers truly empty?") is a prompt-level judgment, not a boolean.

### Phase Routing (rpi)

`skills/rpi/SKILL.md` classifies work complexity (fast/standard/full) using keyword matching and goal length — logic that could be code but benefits from LLM flexibility when edge cases arise. The three-phase rule (discovery, implementation, validation) and the validation-to-crank retry loop are prompt-orchestrated. The LLM decides whether to re-enter crank with findings context or escalate to manual intervention.

### Backend Selection (swarm)

`skills/swarm/SKILL.md` instructs the LLM to detect multi-agent capabilities at runtime and select the native backend. Rather than a code-level `if/else` on runtime type, the prompt says "use runtime capability detection, not hardcoded tool names" and the LLM adapts to whatever tools are available in the current session.

## Anti-Patterns

- **Timing/timeout logic in prompts.** LLMs cannot reliably track wall-clock time. Use code for timeouts, kill switches, and stall detection.
- **Binary validation in prompts.** If the answer is strictly pass/fail (test suite, schema check, lint), run it in code. Prompts add ambiguity where none is needed.
- **Secrets or credentials in prompt-based orchestration.** Prompts are logged, cached, and visible in transcripts. Keep credentials in environment variables and code-level injection.
- **Unbounded loops without code-level caps.** Always pair prompt-level "loop until done" with a hard code-level limit (e.g., `MAX_EPIC_WAVES = 50`). The LLM may misjudge completion.
- **Complex arithmetic or counting.** LLMs make arithmetic errors. Use code for counters, SHA comparisons, and numeric thresholds.

## Origin

Pattern validated by Claude Code's internal `coordinatorMode.ts` (discovered via npm source map leak, March 2026). The coordinator uses prompt-embedded orchestration rules for sub-agent dispatch, phase transitions, and tool routing — the same approach codified in AgentOps skills.
