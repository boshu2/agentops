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
  --validate-workflow <name> GitHub Actions workflow name for current CI (default: Validate)
  --help                   Show this help

Outputs:
  <output-dir>/prs.json
  <output-dir>/runs.json
  <output-dir>/validate-runs.json
  <output-dir>/open-prs.json
  <output-dir>/open-incidents.json
  <output-dir>/prompt-issues.json
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

capture_optional_array() {
  local out_file="$1"
  shift
  local tmp_file="${out_file}.tmp"

  if "$@" >"$tmp_file" && jq -e 'type == "array"' "$tmp_file" >/dev/null 2>&1; then
    jq -n --slurpfile items "$tmp_file" '{status: "available", items: ($items[0] // [])}' >"$out_file"
  else
    jq -n '{status: "unavailable", items: []}' >"$out_file"
  fi
  rm -f "$tmp_file"
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
VALIDATE_WORKFLOW="Validate"
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
    --validate-workflow)
      VALIDATE_WORKFLOW="${2:-}"
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
VALIDATE_RUNS_JSON="$OUTPUT_DIR/validate-runs.json"
OPEN_PRS_JSON="$OUTPUT_DIR/open-prs.json"
OPEN_INCIDENTS_JSON="$OUTPUT_DIR/open-incidents.json"
PROMPT_ISSUES_JSON="$OUTPUT_DIR/prompt-issues.json"
SUMMARY_JSON="$OUTPUT_DIR/summary.json"
PROMPT_TXT="$OUTPUT_DIR/prompt.txt"
BRIEF_MD="$OUTPUT_DIR/brief.md"
PROMPT_ISSUE_TITLE="Nightly RPI auto prompt"

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

capture_optional_array "$VALIDATE_RUNS_JSON" \
  gh run list \
    --workflow "$VALIDATE_WORKFLOW" \
    --limit 20 \
    --json databaseId,displayTitle,createdAt,conclusion,event,headBranch,headSha,url,status

capture_optional_array "$OPEN_PRS_JSON" \
  gh pr list \
    --state open \
    --limit 30 \
    --json number,title,headRefName,baseRefName,isDraft,mergeable,reviewDecision,statusCheckRollup,url,labels

capture_optional_array "$OPEN_INCIDENTS_JSON" \
  gh issue list \
    --state open \
    --search '"Nightly build failed" in:title' \
    --limit 20 \
    --json number,title,createdAt,updatedAt,url,labels

capture_optional_array "$PROMPT_ISSUES_JSON" \
  gh issue list \
    --state open \
    --search "\"${PROMPT_ISSUE_TITLE}\" in:title" \
    --limit 10 \
    --json number,title,createdAt,updatedAt,url

GENERATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
TODAY="$(date -u +%F)"

