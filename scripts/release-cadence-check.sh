#!/usr/bin/env bash
set -euo pipefail

# release-cadence-check.sh
# Compatibility wrapper retained after removing the enforced release cadence policy.
#
# Usage:
#   ./scripts/release-cadence-check.sh
#
# Exit codes:
#   0 = informational pass

if [[ $# -gt 0 ]]; then
    echo "INFO: release cadence flags are ignored; no minimum release spacing is enforced."
fi

echo "PASS: Release cadence policy removed; releases may ship whenever maintainers decide they are ready."

# No spacing is enforced, but surface a 'release due' NUDGE so an overdue release
# (many commits / days since the last vX.Y.Z tag) stays visible. Informational
# only — never blocks (age-push-equals-ci-0ua.5).
_here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -x "$_here/check-release-due.sh" ]]; then
    "$_here/check-release-due.sh" || true
fi
exit 0
