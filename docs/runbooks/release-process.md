# Release Process

How to cut a release of `ao` (the AgentOps CLI).

> See also: [`RELEASING.md`](../RELEASING.md) for the canonical AgentOps release doc and [`release-e2e-checklist.md`](../release-e2e-checklist.md) for the full local gate sequence. This runbook is the operator checklist view of those release contracts.

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

### Goal Gate Pack (Timeout-Prone Gates)

Run this bundle explicitly before release to validate the stabilized gate path:

```bash
timeout 300 bash -c 'cd cli && go test ./... -count=1'
```

AgentOps release gate semantics:

- `go-test`: hard gate on full test suite within a 300s budget.
- `ci-local-release`: full local release validation, including doc, shell, contract, CLI, and release-surface checks.
- `pre-push-fast`: smart changed-file gate for PR iteration before the full release gate.
- `release-e2e`: optional HIL/SIL/VIL release smoke path from `docs/release-e2e-checklist.md`.

### Dogfood Hardening Pack

AgentOps dogfood means running `ao` against this repo's own `.agents/` state and release contracts:

- Build a local `ao` binary from repo source (`cd cli && make build`).
- Run `scripts/ci-local-release.sh` before tagging.
- For explicit release smoke coverage, use `bash scripts/ci-local-release.sh --fast --jobs 4` and the HIL/SIL/VIL targets in `docs/release-e2e-checklist.md`.
- Bound any manual smoke command with `timeout` so model or daemon dependencies cannot hang a release shell.

## Version Bump

1. The version constant lives in the `ao` root command file.

   The default `version` variable lives in `cli/cmd/ao/main.go`; root command
   wiring lives in `cli/cmd/ao/root.go`, and `ao version` output lives in
   `cli/cmd/ao/version.go`. The actual binary version is injected by goreleaser from the git tag (see
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

2. Update install docs if paths or platform support changed.
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
| Dogfood quick | `bash scripts/ci-local-release.sh --fast --jobs 4` |
| Dogfood full | `scripts/ci-local-release.sh` |
| Throughput gate | `docs/release-e2e-checklist.md` HIL/SIL/VIL smoke path |
| Release (full) | `goreleaser release --clean` |
| Release (dry run) | `goreleaser release --clean --snapshot --skip=publish` |
| Retag | `scripts/retag-release.sh vX.Y.Z` |

## AgentOps Release Surface

| Surface | Current AgentOps path |
|---|---|
| Binary | `ao` |
| CLI source | `cli/cmd/ao/` |
| Build/test targets | `cd cli && make build` / `make install` / `make test` |
| Smart commit gate | `scripts/pre-push-gate.sh --fast` |
| Full release gate | `scripts/ci-local-release.sh` |
| Release artifact prefix | `ao_X.Y.Z_*` |
| Release repo path | `boshu2/agentops` |
