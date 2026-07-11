# Codex Validator and Runtime Rules

## Validator dispatch rules

- **Network-touching validators need `-s danger-full-access`.** A Codex validator that must read a Dolt-mode bd ledger, run `git fetch`, or reach any network must be dispatched with `-s danger-full-access`. `-s workspace-write` blocks network and FETCH_HEAD writes. A FAIL caused purely by sandbox denial is infrastructure failure, not a verdict: fix the dispatch and rerun the independent judge.
- **Set `TMPDIR` inside the workspace for any run that commits.** Export `TMPDIR="$REPO/.tmp"` before a Codex run that must commit; the sandbox can block git temp-object writes to the host temp directory.
- **Verdict file contract.** The first line is bare `VERDICT: PASS|FAIL`, then a blank line, bare `COMMANDS RUN:`, commands and output, and `judge_source: codex-<model>`. Do not put headings or parentheticals on parser-owned lines. Fail closed on anything unverifiable.
- **Clamp judge writes.** Every judge brief states: "READ-ONLY except writing your single verdict file at `<path>`. Do NOT commit, push, or run tracker/infra ops (git push, br/bd, dolt)." This prompt-level clamp remains mandatory when a network-touching judge uses `danger-full-access`.
- **Publish the output contract in the judge prompt.** Include the bead, verbatim acceptance, commands to rerun, exact verdict shape, judge identity, and the single allowed verdict path. A judge that ran nothing is a reader, not a verifier.
- **Validate before acting.** Programmatically require the anchored verdict, `COMMANDS RUN:`, and `judge_source:`. Discard and redispatch an incomplete artifact; use `--output-schema` when automation consumes the verdict.

## Runtime work rules

The full review packet is [Codex runtime review: auth and scope](../../../docs/learnings/2026-06-12-codex-runtime-review-auth-and-scope.md).

1. Make the first acceptance test adversarial: test packet-injected `OPENAI_API_KEY`, disabled or missing auth guards, command/sandbox mismatch, missing verdict or command evidence, and path escape.
2. Validate declared JSON Schemas executably; partial hand checks are documentary.
3. A receipt must prove required acceptance commands ran, not merely that Codex ran.
4. Keep the worker-path MVP to packet validation, dispatch, success/failure receipts, timeout/stdin/auth tests, and one fixture smoke.
5. Time-box discovery and route follow-up work through [`discovery`](../../discovery/SKILL.md) instead of absorbing scope.
6. Mirror interactive approval evidence to a tracked durable proof surface before closing the gating bead or epic.
