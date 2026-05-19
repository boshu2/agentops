# End-to-End Tests

The scripts in this directory are end-to-end tests, not unit tests with delusions
of grandeur. They:

- build (or reuse) a real `ao` binary
- create a real, isolated git repo under `mktemp -d`
- exercise the real CLI across multiple phases
- assert on real artifacts (files on disk, JSON output, citation logs)
- **mock nothing** that the system under test depends on

This document captures the contract every e2e script must satisfy, the mock-risk
scoring used to triage candidates, and the shared harness those scripts share.

> Source skill: `testing-real-service-e2e-no-mocks` — adapted from the
> SaaS-billing original to the CLI/CI domain.

## Running

The flagship suite is the flywheel proof, which exercises forge → pool-ingest →
cite → promote → lookup → feedback → nightly across one isolated sandbox:

```bash
bash tests/e2e/proof-run.sh
```

Other scripts (`goals-*.sh`, `rpi-phased-domain.sh`, `closure-integrity-grace.sh`,
…) follow the same harness contract and can be run the same way.

---

## The contract

Every script in `tests/e2e/` MUST:

1. **Source the shared harness.**
   ```bash
   source "$REPO_ROOT/tests/lib/e2e-guards.sh"   # production safety guards
   source "$REPO_ROOT/tests/lib/e2e-factory.sh"  # sandbox + repo + ao binary
   source "$REPO_ROOT/tests/lib/e2e-logger.sh"   # JSON-line sidecar (optional)
   ```

2. **Stand up an isolated sandbox.** Create the work dir via
   `e2e_factory_sandbox <slug>` (never `mktemp -d` by hand), the repo via
   `e2e_factory_repo`, and the binary via `e2e_factory_ao_bin`. Set
   `HOME="$WORK_DIR/home"` so any `~/.agents`-style writes land inside the
   sandbox.

3. **Call the guards.** Each is fast (no I/O after the path check) and refuses
   to run if `$HOME`, the repo dir, or the `ao` binary look real:
   ```bash
   e2e_guard_home    "$HOME_DIR"
   e2e_guard_repo    "$REPO_DIR"
   e2e_guard_ao_bin  "$AO_BIN"
   e2e_guard_not_repo_root   # only if your test runs ao relative to PWD
   ```

4. **Clean up via `trap`.** Always wipe the sandbox on exit. `chmod -R u+w`
   first so a test that drops a read-only file can't pin the directory.

5. **Assert on real artifacts.** No mocked return values. Read the file the
   command produced, parse the JSON the command emitted, walk the directory
   the command wrote into.

A script that mocks the `ao` binary, fakes `git`, or substitutes an in-memory
store for the on-disk one is by definition not an e2e test — it belongs under
`cli/internal/<pkg>/*_test.go`.

---

## Mock Risk Assessment Matrix

The skill scores `Impact × Risk` and demands mock-free for any score ≥ 8.
The agentops snapshot (audited 2026-05-18):

| Path | Impact | Mock Risk | Score | Status |
|------|:------:|:---------:|:-----:|--------|
| Forge transcript → pending learnings | 5 | 2 | 10 | ✅ mock-free (`proof-run.sh` Phase 1) |
| Pool ingest → candidates | 5 | 2 | 10 | ✅ mock-free (`proof-run.sh` Phase 2) |
| Cite → close-loop promotion | 5 | 3 | 15 | ✅ mock-free (`proof-run.sh` Phase 3) |
| Lookup / retrieval | 5 | 2 | 10 | ✅ mock-free (`proof-run.sh` Phase 4) |
| Feedback rewarding | 5 | 2 | 10 | ✅ mock-free (`proof-run.sh` Phase 5) |
| Nightly dream cycle | 4 | 2 | 8 | ✅ mock-free (`proof-run.sh` Phase 6) |
| Goals scenarios link + lint | 4 | 2 | 8 | ✅ mock-free (`goals-scenarios-link.sh`) |
| RPI phased domain dispatch | 4 | 3 | 12 | ✅ mock-free (`rpi-phased-domain.sh`) |
| `install.sh` curl-pipe | 5 | 3 | 15 | ✅ mock-free (`.github/workflows/install-e2e.yml`) |
| openclaw daemon API | 2 | 2 | 4 | ⚠️ httptest fixture — acceptable, internal-only |
| Claude CLI skill invocation | 3 | 4 | 12 | ⚠️ real Claude, non-deterministic — acceptable as advisory |

