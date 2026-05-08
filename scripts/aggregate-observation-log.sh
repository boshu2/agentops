#!/usr/bin/env bash
# aggregate-observation-log.sh — pull-mode aggregator for the
# factory-claim-ledger advisory CI observation artifacts.
#
# Bead:  soc-ejq2 (PR-D, Wave 1G of epic soc-xlw8)
# Plan:  .agents/plans/2026-05-07-drain-open-next-work-items.md §soc-ejq2
# Unblocks: Wave 1E (soc-f42z9) — promotion gate requires ≥20 advisory runs.
#
# What it does
# ------------
# 1. Lists recent runs of `validate.yml` via `gh run list`.
# 2. Downloads the `factory-claim-ledger-observation` artifact for each run
#    (silently skips runs that predate the advisory job; their artifact
#    will simply be missing).
# 3. Schema-validates every observation BEFORE deduping (M3 fix: jq's
#    `unique_by(.run_id)` collapses null keys silently — we reject malformed
#    observations up front).
# 4. Dedups on `run_id` and writes atomically to
#    `.agents/reconcile/observation-log.jsonl`.
# 5. Backfills `merged_anyway` and `ledger_updated` per observation:
#      - `pr_number == null` (push-to-main runs): both fields set to `false`.
#        Push-to-main observations satisfy the no-silent-merge criterion by
#        definition (no PR could have merged "anyway").
#      - Otherwise: `gh pr view <pr_number>` for state, and `git log` against
#        `docs/contracts/factory-claim-ledger.example.json` for the
#        commit-touched check.
#
# Inline schema (post-backfill)
# -----------------------------
#   {
#     "run_id":           string,         # required, non-empty
#     "pr_number":        int|null,       # null on push-to-main
#     "verdict":          "pass"|"fail",
#     "surfaces_touched": [string, ...],
#     "timestamp":        ISO8601 string,
#     "merged_anyway":    bool,           # backfilled
#     "ledger_updated":   bool            # backfilled
#   }
#
# CI-side emit lives in `.github/workflows/validate.yml` (job:
# `factory-claim-ledger-strict (advisory)`). That job emits the first five
# fields; `merged_anyway` and `ledger_updated` are backfilled here.
#
# Empty-output case
# -----------------
# If no validate.yml runs have produced the artifact yet (very common on a
# fresh repo or before commit 82260e97), this script writes a valid empty
# JSONL (zero lines) and exits 0 with a friendly message.
#
# Idempotency
# -----------
# Re-running with the same inputs produces a byte-identical output (dedup on
# `run_id` is stable thanks to `jq -s 'unique_by(.run_id)'`). No state is
# kept across invocations beyond the JSONL itself.
#
# Exit codes
# ----------
#   0 = success (including the empty-but-valid path)
#   1 = unexpected failure
#   2 = precondition error (gh missing/unauth, malformed observation)
#
set -euo pipefail

WORKFLOW="${AGGREGATE_OBS_WORKFLOW:-validate.yml}"
ARTIFACT_NAME="${AGGREGATE_OBS_ARTIFACT:-factory-claim-ledger-observation}"
LIMIT=200
DRY_RUN=0
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="$ROOT/.agents/reconcile"
OUT_FILE="$OUT_DIR/observation-log.jsonl"

# AGGREGATE_OBS_FIXTURE_DIR: when set, skip `gh run list`/`gh run download`
# and treat every `*.json`/`*.jsonl` file under the directory as one
# downloaded observation. Used by the integration tests.
FIXTURE_DIR="${AGGREGATE_OBS_FIXTURE_DIR:-}"

