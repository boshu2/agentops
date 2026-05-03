#!/usr/bin/env bash
# Verify the ao binary in PATH matches the local build version.
# Run before UAT to catch stale-binary false failures.
set -euo pipefail

LOCAL_VERSION=$(git describe --tags --match 'v[0-9]*.[0-9]*.[0-9]*' --always --dirty 2>/dev/null || echo "unknown")
AO_PATH=$(command -v ao 2>/dev/null || true)

if [[ -z "$AO_PATH" ]]; then
    echo "FAIL: ao not found in PATH"
    echo "Fix: cd cli && make build && cp bin/ao /usr/local/bin/ao"
    exit 1
fi

PATH_VERSION=$(ao version 2>&1 | awk '/ao version/{print $3}' || echo "unknown")

if [[ "$LOCAL_VERSION" == "unknown" || "$PATH_VERSION" == "unknown" ]]; then
    echo "WARN: Could not determine versions (local=$LOCAL_VERSION, path=$PATH_VERSION)"
    echo "Skipping version match check."
    exit 0
fi

if [[ "$LOCAL_VERSION" != "$PATH_VERSION" ]]; then
    echo "FAIL: PATH ao is $PATH_VERSION but local build is $LOCAL_VERSION"
    echo "Fix: cd cli && make build && cp bin/ao $AO_PATH"
    exit 1
fi

echo "PASS: ao binary matches local build ($PATH_VERSION)"
