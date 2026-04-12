# Quarantined Tests

Tests in this directory require external runtimes or legacy fixtures that are
not wired into CI. They are **not** executed by `.github/workflows/validate.yml`
or `tests/run-all.sh`. Each remaining suite has an explicit promotion plan
below — this is staging, not a graveyard.

## Triage status (na-gtm.8, 2026-04-10)

Starting point: 7 suites + 2 top-level scripts.
Removed: `opencode/`, `rpi-e2e/`, `skill-triggering/`, `e2e-install-test.sh`,
`marketplace-e2e-test.sh`.
Promoted: `ol-integration/` -> `tests/ol-integration/` (na-gtm.18,
2026-04-12).
Promoted: `team-runner/` -> `tests/team-runner/` (na-gtm.19, 2026-04-12).
Promoted: `codex/` -> `tests/codex/integration/` (na-gtm.17, 2026-04-12).
Promoted: `claude-code/` -> `tests/claude-code/` (na-gtm.20, 2026-04-12).
Remaining: **0 suites**.

No quarantined suites remain.

## Deletions performed

- **`opencode/`** (3 files + prompts/) — Targeted an `opencode` headless test model and wrote to `.agents/opencode-tests/` inside the repo. OpenCode is a peripheral runtime, tests were side-effectful, and no one runs them.
- **`rpi-e2e/run-full-rpi.sh`** — 403-line shell simulation of the RPI lifecycle using mocked `.agents/` dirs. Superseded by real Go unit/integration tests in `cli/cmd/ao/rpi_*_test.go` (10+ files) and the `gc` bridge tests (`TestGC*`). Shell-level RPI e2e is legacy.
- **`skill-triggering/`** — `run-all.sh` sourced `claude-code/test-helpers.sh` (tight coupling) and only tested natural-language trigger phrases. Will be recreated inside the `claude-code/` promotion when a skip-on-absent guard lands. Prompts still recoverable from git history.
- **`e2e-install-test.sh`** — Assumes the old multi-plugin marketplace layout (`agentops/` subdir, per-plugin args). Current repo is a single skills source of truth; this test has no referent.
- **`marketplace-e2e-test.sh`** — Same reason: tests a plugin marketplace model that no longer exists. Replaced in spirit by `tests/install/`, `tests/skills/`, and `tests/spec-consistency/`.

## Running manually

```bash
bash tests/claude-code/run-all.sh               # promoted; skips if claude CLI missing
bash tests/codex/integration/run-all.sh         # promoted; skips if codex CLI missing
bash tests/ol-integration/vibe-ol-test.sh       # promoted, fixture-only
bash tests/ol-integration/swarm-ol-test.sh      # promoted, fixture-only
bash tests/team-runner/run-all.sh               # promoted, fixture-only
```

## Promotion plans

### Plan A — `codex/` (done)

Promoted by na-gtm.17 on 2026-04-12:
`tests/_quarantine/codex` moved to `tests/codex/integration`, the wrapper now
reports Codex-absent child tests as skipped instead of failed, and
`tests/codex/README.md` documents `CODEX_MODEL` and live-runtime cost. The suite
is covered by the existing `tests/run-all.sh --tier=2` Codex integration hook.

**Cost:** ~30 min. No code changes, CLI-skip behavior already correct.

### Plan B — `claude-code/` (done)

Promoted by na-gtm.20 on 2026-04-12:
`tests/_quarantine/claude-code` moved to `tests/claude-code`, missing `claude`
now exits 0 with a `SKIPPED` message, and `tests/claude-code/README.md`
documents the live-runtime cost guards. The suite is covered by the existing
`tests/run-all.sh --tier=3` Claude Code hook.

**Cost:** ~2 hours. Needs a runner that has `claude` available; document cost caps.

### Plan C — `ol-integration/` (done)

Promoted by na-gtm.18 on 2026-04-12:
`tests/_quarantine/ol-integration` moved to `tests/ol-integration`, where the
existing `"$SCRIPT_DIR/../.."` root calculation resolves correctly. No external
`ol` binary is required; the fixture-only scripts now run in the default
`tests/run-all.sh` lane.

**Cost:** ~15 min. Lowest-risk promotion.

### Plan D — `team-runner/` (done)

Promoted by na-gtm.19 on 2026-04-12:
`tests/_quarantine/team-runner` moved to `tests/team-runner`, root path
calculation changed from `../../../` to `../../`, and the fixture-only suite now
runs in the default `tests/run-all.sh` lane. The stream watcher tests replay
JSONL fixtures through local watcher scripts and do not require live Claude or
Codex CLIs.

**Cost:** ~1 hour. Schemas and scripts still live, so ROI is good.

## Follow-up issues

- **na-gtm.17** — CLOSED: promoted `codex/` (Plan A)
- **na-gtm.18** — CLOSED: promoted `ol-integration/` (Plan C)
- **na-gtm.19** — CLOSED: promoted `team-runner/` (Plan D)
- **na-gtm.20** — CLOSED: promoted `claude-code` with skip-on-absent guard (Plan B)
