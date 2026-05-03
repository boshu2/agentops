#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/nightly-rpi-brief.sh"

    TMP_DIR="$(mktemp -d)"
    MOCK_BIN="$TMP_DIR/bin"
    FAKE_REPO="$TMP_DIR/repo"
    FIXTURE_DIR="$TMP_DIR/fixtures"
    mkdir -p "$MOCK_BIN" "$FAKE_REPO/scripts" "$FIXTURE_DIR"

    cp "$SCRIPT" "$FAKE_REPO/scripts/nightly-rpi-brief.sh"
    chmod +x "$FAKE_REPO/scripts/nightly-rpi-brief.sh"

    git -C "$FAKE_REPO" init -q
    git -C "$FAKE_REPO" config user.email test@example.com
    git -C "$FAKE_REPO" config user.name Test
    touch "$FAKE_REPO/README.md"
    git -C "$FAKE_REPO" add README.md
    git -C "$FAKE_REPO" commit -q -m init

    write_fixtures
    write_mock_gh
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_fixtures() {
    cat >"$FIXTURE_DIR/nightly-prs.json" <<'JSON'
[
  {
    "number": 301,
    "title": "Nightly 2026-05-02",
    "state": "MERGED",
    "mergedAt": "2026-05-02T12:00:00Z",
    "createdAt": "2026-05-02T08:00:00Z",
    "headRefName": "nightly/2026-05-02",
    "baseRefName": "main",
    "url": "https://example.test/pull/301",
    "body": "runtime-artifact latest.json corpus-state stale next-work tag push",
    "additions": 10,
    "deletions": 2,
    "changedFiles": 3
  }
]
JSON

    cat >"$FIXTURE_DIR/nightly-runs.json" <<'JSON'
[
  {
    "databaseId": 501,
    "displayTitle": "Nightly",
    "createdAt": "2026-05-02T08:30:00Z",
    "conclusion": "failure",
    "event": "schedule",
    "headBranch": "main",
    "headSha": "abc123",
    "url": "https://example.test/actions/runs/501",
    "status": "completed"
  }
]
JSON

    cat >"$FIXTURE_DIR/validate-runs.json" <<'JSON'
[
  {
    "databaseId": 601,
    "displayTitle": "main breaking change",
    "createdAt": "2026-05-03T02:00:00Z",
    "conclusion": "failure",
    "event": "push",
    "headBranch": "main",
    "headSha": "def456",
    "url": "https://example.test/actions/runs/601",
    "status": "completed"
  },
  {
    "databaseId": 600,
    "displayTitle": "older green run",
    "createdAt": "2026-05-02T02:00:00Z",
    "conclusion": "success",
    "event": "push",
    "headBranch": "main",
    "headSha": "def123",
    "url": "https://example.test/actions/runs/600",
    "status": "completed"
  }
]
JSON

    cat >"$FIXTURE_DIR/open-prs.json" <<'JSON'
[
  {
    "number": 212,
    "title": "feat(eval): baseline primitive",
    "headRefName": "feat/lid-baseline-ab-wave1",
    "baseRefName": "main",
    "isDraft": false,
    "mergeable": "MERGEABLE",
    "reviewDecision": "",
    "url": "https://example.test/pull/212",
    "labels": [],
    "statusCheckRollup": [
      {
        "__typename": "CheckRun",
        "completedAt": "2026-05-03T02:10:00Z",
        "conclusion": "FAILURE",
        "detailsUrl": "https://example.test/actions/runs/700/job/1",
        "name": "cli-docs-parity",
        "startedAt": "2026-05-03T02:05:00Z",
        "status": "COMPLETED",
        "workflowName": "Validate"
      },
      {
        "__typename": "CheckRun",
        "completedAt": "2026-05-03T02:20:00Z",
        "conclusion": "FAILURE",
        "detailsUrl": "https://example.test/actions/runs/700/job/2",
        "name": "agentops-eval-advisory (warn-only)",
        "startedAt": "2026-05-03T02:05:00Z",
        "status": "COMPLETED",
        "workflowName": "Validate"
      }
    ]
  }
]
JSON

    cat >"$FIXTURE_DIR/open-incidents.json" <<'JSON'
[
  {
    "number": 209,
    "title": "Nightly build failed: 2026-05-02",
    "createdAt": "2026-05-02T08:40:00Z",
    "updatedAt": "2026-05-02T08:40:00Z",
    "url": "https://example.test/issues/209",
    "labels": [{"name": "bug"}]
  }
]
JSON

    cat >"$FIXTURE_DIR/prompt-issues.json" <<'JSON'
[
  {
    "number": 210,
    "title": "Nightly RPI auto prompt",
    "createdAt": "2026-05-02T12:00:00Z",
    "updatedAt": "2026-05-02T12:07:00Z",
    "url": "https://example.test/issues/210"
  }
]
JSON
}

