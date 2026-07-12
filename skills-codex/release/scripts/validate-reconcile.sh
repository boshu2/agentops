#!/usr/bin/env bash
set -euo pipefail

[[ $# -ge 1 && $# -le 2 ]] || { echo "usage: validate-reconcile.sh <expected-tag> [report.json|-]" >&2; exit 2; }
EXPECTED_TAG="$1"
REPORT="${2:--}"
SEMVER_TAG_RE='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-((0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(\+([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$'
[[ "$EXPECTED_TAG" =~ $SEMVER_TAG_RE ]] || {
  echo "release-reconcile: invalid expected tag $EXPECTED_TAG" >&2
  exit 2
}

jq -e --arg expected_tag "$EXPECTED_TAG" '
  type == "object" and
  ((.schema_version | type) == "string") and
  ((.schema_version | length) > 0) and
  (.overall_status == "green" or .overall_status == "green_with_warnings") and
  ((.release | type) == "object") and
  (.release.available == true) and
  ((.release.tag_name | type) == "string") and
  ((.release.tag_name | length) > 0) and
  (.release.tag_name == $expected_tag) and
  ((.release.tag_validate_runs | type) == "array") and
  ((.release.tag_validate_runs | length) > 0) and
  (.release.tag_validate_runs[0].status == "completed") and
  (.release.tag_validate_runs[0].conclusion == "success") and
  ((.findings | type) == "array") and
  ([.findings[] |
      select(
        .surface == "release" and
        (.severity == "high" or .severity == "medium" or
         .id == "release-unavailable" or
         (.id | startswith("release-tag-validate-")))
      )
    ] | length == 0)
' "$REPORT" >/dev/null
