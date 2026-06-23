# Headroom Pilot Runbook

This runbook defines how to evaluate
[Headroom](https://github.com/chopratejas/headroom) as an optional context
compression sidecar for AgentOps.

## Decision

Do not make Headroom part of the default AgentOps loop yet. Treat it as an
opt-in pilot around high-volume agent IO.

AgentOps already owns phase-scoped context through `ao context assemble`,
explicit retrieval through `ao lookup` / `ao inject`, and shell-output
compaction through RTK. Headroom's useful boundary is outside that waist:
compressing bulky tool outputs, logs, JSON, and long agent transcripts before a
runtime consumes them.

## Fit

| Surface | Integration posture |
|---|---|
| `ao context assemble` | Keep authoritative. Do not pipe default briefings through Headroom until a fixture proves no loss of acceptance criteria, risks, or validation commands. |
| RTK shell proxy | Keep first for noisy local commands. Use Headroom only when an agent runtime or MCP host will consume the resulting bulk text. |
| `ao mcp serve` | Keep as the AgentOps tool surface. If piloted, Headroom MCP is a separate sidecar server in the host config, not a replacement for `ao mcp serve`. |
| Codex worker sessions | Pilot `headroom wrap codex` only after read-only/audit measurements pass. Do not route Bo-mac AgentOps workers through Claude. |
| `headroom learn` | Dry-run only. Do not let it write `AGENTS.md` or skill files automatically; promote any useful finding through the AgentOps ratchet. |

## Pilot Modes

Start with measurement only:

```bash
python3 -m venv .agents/headroom-pilot/venv
. .agents/headroom-pilot/venv/bin/activate
pip install "headroom-ai[mcp,proxy]"

HEADROOM_TELEMETRY=off \
HEADROOM_DEFAULT_MODE=audit \
headroom proxy \
  --host 127.0.0.1 \
  --port 8787 \
  --log-file .agents/headroom-pilot/proxy.jsonl
```

Verify health and stats from another shell:

```bash
curl -fsS http://127.0.0.1:8787/health
curl -fsS http://127.0.0.1:8787/stats
```

Run a compression-only probe against an AgentOps briefing or large log:

```bash
ao context assemble \
  --phase validation \
  --task "evaluate Headroom integration pilot" \
  --output-file .agents/headroom-pilot/briefing.md

jq -Rs '{messages:[{role:"user", content:.}], model:"gpt-4o"}' \
  .agents/headroom-pilot/briefing.md \
  | curl -fsS http://127.0.0.1:8787/v1/compress \
      -H 'content-type: application/json' \
      --data-binary @- \
  | tee .agents/headroom-pilot/compress-result.json
```

Only after that passes, test runtime wrapping on a read-only task:

```bash
HEADROOM_TELEMETRY=off headroom wrap codex
```

## Acceptance

The pilot passes only if all checks hold:

1. Eligible bulky artifacts show meaningful savings: at least 25% token
   reduction on logs, JSON, or long transcript fixtures.
2. Validation fixtures still surface every fatal/error/warning line required by
   the acceptance examples.
3. `headroom_retrieve` can recover compressed originals when the MCP lane is
   used.
4. No raw secrets appear in `.agents/headroom-pilot/`, `~/.headroom/`, or the
   proxy log; AgentOps redaction remains upstream of any compression.
5. The default AgentOps path still works with Headroom absent.
6. `ao gate check --fast --scope head` passes after any tracked docs or scripts
   change.

## Non-Goals

- Do not replace `ao context assemble`, `ao lookup`, or RTK.
- Do not add Headroom as a required dependency.
- Do not run `headroom learn --apply` against this repo.
- Do not bind the proxy outside `127.0.0.1` during the pilot.
- Do not wrap Claude as an AgentOps executor on Bo-mac.

## Rollback

```bash
pkill -f 'headroom proxy' || true
headroom mcp uninstall || true
unset HEADROOM_TELEMETRY HEADROOM_DEFAULT_MODE HEADROOM_BASE_URL
rm -rf .agents/headroom-pilot
```

If a tracked integration lands later, rollback is a normal revert plus the
runtime cleanup above. Headroom must remain optional: absence of the `headroom`
binary must degrade to the native AgentOps context compiler and RTK path.
