#!/usr/bin/env bash
set -euo pipefail

VERSION="${GOLANGCI_LINT_VERSION:-v2.11.4}"
DISPLAY_VERSION="${VERSION#v}"
MODULE="github.com/golangci/golangci-lint/v2/cmd/golangci-lint"

if [[ -n "${GOLANGCI_LINT_BIN:-}" ]]; then
  exec "$GOLANGCI_LINT_BIN" "$@"
fi

if command -v golangci-lint >/dev/null 2>&1 && golangci-lint version 2>/dev/null | grep -Eq "version v?${DISPLAY_VERSION}([ ,]|$)"; then
  exec golangci-lint "$@"
fi

if ! command -v go >/dev/null 2>&1; then
  echo "golangci-lint ${VERSION} is required and go is not installed to bootstrap it" >&2
  exit 127
fi

# Build golangci-lint with the REPO's pinned toolchain, not golangci-lint's
# minimum. `go install pkg@version` runs module-agnostic, so it ignores this
# repo's `toolchain` directive and, under an old default `go`, downgrades to
# golangci-lint's own `go >= 1.25` minimum (go1.25.12). A golangci-lint built
# with go1.25 then refuses to analyze this `go 1.26` module ("Go language
# version go1.25 ... lower than targeted 1.26.5") — the CI go.lint gate failure.
# Pinning GOTOOLCHAIN to cli/go.mod's `toolchain` (else the `go` directive)
# builds the linter with a toolchain >= the code it must lint. It matches the
# toolchain `go build ./cli` already uses, so it adds no burden in practice.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOMOD_FILE="${GOLANGCI_LINT_GOMOD:-${SCRIPT_DIR}/../cli/go.mod}"
if [[ -z "${GOTOOLCHAIN:-}" && -f "$GOMOD_FILE" ]]; then
  repo_toolchain="$(awk '/^toolchain /{print $2; exit}' "$GOMOD_FILE" 2>/dev/null || true)"
  if [[ -z "$repo_toolchain" ]]; then
    repo_go="$(awk '/^go /{print $2; exit}' "$GOMOD_FILE" 2>/dev/null || true)"
    case "$repo_go" in
      [0-9]*.[0-9]*.[0-9]*) repo_toolchain="go${repo_go}" ;;
      [0-9]*.[0-9]*)        repo_toolchain="go${repo_go}.0" ;;
    esac
  fi
  [[ -n "$repo_toolchain" ]] && export GOTOOLCHAIN="$repo_toolchain"
fi

cache_root="${GOLANGCI_LINT_CACHE_BIN:-${XDG_CACHE_HOME:-$HOME/.cache}/agentops/golangci-lint}"
bin_dir="${cache_root}/${VERSION}"
bin="${bin_dir}/golangci-lint"

if [[ ! -x "$bin" ]]; then
  mkdir -p "$bin_dir"
  GOBIN="$bin_dir" go install "${MODULE}@${VERSION}"
fi

exec "$bin" "$@"
