#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TMP_ROOT="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

write_full_fixture() {
  local path="$1"
  cat >"$path" <<'SH'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "help" ] || [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ] || [ "${2:-}" = "--help" ] || [ "${2:-}" = "-h" ] || [ "${2:-}" = "help" ]; then
  cat <<'EOF_HELP'
AgentOps fixture

Usage: ao-fixture [command]

Available Commands:
  status      Show status
  eval        Run evals
  doctor      Check health

Flags:
  -h, --help  help for ao-fixture
EOF_HELP
  exit 0
fi
case "${1:-}" in
  status) echo "status ok" ;;
  eval) echo "eval ok" ;;
  doctor) echo "doctor ok" ;;
  *) echo "unknown"; exit 1 ;;
esac
SH
  chmod +x "$path"
}

write_missing_status_fixture() {
  local path="$1"
  cat >"$path" <<'SH'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "help" ] || [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ] || [ "${2:-}" = "--help" ] || [ "${2:-}" = "-h" ] || [ "${2:-}" = "help" ]; then
  cat <<'EOF_HELP'
AgentOps fixture

Usage: ao-fixture [command]

Available Commands:
  eval        Run evals
  doctor      Check health

Flags:
  -h, --help  help for ao-fixture
EOF_HELP
  exit 0
fi
exit 0
SH
  chmod +x "$path"
}

write_status_only_fixture() {
  local path="$1"
  cat >"$path" <<'SH'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "help" ] || [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ] || [ "${2:-}" = "--help" ] || [ "${2:-}" = "-h" ] || [ "${2:-}" = "help" ]; then
  cat <<'EOF_HELP'
AgentOps fixture

Usage: ao-fixture [command]

Available Commands:
  status      Show status

Flags:
  -h, --help  help for ao-fixture
EOF_HELP
  exit 0
fi
exit 0
SH
  chmod +x "$path"
}

write_policy_fixture() {
  local path="$1"
  cat >"$path" <<'JSON'
{
  "required_top_level_commands": ["status"],
  "deny_command_patterns": [
    "(^|\\s)--unsafe($|\\s)",
    "(^|\\s)debug-shell($|\\s)"
  ],
  "max_created_files": 50,
  "forbid_file_path_patterns": [
    "(^|/)\\.ssh(/|$)",
    "(^|/)Library/Keychains(/|$)",
    "(^|/)id_rsa($|\\.)"
  ],
  "allow_network_endpoint_patterns": [],
  "deny_network_endpoint_patterns": [],
  "block_if_removed_commands": true,
  "min_command_count": 1
}
JSON
}

run_security_suite() {
  python3 "$REPO_ROOT/skills/security-suite/scripts/security_suite.py" run "$@"
}

case "${1:-}" in
  policy-pass)
    fixture="$TMP_ROOT/ao-fixture"
    policy="$TMP_ROOT/policy.json"
    write_full_fixture "$fixture"
    write_policy_fixture "$policy"
    run_security_suite \
      --binary "$fixture" \
      --out-dir "$TMP_ROOT/out" \
      --policy-file "$policy" \
      --fail-on-policy-fail \
      --max-depth 1 \
      --total-timeout 15 \
      --per-cmd-timeout 2 \
      --timeout 3
    jq -e '.policy_verdict == "PASS" and .command_count >= 3' "$TMP_ROOT/out/suite-summary.json" >/dev/null
    jq -e '.top_level_commands | index("status")' "$TMP_ROOT/out/contract/contract.json" >/dev/null
    jq -e '.verdict == "PASS" and .finding_count == 0' "$TMP_ROOT/out/policy/policy-verdict.json" >/dev/null
    test -s "$TMP_ROOT/out/static/static-analysis.json"
    test -s "$TMP_ROOT/out/dynamic/dynamic-analysis.json"
    test -s "$TMP_ROOT/out/suite-summary.md"
    echo "security-suite-policy-pass-ok"
    ;;
  policy-fail)
    fixture="$TMP_ROOT/ao-fixture"
    policy="$TMP_ROOT/policy.json"
    write_missing_status_fixture "$fixture"
    write_policy_fixture "$policy"
    if run_security_suite \
      --binary "$fixture" \
      --out-dir "$TMP_ROOT/out" \
      --policy-file "$policy" \
      --fail-on-policy-fail \
      --max-depth 1 \
      --total-timeout 15 \
      --per-cmd-timeout 2 \
      --timeout 3; then
      echo "expected policy failure" >&2
      exit 1
    fi
    jq -e '.verdict == "FAIL" and (.findings[] | select(.code == "missing_required_commands"))' "$TMP_ROOT/out/policy/policy-verdict.json" >/dev/null
    jq -e '.policy_verdict == "FAIL"' "$TMP_ROOT/out/suite-summary.json" >/dev/null
    echo "security-suite-policy-fail-ok"
    ;;
  baseline-removed)
    fixture="$TMP_ROOT/ao-fixture"
    write_status_only_fixture "$fixture"
    mkdir -p "$TMP_ROOT/base/contract"
    cat >"$TMP_ROOT/base/contract/contract.json" <<'JSON'
{
  "schema_version": 1,
  "binary_sha256": "baseline",
  "runtime_guess": ["shell"],
  "command_paths": ["legacy", "status"],
  "top_level_commands": ["legacy", "status"]
}
JSON
    if run_security_suite \
      --binary "$fixture" \
      --out-dir "$TMP_ROOT/current" \
      --baseline-dir "$TMP_ROOT/base" \
      --fail-on-removed \
      --max-depth 1 \
      --total-timeout 15 \
      --per-cmd-timeout 2 \
      --timeout 3; then
      echo "expected removed-command failure" >&2
      exit 1
    fi
    jq -e '.status == "fail" and (.removed | index("legacy"))' "$TMP_ROOT/current/compare/baseline-diff.json" >/dev/null
    jq -e '.baseline_status == "fail" and .removed_commands == 1' "$TMP_ROOT/current/suite-summary.json" >/dev/null
    echo "security-suite-baseline-removed-ok"
    ;;
  redteam-repo)
    python3 "$REPO_ROOT/skills/security-suite/scripts/prompt_redteam.py" scan \
      --repo-root "$REPO_ROOT" \
      --pack-file "$REPO_ROOT/skills/security-suite/references/agentops-redteam-pack.json" \
      --out-dir "$TMP_ROOT/redteam" >/dev/null
    jq -e '.verdict == "PASS" and .case_count == 6 and (.results[].id | select(. == "policy-gated-security-suite"))' "$TMP_ROOT/redteam/redteam/redteam-results.json" >/dev/null
    test -s "$TMP_ROOT/redteam/redteam/redteam-results.md"
    echo "security-suite-redteam-ok"
    ;;
  *)
    echo "usage: $0 policy-pass|policy-fail|baseline-removed|redteam-repo" >&2
    exit 2
    ;;
esac
