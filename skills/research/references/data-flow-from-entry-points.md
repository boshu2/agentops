# Data Flow From Entry Points

> Trace requests, jobs, and commands from the surface they enter on through every handler, dependency, and external sink they touch. Linear paths beat speculative breadth-first reads.

## Why Trace From Entry Points

Most architectural questions reduce to "what happens when X arrives?" — an HTTP request, a CLI invocation, a queue message, a scheduled tick. Tracing one of these end-to-end produces:

- An accurate list of files actually involved (vs. files merely related by name).
- The real layering — handler vs. service vs. storage — instead of the layering the docs claim.
- The contract boundaries: what the handler validates, what the service trusts, where errors are caught vs. propagated.
- A reusable diagram other agents can verify by re-running the same trace.

---

## The Four Entry Surfaces

| Surface | Where to look | Common library signals |
|---------|---------------|------------------------|
| CLI | `cmd/`, `bin/`, `src/main.*`, top-level entry files | clap, cobra, click, typer, commander, yargs, argparse |
| HTTP | `routes/`, `api/`, `handlers/`, `controllers/` | axum, actix, fastapi, express, fastify, gin, chi, flask |
| Queue / event | `consumers/`, `workers/`, `subscribers/`, `events/` | bull, sidekiq, celery, kafka clients, rabbitmq clients |
| Scheduler | `jobs/`, `cron/`, `schedules/` | cron strings, `@scheduled` decorators, systemd timers |

A codebase usually has 1–3 of these. Find them all before tracing — you may need to trace one of each surface to understand the full shape.

---

## Trace Procedure

For one chosen entry point:

### Step 1: Locate the dispatcher

Find the registration call (`router.add(...)`, `app.command(...)`, `consumer.subscribe(...)`). Record the `file:line` and the handler symbol it routes to.

### Step 2: Read the handler

Open the handler. Note, in order:

- Inputs and how they are validated.
- Direct dependencies the handler instantiates or receives (DI parameters, module-level singletons).
- External calls (DB, HTTP, filesystem, queue publish) made directly inside the handler.
- Errors caught vs. propagated.

### Step 3: Walk the dependency tree one level deep

For each direct dependency, decide:

- **Self-describing name?** (`UserRepository`, `EmailClient`) — note its purpose without reading.
- **Ambiguous name?** Open it just long enough to write a one-line description.
- **Touches an external sink?** Always open it — the sink is part of the trace.

Stop at the second level unless a third level is obviously the place where the work actually happens.

### Step 4: Find the sinks

Every trace ends at a sink. Common sinks:

| Sink type | Signals to grep |
|-----------|-----------------|
| Database | `query`, `execute`, `INSERT`, `UPDATE`, `db.`, ORM session calls |
| HTTP egress | `fetch`, `reqwest`, `requests.`, `http.Client`, SDK constructors |
| Filesystem | `open`, `File::`, `fs.`, `pathlib`, write/read functions |
| Queue publish | `publish`, `produce`, `send_message`, `enqueue` |
| Stdout / logs | `print`, `println`, structured logger calls when output is the product |

Write the sink down. It is the trace's terminal node.

### Step 5: Note error and retry behavior

Where in the trace are errors caught? Where do they propagate? Are retries or circuit breakers visible? This is where surprise behavior lives.

---

## Output Shape

A trace artifact is short and linear:

```markdown
## Trace: POST /api/jobs

Entry: `src/api/jobs.rs:42` → `create_job` handler

create_job (src/api/jobs.rs:42)
  ↓ validates JobRequest (src/api/jobs.rs:55)
  ↓ JobService::submit (src/services/job.rs:18)
       ↓ calls JobRepository::insert (src/storage/jobs.rs:30) — sink: SQLite
       ↓ calls Queue::publish (src/queue/mod.rs:22) — sink: Redis stream
  ↓ returns 202 with job id

Errors:
- Validation failure → 400 at handler boundary
- Storage failure → bubbles, logged in middleware (src/middleware/log.rs:12), returns 500
- Queue failure → swallowed at JobService::submit:24 — KNOWN GAP, see issue #...
```

One trace, one page. Multiple traces produce multiple short artifacts rather than one sprawling document.

---

## Searches That Help

Use these scoped searches as starting points. Always pass a directory; never grep the whole repo unscoped.

```bash
# Entry-point registration
rg -n "Router::|router\.|@app\.|app\.(get|post)|Cmd\(\"|@click\.command|cobra\.Command" src/

# Handler signatures
rg -n "fn (handle|create|update|get|list|delete)_" src/api/ src/handlers/

# DB sinks
rg -n "query!?\(|execute!?\(|\.query\(|\.exec\(|SELECT |INSERT |UPDATE " src/

# HTTP egress
rg -n "reqwest::|requests\.|fetch\(|http\.Client" src/

# Queue publish
rg -n "publish\(|produce\(|enqueue\(|send_message" src/
```

Pair these with the iterative-retrieval pattern (`skills/research/references/iterative-retrieval.md`) when the first scoped search misses.

---

## Anti-Patterns

| Avoid | Do instead |
|-------|------------|
| Tracing five flows shallowly | Trace one flow end-to-end first |
| Reading every file the handler imports | Use names; only open ambiguous or sink-touching deps |
| Ignoring error paths | Note where errors are caught and where they propagate |
| Calling the trace done at the service layer | Walk to the sink — DB, HTTP egress, filesystem, queue |
| Grepping the whole repo | Always scope to a directory |
| Letting the trace branch into a tree | Pick one path; record alternates as siblings, not children |

---

## When to Use This Reference

- You are answering "what happens when <event> arrives?"
- You need a short artifact that another agent can verify or extend.
- You are about to modify a handler and need to understand its blast radius.
- The architecture docs disagree with the code, and you need the ground-truth path.

For broad onboarding, pair this with `skills/research/references/onboarding-methodology.md`. For prior-work search, see `skills/research/references/iterative-retrieval.md`.

---

> Pattern adopted from `codebase-archaeology` (ACFS skill corpus). Methodology only — no verbatim text.
