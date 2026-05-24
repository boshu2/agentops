> Extracted from council/SKILL.md on 2026-04-11.

# Multi-Agent Architecture

Council uses whatever multi-agent primitives your runtime provides. Each judge is a parallel subagent that writes output to a file and sends a minimal completion signal to the lead.

## Deliberation Protocol

The `--adversarial` flag implements the **deliberation protocol** pattern:
> Independent assessment → evidence exchange → position revision → convergence analysis

- **R1:** Spawn judges as parallel subagents. Each assesses independently, writes verdict to file, signals completion.
- **R2:** Lead sends other judges' verdict summaries to each judge via agent messaging. Judges revise and write R2 files.
- **Consolidation:** Lead reads all output files, computes consensus.
- **Cleanup:** Shut down judges via runtime's cleanup mechanism.

## Communication Rules

- **Judges → lead only.** Judges never message each other directly. This prevents anchoring.
- **Lead → judges.** Only the lead sends follow-ups (for debate R2).
- **No shared task mutation by judges.** Lead manages coordination state.

## Ralph Wiggum Compliance

Council maintains fresh-context isolation (Ralph Wiggum pattern) with one documented exception:

**`--adversarial` reuses judge context across R1 and R2.** This is intentional. Judges persist within a single atomic council invocation — they do NOT persist across separate council calls. The rationale:

- Judges benefit from their own R1 analytical context (reasoning chain, not just the verdict JSON) when evaluating other judges' positions in R2
- Re-spawning with only the verdict summary (~200 tokens) would lose the judge's working memory of WHY they reached their verdict
- The exception is bounded: max 2 rounds, within one invocation, with explicit cleanup

Without `--adversarial`, council is fully Ralph-compliant: each judge is a fresh spawn, executes once, writes output, and terminates.

## Degradation

If no multi-agent capability is detected, council falls back to `--quick` (inline single-agent review). If agent messaging is unavailable, `--adversarial` degrades to single-round review with a note in the report.

## Judge Naming

Convention: `council-YYYYMMDD-<target>` (e.g., `council-20260206-auth-system`).

Judge names: `judge-{N}` for independent judges (e.g., `judge-1`, `judge-2`), or `judge-{perspective}` when using presets/perspectives (e.g., `judge-error-paths`, `judge-feasibility`). Use the same logical names across both Codex and Claude backends.
