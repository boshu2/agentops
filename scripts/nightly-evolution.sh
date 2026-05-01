#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
nightly-evolution.sh

Run or preview the private local AgentOps nightly chain.

Default mode is a safe dry-run: it records readiness, planned branch/runtimes,
open PR context, optional Nightly RPI brief output, and digest files without
mutating source or starting agent work.

Options:
  --execute                 Allow execution of explicitly enabled phases.
  --run-dream               In execute mode, run ao overnight with configured runners.
  --run-evolve              In execute mode, run the supervised RPI/evolve wrapper.
  --skip-brief              Skip scripts/nightly-rpi-brief.sh.
  --emit-systemd            Write systemd user service/timer templates to the run dir.
  --repo-root <path>        Repository root (default: git top-level or cwd).
  --output-dir <path>       Output directory (default: .agents/nightly/<date>/<run-id>).
  --date <YYYY-MM-DD>       Nightly date used for branch naming (default: UTC today).
  --runners <csv>           Dream runners (default: claude,codex).
  --runtime-cmd <cmd>       RPI runtime command for evolve (default: claude).
  --runtime-mode <mode>     RPI runtime mode auto|direct|stream|tmux|gc (default: auto).
  --max-cycles <n>          Max evolve cycles when --run-evolve is used (default: 1).
  --gate-policy <policy>    Evolve gate policy (default: required).
  --landing-policy <policy> Evolve landing policy (default: off).
  --schedule <calendar>     systemd OnCalendar value (default: *-*-* 12:15:00 UTC).
  --no-require-ai-sane      Do not block execute mode when bushido-box ai-sane fails.
  -h, --help                Show this help.

Examples:
  scripts/nightly-evolution.sh --emit-systemd
  scripts/nightly-evolution.sh --execute --run-dream
  scripts/nightly-evolution.sh --execute --run-evolve --runtime-cmd codex
EOF
}

die() {
  echo "nightly-evolution: $*" >&2
  exit 1
}

