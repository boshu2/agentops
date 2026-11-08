#!/usr/bin/env bash
# macos-no-runner-classify.sh — decide whether a workflow run failed only
# because GitHub never assigned a hosted macOS runner to some of its jobs.
#
# Reads a `GET /repos/{owner}/{repo}/actions/runs/{run_id}/jobs` response on
# stdin and prints a one-line JSON summary:
#   {"bad":N,"no_runner":[job names],"real":[job names]}
#
# A no-runner macOS job is cancelled, has runner_id 0/null, ran zero steps and
# targets a macos* label. Every other failed, cancelled or timed-out job is a
# real failure.
#
# Exit 0 when the run is retry-eligible (at least one bad job and all bad jobs
# are no-runner macOS jobs); exit 1 otherwise; exit 2 on unreadable input.
#
# Consumer: .github/workflows/macos-no-runner-retry.yml.

set -euo pipefail

summary="$(jq -c '
  def no_runner_macos:
    .conclusion == "cancelled"
    and ((.runner_id // 0) == 0)
    and (((.steps // []) | length) == 0)
    and any((.labels // [])[]; startswith("macos"));
  [(.jobs // [])[] | select(.conclusion == "failure" or .conclusion == "cancelled" or .conclusion == "timed_out")]
  | {
      bad: length,
      no_runner: [.[] | select(no_runner_macos) | .name],
      real: [.[] | select(no_runner_macos | not) | .name]
    }')" || exit 2
[[ -n "$summary" ]] || exit 2

printf '%s\n' "$summary"
jq -e '.bad > 0 and (.real | length) == 0' <<<"$summary" >/dev/null
