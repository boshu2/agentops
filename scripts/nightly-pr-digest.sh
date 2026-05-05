#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
nightly-pr-digest.sh

Generate a structured PR body from nightly evolution run state.

Reads the most recent nightly run directory and produces a markdown PR body
covering: baseline/final goals, cycle history, runtime-artifact flips, open PR
overlap, tracker degradation, transient flakes, auto-reverts, and tag-push
status.

Options:
  --run-dir <path>         Explicit nightly run directory (default: latest under .agents/nightly/)
  --repo-root <path>       Repository root (default: git top-level or cwd)
  --branch <name>          Branch name for the PR (default: read from run digest.json)
  --output <path>          Output file for PR body markdown (default: stdout)
  --baseline-label <slug>  Baseline label to compare against (default: latest)
  --since <date>           Git log start date for auto-revert scan (default: 24h ago)
  --format <fmt>           Output format: markdown|json (default: markdown)
  -h, --help               Show this help
EOF
}

die() { echo "nightly-pr-digest: $*" >&2; exit 1; }
log() { printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" >&2; }

RUN_DIR=""
REPO_ROOT=""
BRANCH=""
OUTPUT=""
BASELINE_LABEL=""
SINCE=""
FORMAT="markdown"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --run-dir) RUN_DIR="${2:-}"; shift 2 ;;
    --repo-root) REPO_ROOT="${2:-}"; shift 2 ;;
    --branch) BRANCH="${2:-}"; shift 2 ;;
    --output) OUTPUT="${2:-}"; shift 2 ;;
    --baseline-label) BASELINE_LABEL="${2:-}"; shift 2 ;;
    --since) SINCE="${2:-}"; shift 2 ;;
    --format) FORMAT="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown arg: $1" ;;
  esac
done

if [[ -z "$REPO_ROOT" ]]; then
  REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
fi
cd "$REPO_ROOT"

if [[ -z "$RUN_DIR" ]]; then
  RUN_DIR="$(find .agents/nightly -mindepth 2 -maxdepth 2 -type d 2>/dev/null | sort | tail -1)"
  [[ -n "$RUN_DIR" ]] || die "no nightly run directories found under .agents/nightly/"
fi
[[ -d "$RUN_DIR" ]] || die "run directory does not exist: $RUN_DIR"

DIGEST_JSON="$RUN_DIR/digest.json"
[[ -f "$DIGEST_JSON" ]] || die "digest.json not found in $RUN_DIR"

if [[ -z "$SINCE" ]]; then
  SINCE="$(date -u -d '24 hours ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v-24H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo '')"
fi

if [[ -z "$BRANCH" ]]; then
  BRANCH="$(jq -r '.planned_branch // ""' "$DIGEST_JSON")"
fi

# ──── Section builders ────

build_header() {
  local run_id mode run_date generated_at
  run_id="$(jq -r '.run_id // "unknown"' "$DIGEST_JSON")"
  mode="$(jq -r '.mode // "unknown"' "$DIGEST_JSON")"
  run_date="$(jq -r '.run_date // "unknown"' "$DIGEST_JSON")"
  generated_at="$(jq -r '.generated_at // "unknown"' "$DIGEST_JSON")"

  printf '## Nightly Evolution — %s\n\n' "$run_date"
  printf '| Field | Value |\n|---|---|\n'
  printf '| Run ID | `%s` |\n' "$run_id"
  printf '| Mode | %s |\n' "$mode"
  printf '| Branch | `%s` |\n' "$BRANCH"
  printf '| Generated | %s |\n\n' "$generated_at"
}

