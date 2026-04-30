# Release Process

> **Status:** Ported from olympus 2026-04-29. Commands/paths adapted for agentops. Where olympus terms have no agentops equivalent, TODO callouts mark the gap.

How to cut a release of `ao` (the AgentOps CLI).

> See also: [`RELEASING.md`](../RELEASING.md) for the canonical agentops release doc and [`release-e2e-checklist.md`](../release-e2e-checklist.md) for the full local gate sequence. This runbook is the runbook-shaped port from the olympus equivalent and may overlap with both.

## Prerequisites

- [goreleaser](https://goreleaser.com/) (`brew install goreleaser`)
- GitHub token with `repo` scope (set `GITHUB_TOKEN` env var or use `gh auth token`)
- Go toolchain matching `cli/go.mod`

## Pre-flight Checks

All core gates must pass before tagging:

```bash
# Smart, diff-aware gate (recommended)
scripts/pre-push-gate.sh --fast

# Full local release validation gate (mandatory before tagging)
scripts/ci-local-release.sh

# Direct Go build/vet/test sweep
cd cli && go build ./... && go vet ./... && go test ./... -count=1

# GOALS.md / GOALS.yaml — verify all goals are passing
ao goals measure
ao goals validate
```

> **TODO: olympus `bash scripts/run-commit-lane.sh` equivalent in agentops?**
> The closest analog is `scripts/pre-push-gate.sh` (smart) or `scripts/ci-local-release.sh` (full). Confirm the agentops "commit lane" surface lives in `scripts/` and update if a dedicated commit-lane script ships later.

### Goal Gate Pack (Timeout-Prone Gates)

Run this bundle explicitly before release to validate the stabilized gate path:

```bash
timeout 300 bash -c 'cd cli && go test ./... -count=1'
```

> **TODO: olympus `scripts/check-coverage-floors.sh` / `scripts/check-checkpoint-fidelity.sh` / `scripts/smoke-test.sh` equivalents in agentops?**
> agentops has `scripts/pre-push-gate.sh` (33 checks) and `scripts/ci-local-release.sh`, but no 1:1 named scripts for coverage-floor ratcheting, checkpoint fidelity, or a single smoke harness. Re-point this section once the analogs are identified (or confirm the pre-push gate covers them).

Gate semantics (preserved from olympus shape; map to the agentops equivalents above):

- `go-test`: hard gate on full test suite within a 300s budget.
- `coverage-floors`: ratchet enforcement for package coverage floors.
- `checkpoint-fidelity`: minimum RPI / quest-equivalent test-count authority floor.
- `smoke-test`: CRUD + RPI step/run + CLI help in deterministic mode.

### Dogfood Hardening Pack

> **TODO: olympus `make dogfood` / `make dogfood-quick` equivalents in agentops?**
> agentops does not currently ship a `dogfood` make target. The conceptual equivalent — running `ao` against this repo's own `.agents/` and validating end-to-end — is partially covered by `scripts/ci-local-release.sh` plus `ao rpi phased` smoke runs. Add a dedicated `make dogfood` / `make dogfood-quick` target and re-point this section if/when it lands.

Smoke test determinism notes (carried over for shape):

- The smoke harness should build a local `ao` binary from repo source (`cd cli && make build`).
- The harness should install a temporary mock LLM/runner binary in script-local `PATH` to avoid host-environment model dependencies during gate execution.
- Each smoke command should be bounded by a per-command timeout (mirror olympus's `SMOKE_CMD_TIMEOUT_SECONDS` default of `45`).

## Version Bump

1. The version constant lives in the `ao` root command file.

   > **TODO: olympus `cmd/ol/root.go` exact equivalent in agentops?**
   > agentops's CLI entrypoint is `cli/cmd/ao/`. Confirm the file (e.g., `cli/cmd/ao/root.go` or `cli/cmd/ao/version.go`) that holds the goreleaser-overridable `version` constant before tagging.

   The actual binary version is injected by goreleaser from the git tag (see
   `-X main.version={{.Version}}` in `.goreleaser.yml`), so no source change is
   needed unless you want `go install` builds to carry the version.

2. Tag the release:

```bash
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

   For agentops's GoReleaser + GitHub Actions pipeline, the tag push triggers the release workflow automatically. Use `scripts/retag-release.sh vX.Y.Z` to retag if needed.

## Build

Run goreleaser from the repo root:

```bash
goreleaser release --clean
```

This will:

- Run `go mod tidy` and `go vet ./cli/cmd/ao/` (before-hooks; verify exact paths in `.goreleaser.yml`)
- Cross-compile for darwin/linux on amd64/arm64 (CGO disabled)
- Produce `ao_X.Y.Z_{os}_{arch}.tar.gz` archives
- Generate `checksums.txt`
- Create a GitHub release with an auto-generated changelog

For a dry run (no publish):

```bash
goreleaser release --clean --snapshot --skip=publish
```

## GitHub Release

Goreleaser creates the draft automatically. After it finishes:

1. Open the release on GitHub
2. Review the auto-generated changelog (commits sorted asc; `doc:`, `chore:`, `ci:` excluded)
3. Add a summary section at the top if the release has notable changes
4. Publish the release

## Post-release

1. Verify binary downloads:

```bash
# macOS ARM
curl -sL https://github.com/boshu2/agentops/releases/download/vX.Y.Z/ao_X.Y.Z_darwin_arm64.tar.gz | tar xz
./ao version
```

2. Update install docs if paths or supported platforms changed.
3. Close the release bead/epic in beads (`bd close <id>`).

## Quick Reference

| Target | Command |
|--------|---------|
| Local build | `cd cli && make build` (or `cd cli && go build -o bin/ao ./cmd/ao/`) |
| Local install | `cd cli && make install` (installs to `~/go/bin/ao`) |
| Tests | `cd cli && make test` (or `cd cli && go test ./... -count=1`) |
| Smart pre-push gate | `scripts/pre-push-gate.sh --fast` |
| Full pre-push gate | `scripts/pre-push-gate.sh` |
| Local release gate | `scripts/ci-local-release.sh` |
| Dogfood quick | _TODO: agentops equivalent_ |
| Dogfood full | _TODO: agentops equivalent_ |
| Throughput gate | _TODO: agentops equivalent_ |
| Release (full) | `goreleaser release --clean` |
| Release (dry run) | `goreleaser release --clean --snapshot --skip=publish` |
| Retag | `scripts/retag-release.sh vX.Y.Z` |

## Adaptation map (olympus → agentops)

| Olympus | agentops |
|---|---|
| `ol` binary | `ao` binary |
| `cmd/ol/` | `cli/cmd/ao/` |
| `make ol-build` / `make ol-install` / `make ol-test` | `cd cli && make build` / `make install` / `make test` |
| `scripts/run-commit-lane.sh` | `scripts/pre-push-gate.sh` (smart, diff-aware) — see TODO above |
| `make dogfood` / `make dogfood-quick` | _TODO: not yet ported_ |
| `scripts/smoke-test.sh` | _TODO: closest is parts of `scripts/ci-local-release.sh`_ |
| Release artifact prefix `ol_X.Y.Z_*` | `ao_X.Y.Z_*` |
| Release repo path `boshu2/olympus` | `boshu2/agentops` |
