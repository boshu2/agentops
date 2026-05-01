#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
nightly-rpi-brief.sh

Build a rolling Nightly evidence brief and a ready-to-run RPI auto prompt from
recent GitHub Nightly PRs and scheduled Nightly workflow runs.

Options:
  --since <YYYY-MM-DD>     Start date for the evidence window (default: 14 days ago)
  --output-dir <path>      Directory for report files (default: .agents/rpi/nightly-brief)
  --workflow <name>        GitHub Actions workflow name to inspect (default: Nightly)
  --pr-limit <n>           Maximum PRs to fetch (default: 100)
  --run-limit <n>          Maximum workflow runs to fetch (default: 50)
  --help                   Show this help

Outputs:
  <output-dir>/prs.json
  <output-dir>/runs.json
  <output-dir>/summary.json
  <output-dir>/prompt.txt
  <output-dir>/brief.md
EOF
}

die() {
  echo "nightly-rpi-brief: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

default_since() {
  if date -u -d '14 days ago' +%F >/dev/null 2>&1; then
    date -u -d '14 days ago' +%F
    return
  fi
  if date -u -v-14d +%F >/dev/null 2>&1; then
    date -u -v-14d +%F
    return
  fi
  die "could not compute default --since date"
}

SINCE=""
OUTPUT_DIR=".agents/rpi/nightly-brief"
WORKFLOW="Nightly"
PR_LIMIT="100"
RUN_LIMIT="50"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --since)
      SINCE="${2:-}"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="${2:-}"
      shift 2
      ;;
    --workflow)
      WORKFLOW="${2:-}"
      shift 2
      ;;
    --pr-limit)
      PR_LIMIT="${2:-}"
      shift 2
      ;;
    --run-limit)
      RUN_LIMIT="${2:-}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      die "unknown arg: $1"
      ;;
  esac
done

[[ -n "$SINCE" ]] || SINCE="$(default_since)"

