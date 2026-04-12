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
Remaining: **3 suites**.

| Suite | Status | Plan |
|---|---|---|
| `codex/` | PROMOTE — ready | Wire into `validate.yml` as optional job; skips cleanly if `codex` CLI absent (see Plan A). |
| `claude-code/` | PROMOTE — needs skip wrapper | Currently hard-exits if `claude` is missing. Add pre-flight skip-on-absent guard, then wire as optional job (Plan B). |
| `team-runner/` | PROMOTE — needs path fix | `run-all.sh` computes `REPO_ROOT` as `../../../` (correct for `tests/team-runner/`, wrong under `_quarantine/`). Schemas it tests (`lib/schemas/team-spec.json`, `worker-output.json`) and scripts it tests (`lib/scripts/team-runner.sh`, `watch-claude-stream.sh`) still exist (Plan D). |

## Deletions performed

- **`opencode/`** (3 files + prompts/) — Targeted an `opencode` headless test model and wrote to `.agents/opencode-tests/` inside the repo. OpenCode is a peripheral runtime, tests were side-effectful, and no one runs them.
- **`rpi-e2e/run-full-rpi.sh`** — 403-line shell simulation of the RPI lifecycle using mocked `.agents/` dirs. Superseded by real Go unit/integration tests in `cli/cmd/ao/rpi_*_test.go` (10+ files) and the `gc` bridge tests (`TestGC*`). Shell-level RPI e2e is legacy.
- **`skill-triggering/`** — `run-all.sh` sourced `claude-code/test-helpers.sh` (tight coupling) and only tested natural-language trigger phrases. Will be recreated inside the `claude-code/` promotion when a skip-on-absent guard lands. Prompts still recoverable from git history.
- **`e2e-install-test.sh`** — Assumes the old multi-plugin marketplace layout (`agentops/` subdir, per-plugin args). Current repo is a single skills source of truth; this test has no referent.
- **`marketplace-e2e-test.sh`** — Same reason: tests a plugin marketplace model that no longer exists. Replaced in spirit by `tests/install/`, `tests/skills/`, and `tests/spec-consistency/`.

## Running manually

```bash
bash tests/_quarantine/claude-code/run-all.sh   # requires claude CLI
bash tests/_quarantine/codex/run-all.sh         # skips if codex CLI missing
bash tests/_quarantine/team-runner/run-all.sh   # broken path, see Plan D
bash tests/ol-integration/vibe-ol-test.sh       # promoted, fixture-only
bash tests/ol-integration/swarm-ol-test.sh      # promoted, fixture-only
```

## Promotion plans

### Plan A — `codex/` (lowest friction)

1. `git mv tests/_quarantine/codex tests/codex`
2. Add a job to `.github/workflows/validate.yml` that runs `bash tests/codex/run-all.sh`
   on a matrix entry that installs Codex (or makes the job `continue-on-error: true`).
   Tests already skip cleanly when `codex` is absent.
3. Add a short section to `tests/codex/README.md` documenting env vars
   (`CODEX_MODEL`, default `gpt-5.3-codex`).

**Cost:** ~30 min. No code changes, CLI-skip behavior already correct.

### Plan B — `claude-code/`

1. Wrap `run-all.sh` pre-flight so missing `claude` CLI exits 0 with SKIPPED
   (currently exits 1).
2. `git mv tests/_quarantine/claude-code tests/claude-code`
3. Optional job in validate.yml. Tests are budget-capped (`MAX_BUDGET_USD=1.00`)
   and turn-capped (`MAX_TURNS=3`), so a nightly-only cadence is safest.
4. Recreate `skill-triggering/prompts/` inside
   `tests/claude-code/prompts/natural-language/` (recover from git history).

**Cost:** ~2 hours. Needs a runner that has `claude` available; document cost caps.

### Plan C — `ol-integration/` (done)

Promoted by na-gtm.18 on 2026-04-12:
`tests/_quarantine/ol-integration` moved to `tests/ol-integration`, where the
existing `"$SCRIPT_DIR/../.."` root calculation resolves correctly. No external
`ol` binary is required; the fixture-only scripts now run in the default
`tests/run-all.sh` lane.

**Cost:** ~15 min. Lowest-risk promotion.

### Plan D — `team-runner/`

1. Fix `REPO_ROOT` path in `run-all.sh` and `test-schemas.sh`
   (`../../../` → `../../` after move).
2. Verify fixtures in `team-runner/fixtures/` still match current schemas.
3. `git mv tests/_quarantine/team-runner tests/team-runner`
4. Wire `test-schemas.sh` and `test-runner-dry-run.sh` into the default lane.
   The `test-watch-*-stream.sh` tests may need Claude/gc; gate them behind
   runtime-present checks.

**Cost:** ~1 hour. Schemas and scripts still live, so ROI is good.

## Follow-up issues

- **na-gtm.17** — Promote `codex/` (Plan A)
- **na-gtm.18** — CLOSED: promoted `ol-integration/` (Plan C)
- **na-gtm.19** — Promote `team-runner/` (Plan D)
- **na-gtm.20** — Promote `claude-code/` with skip-on-absent guard + optional CI job (Plan B)
