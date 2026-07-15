#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/validate-local.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

! rg -n 'install-dev-hooks|core\.hooksPath|claude|lock|retry|verdict' "$SCRIPT"

mkdir -p "$TMP_DIR/repo/scripts" "$TMP_DIR/repo/cli/bin"
cp "$SCRIPT" "$TMP_DIR/repo/scripts/validate-local.sh"
cat > "$TMP_DIR/repo/cli/bin/ao" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" > "${AO_CAPTURE:?}"
EOF
chmod +x "$TMP_DIR/repo/scripts/validate-local.sh" "$TMP_DIR/repo/cli/bin/ao"

AO_CAPTURE="$TMP_DIR/args" bash "$TMP_DIR/repo/scripts/validate-local.sh" --scope staged --full
grep -qx 'gate check --full --scope staged' "$TMP_DIR/args"

echo "PASS: validate-local is a thin deterministic gate wrapper"
