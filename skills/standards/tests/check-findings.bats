#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  CHECK="$SKILL_DIR/scripts/check-findings.sh"
  FIX="$(mktemp -d)"
}

teardown() { rm -rf "$FIX"; }

@test "complete focused review binds paths findings and selected references" {
  cat >"$FIX/findings.json" <<'JSON'
{
  "decision": "COMPLETE",
  "change": {"paths":["cli/widget.go","cli/widget_test.go"],"language":"go","change_type":"feature","risk_cues":["test_strategy"]},
  "selected_references": ["references/common-standards.md","references/go.md","references/test-pyramid.md"],
  "reference_reasons": {
    "references/common-standards.md":"Cross-language error handling applies.",
    "references/go.md":"The supplied files are Go.",
    "references/test-pyramid.md":"The caller requested test-strategy review."
  },
  "checked": ["error propagation","table-driven cases"],
  "not_checked": [],
  "findings": [{"severity":"warning","path":"cli/widget.go","line":17,"reference":"references/go.md","section":"Error handling","message":"Wrap the returned storage error with operation context."}]
}
JSON
  run "$CHECK" "$FIX/findings.json"
  [ "$status" -eq 0 ]
  [[ "$output" == *'3 references, 1 findings'* ]]
}

@test "nonempty prose-shaped baseline accepts JSON that the contract rejects" {
  printf '{"findings":["looks fine"]}\n' >"$FIX/shallow.json"
  run test -s "$FIX/shallow.json"
  [ "$status" -eq 0 ]
  run "$CHECK" "$FIX/shallow.json"
  [ "$status" -ne 0 ]
}

@test "unsafe finding path and unselected citation fail closed" {
  cat >"$FIX/unsafe.json" <<'JSON'
{
  "decision":"COMPLETE",
  "change":{"paths":["../outside.go"],"language":"go","change_type":"review","risk_cues":["none"]},
  "selected_references":["references/common-standards.md","references/go.md"],
  "reference_reasons":{"references/common-standards.md":"Common rules.","references/go.md":"Go rules."},
  "checked":["file"],"not_checked":[],
  "findings":[{"severity":"error","path":"../outside.go","line":1,"reference":"references/rust.md","section":"Errors","message":"Citation and path are outside the reviewed contract."}]
}
JSON
  run "$CHECK" "$FIX/unsafe.json"
  [ "$status" -ne 0 ]
}

@test "bulk rewrite requires all three mutation checks and incomplete review stays explicit" {
  cat >"$FIX/bulk.json" <<'JSON'
{
  "decision":"INCOMPLETE",
  "change":{"paths":["scripts/rewrite.sh"],"language":"shell","change_type":"bulk_rewrite","risk_cues":["bulk_rewrite"]},
  "selected_references":["references/common-standards.md","references/shell.md"],
  "reference_reasons":{"references/common-standards.md":"Mutation rules.","references/shell.md":"Shell rules."},
  "checked":["mutation-chokepoint","hash-witnessed-backup"],
  "not_checked":["ambition-gate"],
  "findings":[]
}
JSON
  run "$CHECK" "$FIX/bulk.json"
  [ "$status" -ne 0 ]
  ruby -rjson -e 'p=ARGV[0]; d=JSON.parse(File.read(p)); d["checked"] << "ambition-gate"; File.write(p, JSON.pretty_generate(d))' "$FIX/bulk.json"
  run "$CHECK" "$FIX/bulk.json"
  [ "$status" -eq 0 ]
}
