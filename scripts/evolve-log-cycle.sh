#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/evolve-log-cycle.sh [options]

Append one evolve cycle-history.jsonl entry using the canonical schema.

Required:
  --cycle <n>                 Cycle number to append
  --target <id>               Goal or work target
  --result <value>            improved|regressed|harvested|unchanged|quarantined

Productive-cycle only:
  --canonical-sha <sha>       Implementation commit for the cycle
  --sha <sha>                 Alias for --canonical-sha
  --goals-passing <n>         Passing goals after the cycle
  --goals-total <n>           Total goals after the cycle

Optional:
  --history <path>            History file path (default: .agents/evolve/cycle-history.jsonl)
  --repo-root <path>          Repo root for git checks (default: current directory)
  --cycle-start-sha <sha>     Required for improved cycles to enforce substantive-delta gate
  --log-sha <sha>             Distinct bookkeeping/log commit, when different from canonical
  --quality-score <n>         Optional quality score field
  --goal-ids <csv>            Optional parallel goal ids
  --parallel                  Mark entry as parallel
  --timestamp <iso8601>       Override timestamp for deterministic tests
  -h, --help                  Show help
EOF
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

iso_timestamp() {
  date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -Iseconds
}

resolve_path() {
  local base="$1"
  local path="$2"
  if [[ "$path" = /* ]]; then
    printf '%s\n' "$path"
  else
    printf '%s/%s\n' "$base" "$path"
  fi
}

is_numeric() {
  [[ "$1" =~ ^[0-9]+$ ]]
}

validate_commit() {
  local repo_root="$1"
  local label="$2"
  local value="$3"
  if ! git -C "$repo_root" rev-parse --verify "${value}^{commit}" >/dev/null 2>&1; then
    die "$label is not a valid commit: $value"
  fi
}

HISTORY=".agents/evolve/cycle-history.jsonl"
REPO_ROOT="$(pwd)"
CYCLE=""
TARGET=""
RESULT=""
CANONICAL_SHA=""
LOG_SHA=""
GOALS_PASSING=""
GOALS_TOTAL=""
CYCLE_START_SHA=""
QUALITY_SCORE=""
GOAL_IDS=""
PARALLEL=false
TIMESTAMP="$(iso_timestamp)"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --history)
      HISTORY="${2:-}"
      shift 2
      ;;
    --repo-root)
      REPO_ROOT="${2:-}"
      shift 2
      ;;
    --cycle)
      CYCLE="${2:-}"
      shift 2
      ;;
    --target)
      TARGET="${2:-}"
      shift 2
      ;;
    --result)
      RESULT="${2:-}"
      shift 2
      ;;
    --canonical-sha|--sha)
      CANONICAL_SHA="${2:-}"
      shift 2
      ;;
    --log-sha)
      LOG_SHA="${2:-}"
      shift 2
      ;;
    --goals-passing)
      GOALS_PASSING="${2:-}"
      shift 2
      ;;
    --goals-total)
      GOALS_TOTAL="${2:-}"
      shift 2
      ;;
    --cycle-start-sha)
      CYCLE_START_SHA="${2:-}"
      shift 2
      ;;
    --quality-score)
      QUALITY_SCORE="${2:-}"
      shift 2
      ;;
    --goal-ids)
      GOAL_IDS="${2:-}"
      shift 2
      ;;
    --parallel)
      PARALLEL=true
      shift
      ;;
    --timestamp)
      TIMESTAMP="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

require_cmd git
require_cmd jq

[[ -n "$CYCLE" ]] || die "--cycle is required"
[[ -n "$TARGET" ]] || die "--target is required"
[[ -n "$RESULT" ]] || die "--result is required"
is_numeric "$CYCLE" || die "--cycle must be numeric"

case "$RESULT" in
  improved|regressed|harvested|unchanged|quarantined)
    ;;
  *)
    die "--result must be one of improved|regressed|harvested|unchanged|quarantined"
    ;;
esac

if [[ "$RESULT" == "unchanged" && "$TARGET" != "idle" ]]; then
  die "unchanged results must use --target idle; attempted productive cycles should report their intended productive result and let the substantive-delta gate downgrade if needed"
fi

HISTORY_PATH="$(resolve_path "$REPO_ROOT" "$HISTORY")"
mkdir -p "$(dirname "$HISTORY_PATH")"

EXPECTED_CYCLE=1
if [[ -f "$HISTORY_PATH" ]]; then
  LAST_LINE="$(awk 'NF { line = $0 } END { print line }' "$HISTORY_PATH")"
  if [[ -n "$LAST_LINE" ]]; then
    printf '%s\n' "$LAST_LINE" | jq empty >/dev/null 2>&1 || die "last history line is not valid JSON"
    LAST_CYCLE="$(printf '%s\n' "$LAST_LINE" | jq -r '.cycle // empty')"
    is_numeric "$LAST_CYCLE" || die "last history line is missing a numeric cycle"
    EXPECTED_CYCLE=$((LAST_CYCLE + 1))
  fi
fi

[[ "$CYCLE" -eq "$EXPECTED_CYCLE" ]] || die "expected cycle $EXPECTED_CYCLE, got $CYCLE"

PRODUCTIVE=false
if [[ "$RESULT" == "improved" || "$RESULT" == "regressed" || "$RESULT" == "harvested" ]]; then
  PRODUCTIVE=true
fi

FINAL_RESULT="$RESULT"
if [[ "$PRODUCTIVE" == true ]]; then
  [[ -n "$CANONICAL_SHA" ]] || die "productive results require --canonical-sha"
  [[ -n "$GOALS_PASSING" ]] || die "productive results require --goals-passing"
  [[ -n "$GOALS_TOTAL" ]] || die "productive results require --goals-total"
  is_numeric "$GOALS_PASSING" || die "--goals-passing must be numeric"
  is_numeric "$GOALS_TOTAL" || die "--goals-total must be numeric"
  if [[ -n "$QUALITY_SCORE" ]]; then
    is_numeric "$QUALITY_SCORE" || die "--quality-score must be numeric"
  fi

  validate_commit "$REPO_ROOT" "canonical_sha" "$CANONICAL_SHA"
  if [[ -n "$LOG_SHA" ]]; then
    validate_commit "$REPO_ROOT" "log_sha" "$LOG_SHA"
  fi

  if [[ "$RESULT" == "improved" ]]; then
    [[ -n "$CYCLE_START_SHA" ]] || die "improved results require --cycle-start-sha"
    validate_commit "$REPO_ROOT" "cycle_start_sha" "$CYCLE_START_SHA"
    REAL_CHANGES="$(
      git -C "$REPO_ROOT" diff --name-only "${CYCLE_START_SHA}..${CANONICAL_SHA}" -- ':!.agents/**' ':!GOALS.yaml' ':!GOALS.md' \
        | awk 'NF { count++ } END { print count + 0 }'
    )"
    if [[ "$REAL_CHANGES" -eq 0 ]]; then
      FINAL_RESULT="unchanged"
      PRODUCTIVE=false
      echo "INFO: downgraded cycle ${CYCLE} to unchanged; no substantive repo delta found." >&2
    fi
  fi
fi

ENTRY="$(
  jq -cn \
    --argjson cycle "$CYCLE" \
    --arg target "$TARGET" \
    --arg result "$FINAL_RESULT" \
    --arg timestamp "$TIMESTAMP" \
    --arg canonical_sha "$CANONICAL_SHA" \
    --arg log_sha "$LOG_SHA" \
    --arg goal_ids "$GOAL_IDS" \
    --argjson goals_passing "${GOALS_PASSING:-0}" \
    --argjson goals_total "${GOALS_TOTAL:-0}" \
    --argjson productive "$PRODUCTIVE" \
    --argjson parallel "$PARALLEL" \
    --argjson has_quality "$( [[ -n "$QUALITY_SCORE" ]] && echo true || echo false )" \
    --argjson quality_score "${QUALITY_SCORE:-0}" \
    '
      {
        cycle: $cycle,
        target: $target,
        result: $result,
        timestamp: $timestamp
      }
      + (if $productive then {
          sha: $canonical_sha,
          canonical_sha: $canonical_sha,
          goals_passing: $goals_passing,
          goals_total: $goals_total
        } else {} end)
      + (if $productive and ($log_sha | length > 0) and $log_sha != $canonical_sha then {
          log_sha: $log_sha
        } else {} end)
      + (if $parallel then {parallel: true} else {} end)
      + (if ($goal_ids | length) > 0 then {
          goal_ids: ($goal_ids | split(",") | map(gsub("^\\s+|\\s+$"; "") | select(length > 0)))
        } else {} end)
      + (if $productive and $has_quality then {quality_score: $quality_score} else {} end)
    '
)"

printf '%s\n' "$ENTRY" >> "$HISTORY_PATH"
printf '%s\n' "$ENTRY"