jq -n \
  --arg since "$SINCE" \
  --arg generated_at "$GENERATED_AT" \
  --arg validate_workflow "$VALIDATE_WORKFLOW" \
  --arg prompt_issue_title "$PROMPT_ISSUE_TITLE" \
  --slurpfile prs "$PRS_JSON" \
  --slurpfile runs "$RUNS_JSON" \
  --slurpfile validate_runs "$VALIDATE_RUNS_JSON" \
  --slurpfile open_prs "$OPEN_PRS_JSON" \
  --slurpfile open_incidents "$OPEN_INCIDENTS_JSON" \
  --slurpfile prompt_issues "$PROMPT_ISSUES_JSON" '
  def prs: $prs[0];
  def runs: $runs[0];
  def validate_status: ($validate_runs[0].status // "unavailable");
  def validate_runs: ($validate_runs[0].items // []);
  def open_prs_status: ($open_prs[0].status // "unavailable");
  def open_prs: ($open_prs[0].items // []);
  def incidents_status: ($open_incidents[0].status // "unavailable");
  def incidents: ($open_incidents[0].items // []);
  def prompt_status: ($prompt_issues[0].status // "unavailable");
  def prompt_issues: ($prompt_issues[0].items // []);
  def pr_texts: prs | map(((.title // "") + "\n" + (.body // "")));
  def count_prs($re): [pr_texts[] | select(test($re; "i"))] | length;
  def is_soft_check($name):
    ($name // "" | test("warn-only|agentops-eval-advisory|security-toolchain-gate|doctor-check|check-test-staleness"; "i"));
  def blocking_checks($checks): [
    $checks[]?
    | select((.status // "") == "COMPLETED")
    | select((.conclusion // "") == "FAILURE")
    | select((is_soft_check(.name) | not))
    | {name, conclusion, detailsUrl, workflowName}
  ];
  def soft_checks($checks): [
    $checks[]?
    | select((.status // "") == "COMPLETED")
    | select((.conclusion // "") == "FAILURE")
    | select(is_soft_check(.name))
    | {name, conclusion, detailsUrl, workflowName}
  ];
  def scheduled_runs: [
    runs[]
    | select((.event // "") == "schedule")
    | select((.createdAt // "") >= ($since + "T00:00:00Z"))
  ];
  def validate_recent:
    validate_runs
    | sort_by(.createdAt // "")
    | reverse
    | map({
        databaseId,
        displayTitle,
        createdAt,
        conclusion,
        event,
        headBranch,
        headSha,
        url,
        status
      });
  def latest_main:
    ([validate_recent[] | select((.headBranch // "") == "main")] | .[0]) // null;
  def pr_summaries:
    open_prs
    | sort_by(.number // 0)
    | reverse
    | map({
        number,
        title,
        url,
        isDraft,
        mergeable,
        reviewDecision,
        headRefName,
        baseRefName,
        blocking_checks: blocking_checks(.statusCheckRollup // []),
        soft_failures: soft_checks(.statusCheckRollup // [])
      });
  def prompt_issue:
    ([prompt_issues[] | select((.title // "") == $prompt_issue_title)] | .[0]) // (prompt_issues[0] // null);
  def target($score; $title; $reason; $evidence; $suggested_files):
    {
      score: $score,
      title: $title,
      reason: $reason,
      evidence: $evidence,
      suggested_files: $suggested_files
    };
  def base_summary:
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
      },
      current_ci: {
        status: validate_status,
        workflow: $validate_workflow,
        latest_main: latest_main,
        recent_failures: (
          validate_recent
          | map(select((.conclusion // "") != "success"))
          | .[:10]
        )
      },
      open_prs: {
        status: open_prs_status,
        total: (pr_summaries | length),
        blocking_total: ([pr_summaries[] | select((.blocking_checks | length) > 0)] | length),
        list: pr_summaries
      },
      open_incidents: {
        status: incidents_status,
        total: (incidents | length),
        list: (
          incidents
          | sort_by(.createdAt // "")
          | reverse
          | map({number, title, createdAt, updatedAt, url, labels})
        )
      },
      prompt_issue: {
        status: prompt_status,
        title: $prompt_issue_title,
        current: prompt_issue
      }
    };
  base_summary as $summary
  | $summary + {
      stabilization_targets: (
        [
          (
            if ($summary.current_ci.latest_main != null and (($summary.current_ci.latest_main.conclusion // "") != "success")) then
              target(
                100;
                "Restore latest main Validate";
                "Latest Validate run on main concluded \($summary.current_ci.latest_main.conclusion // "unknown") for \($summary.current_ci.latest_main.displayTitle // "unknown run").";
                [$summary.current_ci.latest_main.url] | map(select(. != null));
                []
              )
            else empty end
          ),
          (
            $summary.open_prs.list[]?
            | select((.blocking_checks | length) > 0)
            | target(
                90;
                "Unblock PR #\(.number) Validate";
                "Open PR has blocking failed checks: \([.blocking_checks[].name] | join(", ")).";
                ([.url] + [.blocking_checks[].detailsUrl]) | map(select(. != null));
                []
              )
          ),
          (
            if (($summary.open_incidents.total // 0) > 0 and (($summary.runs.scheduled_failure // 0) > 0)) then
              target(
                80;
                "Close open Nightly failure issues";
                "\($summary.open_incidents.total) open Nightly failure issue(s) remain while \($summary.runs.scheduled_failure) scheduled Nightly run(s) failed in the evidence window.";
                ([($summary.open_incidents.list[]?.url), ($summary.runs.failures[]?.url)] | map(select(. != null)));
                ["scripts/nightly-rpi-brief.sh", ".github/workflows/nightly.yml"]
              )
            else empty end
          ),
          (
            if (($summary.recurring_signals.tag_push_friction // 0) > 0) then
              target(
                40;
                "Audit release tag friction";
                "Nightly PR evidence repeatedly mentions tag push or tag selection friction; keep below current CI blockers unless it has a live failing gate.";
                [];
                ["scripts/release-size-check.sh", "scripts/uat-local-release-dry-run.sh"]
              )
            else empty end
          ),
          (
            if ((($summary.recurring_signals.runtime_artifact_only // 0) + ($summary.recurring_signals.corpus_state_bound // 0) + ($summary.recurring_signals.stale_next_work // 0)) > 0) then
              target(
                20;
                "Convert recurring Nightly signals into code-backed work";
                "Runtime-artifact, corpus-state, or stale next-work signals are diagnostic only until they map to source files, failing gates, or open issues.";
                [];
                ["scripts/nightly-rpi-brief.sh", ".agents/rpi/next-work.jsonl"]
              )
            else empty end
          )
        ]
        | sort_by(-.score)
        | to_entries
        | map(.value + {rank: (.key + 1)} | del(.score))
      )
    }
' >"$SUMMARY_JSON"

top_target="$(jq -r '.stabilization_targets[0].title // "No current stabilization target"' "$SUMMARY_JSON")"
top_reason="$(jq -r '.stabilization_targets[0].reason // "No current blocking evidence found."' "$SUMMARY_JSON")"

cat >"$PROMPT_TXT" <<EOF
\$agentops:rpi --auto "Use Nightly evidence from ${SINCE} through ${TODAY} before selecting work. Start from top stabilization target: ${top_target} - ${top_reason} Only override it with fresher blocking evidence from current CI, open Nightly failure issues, or open PR check rollups. Treat runtime-artifact and corpus-state flips as diagnostics, not success. Avoid flywheel-compounding unless corpus-active preconditions are true. If bd is unavailable, record tracker degradation and continue with an explicit issue-free fallback. Validate with the smallest relevant gate plus scripts/pre-push-gate.sh --fast, commit, rebase, push, and report evidence."
EOF

pr_total="$(jq -r '.prs.total' "$SUMMARY_JSON")"
pr_open="$(jq -r '.prs.open' "$SUMMARY_JSON")"
pr_merged="$(jq -r '.prs.merged' "$SUMMARY_JSON")"
run_total="$(jq -r '.runs.scheduled_total' "$SUMMARY_JSON")"
run_success="$(jq -r '.runs.scheduled_success' "$SUMMARY_JSON")"
run_failure="$(jq -r '.runs.scheduled_failure' "$SUMMARY_JSON")"
main_status="$(jq -r '.current_ci.latest_main.conclusion // .current_ci.status' "$SUMMARY_JSON")"
open_incidents="$(jq -r '.open_incidents.total' "$SUMMARY_JSON")"
blocking_prs="$(jq -r '.open_prs.blocking_total' "$SUMMARY_JSON")"

{
  printf '# Nightly RPI Brief\n\n'
  printf 'Generated: `%s`\n\n' "$GENERATED_AT"
  printf 'Evidence window: `%s` through `%s`\n\n' "$SINCE" "$TODAY"
  printf '## Snapshot\n\n'
  printf '%s\n' "- Nightly PRs: \`$pr_total\` total, \`$pr_merged\` merged, \`$pr_open\` open"
  printf '%s\n' "- Scheduled Nightly runs: \`$run_total\` total, \`$run_success\` success, \`$run_failure\` failure"
  printf '%s\n' "- Latest main Validate: \`$main_status\`"
  printf '%s\n' "- Open PRs with blocking checks: \`$blocking_prs\`"
  printf '%s\n' "- Open Nightly failure issues: \`$open_incidents\`"
  printf '\n## Top Stabilization Target\n\n'
  if jq -e '.stabilization_targets | length > 0' "$SUMMARY_JSON" >/dev/null; then
    jq -r '.stabilization_targets[0] | "- #\(.rank) \(.title): \(.reason)"' "$SUMMARY_JSON"
    jq -r '.stabilization_targets[0].evidence[]? | "  - Evidence: \(.)"' "$SUMMARY_JSON"
  else
    printf 'No current stabilization target found.\n'
  fi
  printf '\n## Current CI\n\n'
  if jq -e '.current_ci.latest_main != null' "$SUMMARY_JSON" >/dev/null; then
    jq -r '.current_ci.latest_main | "- main \(.conclusion // .status // "unknown") \(.createdAt[0:10]): \(.displayTitle) (\(.url))"' "$SUMMARY_JSON"
  else
    printf 'Current Validate data unavailable.\n'
  fi
  if jq -e '.open_prs.list[]? | select((.blocking_checks | length) > 0)' "$SUMMARY_JSON" >/dev/null; then
    jq -r '.open_prs.list[] | select((.blocking_checks | length) > 0) | "- PR #\(.number) blocking checks: \([.blocking_checks[].name] | join(", ")) (\(.url))"' "$SUMMARY_JSON"
  fi
  printf '\n## Open Nightly Failure Issues\n\n'
  if jq -e '.open_incidents.list | length > 0' "$SUMMARY_JSON" >/dev/null; then
    jq -r '.open_incidents.list[] | "- #\(.number) \(.title) (\(.url))"' "$SUMMARY_JSON"
  else
    printf 'None open.\n'
  fi
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