build_goals_section() {
  local history_file=".agents/evolve/cycle-history.jsonl"
  local baseline_dir=".agents/evolve/fitness-baselines"

  printf '### Goals (Baseline → Final)\n\n'

  local baseline_goals=""
  if [[ -n "$BASELINE_LABEL" && -d "$baseline_dir/$BASELINE_LABEL" ]]; then
    local baseline_file
    baseline_file="$(find "$baseline_dir/$BASELINE_LABEL" -name '*.json' | sort | tail -1)"
    if [[ -n "$baseline_file" ]]; then
      baseline_goals="$(jq -r '.goals | length' "$baseline_file" 2>/dev/null || echo '?')"
      local baseline_passing
      baseline_passing="$(jq -r '[.goals[] | select(.status == "passing")] | length' "$baseline_file" 2>/dev/null || echo '?')"
      printf -- '- **Baseline** (`%s`): %s/%s passing\n' "$BASELINE_LABEL" "$baseline_passing" "$baseline_goals"
    fi
  elif [[ -d "$baseline_dir" ]]; then
    local latest_label
    latest_label="$(find "$baseline_dir" -maxdepth 1 -mindepth 1 -type d -printf '%f\n' 2>/dev/null | sort | tail -1)"
    if [[ -n "$latest_label" ]]; then
      local latest_file
      latest_file="$(find "$baseline_dir/$latest_label" -name '*.json' 2>/dev/null | sort | tail -1)"
      if [[ -n "$latest_file" ]]; then
        baseline_goals="$(jq -r '.goals | length' "$latest_file" 2>/dev/null || echo '?')"
        local baseline_passing
        baseline_passing="$(jq -r '[.goals[] | select(.status == "passing")] | length' "$latest_file" 2>/dev/null || echo '?')"
        printf -- '- **Baseline** (`%s`): %s/%s passing\n' "$latest_label" "$baseline_passing" "$baseline_goals"
      fi
    fi
  fi

  if [[ -f "$history_file" ]]; then
    local last_entry
    last_entry="$(tail -1 "$history_file" || true)"
    if [[ -n "$last_entry" ]]; then
      local goals_passing goals_total
      goals_passing="$(echo "$last_entry" | jq -r '.goals_passing // "?"')"
      goals_total="$(echo "$last_entry" | jq -r '.goals_total // "?"')"
      printf -- '- **Final**: %s/%s passing\n' "$goals_passing" "$goals_total"
    fi
  fi

  if [[ -z "$baseline_goals" ]]; then
    printf -- '- _(no baseline snapshot found)_\n'
  fi
  printf '\n'
}

build_cycle_history() {
  local history_file=".agents/evolve/cycle-history.jsonl"
  printf '### Cycle History\n\n'

  if [[ ! -f "$history_file" ]]; then
    printf '_(no cycle-history.jsonl found)_\n\n'
    return
  fi

  local total improved regressed unchanged quarantined
  total="$(wc -l < "$history_file" | tr -d ' ')"
  improved="$(command grep -c '"result":"improved"' "$history_file" || true)"
  improved="${improved%%[^0-9]*}"
  regressed="$(command grep -c '"result":"regressed"' "$history_file" || true)"
  regressed="${regressed%%[^0-9]*}"
  unchanged="$(command grep -c '"result":"unchanged"' "$history_file" || true)"
  unchanged="${unchanged%%[^0-9]*}"
  quarantined="$(command grep -c '"result":"quarantined"' "$history_file" || true)"
  quarantined="${quarantined%%[^0-9]*}"
  : "${improved:=0}" "${regressed:=0}" "${unchanged:=0}" "${quarantined:=0}"

  printf '| Metric | Count |\n|---|---|\n'
  printf '| Total cycles | %s |\n' "$total"
  printf '| Improved | %s |\n' "$improved"
  printf '| Regressed | %s |\n' "$regressed"
  printf '| Unchanged | %s |\n' "$unchanged"
  printf '| Quarantined | %s |\n\n' "$quarantined"

  printf '<details>\n<summary>Last 10 cycles</summary>\n\n'
  printf '| # | Target | Result | Files | Duration |\n|---|---|---|---|---|\n'
  tail -10 "$history_file" | while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    local cycle work_id result files_count duration
    cycle="$(echo "$line" | jq -r '.cycle // "?"')"
    work_id="$(echo "$line" | jq -r '.work_id // .work_title // "?"')"
    result="$(echo "$line" | jq -r '.result // "?"')"
    files_count="$(echo "$line" | jq -r '(.files_changed // []) | length')"
    duration="$(echo "$line" | jq -r '.duration_min // "?"')"
    printf '| %s | `%s` | %s | %s | %sm |\n' "$cycle" "$work_id" "$result" "$files_count" "$duration"
  done
  printf '\n</details>\n\n'
}

