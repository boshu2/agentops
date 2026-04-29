# Remote Compute Contract

Remote compute is the AgentOps product boundary for running agent sessions on a
configured remote node while keeping product names generic. Bushido is a
private dogfood target and soak harness, not a public AgentOps namespace.

This contract freezes the nouns used by `ao compute`, daemon jobs, GasCity
worker sessions, and recovery projections before implementation starts.

## Contract Version

| Field | Value |
|-------|-------|
| `schema_version` | `1` |
| Target schema | `remote-compute-target.schema.json` |
| Session event schema | `remote-session-event.schema.json` |
| Primary session provider | `gascity-api-sse` |
| Bootstrap transports | `ssh`, `local`, `manual`, `none` |

## Core Nouns

| Noun | Definition |
|------|------------|
| `RemoteTarget` | Operator-configured product target for a GasCity city endpoint plus bootstrap metadata. It is not a host alias alone. |
| `RemoteNode` | Host or process environment running the GasCity supervisor and worker providers. A node may be reached by bootstrap transport, but session state comes from GasCity. |
| `BootstrapTransport` | Setup or rescue mechanism such as SSH. It may install, start, or diagnose GasCity; it is not a product session provider. |
| `SessionProvider` | Runtime API that owns sessions, events, transcripts, artifacts, and provider request IDs. The first provider is GasCity API/SSE. |
| `RemoteSession` | AgentOps durable view of one provider session, owned by a daemon job or foreground command record. |
| `RemoteCommand` | Idempotent prompt or control delivery attempt recorded by AgentOps before provider delivery. |

## Target Identity

Every `RemoteTarget` must define stable identity without embedding secrets.

Required target fields:

- `target_id`: local stable name used by CLI and projections.
- `provider`: `gascity`.
- `gascity.endpoint`: HTTP(S) endpoint for the GasCity supervisor.
- `gascity.city`: target city name.
- `bootstrap_transport`: one of `ssh`, `local`, `manual`, or `none`.
- `bootstrap_profile`: optional host, user, workdir, and supervisor hints used
  only for bootstrap or rescue.
- `auth_ref`: pointer to credentials by reference, never inline secret value.
- `capabilities`: explicit booleans for API/SSE, sessions, transcripts,
  artifacts, cancel, provider readiness, and context sync.
- `redaction`: fields that must be redacted from logs and proof artifacts.

Bushido may appear as a private target profile or live-soak label. It must not
create an `ao bushido` command family or a product-specific provider kind.

## Session Identity

Every `RemoteSession` must be reconstructable from the daemon ledger or the
foreground command ledger after a crash.

Required session fields:

- `session_id`: AgentOps remote session identity.
- `provider`: `gascity`.
- `target.target_id` and `target.city`.
- `job_id` and `attempt_id` when daemon-owned.
- `request_id`: AgentOps request correlation ID.
- `provider_request_id`: provider response correlation value such as
  `X-GC-Request-Id`.
- `provider_session_id`: GasCity session ID or alias.
- `event_cursor`: last consumed list/SSE cursor.
- `transcript_refs`: durable transcript references.
- `artifact_refs`: durable artifact references.
- `terminal_status`: one of `completed`, `failed`, `cancelled`, `lost`,
  `provider_unreachable`, or `unknown` when terminal classification is not yet
  proven.

Raw tmux pane state, SSH command exit state, and local process presence are not
authoritative terminal state for product remote sessions.

## Command Ledger

AgentOps must record command intent before delivery to GasCity.

Each `RemoteCommand` includes:

- `command_id`: AgentOps command identity.
- `session_id`: AgentOps remote session identity.
- `idempotency_key`: stable key for provider delivery and replay.
- `kind`: `prompt`, `control`, `cancel`, or future registered kind.
- `payload_ref` or redacted payload summary.
- `status`: `recorded`, `delivery_attempted`, `accepted`, `rejected`,
  `delivery_unknown`, or `superseded`.
- `provider_request_id` when available.
- `recorded_at`, `last_attempted_at`, and optional `accepted_at`.

After a crash or network split, AgentOps must not blindly resend commands whose
delivery state is unknown. It must mark the command `delivery_unknown`, then
reconcile through provider request ID, idempotency key, session events, or
transcript evidence before retrying or reporting failure.

## Event Model

Remote session events are append-only facts consumed by daemon projections,
status UX, and recovery flows.

Required event classes:

- `session_recorded`
- `session_started`
- `command_recorded`
- `command_delivery_attempted`
- `command_accepted`
- `command_rejected`
- `command_delivery_unknown`
- `event_cursor_advanced`
- `transcript_ref`
- `artifact_ref`
- `terminal_state`

Every event carries `session_id`, `provider`, `target`, `event_cursor`,
`terminal_status`, `command_id`, `idempotency_key`, and `artifact_refs` fields.
Fields may be `null` only when the event class does not apply to that value.

## State Ownership

The daemon ledger remains authoritative for daemon-owned remote sessions.
Remote-specific code may keep read models, but projections must rebuild from
ledger events and provider replay.

| Surface | Ownership |
|---------|-----------|
| `.agents/remote/targets.json` | Optional local target registry for CLI use. |
| `.agents/daemon/ledger.jsonl` | Durable job, session, command, and projection facts for daemon-owned work. |
| GasCity API/SSE | Provider source of truth for provider session status, events, transcripts, and artifacts. |
| OpenClaw | Read-only projection consumer; it does not own remote session mutation. |

Foreground `ao compute` commands may write local command/session proof records,
but daemon-submitted work must not report accepted or running until the daemon
ledger append succeeds.

## GasCity First

The product remote execution path is GasCity API/SSE:

1. Diagnose target readiness through GasCity health, readiness, provider
   readiness, city readiness, and event replay surfaces.
2. Record AgentOps session and command intent.
3. Deliver start, nudge, cancel, and attach operations through the public
   GasCity API.
4. Persist `X-GC-Request-Id`, provider session ID, event cursor, transcript
   refs, artifact refs, and terminal classification.
5. Rebuild AgentOps projections from the ledger and provider replay.

SSH/tmux is bootstrap and rescue only. A direct SSH/tmux product provider
requires a new contract revision and must not be introduced in RC-01.

## Recovery Rules

Recovery must prefer durable evidence in this order:

1. AgentOps daemon ledger command/session events.
2. GasCity list/event replay using `event_cursor` or `Last-Event-ID`.
3. GasCity transcript and artifact evidence.
4. Bootstrap/rescue diagnostics.

If the provider is unreachable before terminal proof, classify the session as
`provider_unreachable`. If a previously accepted provider session cannot be
found after reconciliation, classify it as `lost`. Neither state is success.

## Security And Privacy

- No inline secrets in target or session documents.
- Auth values are referenced by `auth_ref` and redacted in logs.
- Remote command payloads are either redacted or stored by durable proof ref.
- Local daemon mutation rules apply before remote session mutations are added.
- Live Bushido GC-city soak tests are opt-in and use private target labels.

## Conformance Checks

RC-01 is valid when these checks pass:

```bash
rg -n "RemoteTarget|RemoteNode|RemoteSession|RemoteCommand|GasCity|bootstrap_transport" docs/contracts/remote-compute.md
python3 -m json.tool schemas/remote-compute-target.schema.json >/dev/null
python3 -m json.tool schemas/remote-session-event.schema.json >/dev/null
bash tests/scripts/test-remote-compute-contracts.sh
scripts/eval-agentops.sh --suite evals/agentops-core/remote-compute-contracts.json
bash scripts/check-contract-compatibility.sh
```