if [[ ! "$SINCE" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
  die "--since must use YYYY-MM-DD format"
fi
if [[ ! "$PR_LIMIT" =~ ^[0-9]+$ ]]; then
  die "--pr-limit must be a positive integer"
fi
if (( PR_LIMIT < 1 )); then
  die "--pr-limit must be a positive integer"
fi
if [[ ! "$RUN_LIMIT" =~ ^[0-9]+$ ]]; then
  die "--run-limit must be a positive integer"
fi
if (( RUN_LIMIT < 1 )); then
  die "--run-limit must be a positive integer"
fi

require_cmd gh
require_cmd jq
require_cmd git

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
[[ -n "$REPO_ROOT" ]] || die "must run inside a git repository"
cd "$REPO_ROOT"

OUTPUT_DIR="$(mkdir -p "$OUTPUT_DIR" && cd "$OUTPUT_DIR" && pwd)"
PRS_JSON="$OUTPUT_DIR/prs.json"
RUNS_JSON="$OUTPUT_DIR/runs.json"
SUMMARY_JSON="$OUTPUT_DIR/summary.json"
PROMPT_TXT="$OUTPUT_DIR/prompt.txt"
BRIEF_MD="$OUTPUT_DIR/brief.md"

PR_SEARCH="Nightly in:title created:>=${SINCE}"
gh pr list \
  --state all \
  --limit "$PR_LIMIT" \
  --search "$PR_SEARCH" \
  --json number,title,state,mergedAt,createdAt,headRefName,baseRefName,url,body,additions,deletions,changedFiles \
  >"$PRS_JSON"

gh run list \
  --workflow "$WORKFLOW" \
  --limit "$RUN_LIMIT" \
  --json databaseId,displayTitle,createdAt,conclusion,event,headBranch,headSha,url,status \
  >"$RUNS_JSON"

GENERATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
TODAY="$(date -u +%F)"

jq -n \
  --arg since "$SINCE" \
  --arg generated_at "$GENERATED_AT" \
  --slurpfile prs "$PRS_JSON" \
  --slurpfile runs "$RUNS_JSON" '
  def prs: $prs[0];
  def runs: $runs[0];
  def pr_texts: prs | map(((.title // "") + "\n" + (.body // "")));
  def count_prs($re): [pr_texts[] | select(test($re; "i"))] | length;
  def scheduled_runs: [
    runs[]
    | select((.event // "") == "schedule")
    | select((.createdAt // "") >= ($since + "T00:00:00Z"))
  ];
  {
    since: $since,
    generated_at: $generated_at,
    prs: {
      total: (prs | length),
      open: ([prs[] | select(.state == "OPEN")] | length),
      merged: ([prs[] | select(.state == "MERGED")] | length),
      changed_files: ([prs[] | .changedFiles // 0] | add // 0),
      additions: ([prs[] | .additions // 0] | add // 0),
      deletions: ([prs[] | .deletions // 0] | add // 0),
      list: (
        prs
        | sort_by(.createdAt)
        | reverse
        | map({
            number,
            title,
            state,
            createdAt,
            mergedAt,
            url,
            changedFiles,
            additions,
            deletions
          })
      )
    },
    runs: {
      workflow_total: (runs | length),
      scheduled_total: (scheduled_runs | length),
      scheduled_success: ([scheduled_runs[] | select(.conclusion == "success")] | length),
      scheduled_failure: ([scheduled_runs[] | select(.conclusion == "failure")] | length),
      failures: (
        scheduled_runs
        | map(select(.conclusion != "success"))
        | map({createdAt, conclusion, url, headSha})
      )
    },
    recurring_signals: {
      stale_next_work: count_prs("stale|staleness|next-work|dream packet|probed_stale_at|mark-probed"),
      runtime_artifact_only: count_prs("runtime-artifact|compile-freshness|compile-no-oscillation|latest[.]json|gitignored"),
      corpus_state_bound: count_prs("corpus-state|flywheel-compounding|rho|sigma|quarantine|quarantined"),
      bd_degraded: count_prs("bd unavailable|beads unavailable|tracker degradation|bd install"),
      tag_push_friction: count_prs("tag push|403|sideband|tag rejected|tag namespace"),
      worktree_disposition: count_prs("worktree-disposition|foreign worktree|canonical root"),
      eval_advisory: count_prs("eval advisory|scorecard|baseline audit|suite hash|agentops-eval"),
      security_toolchain: count_prs("security toolchain|gosec|semgrep|security gate|blocked_high|blocked_critical")
    }
  }
' >"$SUMMARY_JSON"

cat >"$PROMPT_TXT" <<EOF
\$agentops:rpi --auto "Use Nightly evidence from ${SINCE} through ${TODAY} before selecting work. First inspect open Nightly PRs, latest scheduled Nightly failures, and current CI. Choose one code-driven change that reduces repeated stale next-work, security/eval advisory debt, closeout friction, or prompt/runtime drift. Treat runtime-artifact and corpus-state flips as diagnostics, not success. Avoid flywheel-compounding unless corpus-active preconditions are true. If bd is unavailable, record tracker degradation and continue with an explicit issue-free fallback. Validate with the smallest relevant gate plus scripts/pre-push-gate.sh --fast, commit, rebase, push, and report evidence."
EOF

pr_total="$(jq -r '.prs.total' "$SUMMARY_JSON")"
pr_open="$(jq -r '.prs.open' "$SUMMARY_JSON")"
pr_merged="$(jq -r '.prs.merged' "$SUMMARY_JSON")"
run_total="$(jq -r '.runs.scheduled_total' "$SUMMARY_JSON")"
run_success="$(jq -r '.runs.scheduled_success' "$SUMMARY_JSON")"
run_failure="$(jq -r '.runs.scheduled_failure' "$SUMMARY_JSON")"

{
  printf '# Nightly RPI Brief\n\n'
  printf 'Generated: `%s`\n\n' "$GENERATED_AT"
  printf 'Evidence window: `%s` through `%s`\n\n' "$SINCE" "$TODAY"
  printf '## Snapshot\n\n'
  printf '%s\n' "- Nightly PRs: \`$pr_total\` total, \`$pr_merged\` merged, \`$pr_open\` open"
  printf '%s\n' "- Scheduled Nightly runs: \`$run_total\` total, \`$run_success\` success, \`$run_failure\` failure"
  printf '\n## Recurring Signals\n\n'
  printf '| Signal | PR count |\n'
  printf '|--------|----------|\n'
  printf '| Stale next-work / Dream packets | %s |\n' "$(jq -r '.recurring_signals.stale_next_work' "$SUMMARY_JSON")"
  printf '| Runtime-artifact-only wins | %s |\n' "$(jq -r '.recurring_signals.runtime_artifact_only' "$SUMMARY_JSON")"
  printf '| Corpus-state-bound flywheel loops | %s |\n' "$(jq -r '.recurring_signals.corpus_state_bound' "$SUMMARY_JSON")"
  printf '| bd / tracker degradation | %s |\n' "$(jq -r '.recurring_signals.bd_degraded' "$SUMMARY_JSON")"
  printf '| Tag push friction | %s |\n' "$(jq -r '.recurring_signals.tag_push_friction' "$SUMMARY_JSON")"
  printf '| Worktree disposition friction | %s |\n' "$(jq -r '.recurring_signals.worktree_disposition' "$SUMMARY_JSON")"
  printf '| Eval advisory debt | %s |\n' "$(jq -r '.recurring_signals.eval_advisory' "$SUMMARY_JSON")"
  printf '| Security toolchain debt | %s |\n' "$(jq -r '.recurring_signals.security_toolchain' "$SUMMARY_JSON")"
  printf '\n## Recent Nightly PRs\n\n'
  jq -r '.prs.list[:20][] | "- #\(.number) \(.state) \(.createdAt[0:10]): \(.title) (\(.changedFiles) files, +\(.additions)/-\(.deletions))"' "$SUMMARY_JSON"
  printf '\n## Failed Scheduled Runs\n\n'
  if jq -e '.runs.failures | length > 0' "$SUMMARY_JSON" >/dev/null; then
    jq -r '.runs.failures[] | "- \(.createdAt[0:10]) \(.conclusion): \(.url)"' "$SUMMARY_JSON"
  else
    printf 'None in the evidence window.\n'
  fi
  printf '\n## Prompt\n\n'
  printf '```text\n'
  sed -n '1,20p' "$PROMPT_TXT"
  printf '```\n'
} >"$BRIEF_MD"

echo "Nightly RPI brief: $BRIEF_MD"
echo "Nightly RPI prompt: $PROMPT_TXT"
