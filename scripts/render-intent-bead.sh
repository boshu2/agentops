#!/usr/bin/env bash
# practices: [bdd-gherkin, ddd-bounded-context, tdd]
# Render a Directive 12 compliant intent-bead body.

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/render-intent-bead.sh --title TEXT --context bc-loop \
  --scenario TEXT --given TEXT --when TEXT --then TEXT \
  --slice TEXT --proof COMMAND [options]

Required:
  --title TEXT          Bead title / feature name
  --context bc-*        Exactly one bounded context label:
                        bc-corpus, bc-validation, bc-loop, bc-factory, bc-runtime
  --scenario TEXT       Gherkin scenario name
  --given TEXT          Gherkin Given line body
  --when TEXT           Gherkin When line body
  --then TEXT           Gherkin Then line body
  --slice TEXT          Vertical slice description
  --proof COMMAND       First failing proof command or test name

Optional:
  --slice-id ID         Slice id (default: S1)
  --write-scope TEXT    Proposed write scope (default: TBD)
  --json                Emit a bd-like JSON object instead of dry-run text
  -h, --help            Show this help

The text output is designed to be copied into:
  bd create --body-file <file> --labels non-trivial,<context>,operating-loop,bdd,ddd,xp
EOF
}

die() {
  echo "render-intent-bead: $*" >&2
  exit 2
}

TITLE=""
CONTEXT=""
SCENARIO=""
GIVEN=""
WHEN=""
THEN=""
SLICE=""
SLICE_ID="S1"
PROOF=""
WRITE_SCOPE="TBD"
JSON=false

while [ $# -gt 0 ]; do
  case "$1" in
    --title) TITLE="${2:-}"; shift 2 ;;
    --context) CONTEXT="${2:-}"; shift 2 ;;
    --scenario) SCENARIO="${2:-}"; shift 2 ;;
    --given) GIVEN="${2:-}"; shift 2 ;;
    --when) WHEN="${2:-}"; shift 2 ;;
    --then) THEN="${2:-}"; shift 2 ;;
    --slice) SLICE="${2:-}"; shift 2 ;;
    --slice-id) SLICE_ID="${2:-}"; shift 2 ;;
    --proof) PROOF="${2:-}"; shift 2 ;;
    --write-scope) WRITE_SCOPE="${2:-}"; shift 2 ;;
    --json) JSON=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

for pair in \
  "title:$TITLE" \
  "context:$CONTEXT" \
  "scenario:$SCENARIO" \
  "given:$GIVEN" \
  "when:$WHEN" \
  "then:$THEN" \
  "slice:$SLICE" \
  "proof:$PROOF"; do
  name="${pair%%:*}"
  value="${pair#*:}"
  [ -n "$value" ] || die "--$name is required"
done

case "$CONTEXT" in
  bc-corpus|bc-validation|bc-loop|bc-factory|bc-runtime) ;;
  *) die "--context must be one of bc-corpus|bc-validation|bc-loop|bc-factory|bc-runtime" ;;
esac

LABELS="non-trivial,$CONTEXT,operating-loop,bdd,ddd,xp"

BODY="$(
  cat <<EOF
Problem:
<Describe the observed gap in product/runtime terms.>

Bounded context: $CONTEXT
Labels: $LABELS

BDD:
\`\`\`gherkin
Feature: $TITLE
  Scenario: $SCENARIO
    Given $GIVEN
    When $WHEN
    Then $THEN
\`\`\`

Slice candidates:
- $SLICE_ID: $SLICE
  - First failing proof: $PROOF
  - Write scope: $WRITE_SCOPE

Validation evidence:
- Red: run \`$PROOF\` before implementation and capture the expected failure.
- Green: rerun \`$PROOF\` after implementation and capture the passing result.

Residual gaps:
- None known at creation.
EOF
)"

if [ "$JSON" = true ]; then
  command -v jq >/dev/null 2>&1 || die "jq is required for --json"
  labels_json="$(printf '%s' "$LABELS" | jq -R 'split(",")')"
  jq -n \
    --arg id "intent-dry-run" \
    --arg title "$TITLE" \
    --arg description "$BODY" \
    --argjson labels "$labels_json" \
    '{id:$id,title:$title,status:"open",labels:$labels,description:$description}'
  exit 0
fi

cat <<EOF
Labels: $LABELS

$BODY
EOF