log() {
  printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

json_empty_object() {
  printf '{}\n'
}

json_empty_array() {
  printf '[]\n'
}

resolve_repo_root() {
  local root="$1"
  if [[ -n "$root" ]]; then
    cd "$root" && pwd
    return
  fi
  git rev-parse --show-toplevel 2>/dev/null || pwd
}

run_json_capture() {
  local out_file="$1"
  local err_file="$2"
  shift 2

  if "$@" >"$out_file" 2>"$err_file"; then
    return 0
  fi
  return 1
}

split_csv() {
  local value="$1"
  local -n out_ref="$2"
  local item
  IFS=',' read -r -a out_ref <<<"$value"
  for item in "${out_ref[@]}"; do
    if [[ -z "$item" ]]; then
      die "empty item in comma-separated list: $value"
    fi
  done
}

write_runtime_inventory() {
  local out_tsv="$1"
  shift
  local runtime path
  : >"$out_tsv"
  for runtime in "$@"; do
    path="$(command -v "$runtime" 2>/dev/null || true)"
    if [[ -n "$path" ]]; then
      printf '%s\ttrue\t%s\n' "$runtime" "$path" >>"$out_tsv"
    else
      printf '%s\tfalse\t\n' "$runtime" >>"$out_tsv"
    fi
  done
}

compute_branch() {
  local date_value="$1"
  local base="nightly/${date_value}"
  local candidate="$base"
  local suffix=2

  if ! git remote get-url origin >/dev/null 2>&1; then
    printf '%s\n' "$candidate"
    return
  fi

  while git ls-remote --exit-code --heads origin "$candidate" >/dev/null 2>&1; do
    candidate="${base}-v${suffix}"
    suffix=$((suffix + 1))
  done
  printf '%s\n' "$candidate"
}

emit_systemd_templates() {
  local dir="$1"
  local repo_root="$2"
  local schedule="$3"
  local runners="$4"
  local runtime_cmd="$5"
  local max_cycles="$6"
  mkdir -p "$dir"

  cat >"$dir/agentops-nightly-evolution.service" <<EOF
[Unit]
Description=AgentOps private local nightly evolution
Documentation=file://${repo_root}/docs/runbooks/nightly-evolution.md

[Service]
Type=oneshot
WorkingDirectory=${repo_root}
ExecStart=${repo_root}/scripts/nightly-evolution.sh --execute --run-dream --run-evolve --runners ${runners} --runtime-cmd ${runtime_cmd} --max-cycles ${max_cycles}
EOF

  cat >"$dir/agentops-nightly-evolution.timer" <<EOF
[Unit]
Description=Schedule AgentOps private local nightly evolution

[Timer]
OnCalendar=${schedule}
Persistent=true
RandomizedDelaySec=10m
Unit=agentops-nightly-evolution.service

[Install]
WantedBy=timers.target
EOF
}

write_markdown_digest() {
  local path="$1"
  local run_id="$2"
  local mode="$3"
  local branch="$4"
  local ai_sane_status="$5"
  local dream_status="$6"
  local evolve_status="$7"
  local systemd_dir="$8"

  {
    printf '# Nightly Evolution Digest\n\n'
    printf -- '- Run ID: `%s`\n' "$run_id"
    printf -- '- Mode: `%s`\n' "$mode"
    printf -- '- Planned branch: `%s`\n' "$branch"
    printf -- '- AI readiness: `%s`\n' "$ai_sane_status"
    printf -- '- Dream phase: `%s`\n' "$dream_status"
    printf -- '- Evolve phase: `%s`\n' "$evolve_status"
    if [[ -n "$systemd_dir" ]]; then
      printf -- '- Scheduler templates: `%s`\n' "$systemd_dir"
    fi
    printf '\n'
  } >"$path"
}

EXECUTE=false
RUN_DREAM=false
RUN_EVOLVE=false
SKIP_BRIEF=false
EMIT_SYSTEMD=false
REQUIRE_AI_SANE=true
REPO_ROOT=""
OUTPUT_DIR=""
RUN_DATE="$(date -u +%F)"
RUNNERS="claude,codex"
RUNTIME_CMD="claude"
RUNTIME_MODE="auto"
MAX_CYCLES="1"
GATE_POLICY="required"
LANDING_POLICY="off"
SCHEDULE="*-*-* 12:15:00 UTC"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --execute)
      EXECUTE=true
      shift
      ;;
    --run-dream)
      RUN_DREAM=true
      shift
      ;;
    --run-evolve)
      RUN_EVOLVE=true
      shift
      ;;
    --skip-brief)
      SKIP_BRIEF=true
      shift
      ;;
    --emit-systemd)
      EMIT_SYSTEMD=true
      shift
      ;;
    --repo-root)
      REPO_ROOT="${2:-}"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="${2:-}"
      shift 2
      ;;
    --date)
      RUN_DATE="${2:-}"
      shift 2
      ;;
    --runners)
      RUNNERS="${2:-}"
      shift 2
      ;;
    --runtime-cmd)
      RUNTIME_CMD="${2:-}"
      shift 2
      ;;
    --runtime-mode)
      RUNTIME_MODE="${2:-}"
      shift 2
      ;;
    --max-cycles)
      MAX_CYCLES="${2:-}"
      shift 2
      ;;
    --gate-policy)
      GATE_POLICY="${2:-}"
      shift 2
      ;;
    --landing-policy)
      LANDING_POLICY="${2:-}"
      shift 2
      ;;
    --schedule)
      SCHEDULE="${2:-}"
      shift 2
      ;;
    --no-require-ai-sane)
      REQUIRE_AI_SANE=false
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown arg: $1"
      ;;
  esac
done

[[ "$RUN_DATE" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || die "--date must use YYYY-MM-DD"
[[ "$MAX_CYCLES" =~ ^[0-9]+$ ]] || die "--max-cycles must be a non-negative integer"
case "$RUNTIME_MODE" in
  auto|direct|stream|tmux|gc) ;;
  *) die "--runtime-mode must be auto, direct, stream, tmux, or gc" ;;
esac

REPO_ROOT="$(resolve_repo_root "$REPO_ROOT")"
cd "$REPO_ROOT"

require_cmd git
require_cmd jq
require_cmd ao

RUN_STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_ID="nightly-evolution-${RUN_STAMP}"
if [[ -z "$OUTPUT_DIR" ]]; then
  OUTPUT_DIR=".agents/nightly/${RUN_DATE}/${RUN_ID}"
fi
mkdir -p "$OUTPUT_DIR"
OUTPUT_DIR="$(cd "$OUTPUT_DIR" && pwd)"

LOCK_DIR="$REPO_ROOT/.agents/nightly/nightly-evolution.lock"
mkdir -p "$(dirname "$LOCK_DIR")"
if ! mkdir "$LOCK_DIR" 2>/dev/null; then
  die "another nightly evolution run appears active: $LOCK_DIR"
fi
trap 'rm -rf "$LOCK_DIR"' EXIT

