# Phase Skill Isolation Contract

How RPI keeps phase skills (`/discovery`, `/crank`, `/validation`) from compressing each other's work into a single agent context, and the levers used to enforce that boundary.

## Declaration vs enforcement

**Declaration.** [`PRODUCT.md`](../../../PRODUCT.md) operational principle #5 (two-tier execution) DECLARES that phase skills must run as isolated execution contexts that delegate work to other phase skills, not inline. The orchestrator (`/rpi`) owns the lifecycle objective; each phase skill owns its phase artifact, gate, and retry policy. Inlining the work — "I'll just do discovery in this same context" — breaks that ownership chain even when the build still passes.

**Enforcement.** This document plus [`scripts/check-skill-isolation.sh`](../../../scripts/check-skill-isolation.sh) ENFORCE that contract by detecting compression patterns in phase-skill SKILL.md bodies. Once `cli/internal/daemon/rpi_run.go` lands (UW5 / E3.W4), enforcement extends to **true isolation** at the process level — phase skills run as separate daemon-launched processes whose contexts cannot bleed into each other. Until then, the lint plus the text contract are the active surface.

The companion [`shared/references/strict-delegation-contract.md`](../../shared/references/strict-delegation-contract.md) gives the human-readable rationale and the canonical anti-pattern catalogue. Read this file for the mechanical contract; read that one for the philosophy.

## Four levers

These are the four mechanical levers available to keep phase contexts isolated. They are listed in increasing strength, with the recommended posture last.

| Lever | Description | Mechanical strength |
|---|---|---|
| A | Text-only instructions in SKILL.md | Weak — relies on agent compliance |
| B | PreToolUse hook intercepting Edit/Write at runtime | Medium |
| C | Daemon path (process-level isolation via `cli/internal/daemon/rpi_run.go` once E3.W4 lands) | Strong |
| D | Combination — text contract + lint + daemon | Strongest (recommended) |

**Recommended posture: D.** Layered enforcement keeps the contract durable when any single lever weakens. A drifts when a SKILL.md is edited by a tired operator. B silences itself when a hook config gets out of sync. C protects against agent prompt-injection but cannot prevent a compressed-on-paper contract from being declared. The combination keeps each layer honest about the others.

## Compression patterns (what NOT to do)

The lint script (`scripts/check-skill-isolation.sh`) flags the following patterns inside phase-skill SKILL.md bodies (`skills/{rpi,discovery,crank,validation}/SKILL.md`):

1. **Cross-phase first-person verbs.** Phrases like `I will research`, `I will plan`, `I will crank`, `I will validate` (case-insensitive). A phase skill should not describe itself as doing another phase's work.
2. **Inline research vocabulary.** Phrases like `let me grep`, `let me read`, `let me search`, `I'll grep`, `I'll read`, `I'll search` (case-insensitive). These signal that the agent intends to inline research-phase work into the current context instead of delegating.
3. **Phase-skill calling another phase skill.** A `Skill(skill="research")`, `Skill(skill="plan")`, `Skill(skill="crank")`, or `Skill(skill="validation")` callsite inside a phase-skill SKILL.md, **except** for the legitimate orchestration patterns:
   - `/rpi` legitimately orchestrates `discovery`, `crank`, `validation` (this is its core contract). It should NOT call `research` or `plan` directly — those are discovery's sub-skills.
   - `/discovery` legitimately orchestrates `research` and `plan`. It should NOT call `crank` or `validation` — those are downstream phases.
   - `/crank` should NOT call `research`, `plan`, `crank`, or `validation` — phase 2 is sealed.
   - `/validation` should NOT call `research`, `plan`, `crank`, or `validation` — phase 3 is sealed.

### False-positive guard

Citations and reference reads are not compression. The lint explicitly excludes:

- Markdown reference links: `See [research](../research/SKILL.md)` (lines beginning with `See [`).
- Reference document reads: `Read references/foo.md` (lines beginning with `Read ` followed by a path).
- Lines inside fenced code blocks (between triple-backtick fences) — code fences may legitimately quote a `Skill(...)` call as an example without invoking it.

When the lint script needs to evolve, prefer narrowing the trigger over widening the guard — silencing a real compression pattern with an over-broad allowlist defeats the purpose.

## Mechanical enforcement

Two surfaces, layered:

1. **Lint.** [`scripts/check-skill-isolation.sh`](../../../scripts/check-skill-isolation.sh) walks `skills/{rpi,discovery,crank,validation}/SKILL.md` (or any path passed positionally) and exits non-zero on the first compression pattern. Run with `-q` / `--quiet` to suppress diagnostic output and rely on exit code only. The script ships a `--self-test` mode that injects a known violation into a tmpdir and asserts the lint catches it.
2. **Daemon path (forward reference).** Once `cli/internal/daemon/rpi_run.go` lands in E3.W4 (UW5 of the RPI lifecycle sharpening epic), `/rpi` will be able to launch each phase as a separate daemon process. That gives **true isolation**: discovery's context, file-system reads, and accumulated reasoning cannot leak into crank, because crank runs in a fresh process. The lint catches authored compression in SKILL.md text; `rpi_run.go` will catch runtime compression at the process boundary. Together they close the loop.

Cross-references:
- [`PRODUCT.md`](../../../PRODUCT.md) — declaration (operational principle #5)
- [`shared/references/strict-delegation-contract.md`](../../shared/references/strict-delegation-contract.md) — companion contract with rationale and anti-pattern examples
- [`scripts/check-skill-isolation.sh`](../../../scripts/check-skill-isolation.sh) — current mechanical enforcement
- `cli/internal/daemon/rpi_run.go` (forthcoming, E3.W4) — process-level enforcement via daemon path

## See also

- [`docs/learnings/orchestrator-compression-anti-pattern.md`](../../../docs/learnings/orchestrator-compression-anti-pattern.md) — the 2026-04-19 live compression that motivated the layered enforcement approach
- [`skills/rpi/references/phase-data-contracts.md`](phase-data-contracts.md) — how phases pass data via filesystem artifacts (the contract isolation depends on)
