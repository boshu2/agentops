# Codex Task Packet And Run Receipt

> **Status:** Draft
> **Schemas:** `../../schemas/codex-task-packet.schema.json`,
> `../../schemas/codex-run-receipt.schema.json`
> **Examples:** `examples/codex/task-packet.json`,
> `examples/codex/run-receipt.json`
> **Consumers:** future Codex dispatch/run adapters, RPI validators, and
> receipt auditors

This contract defines the portable packet AgentOps hands to Codex and the run
receipt AgentOps expects back. It is intentionally smaller than a full runtime
driver: it describes a non-mutating dispatch boundary, auth guard, sandbox,
stdin closure, timeout, resume, and receipt fields so implementation work does
not reinvent the wire shape.

## Boundary

The task packet is a declaration, not the work itself. A dispatcher may create a
packet and receipt directory, but dispatch must be **non-mutating** with respect
to the target repository until Codex starts. The dispatcher records intent,
command, auth posture, sandbox, output paths, and acceptance commands; Codex
then performs the requested repository work.

Routine Codex execution must not depend on `ao codex start`, `ao codex stop`,
`ao codex ensure-start`, or `ao codex ensure-stop`. Those lifecycle commands
are deprecated for routine validation and remain only for compatibility or
explicit lifecycle investigations.

## Task Packet Fields

`schema_version` is currently `1`.

`packet_id` is the stable correlation key copied into the run receipt.

`objective` is the work request Codex should complete. It should be specific
enough that the receipt can later prove whether the stop condition was met.

`role` records why Codex is being invoked: `worker`, `validator`, `reviewer`, or
`explorer`.

`cwd` is the working directory for the Codex process.

`allowed_paths` and `forbidden_actions` are guidance and audit fields. They do
not replace filesystem permissions.

`sandbox` is the requested Codex sandbox: `read-only`, `workspace-write`, or
`danger-full-access`. The packet must make the requested sandbox explicit so a
receipt can prove what was used.

`auth` is the auth guard. Routine AgentOps Codex work requires a ChatGPT
subscription login and must reject API-key worker authentication. The guard must
check that Codex status contains `Logged in using ChatGPT`, reject
`OPENAI_API_KEY`, and record `forbid_api_key: true`.

`dispatch` records the non-mutating command used to start Codex. Its `mode` is
`non-mutating`, and `mutates_repo` is `false`.

`execution.argv` is the exact Codex command vector. `execution.stdin.mode`
defines how the prompt enters Codex. Routine headless runs use `pipe-prompt`
with `close_after_prompt: true`; interactive runs must still record stdin
behavior. Stdin closure matters because an unclosed pipe can leave `codex exec`
waiting forever.

`execution.timeout_seconds` is the wall-clock cap for the Codex process. A
dispatcher must record the timeout it enforces instead of relying on ambient
terminal behavior.

`output` names the capture mode and paths. The default durable mode is
`output-last-message` plus an optional JSONL event stream. The receipt path is
declared here so later audit code knows where to look before the run starts.

`evidence.required_commands` lists the acceptance commands whose results must be
copied into the receipt.

`resume` records whether a follow-up may resume a prior Codex session. A packet
may use `none`, `session-id`, or `last-session-in-cwd`; if a session id is used,
the receipt must copy it into `codex_session_id` or `resume_from_session`.

`stop_condition` states the observable condition under which the run is done.

## Run Receipt Fields

The receipt is the audit artifact produced after Codex exits. It must copy
`packet_id`, `cwd`, `sandbox`, command argv, stdin mode, timeout, and session
metadata from the attempted run.

Required receipt fields:

- `receipt_id`, `packet_id`, `started_at`, and `ended_at`
- `codex_session_id` when Codex reports one
- `cwd`, `sandbox`, `auth_mode`, and `auth_status`
- `command.argv`
- `stdin.mode`, `stdin.closed_at`, and `stdin.bytes_written`
- `timeout_seconds`, `timed_out`, and `exit_code`
- `outputs.final_message_path`, optional `outputs.jsonl_path`, and
  `outputs.receipt_path`
- `changed_files`
- `commands_run`, with command text, exit code, and optional output excerpt
- `verdict.status`, `verdict.judge_source`, and `verdict.summary`
- for `PASS`, `WARN`, or `FAIL` verdicts: `verdict.author_id`,
  `verdict.judge_name`, `verdict.judge_program`, and
  `verdict.judge_model_family`
- `resume_from_session` when the run resumed an earlier session
- `failure_reason` when the process, auth guard, timeout, or validation failed

`auth_mode: "api-key"` is allowed in the receipt schema only so failed attempts
can be recorded honestly. It is not a passing worker-auth state for routine
AgentOps Codex work.

Successful Codex receipts use the same review-gate floor as `ao tick
verdict-gate`: a final `PASS`, `WARN`, or `FAIL` verdict must include a
non-empty `COMMANDS RUN` body and typed independent judge identity
(`author_id`, `judge_name`, `judge_program`, `judge_model_family`). Missing
command evidence, missing judge identity, self-approval
(`author_neq_validator`), malformed verdict tokens, or contradictory verdict
bodies are rejected and recorded as failed receipt validation.

## Validation

The schemas and examples are committed together:

- `../../schemas/codex-task-packet.schema.json`
- `../../schemas/codex-run-receipt.schema.json`
- `examples/codex/task-packet.json`
- `examples/codex/run-receipt.json`

The Go contract tests parse both schemas and fixtures, assert required fields,
and check enum membership for role, sandbox, auth mode, stdin mode, output mode,
resume policy, receipt verdict status, and receipt judge identity.

Acceptance command:

```bash
cd cli && go test ./cmd/ao ./internal/... -run 'Codex.*Packet|Codex.*Receipt'
```

Fixture-backed smoke is the default CI/local proof because it does not require
Codex billing, network, or live ChatGPT auth:

```bash
cd cli && go test ./cmd/ao ./internal/... -run 'Codex.*Smoke|Codex.*Receipt'
cd cli && go test ./cmd/ao -run 'Codex.*(Stdin|Timeout|Background)'
```

The strict Codex smoke is opt-in for an operator machine with `codex` installed
and ChatGPT subscription auth available:

```bash
HEADLESS_RUNTIME_SKILL_CODEX_STRICT=1 \
  bash scripts/validate-headless-runtime-skills.sh --runtime codex
```

A skip means Codex CLI was unavailable in default non-strict mode. A warning
with load-check fallback means the runtime loaded but did not provide verified
inventory, so it is not a passing strict receipt-path proof. In strict mode,
missing Codex auth, malformed stream JSON, inventory mismatch, timeout, or
fallback all fail instead of manufacturing a `PASS`.