if [[ -f "$REPO_ROOT/.agents/evolve/STOP" || -f "$REPO_ROOT/.agents/rpi/KILL" || -f "$HOME/.config/evolve/KILL" ]]; then
  die "nightly kill switch is present"
fi

MODE="dry-run"
if [[ "$EXECUTE" == true ]]; then
  MODE="execute"
fi

log "run_id=$RUN_ID mode=$MODE output_dir=$OUTPUT_DIR"

BRANCH="$(compute_branch "$RUN_DATE")"
RUNTIME_TSV="$OUTPUT_DIR/runtime-inventory.tsv"
split_csv "$RUNNERS" RUNNER_ARRAY
write_runtime_inventory "$RUNTIME_TSV" "${RUNNER_ARRAY[@]}" "$RUNTIME_CMD" gh bd

AI_SANE_JSON="$OUTPUT_DIR/ai-sane.json"
AI_SANE_ERR="$OUTPUT_DIR/ai-sane.stderr"
AI_SANE_STATUS="skipped"
if command -v bushido-box >/dev/null 2>&1; then
  if command -v timeout >/dev/null 2>&1; then
    if run_json_capture "$AI_SANE_JSON" "$AI_SANE_ERR" timeout 20s bushido-box ai-sane --json; then
      AI_SANE_STATUS="$(jq -r '.status // "unknown"' "$AI_SANE_JSON" 2>/dev/null || echo unknown)"
    else
      AI_SANE_STATUS="failed"
      json_empty_object >"$AI_SANE_JSON"
    fi
  elif run_json_capture "$AI_SANE_JSON" "$AI_SANE_ERR" bushido-box ai-sane --json; then
    AI_SANE_STATUS="$(jq -r '.status // "unknown"' "$AI_SANE_JSON" 2>/dev/null || echo unknown)"
  else
    AI_SANE_STATUS="failed"
    json_empty_object >"$AI_SANE_JSON"
  fi
else
  AI_SANE_STATUS="unavailable"
  json_empty_object >"$AI_SANE_JSON"
  : >"$AI_SANE_ERR"
fi

if [[ "$EXECUTE" == true && "$REQUIRE_AI_SANE" == true && "$AI_SANE_STATUS" != "ok" ]]; then
  die "bushido-box ai-sane is required for execute mode, got: $AI_SANE_STATUS"
fi

DREAM_SETUP_JSON="$OUTPUT_DIR/dream-setup.json"
DREAM_SETUP_ERR="$OUTPUT_DIR/dream-setup.stderr"
if ! run_json_capture "$DREAM_SETUP_JSON" "$DREAM_SETUP_ERR" ao overnight setup --json; then
  json_empty_object >"$DREAM_SETUP_JSON"
fi

OPEN_PRS_JSON="$OUTPUT_DIR/open-prs.json"
OPEN_PRS_ERR="$OUTPUT_DIR/open-prs.stderr"
if command -v gh >/dev/null 2>&1; then
  if ! run_json_capture "$OPEN_PRS_JSON" "$OPEN_PRS_ERR" \
    gh pr list --state open --limit 30 --json number,title,headRefName,baseRefName,changedFiles,url,labels; then
    json_empty_array >"$OPEN_PRS_JSON"
  fi
else
  json_empty_array >"$OPEN_PRS_JSON"
  : >"$OPEN_PRS_ERR"
fi

BRIEF_STATUS="skipped"
if [[ "$SKIP_BRIEF" != true ]]; then
  if [[ -x "$REPO_ROOT/scripts/nightly-rpi-brief.sh" ]]; then
    if "$REPO_ROOT/scripts/nightly-rpi-brief.sh" --output-dir "$OUTPUT_DIR/nightly-brief" >"$OUTPUT_DIR/nightly-brief.log" 2>&1; then
      BRIEF_STATUS="ok"
    else
      BRIEF_STATUS="failed"
    fi
  else
    BRIEF_STATUS="unavailable"
  fi
fi

SYSTEMD_DIR=""
if [[ "$EMIT_SYSTEMD" == true ]]; then
  SYSTEMD_DIR="$OUTPUT_DIR/systemd"
  emit_systemd_templates "$SYSTEMD_DIR" "$REPO_ROOT" "$SCHEDULE" "$RUNNERS" "$RUNTIME_CMD" "$MAX_CYCLES"
fi

