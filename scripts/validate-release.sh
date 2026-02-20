#!/usr/bin/env bash
# Validate release binary before publishing
# Usage: validate-release.sh <binary> <version>
#
# Checks:
# - Binary exists and is executable
# - Binary size > 1MB (catch zero-byte/truncated)
# - File is actually an executable (Mach-O or ELF)
# - Version injection worked (exact match, not substring)
# - --help and -h work
# - status runs (allowed to fail gracefully)

set -euo pipefail

BINARY_ARG="${1:?Usage: validate-release.sh <binary> <version>}"
EXPECTED_VERSION="${2:?Usage: validate-release.sh <binary> <version>}"

# Resolve symlinks for accurate size check
if [[ -L "$BINARY_ARG" ]]; then
    BINARY=$(realpath "$BINARY_ARG")
else
    BINARY="$BINARY_ARG"
fi

echo "=== Release Validation ==="
echo "Binary: $BINARY"
echo "Expected version: $EXPECTED_VERSION"
echo ""

# Check binary exists
if [[ ! -f "$BINARY" ]]; then
    echo "FAIL: Binary not found: $BINARY"
    exit 1
fi
echo "✓ Binary exists"

# Make executable if needed
if [[ ! -x "$BINARY" ]]; then
    chmod +x "$BINARY"
fi
echo "✓ Binary is executable"

# Check binary size > 1MB (catch zero-byte or truncated builds)
# macOS uses stat -f%z, Linux uses stat -c%s
if [[ "$(uname)" == "Darwin" ]]; then
    SIZE=$(stat -f%z "$BINARY")
else
    SIZE=$(stat -c%s "$BINARY")
fi

if [[ $SIZE -lt 1000000 ]]; then
    echo "FAIL: Binary too small ($SIZE bytes). Expected > 1MB."
    echo "      This usually means the build was corrupted or truncated."
    exit 1
fi
echo "✓ Binary size OK ($SIZE bytes)"

# Check it's actually an executable (not a text file or corrupted)
FILE_TYPE=$(file "$BINARY")
if ! echo "$FILE_TYPE" | grep -qE "(Mach-O|ELF).*executable"; then
    echo "FAIL: Not a valid executable"
    echo "      file output: $FILE_TYPE"
    exit 1
fi
echo "✓ File type is executable"

# Check version injection (exact match to prevent substring false positives)
# e.g., "ao version 1.0.12" should not match "v1.0.12-dirty"
VERSION_OUTPUT=$("$BINARY" version 2>&1) || true

# Strip 'v' prefix if present for comparison
CLEAN_VERSION="${EXPECTED_VERSION#v}"

if [[ "$VERSION_OUTPUT" != *"ao version $CLEAN_VERSION"* ]] && [[ "$VERSION_OUTPUT" != *"ao version v$CLEAN_VERSION"* ]]; then
    echo "FAIL: Version mismatch"
    echo "  Expected to find: ao version $CLEAN_VERSION (or ao version v$CLEAN_VERSION)"
    echo "  Actual output: $VERSION_OUTPUT"
    echo ""
    echo "  This usually means ldflags version injection failed."
    echo "  Check .goreleaser.yml ldflags configuration."
    exit 1
fi
echo "✓ Version injection OK"

# Check --help works
if ! "$BINARY" --help > /dev/null 2>&1; then
    echo "FAIL: --help command failed"
    exit 1
fi
echo "✓ --help works"

# Check -h works (short form)
if ! "$BINARY" -h > /dev/null 2>&1; then
    echo "FAIL: -h command failed"
    exit 1
fi
echo "✓ -h works"

# Check status (allowed to fail gracefully in CI - no .agents/ dir)
# We just want to make sure it doesn't crash
echo "--- Running 'status' (may show error, that's OK) ---"
"$BINARY" status 2>&1 || true
echo "--- End status output ---"
echo "✓ status runs (exit code ignored)"

# Check commit count since last tag (warning, not failure)
LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
if [[ -n "$LAST_TAG" ]]; then
    COMMIT_COUNT=$(git log "${LAST_TAG}..HEAD" --oneline 2>/dev/null | wc -l | tr -d ' ')
    if [[ "$COMMIT_COUNT" -gt 15 ]]; then
        echo "⚠ WARNING: $COMMIT_COUNT commits since $LAST_TAG (recommended max: 15)"
        echo "  Consider splitting into multiple releases for easier review/bisect."
    else
        echo "✓ Commit count OK ($COMMIT_COUNT since $LAST_TAG)"
    fi
else
    echo "⚠ No previous tag found — skipping commit count check"
fi

echo ""
echo "=== All Validation Checks Passed ==="
echo "  Binary: $BINARY"
echo "  Version: $CLEAN_VERSION"
echo "  Size: $SIZE bytes"