build_runtime_flips() {
  printf '### Runtime-Artifact Flips\n\n'

  local readiness_status
  readiness_status="$(jq -r '.readiness.ai_sane_status // "unknown"' "$DIGEST_JSON")"
  local dream_status evolve_status
  dream_status="$(jq -r '.phases.dream // "not-requested"' "$DIGEST_JSON")"
  evolve_status="$(jq -r '.phases.evolve // "not-requested"' "$DIGEST_JSON")"

  printf '| Phase | Status |\n|---|---|\n'
  printf '| AI readiness | %s |\n' "$readiness_status"
  printf '| Dream | %s |\n' "$dream_status"
  printf '| Evolve | %s |\n\n' "$evolve_status"

  local runtime_inventory
  runtime_inventory="$(jq -r '.runtime.inventory_tsv // ""' "$DIGEST_JSON")"
  if [[ -n "$runtime_inventory" ]]; then
    local missing
    missing="$(echo "$runtime_inventory" | command grep -c 'false' || true)"
    missing="${missing%%[^0-9]*}"
    : "${missing:=0}"
    if [[ "$missing" -gt 0 ]]; then
      printf '**Missing runtimes:** '
      echo "$runtime_inventory" | command grep 'false' | cut -f1 | tr '\n' ',' | sed 's/,$/\n/'
      printf '\n'
    fi
  fi
}

build_open_pr_overlap() {
  printf '### Open PR Overlap\n\n'

  local pr_count
  pr_count="$(jq -r '.github.open_prs | length' "$DIGEST_JSON" 2>/dev/null || echo 0)"
  if [[ "$pr_count" == "0" ]]; then
    printf '_(no open PRs)_\n\n'
    return
  fi

  printf '| # | Title | Branch | Changed Files |\n|---|---|---|---|\n'
  jq -r '.github.open_prs[] | "| #\(.number) | \(.title) | `\(.headRefName)` | \(.changedFiles) |"' "$DIGEST_JSON" 2>/dev/null | head -15
  printf '\n'

  if [[ "$pr_count" -gt 15 ]]; then
    printf '_... and %d more_\n\n' "$((pr_count - 15))"
  fi
}

build_tracker_degradation() {
  printf '### Tracker Degradation\n\n'

  if ! command -v bd >/dev/null 2>&1; then
    printf '_(bd not available)_\n\n'
    return
  fi

  local open_count in_progress_count
  open_count="$(bd list --status=open 2>/dev/null | command grep -c '○' || true)"
  open_count="${open_count%%[^0-9]*}"
  in_progress_count="$(bd list --status=in_progress 2>/dev/null | command grep -c '◐\|●' || true)"
  in_progress_count="${in_progress_count%%[^0-9]*}"
  : "${open_count:=0}" "${in_progress_count:=0}"

  printf '| Metric | Count |\n|---|---|\n'
  printf '| Open issues | %s |\n' "$open_count"
  printf '| In-progress | %s |\n\n' "$in_progress_count"

  local stale_items
  stale_items="$(bd list --status=in_progress 2>/dev/null | command grep -E '2026-0[0-3]|2025-' | head -5 || true)"
  if [[ -n "$stale_items" ]]; then
    printf '**Stale in-progress (>30d):**\n```\n%s\n```\n\n' "$stale_items"
  fi
}

