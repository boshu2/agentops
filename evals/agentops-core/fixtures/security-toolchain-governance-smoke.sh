#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TMP_ROOT="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

make_fixture_repo() {
  local name="$1"
  local repo="$TMP_ROOT/$name"
  mkdir -p "$repo/scripts" "$repo/mockbin"
  cp "$REPO_ROOT/scripts/toolchain-validate.sh" "$repo/scripts/toolchain-validate.sh"
  cp "$REPO_ROOT/scripts/security-gate.sh" "$repo/scripts/security-gate.sh"
  chmod +x "$repo/scripts/toolchain-validate.sh" "$repo/scripts/security-gate.sh"

  cat >"$repo/scripts/golangci-lint-v2.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${SECURITY_TOOLCHAIN_FIXTURE_GOLANGCI:-clean}" == "findings" ]]; then
  echo "main.go:1:1: fixture golangci quality finding"
  exit 1
fi
exit 0
SH
  chmod +x "$repo/scripts/golangci-lint-v2.sh"

  install_mock_tools "$repo/mockbin"

  (
    cd "$repo"
    git init -q
    git config user.email "agentops-fixture@example.invalid"
    git config user.name "AgentOps Fixture"
    printf 'print("fixture")\n' > sample.py
    printf 'fixture\n' > README.md
    git add README.md sample.py
    git commit -q -m "fixture base"
  )

  printf '%s\n' "$repo"
}

install_mock_tools() {
  local mockbin="$1"

  cat >"$mockbin/ruff" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${SECURITY_TOOLCHAIN_FIXTURE_RUFF:-clean}" == "findings" ]]; then
  echo "sample.py:1:1: F401 fixture quality finding"
  exit 1
fi
exit 0
SH

  cat >"$mockbin/radon" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
exit 0
SH

  cat >"$mockbin/semgrep" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
case "${SECURITY_TOOLCHAIN_FIXTURE_SEMGREP:-clean}" in
  critical)
    printf '{"results":[{"extra":{"severity":"ERROR"}}]}\n'
    ;;
  high)
    printf '{"results":[{"extra":{"severity":"WARNING"}}]}\n'
    ;;
  clean)
    printf '{"results":[]}\n'
    ;;
  *)
    echo "unknown semgrep fixture mode" >&2
    exit 2
    ;;
esac
SH

  cat >"$mockbin/gitleaks" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
exit 0
SH

  cat >"$mockbin/trivy" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "fs" && "${2:-}" == "--help" ]]; then
  echo "Usage: trivy fs [--db-repository]"
  exit 0
fi
printf '{"Results":[]}\n'
SH

  cat >"$mockbin/gosec" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf '{"Issues":[]}\n'
SH

  cat >"$mockbin/hadolint" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf '[]\n'
SH

  cat >"$mockbin/pytest" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
exit 0
SH

  cat >"$mockbin/go" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
exit 0
SH

  cat >"$mockbin/shellcheck" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