write_mock_gh() {
    cat >"$MOCK_BIN/gh" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail

args="$*"

if [[ "$1 $2" == "pr list" ]]; then
  if [[ "$args" == *"--state open"* ]]; then
    if [[ "${FAIL_OPTIONAL_GH:-}" == "1" ]]; then
      exit 42
    fi
    cat "${FIXTURE_DIR:?}/open-prs.json"
  else
    cat "${FIXTURE_DIR:?}/nightly-prs.json"
  fi
  exit 0
fi

if [[ "$1 $2" == "run list" ]]; then
  if [[ "$args" == *"--workflow Validate"* ]]; then
    if [[ "${FAIL_OPTIONAL_GH:-}" == "1" ]]; then
      exit 42
    fi
    cat "${FIXTURE_DIR:?}/validate-runs.json"
  else
    cat "${FIXTURE_DIR:?}/nightly-runs.json"
  fi
  exit 0
fi

if [[ "$1 $2" == "issue list" ]]; then
  if [[ "${FAIL_OPTIONAL_GH:-}" == "1" ]]; then
    exit 42
  fi
  if [[ "$args" == *"Nightly build failed"* ]]; then
    cat "${FIXTURE_DIR:?}/open-incidents.json"
  elif [[ "$args" == *"Nightly RPI auto prompt"* ]]; then
    cat "${FIXTURE_DIR:?}/prompt-issues.json"
  else
    printf '[]\n'
  fi
  exit 0
fi

printf 'unexpected gh invocation: %s\n' "$args" >&2
exit 2
STUB
    chmod +x "$MOCK_BIN/gh"
}

run_brief() {
    local out_dir="$1"
    shift
    run env PATH="$MOCK_BIN:$PATH" FIXTURE_DIR="$FIXTURE_DIR" "$@" \
        bash -c 'cd "$1" && scripts/nightly-rpi-brief.sh --since 2026-05-01 --output-dir "$2"' \
        _ "$FAKE_REPO" "$out_dir"
}

@test "captures current CI, open PRs, incidents, and prompt issue state" {
    run_brief "$TMP_DIR/out"

    [ "$status" -eq 0 ]
    jq -e '
      .current_ci.status == "available" and
      .current_ci.latest_main.conclusion == "failure" and
      .open_prs.total == 1 and
      .open_prs.blocking_total == 1 and
      .open_prs.list[0].blocking_checks == [{"name":"cli-docs-parity","conclusion":"FAILURE","detailsUrl":"https://example.test/actions/runs/700/job/1","workflowName":"Validate"}] and
      .open_prs.list[0].soft_failures[0].name == "agentops-eval-advisory (warn-only)" and
      .open_incidents.total == 1 and
      .prompt_issue.current.number == 210
    ' "$TMP_DIR/out/summary.json"
}

@test "ranks current blocking Validate ahead of recurring runtime-artifact noise" {
    run_brief "$TMP_DIR/out"

    [ "$status" -eq 0 ]
    jq -e '
      .recurring_signals.runtime_artifact_only == 1 and
      .recurring_signals.corpus_state_bound == 1 and
      .stabilization_targets[0].title == "Restore latest main Validate" and
      .stabilization_targets[0].rank == 1 and
      (.stabilization_targets | map(.title) | index("Convert recurring Nightly signals into code-backed work")) != null
    ' "$TMP_DIR/out/summary.json"
    grep -q '## Top Stabilization Target' "$TMP_DIR/out/brief.md"
    grep -q 'Restore latest main Validate' "$TMP_DIR/out/brief.md"
    grep -q 'Start from top stabilization target: Restore latest main Validate' "$TMP_DIR/out/prompt.txt"
}

@test "optional current-state probes fail open without aborting the brief" {
    run_brief "$TMP_DIR/out" FAIL_OPTIONAL_GH=1

    [ "$status" -eq 0 ]
    jq -e '
      .current_ci.status == "unavailable" and
      .open_prs.status == "unavailable" and
      .open_incidents.status == "unavailable" and
      .prompt_issue.status == "unavailable" and
      .prs.total == 1 and
      .runs.scheduled_failure == 1
    ' "$TMP_DIR/out/summary.json"
}