usage() {
    cat <<'EOF'
Usage: aggregate-observation-log.sh [--dry-run] [--limit N] [--help]

Aggregate factory-claim-ledger advisory CI observations into
.agents/reconcile/observation-log.jsonl.

Options:
  --dry-run     Skip writes; print summary to stdout.
  --limit N     Number of recent runs to scan (default: 200).
  --help, -h    Show this help.

Environment overrides (advanced/testing):
  AGGREGATE_OBS_WORKFLOW       Workflow file name (default: validate.yml).
  AGGREGATE_OBS_ARTIFACT       Artifact name (default: factory-claim-ledger-observation).
  AGGREGATE_OBS_FIXTURE_DIR    If set, skip gh and read fixtures from here
                               (used by tests).
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run) DRY_RUN=1; shift ;;
        --limit) LIMIT="${2:?--limit needs an argument}"; shift 2 ;;
        --limit=*) LIMIT="${1#--limit=}"; shift ;;
        --help|-h) usage; exit 0 ;;
        *)
            echo "ERROR: unknown flag: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

err() { echo "ERROR: $*" >&2; }
log() { echo "$*"; }

# Tools required regardless of mode.
command -v jq >/dev/null 2>&1 || { err "jq is required"; exit 2; }

if [ -z "$FIXTURE_DIR" ]; then
    command -v gh >/dev/null 2>&1 || { err "gh CLI is required"; exit 2; }
    if ! gh auth status >/dev/null 2>&1; then
        err "gh is not authenticated (run: gh auth login)"
        exit 2
    fi
fi

WORK_DIR="$(mktemp -d -t aggregate-observation-log.XXXXXX)"
trap 'rm -rf "$WORK_DIR"' EXIT

# Phase 1 — collect raw observations into $WORK_DIR/raw/<run_id>.jsonl
mkdir -p "$WORK_DIR/raw"
run_count=0
artifact_count=0

if [ -n "$FIXTURE_DIR" ]; then
    if [ ! -d "$FIXTURE_DIR" ]; then
        err "AGGREGATE_OBS_FIXTURE_DIR does not exist: $FIXTURE_DIR"
        exit 2
    fi
    log "Fixture mode: reading observations from $FIXTURE_DIR"
    while IFS= read -r f; do
        [ -z "$f" ] && continue
        run_count=$((run_count + 1))
        artifact_count=$((artifact_count + 1))
        cp "$f" "$WORK_DIR/raw/$(basename "$f")"
    done < <(find "$FIXTURE_DIR" -maxdepth 1 \( -name '*.json' -o -name '*.jsonl' \) -type f | sort)
else
    log "Listing up to $LIMIT recent runs of $WORKFLOW ..."
    runs_json="$WORK_DIR/runs.json"
    if ! gh run list \
            --workflow "$WORKFLOW" \
            --json databaseId,number,createdAt,headBranch,event \
            --limit "$LIMIT" > "$runs_json"; then
        err "gh run list failed"
        exit 1
    fi

    while IFS= read -r run_id; do
        [ -z "$run_id" ] && continue
        run_count=$((run_count + 1))
        out_sub="$WORK_DIR/dl/$run_id"
        mkdir -p "$out_sub"
        # Silently skip runs whose artifact is missing — this is the common
        # case for runs that predate the advisory job (commit 82260e97).
        if gh run download "$run_id" \
                -n "$ARTIFACT_NAME" \
                -D "$out_sub" >/dev/null 2>&1; then
            # The artifact contains observation.jsonl at its root.
            if [ -f "$out_sub/observation.jsonl" ]; then
                cp "$out_sub/observation.jsonl" "$WORK_DIR/raw/$run_id.jsonl"
                artifact_count=$((artifact_count + 1))
            fi
        fi
    done < <(jq -r '.[].databaseId' "$runs_json")
fi

