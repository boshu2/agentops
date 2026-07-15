# Handoff: RPI loop improvement context

Date: 2026-07-15

## Caller-supplied purpose

The purpose of this branch and any resulting pull request is now to pass along
context that can help improve the RPI loop. It is no longer a claim that the
session's loop-hardening implementation is complete or merge-ready.

## Repository identity

- Repository: `git@github.com:boshu2/agentops.git`
- Worktree: `/home/bo/dev/agentops-rpi-loop-hardening`
- Branch: `codex/rpi-loop-hardening-20260715`
- Base commit: `baaa9e22dadc423d7b7bb73c043112ecfca425cb`
- Publication scope: this audit and handoff only. The inherited and experimental
  worktree changes remain local and uncommitted.

The worktree was created by replaying a pre-existing dirty tracked diff and
relevant untracked source files from `/home/bo/dev/agentops` onto the base
commit. Consequently, the branch state is an evidence corpus, not a clean
attribution of session-authored implementation changes.

## Primary context artifact

- Full causal audit:
  `docs/audits/2026-07-15-rpi-orchestration-nonconvergence.md`
- Audit SHA-256:
  `eb20d46c7e966e18f1eb5334878a436afbf1cb1ab63ea120566d8cf9731ca5d9`
- Pinned pre-audit transcript prefix SHA-256:
  `37fcf93dba347be99e688b88d50c31c71e576a1a1a1967eeaa7966fe122d2afc`
- Local raw rollout source, not included in the published branch:
  `/home/bo/.codex/sessions/2026/07/15/rollout-2026-07-15T12-04-40-019f6685-eb4b-7c03-a5b0-493e9765ba79.jsonl`

The audit contains the evidence-backed timeline, quantitative orchestration
ledger, supported and rejected causal claims, counterfactuals, unknowns, and
seven suggested experiments. It does not promote those experiments into loop
policy.

## What the local evidence corpus contains

The packet and verdict paths below are ignored runtime evidence and will not be
carried by a normal pull request. Their identities and relevant conclusions are
preserved in the tracked audit above.

- 12 Plan packets in `.agents/plans/`.
- 10 Candidate packets in `.agents/candidates/`.
- Content-addressed evidence under `.agentops/evidence/`.
- 9 immutable verdicts under `.agentops/verdicts/sha256/`:
  - 5 PASS;
  - 3 FAIL;
  - 1 NOT_PROVEN.
- Experimental source changes covering:
  - Plan subject identity and scope preflight;
  - packet capability negotiation;
  - offline schema validation;
  - explicit pre-change and post-change expectations;
  - content-addressed evidence receipts and manifests;
  - Candidate Packet v2 construction; and
  - validator progress reporting.

The five PASS verdict artifacts are:

- `.agentops/verdicts/sha256/67092c38388441aa96f104cef1ef15baa4501343a53af5e77199b6d18dde2209.json`
- `.agentops/verdicts/sha256/9c2020156395d76f4898d39ef308e022169a0142dd646f6608e1b63279be9997.json`
- `.agentops/verdicts/sha256/8e9062372610bb7d4f65659875b2ccfba37199ffe7e3599dc64bb24e97b8d881.json`
- `.agentops/verdicts/sha256/681b8d2c1f2c9fa0b7072a6d1c2f796fe6a7b391ccb87e2d85d4767e17de12c3.json`
- `.agentops/verdicts/sha256/d649ad5df98f63228b625d6f5678750e60f2e9d18d13ca092448448625256c52.json`

The non-PASS verdict artifacts are retained because they contain the failures
that exposed loop behavior:

- `.agentops/verdicts/sha256/e43a56bd9853404bb0df58360e329708738a5fe19dd9d1ec348b14fc8e6c84ab.json`
- `.agentops/verdicts/sha256/b48eb762f096a1c2aafa630434f7c9a86d90201268cdbc93432ef849667851e9.json`
- `.agentops/verdicts/sha256/6fef3d53281bcba996a2ff79632c0c2b62c79c67341cf2e0bb76fd1661bd885e.json`
- `.agentops/verdicts/sha256/34919cbe64fe5d6d303c376d8be4c3fcaecd836fe8ceebdfb0983645abf4f2c7.json`

## Unresolved facts and risks

- The original multi-item hardening objective was not completed.
- The final Learn/finding-identity behavior has only a schema-valid Plan:
  `.agents/plans/slice-7-finding-index-learn-routing.plan-packet.json`, digest
  `4d8ad17f8cd2208f0f35be393b71bc00ce407d32579886bffb6c79b9e13a93cc`.
- `schemas/finding-index.v1.schema.json` is absent, so that Plan has no
  implementation, Candidate, or verdict.
- The promised aggregate repository audit was not run.
- No program-level verdict maps the original P0/P1/P2 brief to final evidence.
- No `rpi-report.v1` artifact was found for the individual invocations.
- The installed `ao` CLI was not updated.
- The worktree currently includes 77 tracked changed files with 1,869
  insertions and 2,062 deletions, plus untracked files. Those counts mix the
  inherited dirty baseline with session work.
- Slice-level PASS verdicts prove their exact acceptance digests; they do not
  prove the combined branch or authorize merging its experimental code.

## Context summary

The session demonstrates a mismatch between a deliberately bounded RPI
micro-loop and an unbounded multi-behavior completion objective. The main
context repeatedly started new RPI-shaped invocations after NOT_BUILT, FAIL,
NOT_PROVEN, and PASS outcomes without an outer invocation limit, repair budget,
wall-clock ceiling, aggregate acceptance map, or caller checkpoint.

The hardening goal ran for 3 hours and 49 minutes, spawned 33 agents, issued 401
agent waits, and added approximately 79.5 million token-counter units. Several
large Plans permitted 27-40 paths and produced Candidates changing 45-55 paths.
Formatting, generated-receipt, schema-adapter, and test-harness defects therefore
incurred the same full serial restart cost as semantic defects.

The evidence supports improving explicit continuation, boundedness, context
transfer, Plan admission, control-bearing progress, and aggregate completion
at the boundary around one RPI invocation. The full postmortem preserves the
counterfactuals and limitations needed to evaluate those hypotheses.
