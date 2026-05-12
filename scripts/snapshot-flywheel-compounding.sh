#!/usr/bin/env bash
# practices: [wiki-knowledge-surface, snapshot-testing, dora-metrics]
# Operator command: write a corpus-state evidence snapshot for the
# flywheel-compounding gate. The snapshot is the durable proof CI reads
# without re-running the live multi-session computation.
#
# Output: docs/releases/flywheel-compounding-snapshot.json (tracked).
#
# Pair: scripts/check-flywheel-compounding-snapshot.sh (CI gate that
# validates this artifact). Companion bead: soc-45sg.1 (G1).

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
SNAPSHOT_PATH="${SNAPSHOT_PATH:-$REPO_ROOT/docs/releases/flywheel-compounding-snapshot.json}"
AO_BIN="${AO_BIN:-}"

if [[ -z "$AO_BIN" ]]; then
    if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
        AO_BIN="$REPO_ROOT/cli/bin/ao"
    else
        AO_BIN="/tmp/ao-fw-snapshot"
        (cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao) >/dev/null 2>&1 || {
            echo "FAIL: could not build ao for snapshot" >&2
            exit 1
        }
    fi
fi

if ! LIVE_JSON="$("$AO_BIN" flywheel status --json 2>/dev/null)"; then
    echo "FAIL: ao flywheel status --json failed" >&2
    exit 1
fi

GIT_SHA="$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
GIT_BRANCH="$(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
RECORDED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

mkdir -p "$(dirname "$SNAPSHOT_PATH")"

# Wrap the live status in a snapshot envelope. jq preserves numeric types so
# downstream consumers can read sigma/rho/delta without re-parsing.
printf '%s' "$LIVE_JSON" | jq \
    --arg recorded_at "$RECORDED_AT" \
    --arg git_sha "$GIT_SHA" \
    --arg git_branch "$GIT_BRANCH" \
    '{
        recorded_at: $recorded_at,
        git_sha: $git_sha,
        git_branch: $git_branch,
        evidence: .
    }' > "$SNAPSHOT_PATH"

echo "Wrote flywheel-compounding snapshot:"
echo "  path:        $SNAPSHOT_PATH"
echo "  git_sha:     $GIT_SHA"
echo "  recorded_at: $RECORDED_AT"
echo "  compounding: $(jq -r '.evidence.escape_velocity_compounding' "$SNAPSHOT_PATH")"
echo "  sigma_rho:   $(jq -r '.evidence.sigma_rho // "n/a"' "$SNAPSHOT_PATH")"
echo "  delta:       $(jq -r '.evidence.delta' "$SNAPSHOT_PATH")"