# Phase 2 — schema validation BEFORE dedup (M3 fix).
shopt -s nullglob
raw_files=("$WORK_DIR"/raw/*)
shopt -u nullglob

if [ "${#raw_files[@]}" -eq 0 ]; then
    log "No observation artifacts found across $run_count run(s)."
    log "Writing empty-but-valid log to $OUT_FILE."
    if [ "$DRY_RUN" -eq 1 ]; then
        log "[dry-run] would create empty $OUT_FILE"
        exit 0
    fi
    mkdir -p "$OUT_DIR"
    : > "$OUT_FILE.tmp"
    mv -f "$OUT_FILE.tmp" "$OUT_FILE"
    exit 0
fi

# Validate every observation has a non-empty string run_id. Other required
# fields (verdict, timestamp) are also checked here.
if ! jq -e '
    . as $o
    | ($o.run_id and ($o.run_id | type == "string") and $o.run_id != "")
    and ($o.verdict and ($o.verdict | type == "string") and ($o.verdict == "pass" or $o.verdict == "fail"))
    and ($o.timestamp and ($o.timestamp | type == "string") and $o.timestamp != "")
' "${raw_files[@]}" >/dev/null; then
    err "malformed observation(s) — missing/null run_id or verdict or timestamp"
    err "files inspected: ${raw_files[*]}"
    exit 2
fi

# Phase 3 — dedup on run_id (jq -s pulls each file as a JSON value).
combined="$WORK_DIR/combined.jsonl"
jq -s 'unique_by(.run_id) | .[]' -c "${raw_files[@]}" > "$combined"

# Phase 4 — backfill merged_anyway + ledger_updated per observation.
backfill_pr() {
    local pr="$1" verdict="$2"
    # In fixture mode (no gh), or when gh is unavailable, conservatively
    # default both fields to false so the file is well-formed.
    if [ -n "$FIXTURE_DIR" ] || ! command -v gh >/dev/null 2>&1; then
        echo "false false"
        return 0
    fi
    local state mergedAt merged_anyway=false ledger_updated=false
    if state="$(gh pr view "$pr" --json state -q '.state' 2>/dev/null)"; then
        if [ "$state" = "MERGED" ] && [ "$verdict" = "fail" ]; then
            merged_anyway=true
        fi
        # Did the merge commit (or any commit on the PR) touch the ledger?
        if mergedAt="$(gh pr view "$pr" --json mergeCommit -q '.mergeCommit.oid' 2>/dev/null)" \
                && [ -n "$mergedAt" ] && [ "$mergedAt" != "null" ]; then
            if git log -1 --name-only --pretty=format: "$mergedAt" 2>/dev/null \
                    | grep -qx 'docs/contracts/factory-claim-ledger.example.json'; then
                ledger_updated=true
            fi
        fi
    fi
    echo "$merged_anyway $ledger_updated"
}

backfilled="$WORK_DIR/backfilled.jsonl"
: > "$backfilled"

while IFS= read -r line; do
    [ -z "$line" ] && continue
    pr_number="$(jq -r '.pr_number // empty' <<<"$line")"
    verdict="$(jq -r '.verdict' <<<"$line")"
    if [ -z "$pr_number" ] || [ "$pr_number" = "null" ]; then
        # Push-to-main: both false by definition (see header).
        merged_anyway=false
        ledger_updated=false
    else
        read -r merged_anyway ledger_updated < <(backfill_pr "$pr_number" "$verdict")
    fi
    jq -c \
        --argjson merged_anyway "$merged_anyway" \
        --argjson ledger_updated "$ledger_updated" \
        '. + {merged_anyway: $merged_anyway, ledger_updated: $ledger_updated}' \
        <<<"$line" >> "$backfilled"
done < "$combined"

obs_count=$(wc -l < "$backfilled" | tr -d ' ')
log "Aggregated $obs_count unique observation(s) from $artifact_count artifact(s) across $run_count run(s)."

if [ "$DRY_RUN" -eq 1 ]; then
    log "[dry-run] would write $obs_count line(s) to $OUT_FILE"
    log "[dry-run] preview:"
    head -n 3 "$backfilled" || true
    exit 0
fi

mkdir -p "$OUT_DIR"
cp "$backfilled" "$OUT_FILE.tmp"
mv -f "$OUT_FILE.tmp" "$OUT_FILE"
log "Wrote $OUT_FILE"
