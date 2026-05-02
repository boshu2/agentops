# Daemon Idempotency Contract

> **Status:** Draft
> **Decision:** `request_id` is trace-only; `idempotency_key` is the submit retry deduplication key.
> **Consumers:** `agentopsd` HTTP clients, daemon-submitting CLI commands, queue replay tests

This contract records the submit-time idempotency decision for `agentopsd`.
It resolves the ambiguity between `request_id` and `idempotency_key` without
changing the queue's existing durable behavior.

## Contract

- `request_id` is a correlation and audit field. Reusing it on `POST /v1/jobs`
  without an `idempotency_key` can create multiple accepted jobs.
- `idempotency_key` is the retry deduplication key for `POST /v1/jobs`.
  Reusing the same key returns the already-accepted job and does not append a
  second `job.accepted` event.
- `job_id` collisions still return the existing job for compatibility, but new
  retry-capable clients must not rely on `job_id` as their only retry contract.
- The submit response echoes the accepted job state. On an idempotent retry, the
  response can therefore contain the original accepted job's `request_id`.

## CLI Helper Policy

Daemon-submitting CLI code should pass a semantic idempotency key when it has a
natural operation identity, for example `rpi.run:<run_id>`,
`dream.run:<run_id>`, `wiki.forge:<run_id>`, or a schedule-derived key.

For lower-level CLI submissions that omit the key, `postDaemonSubmitJob`
generates one before sending the HTTP request:

```text
cli-submit:<job_type>:<sha256(job_type, job_id, request_id, payload)>
```

That helper-generated key is a client-side retry aid. It does not make raw
daemon HTTP `request_id` values dedup keys; raw clients that need retry safety
must send `idempotency_key`.

## Regression Coverage

- HTTP L2: reused `request_id` without `idempotency_key` creates distinct jobs.
- HTTP L2: reused `idempotency_key` without `job_id` dedups to the first job.
- CLI unit: helper-generated keys are stable for identical submit material and
  never overwrite explicit semantic keys.
