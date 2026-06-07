#!/usr/bin/env bash
# Validate cass installation and basic functionality

set -euo pipefail

echo "=== cass Session Search Validation ==="
echo

# Check cass is installed
if ! command -v cass &> /dev/null; then
    echo "ERROR: cass is not installed or not in PATH"
    echo "FIX: Install cass from https://github.com/your/cass"
    exit 2
fi

if ! command -v jq &> /dev/null; then
    echo "ERROR: jq is not installed or not in PATH"
    echo "FIX: Install jq to parse cass JSON output"
    exit 2
fi

echo "✓ cass found: $(command -v cass)"

# Check cass status
echo
echo "Checking cass status..."
if ! STATUS=$(cass status --robot-format json 2>/dev/null); then
    echo "ERROR: cass status failed"
    echo "FIX: Run 'cass doctor' to repair"
    exit 2
fi

# Check if index is fresh
if ! FRESH=$(jq -r '.index.fresh // false' <<< "$STATUS" 2>/dev/null); then
    echo "ERROR: cass status returned invalid JSON"
    exit 2
fi
REBUILDING=$(jq -r '.index.rebuilding // .rebuild.active // false' <<< "$STATUS")
if [ "$REBUILDING" = "true" ]; then
    echo "WARNING: Index rebuild is in progress"
    echo "FIX: Wait for the rebuild to finish, then rerun validation"
elif [ "$FRESH" = "true" ]; then
    echo "✓ Index is fresh"
else
    echo "WARNING: Index is stale"
    echo "FIX: Run 'cass index --json'"
fi

# Check conversation count
CONVOS=$(jq -r '.database.conversations // 0' <<< "$STATUS")
echo "✓ Indexed conversations: $CONVOS"

if [ "$REBUILDING" != "true" ] && [ "$CONVOS" -eq 0 ]; then
    echo "WARNING: No conversations indexed"
    echo "FIX: Run 'cass index --full --json' to rebuild index"
fi

# Test basic search (should not panic)
echo
echo "Testing basic search..."
if cass search "*" --json --limit 1 --fields minimal > /dev/null 2>&1; then
    echo "✓ Basic search works"
else
    echo "ERROR: Basic search failed"
    exit 2
fi

# Test aggregation (the most common pitfall)
echo
echo "Testing aggregation..."
if cass search "*" --json --aggregate agent --limit 1 --fields minimal > /dev/null 2>&1; then
    echo "✓ Aggregation works"
else
    echo "ERROR: Aggregation failed"
    exit 2
fi

echo
echo "=== Validation Complete ==="
echo "cass is ready to use"
exit 0
