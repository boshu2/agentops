#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  RUN="$SKILL_DIR/scripts/safe-clean.sh"
  FIX="$(mktemp -d)"
  FIX="$(cd "$FIX" && pwd -P)"
  mkdir -p "$FIX/root/cache" "$FIX/out"
  printf 'agentops-sbh-clean-root-v1\n' >"$FIX/root/.sbh-clean-root"
  printf 'reclaimable\n' >"$FIX/root/cache/old"
  export MOCK_MOUNT
  MOCK_MOUNT=$(df -P "$FIX/root" | awk 'END {print $6}')
  MOCK="$FIX/sbh"
  cat >"$MOCK" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == --version ]]; then printf 'sbh 0.4.27\n'; exit 0; fi
if [[ "${1:-}" == clean && "${2:-}" == --help ]]; then printf '%s\n' '--dry-run --yes --json --max-items'; exit 0; fi
if [[ " $* " == *' --json status '* ]]; then
  free=1000
  if [[ -s "${MOCK_ACTIONS:-/dev/null}" ]]; then
    if [[ "${MOCK_POST_LOWER:-0}" == 1 ]]; then free=900; else free=1200; fi
  fi
  printf '{"version":"0.4.27","pressure":{"mounts":[{"path":"%s","container_id":"disk-test","free":%s}]}}\n' "$MOCK_MOUNT" "$free"
  exit 0
fi
if [[ "${1:-}" == clean && " $* " == *' --dry-run '* ]]; then
  count=${MOCK_DRY_COUNT:-1}
  printf '{"command":"clean","dry_run":true,"candidates_count":%s,"items_would_delete":%s,"bytes_would_free":512,"protected_count":0}\n' "$count" "$count"
  exit 0
fi
if [[ "${1:-}" == clean ]]; then
  printf '%s\n' "$*" >>"$MOCK_ACTIONS"
  printf '{"command":"clean","dry_run":false,"items_deleted":1,"bytes_freed":512}\n'
  exit 0
fi
exit 2
SH
  chmod +x "$MOCK"
  export SBH_BIN="$MOCK"
  export MOCK_ACTIONS="$FIX/actions"
}

teardown() { rm -rf "$FIX"; }

make_plan() {
  "$RUN" plan --root "$FIX/root" --max-items 2 --plan-out "$FIX/out/plan.json"
}

@test "reviewed plan enables one bounded cleanup and postcondition receipt" {
  make_plan
  token=$(jq -r '.approval_token' "$FIX/out/plan.json")
  run "$RUN" apply --plan "$FIX/out/plan.json" --approve "$token"
  [ "$status" -eq 0 ]
  [ "$(wc -l <"$MOCK_ACTIONS" | tr -d ' ')" -eq 1 ]
  jq -e '.status == "applied" and .result.items_deleted == 1 and .not_checked == ["sbh-implementation","open-file-detection"]' "$FIX/out/plan.json.applied.json" >/dev/null
}

@test "raw clean baseline deletes while absent approval cannot act" {
  run "$MOCK" clean --yes --json "$FIX/root"
  [ "$status" -eq 0 ]
  [ -s "$MOCK_ACTIONS" ]
  rm -f "$MOCK_ACTIONS"
  make_plan
  run "$RUN" apply --plan "$FIX/out/plan.json" --approve sbh:clean:wrong
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "changed candidates invalidate the approved plan before deletion" {
  make_plan
  token=$(jq -r '.approval_token' "$FIX/out/plan.json")
  export MOCK_DRY_COUNT=2
  run "$RUN" apply --plan "$FIX/out/plan.json" --approve "$token"
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "unsafe roots and unsupported destructive operations are blocked" {
  ln -s "$FIX/root" "$FIX/root-link"
  run "$RUN" plan --root "$FIX/root-link" --max-items 2 --plan-out "$FIX/out/plan.json"
  [ "$status" -ne 0 ]
  mkdir "$FIX/root/.git"
  run "$RUN" plan --root "$FIX/root" --max-items 2 --plan-out "$FIX/out/plan.json"
  [ "$status" -ne 0 ]
  run "$RUN" emergency --root "$FIX/root" --approve anything
  [ "$status" -ne 0 ]
  [ ! -e "$MOCK_ACTIONS" ]
}

@test "failed postcondition preserves evidence and cannot report applied success" {
  make_plan
  token=$(jq -r '.approval_token' "$FIX/out/plan.json")
  export MOCK_POST_LOWER=1
  run "$RUN" apply --plan "$FIX/out/plan.json" --approve "$token"
  [ "$status" -eq 3 ]
  jq -e '.status == "postcondition_failed" and (.not_checked | index("cleanup-outcome"))' "$FIX/out/plan.json.applied.json" >/dev/null
}