The "⚠️" rows are intentional. The openclaw daemon's `httptest.NewServer` wires
up the *real* `daemon.NewDaemonRouter` with a temp-dir backing store —
test-local instantiation, not a stub of the system under test (score 4 is below
the 8 threshold). The Claude CLI tests use the real Claude binary; the
non-determinism is the cost of testing prompt-bound behavior end-to-end.

### What does NOT belong on this matrix

Cross-test pollution at the unit layer (Cobra flag globals, `os.Stdout`
redirection, `os.Chdir` cleanup) is a different problem class — isolation, not
faking. Track those in beads, not here.

---

## Shared harness

| File | Purpose | Public API |
|------|---------|------------|
| `tests/lib/e2e-guards.sh` | Refuse-to-run safety checks | `e2e_guard_home`, `e2e_guard_repo`, `e2e_guard_ao_bin`, `e2e_guard_not_repo_root` |
| `tests/lib/e2e-factory.sh` | Sandbox + repo + binary builders | `e2e_factory_sandbox`, `e2e_factory_repo`, `e2e_factory_ao_bin`, `e2e_factory_agents_dir`, `e2e_factory_fixture` |
| `tests/lib/e2e-logger.sh` | JSON-line CI-parseable sidecar | `e2e_log_init`, `e2e_log_phase`, `e2e_log_pass`, `e2e_log_fail`, `e2e_log_assert`, `e2e_log_artifact`, `e2e_log_summary` |

The logger writes one JSON object per line. Schema (fields are stable;
consumers can rely on them):

```json
{"ts":"2026-05-19T00:37:24Z","suite":"flywheel-proof","phase":"cite-promote",
 "event":"pass","message":"close-loop promoted a cited candidate","data":null}
```

`event` is one of: `suite_start`, `phase_start`, `pass`, `fail`, `assert`,
`artifact`, `db_snapshot`, `suite_end`. The `data` field is event-specific
(asserts have `{expected, actual, match}`, artifacts have `{path, size, mtime}`,
the closing `suite_end` has `{pass, fail, duration_s}`).

### Escape hatch

`AGENTOPS_E2E_ALLOW_UNSAFE=1` bypasses every guard. It exists for the rare
maintainer debugging the harness itself. **Never set it in CI.** Every bypass
prints a `[e2e-guard] WARNING:` line to stderr.

---

## Adding a new e2e script

1. Pick a slug. `e2e_factory_sandbox <slug>` puts it in the dirname for grep.
2. Source all three libs from `tests/lib/`.
3. Call `e2e_factory_sandbox` → `e2e_factory_repo` → `e2e_factory_ao_bin`.
4. Call the three guards (`home`, `repo`, `ao_bin`). Add `not_repo_root` if you
   chdir into `$WORK` and run `ao` with relative paths.
5. `e2e_log_init` if you want the JSON-line sidecar (recommended).
6. Trap cleanup. `chmod -R u+w "$WORK"` before `rm -rf`.
7. Wire into `.github/workflows/validate.yml` if the path is on the score-≥-8
   list above.

---

## Currently mock-free e2e scripts

| Script | Phases | Risk paths exercised |
|--------|--------|----------------------|
| `proof-run.sh` | forge → pool-ingest → cite-promote → lookup → feedback → nightly | 5 of the top-10 highest-risk paths in one suite |
| `goals-scenarios-link.sh` | create → verify-bidirectional → lint-clean → break → lint-fails | F1 of the goals epic |
| `goals-measure-scenarios.sh` | measure → assert satisfaction | F2 |
| `rpi-phased-domain.sh` | dispatch → phase trace | F3 |
| `goals-trace-chain.sh` | trace → dependency assert | F4 |
| `goals-steer-auto.sh` | steer → re-prioritize | F5 |
| `closure-integrity-grace.sh` | citation → grace period → closure invariant | citation flow regressions |
| `factory-operator-canary.sh` | factory admission → operator action | factory pipeline contract |

Every script in this list is mock-free **today** — this file is the contract
that keeps it that way.
