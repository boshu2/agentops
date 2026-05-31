# Dreaming / Memory Writers Characterization (Spike ag-cj8mk)

> **Status:** decision/characterization doc (spike). Unblocks the Dreaming→corpus converter
> family: the converter (ag-5vozd), the holdout-leak guard (ag-onf37), the Claude feeder
> (ag-5lllu), and the Codex/NTM feeder + scheduling (ag-8lyx5). It characterizes SHAPES only.
> **No holdout/eval `expected` values, no PII, and no real memory-note bodies are reproduced here.**

This doc answers the spike question: *what are the three foreign Dreaming/memory writers, what
do they emit, where, and what is the normalized `.agents/` target the converter must produce?*
It also pins the hard constraint that gates the whole family: **Managed Agents are NOT zero-data-retention
(ZDR), so holdout/eval ground-truth and PII must never reach the cloud feeders.**

The deliverable is a go/no-go on the Claude-side pull feeder. **Verdict: GO — stable pull.**

## Scope

- BC1-Corpus. Read-side characterization only; no production code ships from this spike.
- Three foreign writers: (1) Anthropic Dreaming / Managed-Agents memory stores, (2) the OpenClaw
  memory-wiki indexer on bushido, (3) the legacy local gc-dream / Tier-3 synthesis pipeline.
- One normalized target: the BC1 `CorpusWriterPort` artifact (`.agents/learnings/*.md`), written
  exclusively through `ao corpus capture` so the leak guard (ag-onf37) always runs.

## The normalized target (what every converter output must be)

The converter's common intermediate is `{title, body, tags, origin_session, timestamp, source}`.
The persisted artifact is a `.agents/learnings/<YYYY-MM-DD>-<id>.md` file matching the canonical
[Lesson Format](lesson-format.md): YAML front-matter (`id`, `date`, `severity`, `trigger`,
`verifiable`, `rule`, `falsified_by`, `practice`, `related`) + `## Context` / `## Why this matters`
/ `## How to apply` body. This is the same shape `forge`/`harvest` already emit, so `/compile`
(Mine → Grow → Defrag → Lint) and `/inject` decay-ranking treat imported notes identically to
forged ones.

### The write port (current state + required delta)

The write seam is `ports.CorpusWriterPort.Capture(ctx, CorpusWriteRequest{Path, Body, Metadata})`
(`cli/internal/ports/corpus_writer.go`), surfaced as the CLI command `ao corpus capture`
(`cli/cmd/ao/corpus_capture.go`).

Current flag surface (`cli/docs/COMMANDS.md`):

```
ao corpus capture --path <relpath> [--body <text>|--body-file <file>|--body-stdin]
                  [--root <dir>] [--meta k=v ...]
```

| Aspect | Current state | Delta the converter family needs |
|---|---|---|
| Body input | `--body` / `--body-file` / `--body-stdin` | sufficient — converter pipes normalized body via `--body-stdin` |
| Provenance | `--meta k=v` (open bag, persisted as front-matter) | **`--source <label>` first-class flag** (ag-onf37): `anthropic-dreaming` / `openclaw-cron` / `gc-dream` / `forge`, stamped to front-matter for `/provenance` + `/trace` |
| Leak guard | **none today** | **ag-onf37 adds the holdout-leak refuse-to-persist check on the write path** |
| Idempotency | port contract guarantees idempotent re-capture | converter adds content-hash dedup so re-pulling the same note is a no-op |

The converter NEVER writes `.agents/` files directly — always through `ao corpus capture`, so the
guard is unbypassable.

## Writer 1 — Anthropic Dreaming (Managed Agents)

**This is the Claude-runtime feeder source and the spike's primary subject.** Dreaming is a real
Managed-Agents Research-Preview feature, not a metaphor. Beta headers: `managed-agents-2026-04-01`
plus `dreaming-2026-04-21`.

### What it is and how it is triggered

A **dream** (`POST /v1/dreams` → `drm_...`) is an asynchronous job that reads a pre-existing
**memory store** plus 1–100 past **sessions**, then produces a *new* **output memory store**
(deduped, reorganized, with surfaced insights). The input store is never modified. Models supported
in preview: `claude-opus-4-8`, `claude-opus-4-7`, `claude-sonnet-4-6`.

### Output format and destination

The dream resource carries `status` (`pending` → `running` → `completed`|`failed`|`canceled`),
`inputs[]`, `outputs[]`, `model`, `instructions`, `session_id`, and `usage`. On `completed`, the
`outputs[]` entry of `type: "memory_store"` references the rebuilt store
(`{ "type": "memory_store", "memory_store_id": "memstore_..." }`).

