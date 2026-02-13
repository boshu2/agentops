# Plan: AO-Olympus Bridge Next Steps

> What to do next after making the AO↔Olympus bridge docs truthful vs Olympus current code/specs.

**Date:** 2026-02-13  
**Status:** draft  
**Primary docs:** `docs/ol-bridge-contracts.md`, `docs/architecture/ao-olympus-ownership-matrix.md`

## Current State (Truthful Baseline)

- AO↔Olympus interop is limited to **exactly 3 contract surfaces**:
  - `INVOCATION_ENVELOPE`
  - `STATE_CHECKPOINT_HANDOFF`
  - `OBSERVABILITY_EVENTS`
- **Envelope enforcement is SPEC_ONLY** until Olympus emits/parses schema-enveloped payloads and conformance tests exist.
- `ol validate stage1 -o json` emits **raw `Stage1Result` JSON** today (not an envelope).
- Olympus autonomous dispatch is **artifact-first** (DD7.3/DD8):
  - error + artifacts present => treat as success (crash recovery)
  - exit 0 + missing required artifacts => retry/fail closed
- In Stage1 JSON mode, consumers must gate on `passed` (not process exit code) until Olympus changes its behavior.

## Goal

1. Make AO consume Olympus deterministic validation results correctly **today** (raw Stage1 JSON).
2. Provide a staged path to make the bridge contracts **implemented and enforced** (envelopes + tests), without capability fiction.
3. Minimize drift between AO docs and Olympus code/specs.

## Milestones / Workstreams

### M0: Docs Truthfulness (Done)

- [x] Bridge doc matches Olympus current Stage1 JSON output (raw `Stage1Result`).
- [x] Bridge doc matches Olympus dispatch semantics (artifact-first DD7.3/DD8).
- [x] Ownership matrix avoids “bridge implemented” claims (separates standalone status vs envelope status).

### M1: AgentOps: Consume Stage1Result (Raw JSON) in `/vibe`

**Outcome:** AO can include Olympus deterministic validation as a first-class gate without pretending envelopes exist.

- [ ] Add an AO integration path that can run `ol validate stage1 -o json` when `.ol/` is present and `ol` is available.
- [ ] Parse raw `Stage1Result` JSON and gate on `passed`.
- [ ] Add a report section (e.g., “Deterministic Validation (Olympus)”) that includes:
  - `passed`
  - step names, durations, exit codes (as emitted)
  - summary string
- [ ] Add failure behavior policy (recommendation):
  - If OL Stage1 is **enabled/required** and the JSON cannot be parsed, FAIL closed.
  - If OL Stage1 is **auto** and `ol` is missing / command not available, degrade gracefully with an explicit note.
- [ ] Add tests using a fixture `Stage1Result` JSON file.

### M2: Olympus: Make Stage1 JSON Mode Exit Code Deterministic

**Outcome:** downstream tooling can rely on exit status without special-casing JSON mode.

- [ ] Change `ol validate stage1 -o json` to:
  - always emit JSON to stdout
  - exit `0` only when `passed=true`
  - exit `1` when `passed=false`
- [ ] Add unit tests for stdout JSON + exit code semantics.

### M3: Olympus: Implement Envelope Emit/Parse + Conformance Tests (Upgrade `SPEC_ONLY` → `ENVELOPE_IMPLEMENTED`)

**Outcome:** the 3-surfaces contract becomes real, testable, and enforceable.

- [ ] Implement schema-enveloped emit/parse for:
  - `INVOCATION_ENVELOPE` (skill/intent declarations)
  - `STATE_CHECKPOINT_HANDOFF` (checkpoint/result handoff)
  - `OBSERVABILITY_EVENTS` (audit/error/kill-switch evidence)
- [ ] Add conformance tests that:
  - validate required fields for `schema_version: 1`
  - reject unknown contract / missing fields / invalid versions
  - fail closed (no checkpoint mutation) on schema mismatch
- [ ] Only after the above: update `docs/architecture/ao-olympus-ownership-matrix.md` envelope status to `ENVELOPE_IMPLEMENTED` for rows that are actually enforced.

### M4 (Optional): Stabilize “Expected Artifacts” as a Contract (Reduce Drift)

**Outcome:** AO can reason about dispatch completion deterministically without copying internal Olympus lists into docs.

- [ ] Expose expected artifact patterns per Zeus phase as machine-readable output (e.g., a JSON subcommand or `ol zeus status -o json` field).
- [ ] Keep docs high-level; point to the Olympus output as source of truth.

## Definition of Done (This Plan)

- M1 shipped in AO (raw Stage1 integration + tests).
- M2 shipped in Olympus (deterministic JSON exit codes).
- A tracked path exists for M3 (envelopes + tests) before any doc claims “bridge envelopes implemented.”

