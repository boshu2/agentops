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
    # Stale-venv pre-flight: a venv whose pyvenv.cfg points to a moved/deleted
    # interpreter passes the directory-existence check but fails on activation.
    # This bit us after the ~/dev/personal/agentops/ -> ~/dev/agentops/ collapse.
    if [[ -d "$VENV_DIR" ]]; then
        if ! "$VENV_DIR/bin/python3" -c "import sys" >/dev/null 2>&1; then
            echo "==> Stale venv detected ($VENV_DIR), recreating"
            rm -rf "$VENV_DIR"
        fi
    fi

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
        # Strict build, discard output dir. `mkdocs build --strict` runs end-to-end
        # (nothing is weakened); the exit is then reinterpreted against an allowlist of
        # DISPOSITIONED warnings — intentional cross-references from docs/ pages to real
        # repo artifacts outside the published site (skills/, scripts/, schemas/, evals/,
        # repo-root docs). Any strict warning NOT in the allowlist still fails the check,
        # so new/accidental broken links break the build.
        #
        # Policy consistency with mkdocs.yml: the PUBLISHED build declares
        # `strict: false` (docs deliberately link outside docs_dir), and
        # tests/docs/validate-links.sh owns link checking for the doc-release
        # checks. This mode is the deliberate strict BACKSTOP layered on top:
        # strict-with-curated-allowlist, per-entry rationale in
        # tests/docs/mkdocs-strict-allowlist.txt (its header is the disposition
        # doc).
        allowlist="$REPO_ROOT/tests/docs/mkdocs-strict-allowlist.txt"
        tmp_site="$(mktemp -d)"
        build_log="$(mktemp)"
        # shellcheck disable=SC2064
        trap "rm -rf '$tmp_site' '$build_log'" EXIT

        set +e
        mkdocs build --strict --site-dir "$tmp_site" >"$build_log" 2>&1
        mkdocs_rc=$?
        set -e

        # Surface the full build log for the operator.
        cat "$build_log"

        # Hard errors (config/plugin failures) are never tolerated.
        if grep -qE '^ERROR ' "$build_log"; then
            echo "" >&2
            echo "FAIL: mkdocs strict build hit a hard error (see log above)" >&2
            exit 1
        fi

        warnings="$(grep -E '^WARNING ' "$build_log" || true)"

        # A nonzero mkdocs exit that produced NO strict warnings (and no ^ERROR,
        # checked above) is an UNEXPLAINED failure — mkdocs missing (exit 127),
        # crashed, or a failure mode we do not pattern. Never certify OK on it:
        # keying the pass purely on grepping the log would let a broken/missing
        # mkdocs (e.g. a fake `mkdocs` that exits 127 first in PATH) print OK.
        # A clean strict pass is rc=0; a strict-warnings failure is rc!=0 WITH
        # warnings (dispositioned below). rc!=0 with no warnings is a hard fail.
        if [[ "$mkdocs_rc" -ne 0 && -z "$warnings" ]]; then
            echo "" >&2
            echo "FAIL: mkdocs build --strict exited ${mkdocs_rc} with no strict warnings to" >&2
            echo "      disposition — mkdocs is missing, crashed, or hit an unrecognized" >&2
            echo "      failure. NOT certifying OK. See the build log above." >&2
            exit 1
        fi

        if [[ -z "$warnings" ]]; then
            echo "OK: mkdocs build --strict passed (0 warnings)"
            exit 0
        fi

        # Filter comments/blank lines out of the allowlist, then subtract exact,
        # whole-line matches. Anything left is an un-dispositioned warning.
        allow_tmp="$(mktemp)"
        # shellcheck disable=SC2064
        trap "rm -rf '$tmp_site' '$build_log' '$allow_tmp'" EXIT
        if [[ -f "$allowlist" ]]; then
            grep -vE '^[[:space:]]*(#|$)' "$allowlist" > "$allow_tmp" || true
        fi

        if [[ -s "$allow_tmp" ]]; then
            undisposed="$(printf '%s\n' "$warnings" | grep -vxF -f "$allow_tmp" || true)"
        else
            undisposed="$warnings"
        fi

        if [[ -n "$undisposed" ]]; then
            echo "" >&2
            echo "FAIL: un-dispositioned mkdocs strict warnings. Fix the link, or — only if it" >&2
            echo "      is an intentional cross-reference to a real repo artifact outside the" >&2
            echo "      docs site — add the verbatim line to tests/docs/mkdocs-strict-allowlist.txt:" >&2
            printf '%s\n' "$undisposed" >&2
            exit 1
        fi

        # All warnings are dispositioned — but a nonzero mkdocs exit must be EXPLAINED
        # by the strict-warnings abort, not a crash that merely ALSO emitted an
        # allowlisted warning. mkdocs prints "Aborted with N warnings in strict mode!"
        # only when it aborts ON warnings; a Python crash/traceback exits nonzero
        # WITHOUT that line. So a nonzero exit lacking the signature is a hard failure
        # (contract: hard errors are never tolerated), even though the allowlist emptied
        # `undisposed`. (Closes: a fake mkdocs that prints one allowlisted warning + a
        # traceback and exits 1 would otherwise certify OK.)
        if [[ "$mkdocs_rc" -ne 0 ]] && ! grep -qE 'Aborted with [0-9]+ warning.* in strict mode' "$build_log"; then
            echo "" >&2
            echo "FAIL: mkdocs build --strict exited ${mkdocs_rc} without the strict-warnings" >&2
            echo "      abort signature — it crashed rather than aborting on warnings, so the" >&2
            echo "      allowlist must not certify it OK. See the build log above." >&2
            exit 1
        fi

        allowed_count="$(printf '%s\n' "$warnings" | grep -c '^WARNING ' || true)"
        echo "OK: mkdocs build --strict clean; ${allowed_count} dispositioned cross-references tolerated (tests/docs/mkdocs-strict-allowlist.txt)"
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
