---
title: Codex runtime receipts need executable guardrails before planning ceremony
date: 2026-06-12
tags: [codex, runtime, receipts, auth, fanout, operationalization]
source: post-review of ag-codex-runtime-enhancement-o0nds
companion: .agents/learnings/2026-06-12-codex-runtime-review-auth-and-scope.md
---

# Codex Runtime Receipts Need Executable Guardrails Before Planning Ceremony

The Codex runtime enhancement shipped useful surfaces, but the post-review found
that the most important guarantees were not fully enforced. The targeted tests
and fast gates passed, Fable approval happened, and the tracker closed cleanly,
but the final system still overclaimed "deterministic receipt path" strength.

## What Shipped

- Task packet and run receipt schemas, examples, and tests.
- `ao codex dispatch` with stdin closure, timeout handling, receipt writing,
  ChatGPT subscription auth checks, and verdict parsing.
- `ao codex image-health` aggregating image, parity, generated artifact,
  override, RPI, lifecycle, and headless-runtime checks.
- Codex Fable approval skill and fanout discovery-to-gate workflow.
- Gate explainability, changed-scope regeneration, and Codex tracker guidance
  cleanup.

## Review Verdict

Overall rating: C+.

The direction was correct, but the implementation should not yet be treated as a
fully trustworthy deterministic Codex worker/receipt path. The MVP was allowed to
grow into ecosystem hardening, and the planning/Fable ceremony did not catch the
core implementation bug.

## Findings To Harvest

1. Packet-provided environment can bypass the API-key guard.

   `validateCodexDispatchAuth` checks ambient `os.Getenv`, but
   `codexDispatchEnv` appends packet-provided `execution.environment` into the
   worker process. Because the schema allows arbitrary environment keys, a task
   packet can inject `OPENAI_API_KEY` after the guard passes.

   Evidence:
   - `cli/cmd/ao/codex.go` around `validateCodexDispatchAuth`
   - `cli/cmd/ao/codex.go` around `codexDispatchEnv`
   - `schemas/codex-task-packet.schema.json` `execution.environment`
   - `docs/contracts/codex-task-packet.md` auth guard contract

2. Receipts do not record required acceptance command results.

   The contract says `evidence.required_commands` results must be copied into
   the receipt, but dispatch currently records only the Codex invocation in
   `commands_run`. Receipt validation only checks that some command evidence is
   present, so "machine-checkable receipt" is overstated.

3. Schema checking is mostly documentary.

   Runtime packet loading uses plain JSON unmarshal plus hand-written partial
   validation. It does not enforce the full JSON Schema, `additionalProperties`,
   auth constants, dispatch/execution command equivalence, capture mode, or the
   full receipt schema.

4. Output paths are not bounded.

   Absolute paths and `..` are accepted by `resolveCodexDispatchPath`, so a
   malformed packet can write receipts or JSONL outside the repository. The
   current `allowed_paths` field is explicitly guidance, not enforcement.

5. `ao codex image-health` can hang or become slow.

   It runs seven shell checks sequentially through a caller context that usually
   has no deadline. Health commands should have per-check budgets and should
   report slow checks in JSON.

6. Closeout evidence was partly stranded.

   Fable plan/council artifacts lived under an ignored worktree
   (`/Users/bo/dev/agentops-wt-codex-fanout-discovery/.agents/...`) rather than
   a durable tracked proof surface. Fast head gates passed, but full worktree
   disposition would still flag unrelated untracked state.

7. Fanout/Fable ceremony did not pay for routine CLI work.

   The fanout and approval artifacts were coherent, but they did not catch the
   auth/env bypass. Use the heavy ceremony for one-way-door architecture,
   product decisions, or multi-agent coordination, not every CLI feature slice.

## Rules For Future Codex Work

1. Make the first acceptance test adversarial.

   Before adding planner artifacts, test packet-injected `OPENAI_API_KEY`,
   `forbid_api_key=false`, missing `reject_env`, bad `required_mode`, command
   and sandbox mismatch, missing final verdict, missing required command
   evidence, and path escape attempts.

2. Contracts need executable validators.

   If a schema is the contract, the dispatch path must validate against that
   schema or use generated validation from it. Fixture inspection is not enough.

3. Receipt means evidence, not presence.

   A receipt that only proves "Codex ran" is not the same as a receipt that
   proves "acceptance commands ran and passed." Either run and record
   `evidence.required_commands`, or rename the field so it does not imply
   acceptance evidence.

4. Keep Codex runtime critical path small.

   The MVP for a worker path is packet validation, dispatch, receipt on
   success/failure, timeout/stdin/auth tests, and one fixture-backed smoke.
   Image health, gate explainability, approval-skill authoring, and docs
   migrations are follow-up work unless explicitly requested.

5. Time-box discovery for implementation slices.

   Default shape: 15 minutes discovery, 90 minutes vertical slice, then decide
   whether to continue. If new work appears, create follow-up beads instead of
   absorbing it into the active bead.

6. Approval evidence needs a durable proof surface.

   If Fable/ATM approval gates implementation, mirror the council artifact or a
   compact proof packet into a tracked `docs/` location or another durable
   artifact, not only ignored `.agents` state in a temporary worktree.

## Operationalization Targets

- Add tests and code to reject packet-provided API-key or auth-mode overrides.
- Enforce full packet and receipt schema validation at runtime.
- Implement or explicitly rename `evidence.required_commands` semantics.
- Bound dispatch output paths to cwd or declared allowed paths.
- Add per-check timeout and slow-check reporting to `ao codex image-health`.
- Add a closeout gate or checklist item that checks durable proof surfaces for
  approval artifacts before closing broad epics.
- Teach Codex planning skills to choose between "MVP vertical slice" and
  "architecture fanout" based on risk class.

## Source Artifacts

- Parent epic: `ag-codex-runtime-enhancement-o0nds`
- Main source head reviewed: `6168a66c9d1f8816b92b6c9848efccb07b431589`
- Tracker close commit: `_beads` `33e3654e509ea71a085389676a64bee6de6a8720`
- Fable approval:
  `/Users/bo/dev/agentops-wt-codex-fanout-discovery/.agents/council/2026-06-12-fable-approval-codex-fanout-discovery-gate.md`
- Fable capture:
  `/Users/bo/dev/agentops-wt-codex-fanout-discovery/.agents/council/ntm-captures/agentops--codex-plan-fable-approval_1.1_20260612_134909.txt`
