# shellcheck shell=bash
# bash4-guard.sh — SOURCE this first (after the shebang) in any script that uses
# bash 4+ features: `declare -A`, `mapfile`/`readarray`, `${var,,}`/`${var^^}`.
#
# macOS ships bash 3.2 at /bin/bash, which SILENTLY misbehaves on those features
# (an associative array becomes a plain var; no error). A gate script running
# under 3.2 can therefore produce a FALSE GREEN. This guard re-execs the script
# under a bash >= 4 if one is installed (Homebrew), and otherwise exits loudly
# (exit 70 = EX_SOFTWARE, which gate runners map to FAIL, never PASS).
#
# Usage (first executable line after `#!/usr/bin/env bash`):
#   . "$(dirname "$0")/../lib/bash4-guard.sh"
if [ "${BASH_VERSINFO:-0}" -lt 4 ]; then
    for _b4 in /opt/homebrew/bin/bash /usr/local/bin/bash; do
        [ -x "$_b4" ] && exec "$_b4" "$0" "$@"
    done
    echo "ERROR: $0 requires bash >= 4 (macOS system bash is 3.2). Run: brew install bash" >&2
    exit 70
fi
