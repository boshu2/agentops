#!/bin/bash
# Internal implementation. The public .sh launcher scrubs shell startup hooks
# before starting this file with a pinned, non-login Bash.
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCHEMA="$SKILL_DIR/schemas/implementation-receipt.schema.json"

die() { echo "implementation-receipt: $*" >&2; exit 1; }

sha_file() {
  if command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | awk '{print $1}'
  else sha256sum "$1" | awk '{print $1}'; fi
}

sha_stdin() {
  if command -v shasum >/dev/null 2>&1; then shasum -a 256 | awk '{print $1}'
  else sha256sum | awk '{print $1}'; fi
}

safe_rel() {
  local rel="$1"
  [[ -n "$rel" && "$rel" != /* && "$rel" != "." && "$rel" != ".." && "$rel" != ../* && "$rel" != */../* && "$rel" != */.. ]] || return 1
}

contained_file() {
  local root="$1" rel="$2" root_abs dir_abs path current part
  local -a parts
  safe_rel "$rel" || return 1
  root_abs="$(cd "$root" && pwd -P)" || return 1
  path="$root/$rel"
  [[ -f "$path" && -s "$path" && ! -L "$path" ]] || return 1
  IFS='/' read -r -a parts <<<"$rel"
  current="$root_abs"
  for part in "${parts[@]}"; do current="$current/$part"; [[ ! -L "$current" ]] || return 1; done
  dir_abs="$(cd "$(dirname "$path")" && pwd -P)" || return 1
  case "$dir_abs/$(basename "$path")" in "$root_abs"/*) ;; *) return 1 ;; esac
  printf '%s/%s\n' "$dir_abs" "$(basename "$path")"
}

verify_evidence() {
  local root="$1" object="$2" path digest actual
  path="$(jq -er '.path' <<<"$object")" || return 1
  digest="$(jq -er '.sha256' <<<"$object")" || return 1
  path="$(contained_file "$root" "$path")" || return 1
  actual="$(sha_file "$path")"
  [[ "$actual" == "$digest" ]]
}

git_blob_digest() {
  local repo="$1" sha="$2" rel="$3" mode
  safe_rel "$rel" || return 1
  mode="$(git -C "$repo" ls-tree "$sha" -- "$rel" | awk 'NR==1{print $1}')"
  [[ -n "$mode" && "$mode" != "120000" ]] || return 1
  git -C "$repo" show "$sha:$rel" 2>/dev/null | sha_stdin
}

run_at_sha() {
  local repo="$1" sha="$2" command="$3" expected="$4" envelope="$5" tmp rc=0 output_hash account_home
  tmp="$(mktemp -d)"
  git -C "$repo" worktree add --detach --quiet "$tmp/work" "$sha" >/dev/null 2>&1 || { rm -rf "$tmp"; return 1; }
  set +e
  account_home="$(/usr/bin/python3 -c 'import os,pwd; print(pwd.getpwuid(os.getuid()).pw_dir)')"
  (cd "$tmp/work" && env -i HOME="$account_home" PATH=/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin /bin/bash --noprofile --norc -c "$command") >"$tmp/output" 2>&1
  rc=$?
  set -e
  [[ -z "$(git -C "$tmp/work" status --porcelain --untracked-files=no)" ]] || rc=255
  output_hash="$(sha_file "$tmp/output")"
  jq -e --arg command "$command" --argjson exit_code "$expected" --arg output_sha256 "$output_hash" 'type=="object" and .command==$command and .exit_code==$exit_code and .output_sha256==$output_sha256' "$envelope" >/dev/null 2>&1 || rc=254
  git -C "$repo" worktree remove --force "$tmp/work" >/dev/null 2>&1 || true
  rm -rf "$tmp"
  [[ "$rc" -eq "$expected" ]]
}

trusted_ao() {
  local candidate account_home owner uid
  account_home="$(/usr/bin/python3 -c 'import os,pwd; print(pwd.getpwuid(os.getuid()).pw_dir)')" || return 1
  uid="$(id -u)"
  for candidate in "$account_home/go/bin/ao" /usr/local/bin/ao /opt/homebrew/bin/ao; do
    [[ -f "$candidate" && -x "$candidate" && ! -L "$candidate" ]] || continue
    owner="$(stat -f %u "$candidate" 2>/dev/null || stat -c %u "$candidate" 2>/dev/null)"
    [[ "$owner" == "$uid" || ( "$candidate" == /usr/local/bin/ao || "$candidate" == /opt/homebrew/bin/ao ) && "$owner" == "0" ]] || continue
    printf '%s\n' "$candidate"; return 0
  done
  return 1
}

trusted_python() { local p; for p in /opt/homebrew/bin/python3 /usr/local/bin/python3; do [[ -x "$p" ]] && { printf '%s\n' "$p"; return; }; done; return 1; }

verify_command_record() {
  local repo="$1" root="$2" sha="$3" object="$4" command exit_code evidence envelope
  command="$(jq -er '.command' <<<"$object")"
  exit_code="$(jq -er '.exit_code' <<<"$object")"
  evidence="$(jq -ce '.evidence' <<<"$object")"
  verify_evidence "$root" "$evidence" || return 1
  envelope="$(contained_file "$root" "$(jq -r '.path' <<<"$evidence")")" || return 1
  run_at_sha "$repo" "$sha" "$command" "$exit_code" "$envelope"
}

verify_receipt() {
  local issue="$1" receipt="$2" repo="$3" pawl_script
  pawl_script="$repo/scripts/pawl-verdict.sh"
  local repo_abs receipt_abs base head receipt_root expected work_class red_kind account_home python_bin python_version user_site system_site
  repo_abs="$(cd "$repo" && pwd -P)" || die "repo root is unreadable"
  account_home="$(/usr/bin/python3 -c 'import os,pwd; print(pwd.getpwuid(os.getuid()).pw_dir)')"; python_bin="$(trusted_python)" || die "trusted Python runtime unavailable"
  [[ -z "${AGENTOPS_TRACKER+x}" && -z "${BEADS_DIR+x}" && -z "${AGENTOPS_REPO_ROOT+x}" && -z "${AGENTOPS_CONFIG+x}" && -z "${PYTHONPATH+x}" && -z "${PYTHONHOME+x}" && -z "${PYTHONSTARTUP+x}" && -z "${PYTHONUSERBASE+x}" && -z "${VIRTUAL_ENV+x}" && -z "${BASH_ENV+x}" && -z "${ENV+x}" && -z "${CDPATH+x}" ]] || die "tracker/config/repo-root/Python/shell environment overrides are forbidden"
  [[ -z "$(git -C "$repo_abs" status --porcelain --untracked-files=no)" ]] || die "tracked worktree is dirty"
  [[ -f "$receipt" && ! -L "$receipt" ]] || die "receipt missing or symlinked"
  receipt_abs="$(cd "$(dirname "$receipt")" && pwd -P)/$(basename "$receipt")"
  python_version="$(env -u PYTHONPATH -u PYTHONHOME -u PYTHONSTARTUP -u PYTHONUSERBASE -u VIRTUAL_ENV HOME="$account_home" "$python_bin" -I -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")')"
  user_site="$account_home/Library/Python/$python_version/lib/python/site-packages"; system_site="/opt/homebrew/lib/python$python_version/site-packages"
  env -u PYTHONPATH -u PYTHONHOME -u PYTHONSTARTUP -u PYTHONUSERBASE -u VIRTUAL_ENV HOME="$account_home" "$python_bin" -I -c 'import json,sys; sys.path[:0]=sys.argv[1:3]; import jsonschema; jsonschema.validate(json.load(open(sys.argv[4])),json.load(open(sys.argv[3])))' "$user_site" "$system_site" "$SCHEMA" "$receipt_abs" >/dev/null 2>&1 || die "receipt fails schema"
  [[ "$(jq -er '.issue_id' "$receipt_abs")" == "$issue" ]] || die "issue mismatch"
  base="$(jq -er '.base_sha' "$receipt_abs")"
  head="$(jq -er '.head_sha' "$receipt_abs")"
  receipt_root="$repo_abs/.agents/evidence/implement/$issue/$head"
  expected="$receipt_root/$issue-$head-receipt.json"
  [[ "$receipt_abs" == "$expected" ]] || die "receipt is not at canonical issue/head path"
  [[ "$(git -C "$repo_abs" rev-parse HEAD)" == "$head" ]] || die "receipt head is not HEAD"
  [[ "$base" != "$head" ]] || die "base_sha equals head_sha"
  git -C "$repo_abs" cat-file -e "$base^{commit}" 2>/dev/null || die "unknown base_sha"
  git -C "$repo_abs" cat-file -e "$head^{commit}" 2>/dev/null || die "unknown head_sha"
  git -C "$repo_abs" merge-base --is-ancestor "$base" "$head" || die "base_sha is not an ancestor of head_sha"

  local actual_files="" receipt_files path mode derived_class="docs-only"
  while IFS= read -r -d '' path; do
    [[ "$path" != *$'\n'* && "$path" != *$'\r'* ]] || die "newline-bearing paths are unsupported"
    actual_files+="${actual_files:+$'\n'}$path"
  done < <(git -C "$repo_abs" diff --name-only -z --diff-filter=ACDMRTUXB "$base" "$head")
  actual_files="$(printf '%s\n' "$actual_files" | LC_ALL=C sort)"
  receipt_files="$(jq -r '.changed_files[]' "$receipt_abs" | LC_ALL=C sort)"
  [[ -n "$actual_files" && "$actual_files" == "$receipt_files" ]] || die "changed_files do not equal base_sha..head_sha"

  while IFS= read -r path; do
    mode="$(git -C "$repo_abs" ls-tree "$head" -- "$path" | awk 'NR==1{print $1}')"
    [[ "$mode" == "100644" ]] || derived_class="behavior"
    [[ "$path" =~ ^(README|CHANGELOG|CONTRIBUTING)(\.[A-Za-z0-9_-]+)?\.(md|mdx|rst|txt)$|^docs/.*\.(md|mdx|rst|txt)$ ]] || derived_class="behavior"
  done < <(printf '%s\n' "$actual_files")

  local ao bead_json acceptance_json acceptance_ids canonical_commands green_commands
  ao="$(trusted_ao)" || die "trusted ao binary is unavailable"
  bead_json="$(cd "$repo_abs" && env -u AGENTOPS_TRACKER -u BEADS_DIR -u AGENTOPS_CONFIG HOME="$account_home" PATH=/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin "$ao" beads exec show "$issue" --json 2>/dev/null)" || die "exact bead lookup failed"
  [[ "$(jq -r 'length' <<<"$bead_json")" == "1" && "$(jq -r '.[0].id' <<<"$bead_json")" == "$issue" ]] || die "bead lookup did not return exact id"
  # shellcheck disable=SC2016
  acceptance_json="$(env -u PYTHONPATH -u PYTHONHOME -u PYTHONSTARTUP -u PYTHONUSERBASE -u VIRTUAL_ENV HOME="$account_home" "$python_bin" -I -c 'import json,re,sys; sys.path[:0]=sys.argv[1:3]; import yaml; b=json.load(sys.stdin)[0]; s=b.get("description",""); m=re.search(r"```(?:yaml|acceptance_criteria)?\n(acceptance_criteria:\n.*?)\n```",s,re.S); d=yaml.safe_load(m.group(1)) if m else {}; print(json.dumps(d.get("acceptance_criteria",[]),separators=(",",":")))' "$user_site" "$system_site" <<<"$bead_json")" || die "bead acceptance criteria are unreadable"
  acceptance_ids="$(jq -r '.[].id' <<<"$acceptance_json" | LC_ALL=C sort)"
  [[ -n "$acceptance_ids" && "$acceptance_ids" == "$(jq -r '.acceptance_ids[]' "$receipt_abs" | LC_ALL=C sort)" ]] || die "acceptance ids do not match bead"
  canonical_commands="$(jq -r '.[] | .check_command // empty' <<<"$acceptance_json" | LC_ALL=C sort -u)"
  [[ -n "$canonical_commands" ]] || die "bead has no executable acceptance command"
  if grep -Eq '^(true|:|exit[[:space:]]+0|test[[:space:]]+1[[:space:]]*=[[:space:]]*1)$' <<<"$canonical_commands"; then die "bead acceptance command is trivial"; fi
  green_commands="$(jq -r '.green[].command' "$receipt_abs" | LC_ALL=C sort -u)"
  [[ "$canonical_commands" == "$green_commands" ]] || die "GREEN commands are not exactly the bead acceptance commands"

  work_class="$(jq -er '.work_class' "$receipt_abs")"
  red_kind="$(jq -er '.red.kind' "$receipt_abs")"
  if [[ "$derived_class" == "docs-only" ]]; then [[ "$work_class" == "docs-only" ]] || die "strict docs-only diff is mislabeled";
  else [[ "$work_class" != "docs-only" ]] || die "docs-only waiver contains non-doc, executable, symlink, or submodule content"; fi
  if [[ "$work_class" == "behavior" && "$red_kind" != "captured" ]]; then die "behavior work cannot waive RED"; fi

  if [[ "$red_kind" == "captured" ]]; then
    local observed red_command red_exit red_evidence test_row test_path test_digest at_red at_head
    observed="$(jq -er '.red.observed_sha' "$receipt_abs")"
    [[ "$observed" != "$head" ]] || die "RED observed_sha equals final head"
    git -C "$repo_abs" merge-base --is-ancestor "$base" "$observed" || die "RED observed_sha predates base"
    git -C "$repo_abs" merge-base --is-ancestor "$observed" "$head" || die "RED observed_sha is not an ancestor of head"
    red_command="$(jq -er '.red.command' "$receipt_abs")"
    red_exit="$(jq -er '.red.exit_code' "$receipt_abs")"
    grep -Fqx -- "$red_command" <<<"$canonical_commands" || die "RED command is not a canonical bead acceptance command"
    [[ "$red_exit" -ne 2 && "$red_exit" -ne 126 && "$red_exit" -ne 127 && "$red_exit" -lt 128 ]] || die "RED exit is a shell/syntax/not-found/signal failure class"
    red_evidence="$(jq -ce '.red.evidence' "$receipt_abs")"
    verify_evidence "$receipt_root" "$red_evidence" || die "RED evidence missing, unsafe, or digest-mismatched"
    run_at_sha "$repo_abs" "$observed" "$red_command" "$red_exit" "$(contained_file "$receipt_root" "$(jq -r '.path' <<<"$red_evidence")")" || die "RED command/output envelope does not match fresh replay"
    while IFS= read -r test_row; do
      test_path="$(jq -er '.path' <<<"$test_row")"
      test_digest="$(jq -er '.sha256' <<<"$test_row")"
      at_red="$(git_blob_digest "$repo_abs" "$observed" "$test_path")" || die "RED test file missing or symlinked"
      at_head="$(git_blob_digest "$repo_abs" "$head" "$test_path")" || die "final test file missing or symlinked"
      [[ "$at_red" == "$test_digest" && "$at_head" == "$test_digest" ]] || die "test contract digest changed between RED and HEAD"
    done < <(jq -c '.red.test_files[]' "$receipt_abs")
  else
    local waiver_reason waiver_evidence before after
    waiver_reason="$(jq -er '.red.reason' "$receipt_abs")"
    [[ "$work_class" == "$waiver_reason" && "$work_class" != "behavior" ]] || die "RED waiver does not match non-behavior work_class"
    waiver_evidence="$(jq -ce '.red.evidence' "$receipt_abs")"
    verify_evidence "$receipt_root" "$waiver_evidence" || die "waiver evidence missing, unsafe, or digest-mismatched"
    jq -e --arg reason "$waiver_reason" --arg base "$base" --arg head "$head" '.kind=="waiver" and .reason==$reason and .base_sha==$base and .head_sha==$head' "$(contained_file "$receipt_root" "$(jq -r '.path' <<<"$waiver_evidence")")" >/dev/null || die "waiver envelope does not bind class and endpoints"
    if [[ "$work_class" == "docs-only" ]]; then
      while IFS= read -r test_path; do
        [[ "$test_path" =~ (^|/)(docs?|README|CHANGELOG|CONTRIBUTING)(/|\.|$)|\.(md|mdx|rst|txt)$ ]] || die "docs-only waiver contains non-document path: $test_path"
      done < <(jq -r '.changed_files[]' "$receipt_abs")
    elif [[ "$work_class" == "pure-refactor" ]]; then
      local before_commands after_commands driver driver_path driver_digest base_digest head_digest
      before_commands="$(jq -r '.red.baseline_before[].command' "$receipt_abs" | LC_ALL=C sort -u)"; after_commands="$(jq -r '.red.baseline_after[].command' "$receipt_abs" | LC_ALL=C sort -u)"
      [[ "$before_commands" == "$canonical_commands" && "$after_commands" == "$canonical_commands" ]] || die "pure-refactor baselines are not the canonical acceptance commands"
      while IFS= read -r before; do verify_command_record "$repo_abs" "$receipt_root" "$base" "$before" || die "pure-refactor before baseline fails"; done < <(jq -c '.red.baseline_before[]' "$receipt_abs")
      while IFS= read -r after; do verify_command_record "$repo_abs" "$receipt_root" "$head" "$after" || die "pure-refactor after baseline fails"; done < <(jq -c '.red.baseline_after[]' "$receipt_abs")
      while IFS= read -r driver; do
        driver_path="$(jq -r '.path' <<<"$driver")"; driver_digest="$(jq -r '.sha256' <<<"$driver")"; base_digest="$(git_blob_digest "$repo_abs" "$base" "$driver_path")"; head_digest="$(git_blob_digest "$repo_abs" "$head" "$driver_path")"
        [[ "$base_digest" == "$driver_digest" && "$head_digest" == "$driver_digest" ]] || die "pure-refactor test driver changed"
      done < <(jq -c '.red.test_drivers[]' "$receipt_abs")
    fi
  fi

  local green
  while IFS= read -r green; do
    verify_command_record "$repo_abs" "$receipt_root" "$head" "$green" || die "GREEN evidence or rerun failed"
  done < <(jq -c '.green[]' "$receipt_abs")

  if jq -e '.behavioral_spec.path? != null' "$receipt_abs" >/dev/null; then
    verify_evidence "$receipt_root" "$(jq -ce '.behavioral_spec' "$receipt_abs")" || die "behavioral spec evidence invalid"
  fi

  [[ "$(jq -er '.independent_validation.disposition' "$receipt_abs")" == "CONFIRMED" ]] || die "independent disposition is not CONFIRMED"
  local pr source_rel source_abs copied copied_path copied_digest source_digest review_expected review_actual ev
  pr="$(jq -er '.independent_validation.pr' "$receipt_abs")"
  source_rel="$(jq -er '.independent_validation.source_verdict_path' "$receipt_abs")"
  [[ "$source_rel" == ".agents/pawl-verdicts/$issue.json" ]] || die "source verdict is not the canonical bead path"
  source_abs="$(contained_file "$repo_abs" "$source_rel")" || die "canonical source verdict missing, unsafe, or empty"
  [[ -x "$pawl_script" && ! -L "$pawl_script" ]] || die "pinned canonical pawl verifier is unavailable"
  env -u AGENTOPS_REPO_ROOT -u PAWL_UNTRUSTED_REPO PAWL_AUTOBIND=0 /bin/bash "$pawl_script" check "$issue" "$pr" --verdict-file "$source_abs" --head "$head" >/dev/null || die "canonical-root pawl verdict check failed"
  copied="$(jq -ce '.independent_validation.copied_verdict' "$receipt_abs")"
  verify_evidence "$receipt_root" "$copied" || die "copied verdict missing, unsafe, or digest-mismatched"
  copied_path="$(contained_file "$receipt_root" "$(jq -r '.path' <<<"$copied")")"
  copied_digest="$(sha_file "$copied_path")"; source_digest="$(sha_file "$source_abs")"
  [[ "$copied_digest" == "$source_digest" ]] || die "copied verdict bytes differ from canonical source"

  review_expected="$(jq -r '.independent_validation.review_evidence[].path' "$receipt_abs" | LC_ALL=C sort)"
  review_actual="$(jq -r '[.refuters[]?.evidence // empty, .council_artifact // empty] | .[]' "$copied_path" | LC_ALL=C sort)"
  [[ -n "$review_actual" && "$review_actual" == "$review_expected" ]] || die "review evidence set does not match pawl verdict"
  while IFS= read -r ev; do
    verify_evidence "$receipt_root" "$ev" || die "archived review evidence invalid"
    path="$(jq -r '.path' <<<"$ev")"; [[ "$(sha_file "$(contained_file "$repo_abs" "$path")")" == "$(sha_file "$(contained_file "$receipt_root" "$path")")" ]] || die "archived review evidence differs from canonical source"
  done < <(jq -c '.independent_validation.review_evidence[]' "$receipt_abs")
  echo "implementation receipt: PASS ($issue ${head:0:12})"
}

ISSUE=""; RECEIPT=""; REPO="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --issue) ISSUE="${2:-}"; shift 2 ;;
    --receipt) RECEIPT="${2:-}"; shift 2 ;;
    *) die "unknown argument: $1" ;;
  esac
done
[[ -n "$ISSUE" && -n "$RECEIPT" ]] || die "usage: $0 --issue ID --receipt PATH"
verify_receipt "$ISSUE" "$RECEIPT" "$REPO"
