---
id: plan-2026-05-15-agentops-3-hookless-cdlc-rearchitecture
type: plan
date: 2026-05-15
goal: Rearchitect AgentOps 3.0 around CDLC, DDD bounded contexts, hexagonal ports, and hookless-first runtime adapters
detail_level: standard
research_refs:
  - .agents/research/2026-05-15-agentops-3-hookless-cdlc-rearchitecture.md
  - .agents/research/2026-05-12-bounded-contexts-and-ports.md
  - .agents/research/2026-05-15-operator-loop-token-reduction.md
  - docs/adr/ADR-0002-agentops-3-hookless-cdlc-rearchitecture.md
---

# AgentOps 3.0 Hookless CDLC Rearchitecture Plan

## Goal

AgentOps 3.0 should be a hookless-first CDLC system: compact, verifiable
software-engineering practice for LLM agents under token scarcity.

Hooks are not deleted first. Hooks are demoted first. Each hook must earn a
lease on life as an optional runtime adapter, a typed event subscriber, an
explicit command, a validation gate, or a removal candidate.

## Greenfield Shape

Build around noun-centered bounded contexts, not around the current skill and
hook files.

| Bubble | Owns | Primary artifacts |
|--------|------|-------------------|
| Work Lifecycle | Objective, intent, slice, wave, execution packet, work order, phase run | `.agents/rpi/execution-packet.json`, beads, phase summaries |
| Context Compiler | Context packet, citation, token budget, density score, freshness score | context packets, citations, retrieval reports |
| Evidence and Trust | Acceptance criterion, validation lane, evidence, verdict, gate result | validation reports, council reports, gate results |
| Knowledge Flywheel | Finding, learning, ratchet, promotion rule, provenance | learnings, findings, ratchets, promoted rules |
| Skill Catalog | Skill definition, runtime projection, practice citation, lease on life | `skills/`, `skills-codex/`, schemas, catalog reports |
| Runtime Shell | Harness, runtime adapter, event sink, agent session | CLI, daemon, hooks, filesystem, `bd`, git, LLM providers |

Runtime Shell is not core product logic. It adapts the core bubbles to Codex,
Claude, OpenCode, shell, CI, daemon, and future harnesses.

## Hook Disposition Model

Every hook receives one of five dispositions:

| Disposition | Criteria | Replacement |
|-------------|----------|-------------|
| Remove | No proven behavior delta or only prompt/context bloat | Delete from default runtime |
| Convert to gate | Deterministic safety/quality check | `GateRunnerPort` validation lane |
| Convert to event subscriber | Useful side effect with no prompt injection | `EventBusPort` subscriber |
| Convert to explicit command | Useful only on demand | `ao <verb>` or skill step |
| Retain optional | Runtime-specific and eval-proven | Optional activation profile |

Default rule for 3.0: no hook is installed, enabled, or prompt-injecting by
default.

## Replacement Map

| Current hook class | 3.0 replacement |
|--------------------|-----------------|
| Session startup context | `ContextCompilerPort` called by RPI/discovery or explicit `ao context assemble` |
| Prompt nudges | `next_action` in execution packets and operator-visible status |
| PreToolUse safety blockers | `SafetyPolicyPort` or `GateRunnerPort` before mutation |
| PostToolUse quality scans | validation lanes selected from execution packet |
| Session closeout | explicit `/post-mortem` or `ao closeout` lifecycle command |
| Hook-generated findings | `FindingCompilerPort` over explicit transcript/artifact inputs |
| Codex parity warnings | `HarnessPort` compiler and CI gate |
| Worktree setup/cleanup | `WorkspacePort` or daemon job lifecycle |

## Migration Waves

### S0 - Freeze the Direction

Add the ADR and this plan. Do not delete hooks. Mark hooks as adapter
candidates, not product core.

Acceptance:

- ADR exists and links from the documentation index.
- This plan defines the bounded contexts and hook disposition model.
- Existing validation remains green.

### S1 - Hook Lease Inventory

Generate a hook inventory from `hooks/hooks.json`, hook file headers, CI
consumers, and eval references.

Acceptance:

- Each hook has event, matcher, command, timeout, side effects, context
  injection behavior, owner bubble, proposed disposition, and evidence status.
- No hook remains "unclassified".

### S2 - Port Replacements

Add or formalize the ports that replace hidden hook behavior:

- `ContextCompilerPort`
- `GateRunnerPort`
- `SafetyPolicyPort`
- `EventBusPort`
- `HarnessPort`
- `WorkspacePort`
- `CloseoutPort`

Acceptance:

- Each port has a small interface, at least one adapter, and tests.
- No port is introduced without a concrete hook or runtime behavior to absorb.

### S3 - Hookless RPI Path

Make RPI run discovery, crank, validation, and closeout without relying on
runtime hooks.

Acceptance:

- Full RPI cycle can run with `AGENTOPS_HOOKS_DISABLED=1`.
- Context crosses phases only through execution packets and summaries.
- Token cost is remeasured against the 10.35M token baseline.

### S4 - Default Install Changes

Change install/bootstrap/product docs so hooks are optional profiles, not the
default first-value path.

Acceptance:

- Default install path is hookless.
- Hook-capable runtime docs move to optional advanced profiles.
- README, PRODUCT, quickstart, and onboarding docs no longer imply hooks are
  required for core AgentOps value.

### S5 - Removal and Retention

Remove hooks with no lease. Retain only optional hooks with evidence.

Acceptance:

- CI gates that validated hook shape are removed or recast around replacement
  ports.
- Hook A/B evals show no regression for the default hookless path.
- Retained hooks are disabled by default and documented as adapters.

## First Implementation Slice

The first slice should be inventory and contract before deletion:

1. Generate `docs/contracts/hook-lease-inventory.md`.
2. Add `schemas/hook-lease.v1.schema.json`.
3. Add a script that reads `hooks/hooks.json` and emits the inventory.
4. Classify `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`,
   and `Stop` hooks first.
5. Run markdown, hook manifest, and doc-release validation.

Landing note: after the lease inventory and replacement-port checks exist, the
first removal is limited to the three prompt-only context-injection hooks
(`new-user-welcome.sh`, `prompt-nudge.sh`, `intent-echo.sh`). Remaining hooks
stay lease-bound until their disposition is proven through the inventory,
replacement port, and validation gates.

## Non-Goals

- Do not delete `hooks/` as a surface in the first slice.
- Do not break existing hook-capable users before the hookless path is proven.
- Do not make Runtime Shell a domain bubble.
- Do not replace hooks with hidden daemon magic.
- Do not move context bloat from hooks into larger skill billboards.

## Success Metrics

- Hookless RPI trivial cycle token cost trends toward 2-3M tokens from the
  measured 10.35M baseline.
- Default install can run a complete RPI cycle without hooks.
- Every retained hook has positive evidence or a deterministic safety reason.
- Product language centers CDLC, execution packets, evidence, and knowledge
  compounding, not automatic hook injection.
- The context density rule is preserved at every boundary.
