#!/usr/bin/env bash
# Build the MkDocs Material site with strict link checking.
# Usage:
#   scripts/docs-build.sh              # strict build to _site/
#   scripts/docs-build.sh --serve      # local dev server
#   scripts/docs-build.sh --check      # strict build (exit 1 on warnings), no output kept

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

VENV_DIR=".venv-docs"

ensure_venv() {
    if [[ ! -d "$VENV_DIR" ]]; then
        echo "==> Creating MkDocs venv ($VENV_DIR)"
        if command -v uv >/dev/null 2>&1; then
            uv venv "$VENV_DIR" --python 3.12
        else
            python3 -m venv "$VENV_DIR"
        fi
    fi

    # shellcheck disable=SC1091
    source "$VENV_DIR/bin/activate"

    if ! python -c "import mkdocs_material" >/dev/null 2>&1; then
        echo "==> Installing MkDocs toolchain"
        if command -v uv >/dev/null 2>&1; then
            uv pip install -r requirements-docs.txt
        else
            pip install -q -r requirements-docs.txt
        fi
    fi
}

mode="${1:-build}"

ensure_venv

case "$mode" in
    --serve|serve)
        exec mkdocs serve --dev-addr 127.0.0.1:8000
        ;;
    --check|check)
        # Strict build, discard output dir
        tmp_site="$(mktemp -d)"
        trap 'rm -rf "$tmp_site"' EXIT
        mkdocs build --strict --site-dir "$tmp_site"
        echo "OK: mkdocs build --strict passed"
        ;;
    --clean|clean)
        rm -rf _site
        echo "OK: removed _site"
        ;;
    build|--build)
        mkdocs build --strict
        echo "OK: built site at _site/"
        ;;
    *)
        echo "Usage: $0 [build|--check|--serve|--clean]" >&2
        exit 2
        ;;
esac
