#!/usr/bin/env bats
#
# Tests for the metrics-harness capture (age-5l6c):
# scripts/capture-repo-metrics.sh snapshots GitHub repo + traffic metrics into a
# dated JSONL record. Uses a `gh` STUB on PATH so the test is hermetic (no
# network, no real auth). Verifies the record shape, the append, and that the
# script fails CLOSED (no partial record) when gh is unauthenticated or an
# endpoint fetch fails.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/capture-repo-metrics.sh"
    FIX="$BATS_TEST_TMPDIR/repo"
    mkdir -p "$FIX/bin"
    # repo-root.sh resolves the REAL checkout unless overridden; without this
    # the script under test writes into the live repo (fixture-pollution class).
    export AGENTOPS_REPO_ROOT="$FIX"
    git -C "$FIX" init -q 2>/dev/null || { mkdir -p "$FIX"; git -C "$FIX" init -q; }
    git -C "$FIX" config user.email t@t.t
    git -C "$FIX" config user.name t
    # gh stub: dispatches on args. GH_AUTH_RC and GH_VIEWS_RC let a case force a
    # failure to exercise the fail-closed paths.
    cat > "$FIX/bin/gh" <<'STUB'
#!/usr/bin/env bash
if [ "$1 $2" = "auth status" ]; then exit "${GH_AUTH_RC:-0}"; fi
if [ "$1" = "api" ]; then
  case "$2" in
    */traffic/views)
      [ "${GH_VIEWS_RC:-0}" -ne 0 ] && exit "${GH_VIEWS_RC}"
      echo '{"count":993,"uniques":485}' ;;
    */traffic/clones)
      [ "${GH_CLONES_BAD:-0}" -ne 0 ] && { echo '{}'; exit 0; }
      echo '{"count":43203,"uniques":10892}' ;;
    */traffic/popular/paths)     echo '[{"path":"/r/blob/main/.claude/workflows/operating-loop.js","count":37,"uniques":18}]' ;;
    */traffic/popular/referrers) echo '[{"referrer":"reddit.com","count":204,"uniques":144}]' ;;
    repos/*)                     echo '{"stargazers_count":399,"forks_count":40,"subscribers_count":4,"open_issues_count":3}' ;;
  esac
  exit 0
fi
exit 0
STUB
    chmod +x "$FIX/bin/gh"
    PATH="$FIX/bin:$PATH"
    export PATH
}

run_capture() { ( cd "$FIX" && bash "$SCRIPT" "$@" ); }

@test "dry-run emits a record with stars, views, and top_paths" {
    run run_capture --dry-run
    [ "$status" -eq 0 ]
    [[ "$output" == *'"stars":399'* ]]
    [[ "$output" == *'"views_14d":993'* ]]
    [[ "$output" == *'operating-loop.js'* ]]
    [[ "$output" == *'clones_note'* ]]
}

@test "append writes one line to docs/metrics/traffic.jsonl" {
    run run_capture
    [ "$status" -eq 0 ]
    [ -f "$FIX/docs/metrics/traffic.jsonl" ]
    [ "$(wc -l < "$FIX/docs/metrics/traffic.jsonl")" -eq 1 ]
    run run_capture
    [ "$(wc -l < "$FIX/docs/metrics/traffic.jsonl")" -eq 2 ]
}

@test "fails closed (no write) when gh is unauthenticated" {
    GH_AUTH_RC=1 run run_capture
    [ "$status" -ne 0 ]
    [ ! -f "$FIX/docs/metrics/traffic.jsonl" ]
}

@test "fails closed (no write) when an endpoint fetch fails" {
    GH_VIEWS_RC=1 run run_capture
    [ "$status" -ne 0 ]
    [ ! -f "$FIX/docs/metrics/traffic.jsonl" ]
}

# pawl/codex catch: a gh exit-0 payload with null scalars (e.g. {} for clones,
# a permission-limited response) must NOT append a junk record.
@test "fails closed (no write) when a scalar payload is garbled (null clones)" {
    GH_CLONES_BAD=1 run run_capture
    [ "$status" -ne 0 ]
    [[ "$output" == *"clones_14d"* ]]
    [ ! -f "$FIX/docs/metrics/traffic.jsonl" ]
}
