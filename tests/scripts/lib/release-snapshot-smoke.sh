#!/usr/bin/env bash
# release-snapshot-smoke.sh — exercise the goreleaser RELEASE PATH between releases.
#
# WHY: the release path is only executed by a real `v*` tag push. A break in it
# is therefore invisible until the next release. That is not hypothetical: a
# retired `before.hooks` entry in .goreleaser.yml sat broken for 20 days and was
# only found when the tag was pushed (docs/audits/release-readiness-v3.3.0.md,
# "Process lesson"). This script runs the same GoReleaser pipeline the publisher
# runs — config load, before-hooks, cross-platform builds, archives, checksums,
# Homebrew formula render — with publishing skipped, so CI can prove the path
# still works without cutting a release.
#
# WHY NOT `goreleaser check`: `check` exits non-zero on DEPRECATION warnings
# alone (the current config's `brews:` block), so it cannot serve as the
# between-releases gate without changing release behavior. It also never runs
# hooks, builds, archives, or the formula template — precisely the stages that
# broke. The snapshot is the real path; use it.
#
# Usage:
#   bash tests/scripts/lib/release-snapshot-smoke.sh [--config PATH] [--timeout DUR]
#
# Options:
#   --config PATH   GoReleaser config to exercise (default: <repo>/.goreleaser.yml)
#   --timeout DUR   GoReleaser --timeout value (default: 20m)
#   --help          Print this usage and exit 0
#
# Exit codes:
#   0 - the release path built end-to-end (publishing skipped)
#   1 - the release path is BROKEN (bad config, failing hook, build/archive error)
#   2 - usage error, or goreleaser is not installed (cannot judge)
#
# Must run from the repository root: the config's build `dir: cli` and archive
# `files:` entries are resolved relative to the working directory, not relative
# to the config file.

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../../.." && pwd)"

CONFIG="$REPO_ROOT/.goreleaser.yml"
TIMEOUT="20m"

usage() {
    sed -n '2,36p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --config)
            [ "$#" -ge 2 ] || { echo "ERROR: --config requires a path" >&2; exit 2; }
            CONFIG="$2"
            shift 2
            ;;
        --config=*)
            CONFIG="${1#--config=}"
            shift
            ;;
        --timeout)
            [ "$#" -ge 2 ] || { echo "ERROR: --timeout requires a duration" >&2; exit 2; }
            TIMEOUT="$2"
            shift 2
            ;;
        --timeout=*)
            TIMEOUT="${1#--timeout=}"
            shift
            ;;
        --help | -h)
            usage
            exit 0
            ;;
        *)
            echo "ERROR: unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if [ ! -f "$CONFIG" ]; then
    echo "ERROR: config not found: $CONFIG" >&2
    exit 2
fi

if ! command -v goreleaser > /dev/null 2>&1; then
    echo "ERROR: goreleaser is not installed; cannot exercise the release path" >&2
    exit 2
fi

cd "$REPO_ROOT"

echo "== release-path smoke: goreleaser snapshot =="
echo "config:  $CONFIG"
echo "version: $(goreleaser --version 2>/dev/null | tr -d '\r' | grep -i 'GitVersion' || echo 'unknown')"

# --snapshot: no tag required, so this runs on any commit.
# --clean:    start from an empty dist/, exactly like the publisher.
# --skip=publish: never touch GitHub releases or the Homebrew tap. The formula
#                 is still RENDERED to dist/, so a broken brews template is
#                 still caught here.
set +e
goreleaser release \
    --snapshot \
    --clean \
    --skip=publish \
    --config "$CONFIG" \
    --timeout "$TIMEOUT"
rc=$?
set -e

if [ "$rc" -ne 0 ]; then
    echo "FAIL  release path is broken (goreleaser exited $rc)" >&2
    echo "      A real 'v*' tag push would fail the same way." >&2
    exit 1
fi

echo "OK    release path built end-to-end (publish skipped)"
exit 0
