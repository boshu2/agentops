# Testing

Use the narrowest deterministic check that proves the changed behavior during an
experiment. Run the repository's ordinary full deterministic suite once on the
complete candidate.

## Core contract checks

```bash
python3 -m unittest discover -s skills/validate/tests
./scripts/check-cathedral-cut-conformance.py
python3 scripts/generate-skill-mesh.py --check
cd cli && go test ./...
```

The Validate helper is also probed in a temporary non-Git directory with no
`ao` on `PATH`. Fake `git`, `ao`, tracker, push, and delivery executables ensure
identity and verdict helpers do not acquire hidden substrate dependencies.

`ao gate check` is an ordinary test runner. Its exit status is not a semantic
verdict and does not authorize delivery.

## Explicit skill requests

`tests/explicit-skill-requests/` holds one explicit qualified request per current
canonical skill (`prompts/<skill>.txt`). `run-all.sh` checks manifest validity,
canonical and Codex artifact existence, and matching skill names. It is Tier S
structural proof, with negative regression fixtures for broken resolution. It
launches no runtime and does not establish live selection or first-tool ordering.
See the [suite contract](https://github.com/boshu2/agentops/blob/main/tests/explicit-skill-requests/README.md).

`tests/run-all.sh --all` includes these structural checks and the three live
[Codex CLI primitive probes](https://github.com/boshu2/agentops/blob/main/tests/codex/README.md). A successful aggregate
is not release C1/C8 live workflow qualification. Each aggregate gets a unique
retained log directory; set `RUN_ALL_LOG_DIR` to choose its artifact directory. Set `RUN_ALL_CODEX_MODEL` for an invocation-local
model override scoped to the live Codex lane.
Native errors and timeouts remain failures, with full diagnostics retained.
