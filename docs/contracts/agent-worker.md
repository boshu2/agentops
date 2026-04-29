# AgentWorker Runtime Contract

`AgentWorker` is the AgentOps contract for headless Claude, Codex, and future
worker runtimes. Wiki/forge consumes this contract; it does not own the worker
lifecycle.

## Interfaces

`AgentWorker` starts or attaches to worker sessions. `AgentSession` is the
durable session handle associated with one accepted job attempt.

Required operations:

| Operation | Required Behavior |
|-----------|-------------------|
| `start` | create a worker session for a job attempt |
| `attach` | reconnect to an existing session by provider/session ID |
| `nudge` | send an additional prompt or control message |
| `cancel` | request cooperative stop |
| `stream` | read structured events or output frames |
| `transcript` | fetch durable conversation/output history |
| `artifacts` | list or fetch produced artifacts |
| `terminal` | classify final state and failure reason |

## Session Fields

Every `AgentSession` must expose:

| Field | Description |
|-------|-------------|
| `worker_kind` | `codex`, `claude`, or another registered runtime |
| `provider` | `gascity`, `cli-fallback`, or explicit future provider |
| `job_id` | daemon job that owns the session |
| `attempt_id` | queue attempt that started the worker |
| `request_id` | AgentOps request correlation ID |
| `provider_request_id` | provider request/log correlation ID such as `X-GC-Request-Id` |
| `session_id` | provider session identity |
| `event_cursor` | last consumed stream/list cursor |
| `status` | `starting`, `running`, `waiting`, `completed`, `failed`, `cancelled`, `lost`, or `provider_unreachable` |

## Terminal Classification

Terminal state must be explicit:

| Provider Observation | AgentOps Status |
|----------------------|-----------------|
| completed with usable artifacts | `completed` |
| completed but artifact validation failed | `failed` |
| cancelled by AgentOps | `cancelled` |
| session ID previously known but provider cannot find it | `lost` |
| provider readiness unavailable before terminal state | `provider_unreachable` |
| stream disconnected but REST reconciliation pending | `running` with degraded stream |

`lost` and `provider_unreachable` must not be reported as success.

## Streaming And Artifacts

Worker streams are advisory until backed by transcript or artifact evidence.
Consumers must tolerate duplicate stream frames and replay after reconnect.

Artifacts must carry:

- artifact path or provider URI
- producing `job_id`
- producing `attempt_id`
- producing `session_id`
- validation status

## Current Product Path

The daemon-owned wiki/forge path is:

1. `ao overnight` or another producer submits a `wiki.forge` daemon job.
2. `agentopsd` claims the job from the ledger-backed queue.
3. `WikiForgeRunner` starts one `AgentWorker` session per source through
   `wikiworker.Worker`.
4. The worker runtime returns an `OutputEnvelope` with `schema_version: 1`,
   terminal session status, structured payload, optional refusal, and artifact
   refs.
5. Wiki extraction validates the nested `schema_version: 1` extraction payload.
6. Invalid JSON, refusals, schema drift, or malformed artifacts are retried to
   the job cap and then written to `.agents/quarantine/agentworker/`.
7. Successful daemon jobs write
   `.agents/daemon/wiki/<job>-worker-sessions.json` and expose that path as the
   `worker_session_refs` artifact.

The initial production adapter is `GasCityWorker` with Codex and Claude worker
kinds. `AgentWorkerGenerator` also lets existing LLM-style call sites consume
the same session contract while older call sites are migrated.

## Legacy Local LLM

Gemma and Ollama remain explicit compatibility paths outside the daemon
`AgentWorker` runtime. They require visible configuration such as
`--legacy-local-llm`, `AGENTOPS_FORGE_LEGACY_LOCAL_LLM=1`, or
`AGENTOPS_DREAM_CURATOR_ENGINE=ollama`.

Daemon-backed wiki/forge work should use Codex or Claude through this contract.
Legacy local LLM tests must prove that the product path does not instantiate
Ollama or require a Gemma model.

## Legacy Bridge Deprecation

The local Ollama/Gemma bridge is a compatibility bridge, not an AgentWorker
provider. Its retirement schedule is:

| Target | Contract Rule |
|--------|---------------|
| v2.40.x | explicit opt-in only |
| v2.41.x | warn whenever the legacy bridge is used |
| v2.42.x | keep compatibility docs only; remove from product-path examples |
| v3.0.0 | remove from core CLI or move to an external compatibility plugin |

No daemon product path may depend on the bridge during this schedule.
