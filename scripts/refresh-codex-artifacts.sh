#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

SCOPE="worktree"

usage() {
  cat <<'EOF'
refresh-codex-artifacts.sh

One obvious repair/verification flow for Codex skill prompt drift and generated
artifact drift.

Usage:
  bash scripts/refresh-codex-artifacts.sh [--scope auto|upstream|staged|worktree|head]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --scope)
      SCOPE="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown arg: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

case "$SCOPE" in
  auto|upstream|staged|worktree|head) ;;
  *)
    echo "Invalid --scope: $SCOPE" >&2
    exit 2
    ;;
esac

echo "== Codex artifact maintenance flow =="
echo "Repo:  $REPO_ROOT"
echo "Scope: $SCOPE"

bash scripts/regen-codex-hashes.sh
bash scripts/validate-codex-override-coverage.sh
bash scripts/validate-codex-generated-artifacts.sh --scope "$SCOPE"
bash scripts/audit-codex-parity.sh

echo "Codex artifact maintenance flow passed."