exit 0
SH

  chmod +x "$mockbin"/*
}

run_toolchain_case() {
  local name="$1"
  local semgrep_mode="$2"
  local ruff_mode="$3"
  local expected_exit="$4"
  local expected_gate="$5"
  local jq_assertion="$6"
  local repo
  repo="$(make_fixture_repo "$name")"
  local output_dir="$repo/out"
  local stdout_json="$repo/stdout.json"
  local stderr_log="$repo/stderr.log"

  set +e
  env \
    PATH="$repo/mockbin:/usr/bin:/bin" \
    TOOLCHAIN_OUTPUT_DIR="$output_dir" \
    SECURITY_TOOLCHAIN_FIXTURE_SEMGREP="$semgrep_mode" \
    SECURITY_TOOLCHAIN_FIXTURE_RUFF="$ruff_mode" \
    "$repo/scripts/toolchain-validate.sh" --gate --json >"$stdout_json" 2>"$stderr_log"
  local rc=$?
  set -e

  [[ "$rc" -eq "$expected_exit" ]] || fail "$name expected exit $expected_exit, got $rc: $(cat "$stderr_log")"
  jq empty "$stdout_json" >/dev/null || fail "$name did not emit valid JSON"
  jq -e --arg gate "$expected_gate" '.gate_status == $gate' "$stdout_json" >/dev/null || fail "$name gate_status mismatch"
  jq -e "$jq_assertion" "$stdout_json" >/dev/null || fail "$name JSON assertion failed: $jq_assertion"
}

write_mock_security_toolchain() {
  local path="$1"
  cat >"$path" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
case "${SECURITY_GATE_FIXTURE_MODE:-missing}" in
  missing)
    cat <<'JSON'
{
  "timestamp": "2026-04-25T00:00:00Z",
  "target": "/tmp/agentops-security-fixture",
  "tools_run": 2,
  "tools_skipped": 2,
  "tools": {
    "ruff": "pass",
    "semgrep": "not_installed",
    "gitleaks": "error",
    "pytest": "skipped"
  },
  "findings": {
    "critical": 0,
    "high": 0,
    "security_high": 0,
    "quality_high": 0,
    "medium": 0,
    "low": 0
  },
  "gate_status": "PASS",
  "output_dir": "/tmp/agentops-tooling"
}
JSON
    ;;
  invalid)
    printf 'not-json\n'
    ;;
  *)
    echo "unknown security gate fixture mode" >&2
    exit 2
    ;;
esac
SH
  chmod +x "$path"
}

run_security_gate_case() {
  local name="$1"
  local mode="$2"
  local expected_exit="$3"
  local jq_assertion="$4"
  local repo
  repo="$(make_fixture_repo "$name")"
  local mock="$repo/mock-toolchain.sh"
  local stdout_json="$repo/security-gate.json"
  local stderr_log="$repo/security-gate.stderr"
  write_mock_security_toolchain "$mock"

  set +e
  env \
    SECURITY_GATE_FIXTURE_MODE="$mode" \
    SECURITY_GATE_TOOLCHAIN_SCRIPT="$mock" \
    SECURITY_GATE_OUTPUT_DIR="$repo/security" \
    TOOLCHAIN_OUTPUT_DIR="$repo/tooling" \
    "$repo/scripts/security-gate.sh" --mode quick --require-tools --json >"$stdout_json" 2>"$stderr_log"
  local rc=$?
  set -e

  [[ "$rc" -eq "$expected_exit" ]] || fail "$name expected exit $expected_exit, got $rc: $(cat "$stderr_log")"
  jq empty "$stdout_json" >/dev/null || fail "$name did not emit valid JSON"
  jq -e "$jq_assertion" "$stdout_json" >/dev/null || fail "$name JSON assertion failed: $jq_assertion"
}

case "${1:-}" in
  gate-exit-semantics)
    run_toolchain_case "gate-pass" "clean" "clean" 0 "PASS" '.findings.critical == 0 and .findings.security_high == 0'
    run_toolchain_case "gate-critical" "critical" "clean" 2 "BLOCKED_CRITICAL" '.findings.critical == 1'
    run_toolchain_case "gate-high" "high" "clean" 3 "BLOCKED_HIGH" '.findings.security_high == 1 and .findings.quality_high == 0'
    echo "toolchain-gate-exit-semantics-ok"
    ;;
  quality-separation)
    run_toolchain_case "quality-high" "clean" "findings" 0 "WARN_QUALITY" '.findings.high == 1 and .findings.security_high == 0 and .findings.quality_high == 1'
    echo "toolchain-quality-separation-ok"
    ;;
  security-gate-guards)
    run_security_gate_case "require-tools" "missing" 4 '.require_tools == true and .missing_tool_count == 2 and .gate_status == "PASS"'
    run_security_gate_case "invalid-json" "invalid" 1 '.parse_error == true and (.raw_output | contains("not-json"))'
    echo "security-gate-guards-ok"
    ;;
  quick-mode-skips)
    repo="$(make_fixture_repo "quick-mode")"
    output_dir="$repo/out"
    stdout_json="$repo/stdout.json"
    env PATH="$repo/mockbin:/usr/bin:/bin" TOOLCHAIN_OUTPUT_DIR="$output_dir" "$repo/scripts/toolchain-validate.sh" --quick --gate --json >"$stdout_json"
    jq -e '.tools.gitleaks == "skipped" and .tools.pytest == "skipped" and .tools["go-test"] == "skipped"' "$stdout_json" >/dev/null
    grep -Fxq "SKIPPED_QUICK_MODE" "$output_dir/gitleaks.txt"
    grep -Fxq "SKIPPED_QUICK_MODE" "$output_dir/pytest.txt"
    grep -Fxq "SKIPPED_QUICK_MODE" "$output_dir/gotest.txt"
    echo "toolchain-quick-mode-skips-ok"
    ;;
  ci-policy)
    python3 - "$REPO_ROOT/.github/workflows/validate.yml" <<'PY'
from pathlib import Path
import sys

workflow = Path(sys.argv[1]).read_text(encoding="utf-8")
job_start = workflow.index("  security-toolchain-gate:")
job_end = workflow.index("\n  skill-integrity:", job_start)
job = workflow[job_start:job_end]
required_job_bits = [
    "continue-on-error: true",
    "./scripts/security-gate.sh --mode quick",
    "uses: actions/upload-artifact@",
    "if: always()",
    "name: security-gate",
]
missing = [bit for bit in required_job_bits if bit not in job]
if missing:
    raise SystemExit(f"security-toolchain-gate job missing: {missing}")

summary_start = workflow.index("  summary:")
summary = workflow[summary_start:]
if "security-toolchain-gate" not in summary:
    raise SystemExit("summary does not report security-toolchain-gate")
failure_start = summary.index('          if [[')
failure_end = summary.index("then", failure_start)
failure_expr = summary[failure_start:failure_end]
if "needs.security-toolchain-gate.result" in failure_expr:
    raise SystemExit("security-toolchain-gate is blocking in summary failure expression")

print("security-toolchain-ci-policy-ok")
PY
    ;;
  docs-heading)
    grep -Fq '### scripts/toolchain-validate.sh' "$REPO_ROOT/docs/CI-CD.md"
    if grep -Fq 'scripts/security-toolchain-validate.sh' "$REPO_ROOT/docs/CI-CD.md"; then
      fail "stale security-toolchain-validate heading remains in docs/CI-CD.md"
    fi
    echo "security-toolchain-docs-heading-ok"
    ;;
  *)
    echo "usage: $0 gate-exit-semantics|quality-separation|security-gate-guards|quick-mode-skips|ci-policy|docs-heading" >&2
    exit 2
    ;;
esac
