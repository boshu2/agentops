#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EVIDENCE="$ROOT/.agents/rpi/evidence/go-cli-integration-baseline.json"
RECEIPT_DIR="$ROOT/.agents/evidence/go-cli-production-hardening"
FAMILIES=(beads capabilities claim close config council-gate doctor "done" eval gate)
SEAL_FILES=(case.json ownership.json lineage.json)

fail() {
  printf 'go-cli integration baseline FAIL: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

require_command git
require_command jq
require_command comm
require_command sort

git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail "not a Git worktree: $ROOT"

head_sha="$(git -C "$ROOT" rev-parse --verify HEAD)" || fail "cannot resolve HEAD"
origin_main_sha="$(git -C "$ROOT" rev-parse --verify refs/remotes/origin/main)" || fail "cannot resolve origin/main"
git -C "$ROOT" merge-base --is-ancestor refs/remotes/origin/main HEAD || fail "origin/main is not an ancestor of HEAD"

test -f "$EVIDENCE" || fail "missing integration evidence: ${EVIDENCE#"$ROOT"/}"
jq -e -s 'length == 1 and (.[0] | type == "object")' "$EVIDENCE" >/dev/null || fail "integration evidence is not one clean JSON object"

PRE_INTEGRATION_SHA="$(jq -er '.pre_integration_sha | select(type == "string" and test("^[0-9a-f]{40}$"))' "$EVIDENCE")" || fail "invalid pre-integration SHA"
RESCUE_REF="$(jq -er '.rescue_ref | select(type == "string" and startswith("refs/heads/rescue/age-nw28h-7-8-pre-integration-"))' "$EVIDENCE")" || fail "invalid rescue ref"
expected_rescue_ref="refs/heads/rescue/age-nw28h-7-8-pre-integration-${PRE_INTEGRATION_SHA:0:12}"
test "$RESCUE_REF" = "$expected_rescue_ref" || fail "rescue ref is not derived from pre-integration SHA"
rescue_sha="$(git -C "$ROOT" rev-parse --verify "$RESCUE_REF")" || fail "cannot resolve rescue ref: $RESCUE_REF"
test "$rescue_sha" = "$PRE_INTEGRATION_SHA" || fail "rescue ref mismatch: expected $PRE_INTEGRATION_SHA, got $rescue_sha"
git -C "$ROOT" merge-base --is-ancestor "$RESCUE_REF" HEAD || fail "rescue ref is not an ancestor of HEAD"

merge_base="$(git -C "$ROOT" merge-base "$PRE_INTEGRATION_SHA" "$origin_main_sha")" || fail "cannot compute live merge base"
read -r ahead behind < <(git -C "$ROOT" rev-list --left-right --count "$PRE_INTEGRATION_SHA...$origin_main_sha")

overlap_paths="$(
  comm -12 \
    <(git -C "$ROOT" diff --name-only "$merge_base..$PRE_INTEGRATION_SHA" | LC_ALL=C sort -u) \
    <(git -C "$ROOT" diff --name-only "$merge_base..$origin_main_sha" | LC_ALL=C sort -u)
)"
overlap_json="$(printf '%s\n' "$overlap_paths" | jq -Rsc 'split("\n") | map(select(length > 0)) | sort')"
families_json="$(printf '%s\n' "${FAMILIES[@]}" | jq -Rsc 'split("\n") | map(select(length > 0)) | sort')"

jq -e \
  --arg issue "age-nw28h.7.8" \
  --arg pre "$PRE_INTEGRATION_SHA" \
  --arg rescue_ref "$RESCUE_REF" \
  --arg rescue_sha "$rescue_sha" \
  --arg origin "$origin_main_sha" \
  --arg integrated "$head_sha" \
  --arg merge_base "$merge_base" \
  --argjson ahead "$ahead" \
  --argjson behind "$behind" \
  --argjson overlaps "$overlap_json" '
    .schema_version == 1
    and .issue_id == $issue
    and (.admission_baseline_sha | type == "string" and test("^[0-9a-f]{40}$"))
    and .pre_integration_sha == $pre
    and .rescue_ref == $rescue_ref
    and .rescue_sha == $rescue_sha
    and .rescue_verified == true
    and .origin_main_sha == $origin
    and .integrated_sha == $integrated
    and .merge_base == $merge_base
    and .divergence.ahead == $ahead
    and .divergence.behind == $behind
    and .ancestry.pre_integration == true
    and .ancestry.origin_main == true
    and (.overlap_dispositions | type == "array")
    and ([.overlap_dispositions[].path] | sort) == $overlaps
    and ([.overlap_dispositions[].path] | length == (unique | length))
    and all(.overlap_dispositions[];
      (.path | type == "string" and length > 0)
      and (.disposition | type == "string" and length > 0)
      and (.branch_change | type == "string" and length > 0)
      and (.upstream_change | type == "string" and length > 0)
      and (.resolution | type == "string" and length > 0)
      and (.verification | type == "string" and length > 0))
  ' "$EVIDENCE" >/dev/null || fail "integration evidence does not match live refs, divergence, ancestry, or overlap dispositions"