The durable note shape lives in that memory store, read via the **Memory Stores API**
(`GET /v1/memory_stores/{id}/memories`, `.../memories/{mem_id}`). A memory note (`mem_...`) is:

- `path` — addressing key within the store (e.g. `/preferences/formatting.md`)
- `content` — **freeform markdown text body** (≤ 100 kB / ~25k tokens per note)
- `content_sha256` — content hash (the converter's dedup + optimistic-concurrency key)
- versioning: every write creates an immutable `memver_...` with a `created_by` actor and an
  `operation` (`created`/`modified`/`deleted`); versions support `redact` for compliance scrubs

There are **no structured tags** on a memory note — it is path + freeform content. Tagging is the
converter's job (derive `tags`/`severity` heuristically, stamp `source: anthropic-dreaming`).

### Stable pull vs opportunistic SSE — GO

There IS a stable pull surface. Two layers, both pull:

1. **Dream lifecycle (pull):** poll `GET /v1/dreams/{id}` until `status: completed`, read `outputs[]`.
2. **Note read (pull):** enumerate the output store via `memories.list` / `memories.retrieve`.

SSE is optional and only a live-progress view: while a dream is `running`, its `session_id` points
at the executing session whose [event stream](https://platform.claude.com/docs/en/managed-agents/events-and-streaming)
(`agent.tool_use` / `agent.tool_result` on the `/mnt/memory/` mount) can be watched. The converter
does **not** depend on SSE — it reads the finished store by ID. **Feeder design (ag-5lllu): track a
high-water mark on dream `created_at`/`id` + per-note `content_sha256`; pull only new notes.**
No manual-export fallback is required.

### Self-hosted / bushido path

If the Managed Agent runs in the bushido self-hosted sandbox, the puller reaches it over the
tailnet (`100.109.17.108`); the dream/memory-store control plane is still the hosted REST API.

## Writer 2 — OpenClaw memory-wiki indexer (bushido, Codex/NTM path)

`~/.openclaw/` on bushido WSL. The cron scheduler (`~/.openclaw/cron/jobs.json`) is **empty** as of
2026-05-31 (consistent with the dream-pipeline retirement in the vault contract); historical backups
(`jobs.json.bak-*`) show prior `agentTurn` jobs (health-check, morning/good-night reports), not a
note-emitting dreaming writer.

The actual "memory writer" is an **indexer, not a structured-note emitter**: `~/.openclaw/memory/main.sqlite`
is a chunk + embedding store (`files`, `chunks`, `chunks_fts`, `chunks_vec`). The canonical notes are
the **source markdown files on disk** that it indexes — `files.path` rows are `MEMORY.md` and
`memory/<YYYY-MM-DD>.md` (`source = "memory"`), i.e. it indexes the `~/wiki`/memory markdown tree.

| Aspect | Value |
|---|---|
| Output format | source-of-truth is on-disk markdown (`memory/<date>.md`); sqlite holds derived `chunks(text, embedding, hash)` + `files(path, source, hash, mtime)` |
| Destination | `~/.openclaw/memory/main.sqlite` (index) over `~/wiki`/`MEMORY.md` (source) |
| Already touches `.agents/`? | **No** |
| Converter delta | adapter reads the source markdown note (title from path/heading, body = file content), maps to the common intermediate, captures with `--source openclaw-cron`; serialize concurrent `.agents/learnings/` writes with agent-mail file locks (ag-8lyx5) |

## Writer 3 — Local gc-dream / Tier-3 synthesis (bushido, legacy, Codex/NTM path)

`~/dream/` and `~/ops/dream/` on bushido WSL. The Tier-1 gemma night-run produces dream drafts; a
Tier-3 synthesis pass reviews them. **Pipeline is retired** (vault contract, 2026-05-31) but the
output shapes are on disk and stable.

- **Dream draft** (e.g. `~/wiki/reviewed/dream-<date>-<seedhash>.md`): YAML front-matter
  (`source_kind: dream-seed`, `ingested_at`, `ingested_by`, `job_id`, `seed_hash`, `seed_preview`,
  `status`) + markdown body + a producer footer.
- **T3 verdict** (`~/ops/dream/t3-synthesis-verdicts/dream-<date>-<seedhash>.json`): JSON with
  `draft_file`, `source_kind`, `seed_hash`, scores (`faithfulness`/`specificity`/`novelty`/`grounding`),
  `verdict` (`PROMOTE`/`REWRITE`/`KILL`/`DEDUPE_KILL`), `reason`, `avg_score`, `reviewed_at`,
  `reviewed_by`, and on promote a `promoted_to` path.

| Aspect | Value |
|---|---|
| Output format | draft = front-matter markdown; verdict = scored JSON (above) |
| Destination | `~/dream/`, `~/ops/dream/t3-synthesis-verdicts/`, promoted to `~/wiki/reviewed/` |
| Already touches `.agents/`? | **No** (writes the `~/wiki` tree) |
| Converter delta | adapter consumes the draft front-matter + body (only `verdict == PROMOTE` drafts), maps `seed_hash`→dedup key, captures with `--source gc-dream` |

## Holdout / leak constraint (load-bearing, gates the whole family)

**Managed Agents are NOT ZDR.** The live data-usage / managed-agents docs confirm cloud retention:
dream/session transcripts are retained (the dream's pipeline session is *archived, not deleted*),
and memory versions are retained ~30 days (recent versions kept longer). Therefore any content that
flows to a cloud feeder is retained server-side and is **not** zero-retention.

Consequence for this family: **holdout/eval `expected` answers, eval ground-truth, and PII must NEVER
be sent to the cloud feeders** (nor reproduced in this doc). The eval holdout surface
(`~/.agents/evals/SCHEMA.md`) keys quota on `(task_id, split, quarter)`, stores answers in the
`expected` field of `samples-holdout.jsonl` rows, and tracks `holdout_burn_ledger` in Dolt.

The enforcement boundary is the converter + leak guard, **on the capture write path**, not on the
feeders:

- **ag-onf37** adds the holdout-leak guard inside `ao corpus capture`. It keys on `(task_id, split)`
  semantics (NOT harness path), fingerprints each holdout `expected` value (whitespace-collapsed,
  lowercased substring + content hash), and **refuses to persist** any artifact containing a holdout
  answer (non-zero exit, clear message). The guard's job is **refuse-to-persist, never burn** — it
  does not write `holdout_burn_ledger` because it never scores. Env escape `AGENTOPS_HOLDOUT_EVALUATOR=1`
  downgrades to warn (legitimate evaluators only); default is hard refuse.
- **ag-5vozd** adds a secret/PII scrub upstream of capture: credential-pattern matches are quarantined
  (dropped to a quarantine dir), not captured.
- **ag-8lyx5** adds the nightly CI backstop: scan the checked-in corpus for any holdout-answer leak and
  FAIL the build if found.

Because every feeder terminates in `ao corpus capture`, the guard is the single authoritative gate
for all three writers on both runtimes.

## Cross-runtime split (Claude vs Codex/NTM)

| Runtime | Feeder source | Reaches Dreaming? |
|---|---|---|
| Claude | Managed Agents `/v1/dreams` + Memory Stores REST pull (ag-5lllu) | yes (REST) |
| Codex/NTM | OpenClaw memory markdown + gc-dream drafts via `ssh bushido` (ag-8lyx5) | no — covers the bushido-local writers |

Both paths feed the same runtime-agnostic converter (ag-5vozd) and the same guard (ag-onf37) in the
shared `ao` binary, so leak-safety and corpus shape are identical regardless of runtime.

## Go/no-go recommendation

**GO on the Claude-side pull feeder.** Anthropic Dreaming exposes a stable, pollable pull surface
(`/v1/dreams` lifecycle → output memory store → Memory Stores REST), with `content_sha256` as a
natural dedup/high-water key. No opportunistic-SSE-only fallback or manual export is required; SSE is
an optional live-progress view. Proceed with ag-5lllu as a REST puller (`ao corpus import-dreaming`
or a script), gated unconditionally by the ag-onf37 leak guard on the `ao corpus capture` write path.

## Sources

- Anthropic Managed Agents — Dreams: `https://platform.claude.com/docs/en/managed-agents/dreams.md`
- Anthropic Managed Agents — Memory: `https://platform.claude.com/docs/en/managed-agents/memory.md`
- BC1 write port: `cli/internal/ports/corpus_writer.go`; CLI: `cli/cmd/ao/corpus_capture.go`, `cli/docs/COMMANDS.md`
- Normalized target: [Lesson Format Contract](lesson-format.md)
- OpenClaw memory store: `~/.openclaw/memory/main.sqlite`, `~/.openclaw/cron/jobs.json` (bushido WSL)
- Local dream pipeline: `~/dream/`, `~/ops/dream/t3-synthesis-verdicts/`, `~/wiki/reviewed/` (bushido WSL)
- Holdout surface: `~/.agents/evals/SCHEMA.md` (quota key `(task_id, split, quarter)`, `samples-holdout.jsonl`, `holdout_burn_ledger`)
