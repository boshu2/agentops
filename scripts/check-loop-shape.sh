#!/usr/bin/env bash
# practices: [bdd-gherkin, ddd-bounded-context, tdd]
# check-loop-shape.sh — warn-only loop-shape gate for non-trivial beads.
#
# Implements the initial gate declared by GOALS.md Directive 12: beads tagged
# `non-trivial` should expose loop shape before implementation — exactly one
# bounded context label, at least one Gherkin block (Given/When/Then), at least
# one slice candidate, and one first-failing proof. This is the
# BDD/Gherkin/DDD/XP operating loop's mechanical warning.
#
# Posture: WARN-ONLY. The script exits 0 even when beads are missing loop
# shape, so it never blocks a push. It flips to blocking only when invoked
# with --strict (or AGENTOPS_LOOP_SHAPE_STRICT=1), which Directive 12 reserves
# for once the corpus-wide pass rate is stable.
#
# Usage:
#   check-loop-shape.sh                 # inspect live `bd` open + in_progress beads
#   check-loop-shape.sh --json FILE     # inspect a bd-JSON array from FILE
#   check-loop-shape.sh --strict        # exit 1 if any non-trivial bead lacks shape
#   check-loop-shape.sh --self-test     # run built-in fixtures and assert behavior
#   check-loop-shape.sh --help
#
# Exit codes:
#   0 = warn-only run (always), or strict run with no offenders, or self-test pass
#   1 = strict run with at least one offender, or self-test failure
#   2 = script error (bad invocation, missing dependency)

set -uo pipefail

STRICT="${AGENTOPS_LOOP_SHAPE_STRICT:-0}"
JSON_FILE=""
SELF_TEST=0

while [ $# -gt 0 ]; do
  case "$1" in
    --strict) STRICT=1 ;;
    --self-test) SELF_TEST=1 ;;
    --json)
      JSON_FILE="${2:-}"
      if [ -z "$JSON_FILE" ]; then
        echo "check-loop-shape: --json needs a file argument" >&2
        exit 2
      fi
      shift
      ;;
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "check-loop-shape: unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

if ! command -v jq >/dev/null 2>&1; then
  echo "check-loop-shape: SKIP (jq not available)"
  exit 0
fi

