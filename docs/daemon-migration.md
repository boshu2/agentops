# Legacy To Daemon Migration

AgentOps is moving from foreground command flows to an always-on local daemon.
The daemon path is currently opt-in: existing one-shot RPI, Dream, and
wiki/forge commands continue to work unless you pass the daemon flags.

Use this guide when migrating local automation, wrappers, or OpenClaw
integrations to `agentopsd`.

## What Changes

Before daemon mode, each command owned its own state:

- `ao rpi` wrote RPI registry files directly.
- `ao overnight` owned Dream locks, stages, and summaries in the foreground.
- wiki/forge work could use direct local LLM paths when configured.
- OpenClaw-style consumers had to read projection files or `.agents` state
  directly.

In daemon mode, `agentopsd` owns the runtime ledger:

- accepted jobs are appended to `.agents/daemon/ledger.jsonl`
- projections such as RPI status, Dream summaries, wiki outputs, and OpenClaw
  snapshots are rebuilt from the ledger
- `ao doctor` reports daemon, GasCity, and OpenClaw readiness
- OpenClaw reads `/openclaw/v1/*` resources and uses authorized trigger
  endpoints instead of writing `.agents` directly

## Start The Daemon

Run the daemon in the foreground first. Service installation remains a dry-run
planning surface until foreground readiness is boring.

```bash
ao daemon run --addr 127.0.0.1:8765 --token "$AGENTOPS_DAEMON_TOKEN"
```

The daemon writes `.agents/daemon/activation.json` with its URL and readiness
metadata. Other commands use that activation file when `--daemon-url` is not
provided.

Check readiness:

```bash
ao daemon ready
ao daemon status
ao doctor --json
```

`ao doctor` warnings for daemon, GasCity, or OpenClaw mean the product runtime
is not fully available. They do not make the existing foreground command path
invalid.

## Submit RPI Through The Daemon

Foreground RPI remains the compatibility default:

```bash
ao rpi --phased "ship feature"
```

Daemon submission is explicit:

```bash
ao rpi --phased --daemon-submit \
  --daemon-token "$AGENTOPS_DAEMON_TOKEN" \
  "ship feature"
```

Use `--daemon-url` when you do not want to read the activation file. Use
`--daemon-fallback` only for interactive migration runs where foreground
execution is acceptable if the daemon is not ready.

```bash
ao rpi status --daemon --daemon-fallback
```

Status reads can fall back to the local RPI registry while projections are being
migrated, but daemon job acceptance is authoritative once the ledger append
succeeds.

## Submit Dream Through The Daemon

Foreground Dream remains the compatibility default:

```bash
ao overnight start --goal "compound the corpus"
```

Daemon submission is explicit:

```bash
ao overnight start \
  --goal "compound the corpus" \
  --daemon-submit \
  --daemon-token "$AGENTOPS_DAEMON_TOKEN"
```

If daemon readiness fails, the command errors by default. Pass
`--daemon-fallback` only when the one-shot foreground path is allowed to proceed.

Daemon Dream jobs preserve the same INGEST, REDUCE, MEASURE, and COMMIT
semantics. The difference is ownership: job acceptance, worker session refs, and
terminal state are recorded in the daemon ledger before projections claim them.

## Wiki And Forge

Daemon-backed wiki/forge jobs use the `AgentWorker` runtime and GasCity-backed
Codex or Claude sessions.

Legacy local Ollama/Gemma paths are not product defaults. They require an
explicit escape hatch:

```bash
ao forge transcript --tier=1 --legacy-local-llm --model gemma2:9b <transcript>
```

or:

```bash
AGENTOPS_FORGE_LEGACY_LOCAL_LLM=1 ao forge transcript --tier=1 --model gemma2:9b <transcript>
```

Dream local curator diagnostics may still report Ollama/Gemma state when
configured with `AGENTOPS_DREAM_CURATOR_ENGINE=ollama`, but daemon-backed
wiki/forge does not require a Gemma model.

### Legacy Local LLM Deprecation Schedule

This schedule applies only to the local Ollama/Gemma bridge. It does not remove
GasCity-backed Codex/Claude workers or the `AgentWorker` contract.

| Target | Behavior |
|--------|----------|
| v2.40.x | Legacy local LLM requires `--legacy-local-llm`, `AGENTOPS_FORGE_LEGACY_LOCAL_LLM=1`, or `AGENTOPS_DREAM_CURATOR_ENGINE=ollama` |
| v2.41.x | Legacy local LLM emits a CLI deprecation warning whenever the path is used |
| v2.42.x | Legacy local LLM docs move out of product-path examples and remain only in compatibility notes |
| v3.0.0 | Core CLI local Ollama/Gemma bridge may be removed or moved to an external compatibility plugin |

Do not add new product features to the legacy bridge. New daemon wiki/forge work
should target AgentWorker sessions backed by GasCity.

## GasCity Readiness

GasCity is the preferred substrate for headless Codex and Claude worker
sessions. The AgentOps adapter distinguishes:

- missing `gc` binary
- version below the supported bridge minimum
- API unavailable
- controller or provider not ready
- CLI fallback being available but not equivalent to API/SSE readiness

Normal CI uses fake GasCity fixtures. Live GasCity checks should stay opt-in
with `AGENTOPS_LIVE_GASCITY=1`.

The full no-GasCity, CLI fallback, API/SSE, daemon mode, and OpenClaw consumer
matrix lives in [GasCity Integration](contracts/gascity-integration.md).

## OpenClaw Migration

OpenClaw should consume daemon projections through local HTTP:

- `GET /openclaw/v1/health`
- `GET /openclaw/v1/snapshot/latest`
- `GET /openclaw/v1/runs`
- `GET /openclaw/v1/jobs`
- `GET /openclaw/v1/wiki`

OpenClaw must not write `.agents` directly. When it needs work to happen, it
calls an authorized trigger endpoint such as `/openclaw/v1/triggers/jobs`; the
daemon validates local trust, appends the ledger event, and returns accepted job
IDs.

## Rollback And Fallback

During migration:

- keep foreground RPI and Dream commands available for compatibility
- use daemon fallbacks only when the caller can tolerate foreground execution
- treat `.agents/daemon/ledger.jsonl` as authoritative for daemon-accepted work
- rebuild projections from the ledger after crashes or partial projection writes
- mark degraded projections instead of claiming successful state from stale files

If you need to disable daemon mode, remove the `--daemon-submit` flag and run
the foreground command path. Do not edit `.agents/daemon/ledger.jsonl` by hand.

## Validation Checklist

Run this before making daemon mode the default for a workflow:

```bash
ao daemon ready
ao daemon status
ao doctor --json
ao rpi verify --latest --json
scripts/validate-daemon-product-e2e.sh --fixture
scripts/check-closeout-gate.sh --json
scripts/pre-push-gate.sh --fast
```

The closeout proof should also include OpenClaw health, GasCity bridge
diagnostics, fake GasCity worker fixtures, ledger replay, projection rebuild,
state-machine invariants, boundary failpoints, and worktree disposition.