DREAM_STATUS="not-requested"
if [[ "$RUN_DREAM" == true ]]; then
  if [[ "$EXECUTE" != true ]]; then
    DREAM_STATUS="planned"
  else
    dream_args=(overnight start --warn-only --max-iterations 1 --output-dir "$OUTPUT_DIR/dream")
    dream_args+=(--goal "private local nightly Dream before RPI/evolve")
    for runner in "${RUNNER_ARRAY[@]}"; do
      dream_args+=(--runner "$runner")
    done
    if ao "${dream_args[@]}" >"$OUTPUT_DIR/dream.log" 2>&1; then
      DREAM_STATUS="ok"
    else
      DREAM_STATUS="failed"
      die "Dream phase failed; see $OUTPUT_DIR/dream.log"
    fi
  fi
fi

EVOLVE_STATUS="not-requested"
if [[ "$RUN_EVOLVE" == true ]]; then
  if [[ "$EXECUTE" != true ]]; then
    EVOLVE_STATUS="planned"
  else
    export AGENTOPS_RPI_RUNTIME_MODE="$RUNTIME_MODE"
    export AGENTOPS_RPI_RUNTIME_COMMAND="$RUNTIME_CMD"
    if [[ -n "$BRANCH" ]]; then
      evolve_args=(--max-cycles "$MAX_CYCLES" --gate-policy "$GATE_POLICY" --landing-policy "$LANDING_POLICY" --landing-branch "$BRANCH")
    else
      evolve_args=(--max-cycles "$MAX_CYCLES" --gate-policy "$GATE_POLICY" --landing-policy "$LANDING_POLICY")
    fi
    if "$REPO_ROOT/scripts/ao-rpi-autonomous-cycle.sh" "${evolve_args[@]}" >"$OUTPUT_DIR/evolve.log" 2>&1; then
      EVOLVE_STATUS="ok"
    else
      EVOLVE_STATUS="failed"
      die "Evolve phase failed; see $OUTPUT_DIR/evolve.log"
    fi
  fi
fi

DIGEST_JSON="$OUTPUT_DIR/digest.json"
DIGEST_MD="$OUTPUT_DIR/digest.md"

jq -n \
  --arg schema_version "1" \
  --arg run_id "$RUN_ID" \
  --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg mode "$MODE" \
  --arg repo_root "$REPO_ROOT" \
  --arg run_date "$RUN_DATE" \
  --arg branch "$BRANCH" \
  --arg runners "$RUNNERS" \
  --arg runtime_cmd "$RUNTIME_CMD" \
  --arg runtime_mode "$RUNTIME_MODE" \
  --arg ai_sane_status "$AI_SANE_STATUS" \
  --arg brief_status "$BRIEF_STATUS" \
  --arg dream_status "$DREAM_STATUS" \
  --arg evolve_status "$EVOLVE_STATUS" \
  --arg output_dir "$OUTPUT_DIR" \
  --arg systemd_dir "$SYSTEMD_DIR" \
  --slurpfile ai "$AI_SANE_JSON" \
  --slurpfile dreamSetup "$DREAM_SETUP_JSON" \
  --slurpfile openPrs "$OPEN_PRS_JSON" \
  --rawfile runtimeInventory "$RUNTIME_TSV" '
  {
    schema_version: ($schema_version | tonumber),
    run_id: $run_id,
    generated_at: $generated_at,
    mode: $mode,
    repo_root: $repo_root,
    run_date: $run_date,
    planned_branch: $branch,
    runners: ($runners | split(",")),
    runtime: {
      command: $runtime_cmd,
      mode: $runtime_mode,
      inventory_tsv: $runtimeInventory
    },
    readiness: {
      ai_sane_status: $ai_sane_status,
      ai_sane: ($ai[0] // {}),
      dream_setup: ($dreamSetup[0] // {})
    },
    github: {
      open_prs: ($openPrs[0] // [])
    },
    phases: {
      nightly_brief: $brief_status,
      dream: $dream_status,
      evolve: $evolve_status
    },
    artifacts: {
      output_dir: $output_dir,
      digest_json: ($output_dir + "/digest.json"),
      digest_md: ($output_dir + "/digest.md"),
      systemd_dir: (if $systemd_dir == "" then null else $systemd_dir end)
    }
  }' >"$DIGEST_JSON"

write_markdown_digest "$DIGEST_MD" "$RUN_ID" "$MODE" "$BRANCH" "$AI_SANE_STATUS" "$DREAM_STATUS" "$EVOLVE_STATUS" "$SYSTEMD_DIR"

log "digest=$DIGEST_JSON"
log "done"