# analyze_beads <json-string>
# Emits one "WARN: ..." line per non-trivial bead missing loop shape on stdout,
# then a final "OFFENDERS: <n>" line. Returns 0 always; callers read the count.
analyze_beads() {
  local json="$1"
  local rows
  rows=$(printf '%s' "$json" | jq -r '
    def evidence_text:
      [(.description // ""), (.acceptance_criteria // ""), (.notes // "")]
      | join("\n");
    def context_labels:
      [(.labels // [])[]
       | select(test("^bc-(corpus|validation|loop|factory|runtime)$"))]
      | unique;
    [ .[]
      | select((.labels // []) | index("non-trivial"))
      | { id: .id,
          gherkin: ((evidence_text | test("\\bGiven\\b") and test("\\bWhen\\b") and test("\\bThen\\b"))),
          slice: ((evidence_text | test("(?i)\\bslice(s| candidates)?\\b") or test("\\bS[0-9]+\\b"))),
          proof: ((evidence_text | test("(?i)first[ _-]*failing[ _-]*(proof|test)"))),
          contexts: context_labels }
    ] | .[] | [.id, (.gherkin|tostring), (.slice|tostring), (.proof|tostring), ((.contexts|length)|tostring), (.contexts|join(","))] | @tsv
  ' 2>/dev/null)

  local offenders=0
  if [ -n "$rows" ]; then
    while IFS=$'\t' read -r id gherkin slice proof context_count contexts; do
      [ -z "$id" ] && continue
      local missing=""
      if [ "$context_count" != "1" ]; then
        missing="bounded context label (exactly one of bc-corpus|bc-validation|bc-loop|bc-factory|bc-runtime"
        if [ -n "$contexts" ]; then
          missing="$missing; found $contexts"
        fi
        missing="$missing)"
      fi
      if [ "$gherkin" != "true" ]; then
        [ -n "$missing" ] && missing="$missing and "
        missing="${missing}Gherkin block (Given/When/Then)"
      fi
      if [ "$slice" != "true" ]; then
        [ -n "$missing" ] && missing="$missing and "
        missing="${missing}slice candidate"
      fi
      if [ "$proof" != "true" ]; then
        [ -n "$missing" ] && missing="$missing and "
        missing="${missing}first failing proof"
      fi
      if [ -n "$missing" ]; then
        echo "WARN: $id — non-trivial bead is missing: $missing"
        offenders=$((offenders + 1))
      fi
    done <<< "$rows"
  fi
  echo "OFFENDERS: $offenders"
}

self_test() {
  local fixture
  fixture=$(cat <<'EOF'
[
  { "id": "fix-good",  "labels": ["non-trivial", "bc-loop", "xp"],
    "description": "Feature: x\n  Scenario: y\n    Given a\n    When b\n    Then c\nSlice candidates: S1 do the thing\nFirst failing proof: go test ./..." },
  { "id": "fix-split-fields", "labels": ["non-trivial", "bc-corpus"],
    "description": "Problem statement only.",
    "acceptance_criteria": "Slice candidates: S1 do the thing\nFirst failing proof: go test ./...",
    "notes": "Feature: x\n  Scenario: y\n    Given a\n    When b\n    Then c" },
  { "id": "fix-nogherkin", "labels": ["non-trivial", "bc-loop"],
    "description": "Just do the work. Slice S1 covers it. First failing proof: go test ./..." },
  { "id": "fix-noslice", "labels": ["non-trivial", "bc-loop"],
    "description": "Given a\n  When b\n  Then c\nFirst failing proof: go test ./..." },
  { "id": "fix-nocontext", "labels": ["non-trivial"],
    "description": "Given a\nWhen b\nThen c\nSlice candidates: S1\nFirst failing proof: go test ./..." },
  { "id": "fix-multicontext", "labels": ["non-trivial", "bc-loop", "bc-corpus"],
    "description": "Given a\nWhen b\nThen c\nSlice candidates: S1\nFirst failing proof: go test ./..." },
  { "id": "fix-noproof", "labels": ["non-trivial", "bc-loop"],
    "description": "Given a\nWhen b\nThen c\nSlice candidates: S1" },
  { "id": "fix-trivial", "labels": ["chore"],
    "description": "rename a variable" }
]
EOF
)
  local out
  out=$(analyze_beads "$fixture")
  local count
  count=$(printf '%s\n' "$out" | sed -n 's/^OFFENDERS: //p')
  local fails=0

  if [ "$count" != "5" ]; then
    echo "SELF-TEST FAIL: expected 5 offenders, got '$count'" >&2
    fails=$((fails + 1))
  fi
  if ! printf '%s\n' "$out" | grep -q "^WARN: fix-nogherkin .*Gherkin block"; then
    echo "SELF-TEST FAIL: fix-nogherkin should warn about a missing Gherkin block" >&2
    fails=$((fails + 1))
  fi
  if ! printf '%s\n' "$out" | grep -q "^WARN: fix-noslice .*slice candidate"; then
    echo "SELF-TEST FAIL: fix-noslice should warn about a missing slice candidate" >&2
    fails=$((fails + 1))
  fi
  if ! printf '%s\n' "$out" | grep -q "^WARN: fix-nocontext .*bounded context label"; then
    echo "SELF-TEST FAIL: fix-nocontext should warn about a missing bounded context label" >&2
    fails=$((fails + 1))
  fi
  if ! printf '%s\n' "$out" | grep -q "^WARN: fix-multicontext .*bounded context label"; then
    echo "SELF-TEST FAIL: fix-multicontext should warn about multiple bounded context labels" >&2
    fails=$((fails + 1))
  fi
  if ! printf '%s\n' "$out" | grep -q "^WARN: fix-noproof .*first failing proof"; then
    echo "SELF-TEST FAIL: fix-noproof should warn about a missing first failing proof" >&2
    fails=$((fails + 1))
  fi
  if printf '%s\n' "$out" | grep -q "^WARN: fix-good"; then
    echo "SELF-TEST FAIL: fix-good has full loop shape and must not warn" >&2
    fails=$((fails + 1))
  fi
  if printf '%s\n' "$out" | grep -q "^WARN: fix-split-fields"; then
    echo "SELF-TEST FAIL: fix-split-fields has evidence across bd fields and must not warn" >&2
    fails=$((fails + 1))
  fi
  if printf '%s\n' "$out" | grep -q "^WARN: fix-trivial"; then
    echo "SELF-TEST FAIL: fix-trivial is not labeled non-trivial and must be ignored" >&2
    fails=$((fails + 1))
  fi

  if [ "$fails" -ne 0 ]; then
    echo "check-loop-shape: self-test FAILED ($fails assertion(s))" >&2
    return 1
  fi
  echo "check-loop-shape: self-test PASS"
  return 0
}

if [ "$SELF_TEST" -eq 1 ]; then
  self_test
  exit $?
fi

# Resolve the bead JSON: explicit fixture file, or live bd.
if [ -n "$JSON_FILE" ]; then
  if [ ! -f "$JSON_FILE" ]; then
    echo "check-loop-shape: --json file not found: $JSON_FILE" >&2
    exit 2
  fi
  BEAD_JSON=$(cat "$JSON_FILE")
else
  if ! command -v bd >/dev/null 2>&1; then
    echo "check-loop-shape: SKIP (bd not available; pass --json FILE to inspect a fixture)"
    exit 0
  fi
  OPEN_JSON=$(bd list --status=open --json 2>/dev/null || echo '[]')
  PROG_JSON=$(bd list --status=in_progress --json 2>/dev/null || echo '[]')
  BEAD_JSON=$(printf '%s\n%s' "$OPEN_JSON" "$PROG_JSON" | jq -s 'add // []' 2>/dev/null || echo '[]')
fi

RESULT=$(analyze_beads "$BEAD_JSON")
WARNINGS=$(printf '%s\n' "$RESULT" | grep '^WARN: ' || true)
OFFENDERS=$(printf '%s\n' "$RESULT" | sed -n 's/^OFFENDERS: //p')
OFFENDERS="${OFFENDERS:-0}"

if [ -n "$WARNINGS" ]; then
  printf '%s\n' "$WARNINGS"
fi

if [ "$OFFENDERS" -eq 0 ]; then
  echo "check-loop-shape: PASS (no non-trivial beads missing loop shape)"
  exit 0
fi

if [ "$STRICT" = "1" ]; then
  echo "check-loop-shape: FAIL — $OFFENDERS non-trivial bead(s) missing loop shape (--strict)"
  exit 1
fi

echo "check-loop-shape: WARN-ONLY — $OFFENDERS non-trivial bead(s) missing loop shape (Directive 12 posture; --strict to block)"
exit 0
