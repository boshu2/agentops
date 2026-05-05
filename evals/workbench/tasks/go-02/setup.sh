#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GOLDEN="$(cd "$SCRIPT_DIR/../../go-cli" && pwd)"

mkdir -p "$WORKDIR"
cp -r "$GOLDEN/." "$WORKDIR/"

# Remove TestDelete, TestDeleteNotFound, and TestGetNotFound from store_test.go
# Use awk to remove entire test functions by name
awk '
  /^func TestDelete\(/ { skip=1 }
  /^func TestDeleteNotFound\(/ { skip=1 }
  /^func TestGetNotFound\(/ { skip=1 }
  skip && /^}$/ { skip=0; next }
  !skip { print }
' "$WORKDIR/internal/store/store_test.go" > "$WORKDIR/internal/store/store_test.go.tmp"
mv "$WORKDIR/internal/store/store_test.go.tmp" "$WORKDIR/internal/store/store_test.go"

# Remove unused "errors" import (was only used by the removed test functions)
sed -i '/"errors"/d' "$WORKDIR/internal/store/store_test.go"