admission_sha="$(jq -r '.admission_baseline_sha' "$EVIDENCE")"
git -C "$ROOT" cat-file -e "$admission_sha^{commit}" 2>/dev/null || fail "admission baseline is not a commit: $admission_sha"
git -C "$ROOT" merge-base --is-ancestor "$admission_sha" "$PRE_INTEGRATION_SHA" || fail "admission baseline is not an ancestor of pre-integration SHA"
git -C "$ROOT" merge-base --is-ancestor "$PRE_INTEGRATION_SHA" HEAD || fail "pre-integration SHA is not an ancestor of HEAD"

jq -e --argjson families "$families_json" '(.historical_seal_blobs | keys | sort) == $families' "$EVIDENCE" >/dev/null || fail "integration evidence historical seal family set mismatch"

receipt_count=0
while IFS= read -r receipt; do
  receipt_count=$((receipt_count + 1))
done < <(find "$RECEIPT_DIR" -maxdepth 1 -type f -name 'descendant-revalidation-*.json' 2>/dev/null | LC_ALL=C sort)
test "$receipt_count" -eq "${#FAMILIES[@]}" || fail "expected exactly ${#FAMILIES[@]} descendant receipts, found $receipt_count"

for family in "${FAMILIES[@]}"; do
  family_dir="cli/testdata/compatibility-baseline/families/$family"
  lineage="$ROOT/$family_dir/lineage.json"
  receipt="$RECEIPT_DIR/descendant-revalidation-$family.json"

  test -f "$lineage" || fail "missing historical lineage: $family"
  test -f "$receipt" || fail "missing descendant receipt: $family"
  jq -e -s 'length == 1 and (.[0] | type == "object")' "$receipt" >/dev/null || fail "descendant receipt is not one clean JSON object: $family"

  historical_accepted_sha="$(jq -er '.accepted_sha | select(type == "string" and test("^[0-9a-f]{40}$"))' "$lineage")" || fail "invalid historical accepted_sha: $family"
  git -C "$ROOT" merge-base --is-ancestor "$historical_accepted_sha" HEAD || fail "historical accepted SHA is not an ancestor of HEAD: $family"

  for seal_file in "${SEAL_FILES[@]}"; do
    repo_path="$family_dir/$seal_file"
    pre_blob="$(git -C "$ROOT" rev-parse "$PRE_INTEGRATION_SHA:$repo_path")" || fail "missing pre-integration seal blob: $repo_path"
    head_blob="$(git -C "$ROOT" rev-parse "$head_sha:$repo_path")" || fail "missing integrated seal blob: $repo_path"
    test "$head_blob" = "$pre_blob" || fail "historical seal blob changed: $repo_path"

    evidence_blob="$(jq -er --arg family "$family" --arg file "$seal_file" '.historical_seal_blobs[$family][$file] | select(type == "string" and test("^[0-9a-f]{40}$"))' "$EVIDENCE")" || fail "missing integration evidence seal blob: $repo_path"
    test "$evidence_blob" = "$pre_blob" || fail "integration evidence seal blob mismatch: $repo_path"
  done

  jq -e \
    --arg issue "age-nw28h.7.8" \
    --arg family "$family" \
    --arg accepted "$historical_accepted_sha" \
    --arg pre "$PRE_INTEGRATION_SHA" \
    --arg origin "$origin_main_sha" \
    --arg integrated "$head_sha" '
      .schema_version == 1
      and .issue_id == $issue
      and .family == $family
      and .historical_accepted_sha == $accepted
      and .pre_integration_sha == $pre
      and .origin_main_sha == $origin
      and .integrated_sha == $integrated
      and .accepted_sha_is_ancestor == true
      and .architecture_command == ("scripts/check-go-cli-architecture.sh --family " + $family)
      and .architecture_exit == 0
      and (.architecture_output_tail | type == "string" and length > 0)
      and .compatibility_command == ("scripts/check-go-cli-compatibility.sh --oracle-version current --verify-frozen --profiles default,flywheel,legacy,combined --family " + $family)
      and .compatibility_exit == 0
      and (.compatibility_output_tail | type == "string" and length > 0)
      and (.historical_seal_blobs | keys | sort) == ["case.json", "lineage.json", "ownership.json"]
      and all(.historical_seal_blobs[]; type == "string" and test("^[0-9a-f]{40}$"))
    ' "$receipt" >/dev/null || fail "invalid or stale descendant receipt: $family"

  for seal_file in "${SEAL_FILES[@]}"; do
    repo_path="$family_dir/$seal_file"
    pre_blob="$(git -C "$ROOT" rev-parse "$PRE_INTEGRATION_SHA:$repo_path")"
    receipt_blob="$(jq -r --arg file "$seal_file" '.historical_seal_blobs[$file]' "$receipt")"
    test "$receipt_blob" = "$pre_blob" || fail "descendant receipt seal blob mismatch: $repo_path"
  done
done

printf 'Go CLI integration baseline PASS: integrated=%s origin/main=%s families=%s\n' "$head_sha" "$origin_main_sha" "${#FAMILIES[@]}"