build_transient_flakes() {
  printf '### Transient Flakes\n\n'

  local flake_log=".agents/evolve/flakes.jsonl"
  if [[ -f "$flake_log" ]]; then
    local flake_count
    flake_count="$(wc -l < "$flake_log" | tr -d ' ')"
    printf -- '- Recorded flakes: %s\n' "$flake_count"
    printf '<details>\n<summary>Recent flakes</summary>\n\n'
    tail -5 "$flake_log" | while IFS= read -r fline; do
      jq -r '"- \(.timestamp // "?"): \(.test // .message // "unknown")"' <<<"$fline" 2>/dev/null || true
    done
    printf '\n</details>\n\n'
  else
    local session_state=".agents/evolve/session-state.json"
    if [[ -f "$session_state" ]]; then
      local last_result
      last_result="$(jq -r '.last_cycle_result // "unknown"' "$session_state")"
      if [[ "$last_result" == "quarantined" ]]; then
        printf -- '- Last cycle was quarantined (possible flake)\n\n'
      else
        printf '_(no flake log; last cycle result: %s)_\n\n' "$last_result"
      fi
    else
      printf '_(no flake tracking data found)_\n\n'
    fi
  fi
}

build_auto_reverts() {
  printf '### Auto-Reverts\n\n'

  if [[ -z "$SINCE" ]]; then
    printf '_(--since not resolved; skipping)_\n\n'
    return
  fi

  local revert_count reverts
  reverts="$(git log --oneline --since="$SINCE" --grep='[Rr]evert' 2>/dev/null || true)"
  if [[ -z "$reverts" ]]; then
    printf '_(none in last 24h)_\n\n'
  else
    revert_count="$(echo "$reverts" | wc -l | tr -d ' ')"
    printf '**%s revert(s) in window:**\n```\n%s\n```\n\n' "$revert_count" "$reverts"
  fi
}

build_tag_push_status() {
  printf '### Tag-Push Status\n\n'

  local latest_tag
  latest_tag="$(git describe --tags --abbrev=0 2>/dev/null || echo '')"
  if [[ -z "$latest_tag" ]]; then
    printf '_(no tags found)_\n\n'
    return
  fi

  local tag_date tag_behind
  tag_date="$(git log -1 --format=%ci "$latest_tag" 2>/dev/null || echo '?')"
  tag_behind="$(git rev-list "$latest_tag"..HEAD --count 2>/dev/null || echo '?')"

  printf '| Field | Value |\n|---|---|\n'
  printf '| Latest tag | `%s` |\n' "$latest_tag"
  printf '| Tag date | %s |\n' "$tag_date"
  printf '| Commits since tag | %s |\n\n' "$tag_behind"
}

# ──── Assemble ────

emit_markdown() {
  build_header
  build_goals_section
  build_cycle_history
  build_runtime_flips
  build_open_pr_overlap
  build_tracker_degradation
  build_transient_flakes
  build_auto_reverts
  build_tag_push_status

  printf '%s\n*Generated by `nightly-pr-digest.sh` at %s*\n' '---' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}

emit_json() {
  local tmp_md
  tmp_md="$(mktemp "${TMPDIR:-/tmp}/pr-digest-XXXXXX.md")"
  emit_markdown > "$tmp_md"

  jq -n \
    --arg schema_version "1" \
    --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg branch "$BRANCH" \
    --arg run_dir "$RUN_DIR" \
    --rawfile body "$tmp_md" \
    '{
      schema_version: ($schema_version | tonumber),
      generated_at: $generated_at,
      branch: $branch,
      run_dir: $run_dir,
      pr_body: $body
    }'
  rm -f "$tmp_md"
}

# ──── Main ────

log "run_dir=$RUN_DIR branch=$BRANCH format=$FORMAT"

case "$FORMAT" in
  markdown)
    if [[ -n "$OUTPUT" ]]; then
      emit_markdown > "$OUTPUT"
      log "wrote $OUTPUT"
    else
      emit_markdown
    fi
    ;;
  json)
    if [[ -n "$OUTPUT" ]]; then
      emit_json > "$OUTPUT"
      log "wrote $OUTPUT"
    else
      emit_json
    fi
    ;;
  *) die "unknown format: $FORMAT (use markdown or json)" ;;
esac
