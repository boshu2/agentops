#!/usr/bin/env bats
# Acceptance harness for age-h433.1: one canonical skill-conformance profile.
#
# The fixtures exercise the public scanner/auditor/healer/factory processes.
# They intentionally avoid sourcing implementation functions so the suite stays
# coupled to the contract rather than to any one implementation.

setup() {
  REPO_ROOT="$(cd "${BATS_TEST_DIRNAME}/../.." && pwd)"
  SCANNER="$REPO_ROOT/skills/skill-builder/scripts/scan_descriptions.py"
  AUDITOR="$REPO_ROOT/skills/heal-skill/scripts/audit.sh"
  HEALER="$REPO_ROOT/skills/heal-skill/scripts/heal.sh"
  BUILDER="$REPO_ROOT/skills/skill-builder/scripts/build.sh"
  PROFILE="$REPO_ROOT/skills/skill-builder/references/skill-conformance-profiles.yaml"
  KNOWN_GOOD="$REPO_ROOT/tests/fixtures/skills/known-good"
}

assert_json() {
  local file="$1"
  local expression="$2"
  local expectation="$3"

  if ! jq -e "$expression" "$file" >/dev/null; then
    echo "JSON assertion failed: $expectation" >&2
    jq . "$file" >&2 || true
    return 1
  fi
}

write_conforming_skill() {
  local target="$1"
  local name="$2"
  local description="$3"
  local output_variant="${4:-complete}"

  mkdir -p "$target"
  {
    printf '%s\n' '---'
    printf 'name: %s\n' "$name"
    printf 'description: %s\n' "$description"
    printf '%s\n' \
      'skill_api_version: 1' \
      'context:' \
      '  window: fork' \
      '  intent:' \
      '    mode: task' \
      '  sections:' \
      '    exclude: [HISTORY]' \
      '  intel_scope: topic' \
      'metadata:' \
      '  tier: execution' \
      '  dependencies: []' \
      'output_contract: ".agents/out/report.json: validated JSON handoff"' \
      '---' \
      "# $name" \
      '' \
      '## Critical Constraints' \
      '' \
      '- Keep the fixture isolated. **Why:** acceptance tests must not mutate live skills.' \
      '' \
      '## Workflow' \
      '' \
      'Create the deterministic fixture.' \
      '' \
      '**Checkpoint:** the fixture exists and is parseable.' \
      '' \
      '## Output Specification' \
      ''
    if [[ "$output_variant" == "incomplete" ]]; then
      printf '%s\n' \
        '**Format:** JSON' \
        '**Path:** `.agents/out/report.json`'
    else
      printf '%s\n' \
        '**Artifact directory:** `.agents/out/`' \
        '**Filename convention:** `report.json`' \
        '**Serialization/schema format:** JSON matching `schemas/report.schema.json`.' \
        '**Validator command:** `jq -e . .agents/out/report.json`' \
        '**Downstream handoff:** consumed by the validation wave.'
    fi
    printf '%s\n' \
      '' \
      '## Quality Rubric' \
      '' \
      '- [ ] Frontmatter is valid.' \
      '- [ ] Constraints include rationale.' \
      '- [ ] Workflow includes a checkpoint.' \
      '- [ ] Output handoff is executable.' \
      '- [ ] References resolve.'
  } >"$target/SKILL.md"
}

write_report_from_output() {
  local output_file="$1"
  printf '%s\n' "$output" >"$output_file"
  jq . "$output_file" >/dev/null
}

materialize_line_fixture() {
  local target="$1"
  local line_count="$2"
  local skill_md

  cp -R "$KNOWN_GOOD" "$target"
  skill_md="$target/SKILL.md"
  if [[ "$(wc -l <"$skill_md")" -gt "$line_count" ]]; then
    echo "known-good fixture already exceeds requested line count" >&2
    return 1
  fi
  while [[ "$(wc -l <"$skill_md")" -lt "$line_count" ]]; do
    printf '<!-- deterministic line-boundary padding -->\n' >>"$skill_md"
  done
  [[ "$(wc -l <"$skill_md")" -eq "$line_count" ]]
}

prepare_builder_root() {
  local scratch="$1"

  mkdir -p "$scratch/docs" "$scratch/skills-codex"
  cp -R "$REPO_ROOT/skills" "$scratch/skills"
  cp -R "$REPO_ROOT/scripts" "$scratch/scripts"
  cp -R "$REPO_ROOT/docs/contracts" "$scratch/docs/contracts"
  cp -R "$REPO_ROOT/docs/reference" "$scratch/docs/reference"
  cp -R "$REPO_ROOT/skills-codex-overrides" "$scratch/skills-codex-overrides"
  if [[ -f "$REPO_ROOT/registry.json" ]]; then
    cp "$REPO_ROOT/registry.json" "$scratch/registry.json"
  fi
  git -C "$scratch" init -q
}

set_protected_frontmatter_fields() {
  local profile="$1"
  local mutation="$2"

  python3 - "$profile" "$mutation" <<'PY'
import sys
from pathlib import Path

import yaml

path = Path(sys.argv[1])
mutation = sys.argv[2]
document = yaml.safe_load(path.read_text(encoding="utf-8"))
for profile in document["profiles"].values():
    copy_detection = profile["clean_room"]["copy_detection"]
    if mutation == "valid":
        copy_detection["protected_frontmatter_fields"] = ["description"]
    elif mutation == "missing":
        copy_detection.pop("protected_frontmatter_fields", None)
    elif mutation == "empty":
        copy_detection["protected_frontmatter_fields"] = []
    elif mutation == "invalid":
        copy_detection["protected_frontmatter_fields"] = "description"
    elif mutation == "duplicate":
        copy_detection["protected_frontmatter_fields"] = [
            "description",
            "description",
        ]
    elif mutation == "unknown":
        copy_detection["protected_frontmatter_fields"] = [
            "description",
            "summary",
        ]
    else:
        raise SystemExit(f"unknown test mutation: {mutation}")
path.write_text(yaml.safe_dump(document, sort_keys=False), encoding="utf-8")
PY
}

write_external_description_fixture() {
  local target="$1"
  local scalar_style="$2"
  local scalar_parts="$3"
  local first_part="${scalar_parts%%|*}"
  local second_part="${scalar_parts#*|}"

  {
    printf '%s\n' '---' 'name: foreign-description-probe'
    case "$scalar_style" in
      inline)
        printf 'description: "%s"\n' "$scalar_parts"
        ;;
      folded)
        printf '%s\n' 'description: >-'
        printf '  %s\n' "$first_part" "$second_part"
        ;;
      literal)
        printf '%s\n' 'description: |-'
        printf '  %s\n' "$first_part" "$second_part"
        ;;
      *)
        echo "unknown scalar style: $scalar_style" >&2
        return 1
        ;;
    esac
    printf '%s\n' '---' '# External Input' '' 'Synthetic body text is intentionally unrelated.'
  } >"$target"
}

@test "L2: inline trigger decisions converge on one observable profile and form id" {
  local skills_root="$BATS_TEST_TMPDIR/inline-skills"
  local scanner_json="$BATS_TEST_TMPDIR/inline-scanner.json"
  local audit_json="$BATS_TEST_TMPDIR/inline-audit.json"

  write_conforming_skill "$skills_root/inline-trigger" inline-trigger \
    "'Audits one skill. Triggers: \"audit skill\".'"

  run python3 "$SCANNER" "$skills_root" --json --strict
  [[ "$status" -eq 0 ]]
  write_report_from_output "$scanner_json"

  run bash "$AUDITOR" --strict --json "$audit_json" "$skills_root/inline-trigger"
  [[ "$status" -eq 0 ]]

  assert_json "$scanner_json" '.profile_id | type == "string" and length > 0' \
    'scanner identifies the selected profile'
  assert_json "$scanner_json" '.skills[0].forms == ["inline-marker"]' \
    'scanner reports the canonical inline-marker form'
  assert_json "$audit_json" '.profile_id | type == "string" and length > 0' \
    'auditor identifies the selected profile'
  assert_json "$audit_json" \
    '.pass2.checks[] | select(.id == "description-has-triggers") | .status == "pass" and (.forms | index("inline-marker") != null)' \
    'auditor accepts the same inline-marker form'
  [[ "$(jq -r .profile_id "$scanner_json")" == "$(jq -r .profile_id "$audit_json")" ]]
}

@test "L2: missing trigger is the same profile-declared WARN in scanner and auditor strict modes" {
  local skills_root="$BATS_TEST_TMPDIR/missing-trigger-skills"
  local scanner_json="$BATS_TEST_TMPDIR/missing-trigger-scanner.json"
  local audit_json="$BATS_TEST_TMPDIR/missing-trigger-audit.json"

  write_conforming_skill "$skills_root/missing-trigger" missing-trigger \
    "'Audits one skill without an explicit activation marker.'"

  run python3 "$SCANNER" "$skills_root" --json --strict
  [[ "$status" -eq 1 ]]
  write_report_from_output "$scanner_json"
  assert_json "$scanner_json" '.skills[0].has_trigger == false and .skills[0].forms == []' \
    'scanner accepts no trigger form from a plain description'

  run bash "$AUDITOR" --strict --json "$audit_json" "$skills_root/missing-trigger"
  [[ "$status" -eq 1 ]]
  assert_json "$audit_json" \
    '.pass2.checks[] | select(.id == "description-has-triggers") | .status == "warn" and ((.severity // "") | ascii_upcase) == "WARN"' \
    'auditor emits the canonical profile WARN for a missing trigger'
  [[ "$(jq -r .profile_id "$scanner_json")" == "$(jq -r .profile_id "$audit_json")" ]]
}

@test "L2: repo-runtime kernel boundary accepts 250 lines and flags 251 lines" {
  local at_limit="$BATS_TEST_TMPDIR/at-limit"
  local over_limit="$BATS_TEST_TMPDIR/over-limit"
  local at_json="$BATS_TEST_TMPDIR/at-limit.json"
  local over_json="$BATS_TEST_TMPDIR/over-limit.json"

  materialize_line_fixture "$at_limit" 250
  run bash "$AUDITOR" --strict --json "$at_json" "$at_limit"
  [[ "$status" -eq 0 ]]
  assert_json "$at_json" \
    '.pass2.checks[] | select(.id == "references-modularization") | .status == "pass"' \
    'a 250-line kernel remains within the profile boundary'

  materialize_line_fixture "$over_limit" 251
  run bash "$AUDITOR" --strict --json "$over_json" "$over_limit"
  [[ "$status" -ne 0 ]]
  assert_json "$over_json" \
    '.pass2.checks[] | select(.id == "references-modularization") | .status != "pass" and (.severity | type == "string" and length > 0)' \
    'a 251-line kernel emits the profile-declared modularization finding'
}

@test "L2: output-spec-explicit requires the complete executable handoff" {
  local incomplete="$BATS_TEST_TMPDIR/output-incomplete"
  local complete="$BATS_TEST_TMPDIR/output-complete"
  local incomplete_json="$BATS_TEST_TMPDIR/output-incomplete.json"
  local complete_json="$BATS_TEST_TMPDIR/output-complete.json"

  write_conforming_skill "$incomplete" output-incomplete \
    "'Builds a report. Triggers: \"build report\".'" incomplete
  run bash "$AUDITOR" --json "$incomplete_json" "$incomplete"
  [[ "$status" -eq 1 ]]
  assert_json "$incomplete_json" \
    '.verdict == "FAIL" and (.pass2.checks[] | select(.id == "output-spec-explicit") | .status == "fail")' \
    'JSON plus a path alone is an output-spec-explicit failure'

  write_conforming_skill "$complete" output-complete \
    "'Builds a report. Triggers: \"build report\".'" complete
  run bash "$AUDITOR" --json "$complete_json" "$complete"
  [[ "$status" -eq 0 ]]
  assert_json "$complete_json" \
    '.pass2.checks[] | select(.id == "output-spec-explicit") | .status == "pass"' \
    'artifact path, filename, schema, validator, and handoff pass together'
}

@test "L2: healer resolves canonical cross-skill and repo-root references without hiding DEAD_REF" {
  local fixture_root="$BATS_TEST_TMPDIR/heal-root"
  local skill_md="$fixture_root/skills/reference-fixture/SKILL.md"
  local valid_output

  mkdir -p "$fixture_root/skills/standards/references" \
    "$fixture_root/skills/reference-fixture" "$fixture_root/docs/contracts"
  cp "$REPO_ROOT/skills/standards/references/shell.md" \
    "$fixture_root/skills/standards/references/shell.md"
  write_conforming_skill "$fixture_root/skills/reference-fixture" reference-fixture \
    "'Checks references. Triggers: \"check references\".'"
  printf '%s\n' \
    '' \
    '## References' \
    '' \
    '- [Cross-skill](../standards/references/shell.md)' \
    '- [Repo-root](skills/standards/references/shell.md)' >>"$skill_md"
  printf '%s\n' \
    'dispositions:' \
    '  - skill: reference-fixture' \
    '    domain: "BC1 Corpus"' \
    '    hexagonal_role: supporting' \
    '    disposition: keep' \
    '    rationale: "acceptance fixture"' >"$fixture_root/docs/contracts/skill-dispositions.yaml"

  run env HEAL_REPO_ROOT="$fixture_root" bash "$HEALER" --check --strict skills/reference-fixture
  valid_output="$output"
  [[ "$valid_output" != *'DEAD_REF'* ]]
  [[ "$valid_output" != *'UNLINKED_REF'* ]]

  sed -i.bak 's#references/shell\.md#references/missing-shell.md#g' "$skill_md"
  rm -f "$skill_md.bak"
  run env HEAL_REPO_ROOT="$fixture_root" bash "$HEALER" --check --strict skills/reference-fixture
  [[ "$status" -eq 1 ]]
  [[ "$output" == *'DEAD_REF'* ]]
  [[ "$output" == *'missing-shell.md'* ]]
}

@test "L2: absorb-external synthesizes clean-room source and Codex outputs" {
  local scratch="$BATS_TEST_TMPDIR/builder-root"
  local external="$BATS_TEST_TMPDIR/external-skill.md"
  local name="clean-room-sentinel"
  local sentinel="MAGENTA ORBITAL WALRUS signs every borrowed paragraph."
  local report

  prepare_builder_root "$scratch"
  {
    printf '%s\n' \
      '---' \
      'name: foreign-sentinel' \
      'description: '\''External package. Triggers: "foreign sentinel".'\''' \
      '---' \
      '# Foreign Sentinel' \
      '' \
      "$sentinel" \
      '' \
      'Run the source prompt exactly and preserve its examples.'
  } >"$external"

  run env SKILL_BUILDER_REPO_ROOT="$scratch" SKILL_TIER=execution \
    SKILL_INTENT_MODE=task bash "$BUILDER" absorb-external "$name" --from "$external"
  [[ "$status" -eq 0 ]]
  if rg -F "$sentinel" "$scratch/skills/$name" "$scratch/skills-codex/$name"; then
    echo 'clean-room violation: external sentinel was copied into generated output' >&2
    return 1
  fi

  report="$scratch/.agents/audits/$name-build.json"
  [[ -s "$report" ]]
  assert_json "$report" \
    '.mode == "absorb-external" and .audit_pass == true and (.profile_id | type == "string" and length > 0)' \
    'successful clean-room build records its selected profile'
}

@test "L2: unknown profile severity fails both consumers closed with no verdict" {
  local scratch="$BATS_TEST_TMPDIR/bad-profile-root"
  local skills_root="$scratch/profile-fixtures"
  local copied_profile="$scratch/skills/skill-builder/references/skill-conformance-profiles.yaml"
  local scanner_output auditor_output

  [[ -f "$PROFILE" ]] || {
    echo "profile configuration missing: $PROFILE" >&2
    return 1
  }
  prepare_builder_root "$scratch"
  write_conforming_skill "$skills_root/bad-profile" bad-profile \
    "'Checks profile errors. Triggers: \"check profile\".'"
  perl -0pi -e 's/(severity:\s*)[A-Za-z]+/${1}BOGUS/' "$copied_profile"
  rg -q 'severity:[[:space:]]*BOGUS' "$copied_profile"

  run python3 "$scratch/skills/skill-builder/scripts/scan_descriptions.py" \
    "$skills_root" --json --strict
  scanner_output="$output"
  [[ "$status" -ne 0 ]]
  [[ "$scanner_output" =~ [Pp]rofile|[Cc]onfiguration ]]
  [[ "$scanner_output" == *'BOGUS'* ]]
  [[ "$scanner_output" != *'"verdict": "PASS"'* ]]
  [[ "$scanner_output" != *'"verdict": "WARN"'* ]]

  run env HEAL_REPO_ROOT="$scratch" \
    bash "$scratch/skills/heal-skill/scripts/audit.sh" --strict "$skills_root/bad-profile"
  auditor_output="$output"
  [[ "$status" -ne 0 ]]
  [[ "$auditor_output" =~ [Pp]rofile|[Cc]onfiguration ]]
  [[ "$auditor_output" == *'BOGUS'* ]]
  [[ "$auditor_output" != *'"verdict": "PASS"'* ]]
  [[ "$auditor_output" != *'"verdict": "WARN"'* ]]
}

@test "L1: source and Codex treatments resolve to one profile and clean-room doctrine" {
  local owner tree

  [[ -s "$PROFILE" ]]
  python3 - "$PROFILE" <<'PY'
import sys
from pathlib import Path

import yaml

payload = yaml.safe_load(Path(sys.argv[1]).read_text(encoding="utf-8"))
if not isinstance(payload, dict):
    raise SystemExit("profile document must be a YAML mapping")
if "repo-runtime" not in str(payload):
    raise SystemExit("profile document does not declare the repo-runtime profile")
PY

  for owner in skill-builder heal-skill standards; do
    for tree in skills skills-codex; do
      if ! rg -q 'skill-conformance-profiles\.yaml|repo-runtime' "$REPO_ROOT/$tree/$owner"; then
        echo "$tree/$owner does not resolve to the authoritative repo-runtime profile" >&2
        return 1
      fi
    done
  done

  [[ ! -e "$REPO_ROOT/skills-codex/skill-builder/references/skill-conformance-profiles.yaml" ]]
  rg -qi 'clean-room' "$REPO_ROOT/skills/skill-builder" "$REPO_ROOT/skills-codex/skill-builder"
  if rg -qi 'verbatim (preservation|copy)|preserve (the )?(external|source).*(verbatim|body)' \
      "$REPO_ROOT/skills/skill-builder" "$REPO_ROOT/skills-codex/skill-builder"; then
    echo 'builder doctrine still promises verbatim external-source preservation' >&2
    return 1
  fi
}

@test "L1: removing a required rule from both rule collections fails consumers closed" {
  local scratch="$BATS_TEST_TMPDIR/missing-rule-root"
  local skills_root="$scratch/profile-fixtures"
  local copied_profile="$scratch/skills/skill-builder/references/skill-conformance-profiles.yaml"
  local scanner_status scanner_output auditor_status auditor_output

  prepare_builder_root "$scratch"
  write_conforming_skill "$skills_root/missing-rule" missing-rule \
    "'Checks required rules. Triggers: \"check required rules\".'"
  python3 - "$copied_profile" <<'PY'
import sys
from pathlib import Path

import yaml

path = Path(sys.argv[1])
document = yaml.safe_load(path.read_text(encoding="utf-8"))
profile = document["profiles"][document["default_profile"]]
profile["rule_order"].remove("trigger-clarity")
del profile["rules"]["trigger-clarity"]
path.write_text(yaml.safe_dump(document, sort_keys=False), encoding="utf-8")
PY

  run python3 "$scratch/skills/skill-builder/scripts/scan_descriptions.py" \
    "$skills_root" --json --strict
  scanner_status="$status"
  scanner_output="$output"

  run env HEAL_REPO_ROOT="$scratch" \
    bash "$scratch/skills/heal-skill/scripts/audit.sh" --strict "$skills_root/missing-rule"
  auditor_status="$status"
  auditor_output="$output"

  [[ "$scanner_status" -ne 0 ]] || {
    echo 'scanner accepted a profile missing the required trigger-clarity rule' >&2
    return 1
  }
  [[ "$scanner_output" == *'profile configuration error'* && "$scanner_output" == *'trigger-clarity'* ]] || {
    echo "scanner did not report the missing rule as an actionable profile configuration error: $scanner_output" >&2
    return 1
  }
  [[ "$auditor_status" -ne 0 ]] || {
    echo 'auditor accepted a profile missing the required trigger-clarity rule' >&2
    return 1
  }
  [[ "$auditor_output" == *'profile configuration error'* && "$auditor_output" == *'trigger-clarity'* ]] || {
    echo "auditor did not report the missing rule as an actionable profile configuration error: $auditor_output" >&2
    return 1
  }
  [[ "$scanner_output" != *'"verdict": "PASS"'* && "$auditor_output" != *'VERDICT: PASS'* ]]
}

@test "L1: adding an invented rule to both rule collections fails consumers closed" {
  local scratch="$BATS_TEST_TMPDIR/invented-rule-root"
  local skills_root="$scratch/profile-fixtures"
  local copied_profile="$scratch/skills/skill-builder/references/skill-conformance-profiles.yaml"
  local scanner_status scanner_output auditor_status auditor_output

  prepare_builder_root "$scratch"
  write_conforming_skill "$skills_root/invented-rule" invented-rule \
    "'Checks known rules. Triggers: \"check known rules\".'"
  python3 - "$copied_profile" <<'PY'
import sys
from pathlib import Path

import yaml

path = Path(sys.argv[1])
document = yaml.safe_load(path.read_text(encoding="utf-8"))
profile = document["profiles"][document["default_profile"]]
profile["rule_order"].append("invented-rule")
profile["rules"]["invented-rule"] = {"severity": "WARN"}
path.write_text(yaml.safe_dump(document, sort_keys=False), encoding="utf-8")
PY

  run python3 "$scratch/skills/skill-builder/scripts/scan_descriptions.py" \
    "$skills_root" --json --strict
  scanner_status="$status"
  scanner_output="$output"

  run env HEAL_REPO_ROOT="$scratch" \
    bash "$scratch/skills/heal-skill/scripts/audit.sh" --strict "$skills_root/invented-rule"
  auditor_status="$status"
  auditor_output="$output"

  [[ "$scanner_status" -ne 0 ]] || {
    echo 'scanner accepted the undeclared invented-rule profile entry' >&2
    return 1
  }
  [[ "$scanner_output" == *'profile configuration error'* && "$scanner_output" == *'invented-rule'* ]] || {
    echo "scanner did not report the invented rule as an actionable profile configuration error: $scanner_output" >&2
    return 1
  }
  [[ "$auditor_status" -ne 0 ]] || {
    echo 'auditor accepted the undeclared invented-rule profile entry' >&2
    return 1
  }
  [[ "$auditor_output" == *'profile configuration error'* && "$auditor_output" == *'invented-rule'* ]] || {
    echo "auditor did not report the invented rule as an actionable profile configuration error: $auditor_output" >&2
    return 1
  }
  [[ "$scanner_output" != *'"verdict": "PASS"'* && "$auditor_output" != *'VERDICT: PASS'* ]]
}

@test "L2: metadata-only trigger emits trigger-clarity WARN and fails both strict consumers" {
  local skills_root="$BATS_TEST_TMPDIR/metadata-only-skills"
  local skill_md="$skills_root/metadata-only/SKILL.md"
  local scanner_json="$BATS_TEST_TMPDIR/metadata-only-scanner.json"
  local audit_json="$BATS_TEST_TMPDIR/metadata-only-audit.json"
  local scanner_status auditor_status

  write_conforming_skill "$skills_root/metadata-only" metadata-only \
    "'Audits one skill using only structured activation metadata.'"
  python3 - "$skill_md" <<'PY'
import sys
from pathlib import Path

path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8")
text = text.replace(
    "metadata:\n  tier:",
    "metadata:\n  triggers:\n    - audit metadata\n    - scan metadata\n    - validate metadata\n  tier:",
    1,
)
path.write_text(text, encoding="utf-8")
PY

  run python3 "$SCANNER" "$skills_root" --json --strict
  scanner_status="$status"
  write_report_from_output "$scanner_json"
  assert_json "$scanner_json" '.skills[0].forms == ["metadata-list"]' \
    'metadata-list is the only accepted general trigger form'
  assert_json "$scanner_json" \
    '.skills[0].checks[] | select(.id == "trigger-clarity") | .status == "warn" and .severity == "WARN"' \
    'scanner emits the profile-declared trigger-clarity WARN'

  run bash "$AUDITOR" --strict --json "$audit_json" "$skills_root/metadata-only"
  auditor_status="$status"
  assert_json "$audit_json" \
    '.pass2.checks[] | select(.id == "trigger-clarity") | .status == "warn" and .severity == "WARN"' \
    'auditor emits the matching trigger-clarity WARN'
  [[ "$(jq -r .profile_id "$scanner_json")" == "$(jq -r .profile_id "$audit_json")" ]]

  [[ "$scanner_status" -ne 0 ]] || {
    echo 'scanner strict mode returned zero despite its emitted trigger-clarity WARN' >&2
    return 1
  }
  [[ "$auditor_status" -ne 0 ]] || {
    echo 'auditor strict mode returned zero despite its emitted trigger-clarity WARN' >&2
    return 1
  }
}

@test "L2: scanner human report identifies the selected profile" {
  local skills_root="$BATS_TEST_TMPDIR/human-profile-skills"

  write_conforming_skill "$skills_root/human-profile" human-profile \
    "'Audits one skill. Triggers: \"audit human profile\".'"

  run python3 "$SCANNER" "$skills_root"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *'repo-runtime'* ]] || {
    echo "scanner human output omitted selected profile repo-runtime: $output" >&2
    return 1
  }
}

@test "L1: absorb-external fails closed when declared clean-room policy is missing" {
  local scratch="$BATS_TEST_TMPDIR/missing-clean-room-policy-root"
  local copied_profile="$scratch/skills/skill-builder/references/skill-conformance-profiles.yaml"
  local external="$BATS_TEST_TMPDIR/missing-clean-room-policy-external.md"
  local name="missing-clean-room-policy"
  local build_status build_output report

  prepare_builder_root "$scratch"
  python3 - "$copied_profile" <<'PY'
import sys
from pathlib import Path

import yaml

path = Path(sys.argv[1])
document = yaml.safe_load(path.read_text(encoding="utf-8"))
profile = document["profiles"][document["default_profile"]]
del profile["clean_room"]["external_content_policy"]
path.write_text(yaml.safe_dump(document, sort_keys=False), encoding="utf-8")
PY
  {
    printf '%s\n' \
      '---' \
      'name: foreign-policy-probe' \
      'description: '\''External package. Triggers: "foreign policy probe".'\''' \
      '---' \
      '# Foreign Policy Probe' \
      '' \
      'SAPPHIRE CLOCKWORK HERON marks content that policy must govern.'
  } >"$external"

  run env SKILL_BUILDER_REPO_ROOT="$scratch" SKILL_TIER=execution \
    SKILL_INTENT_MODE=task bash "$scratch/skills/skill-builder/scripts/build.sh" \
    absorb-external "$name" --from "$external"
  build_status="$status"
  build_output="$output"

  [[ "$build_status" -ne 0 ]] || {
    echo 'builder accepted absorb-external without the declared external_content_policy' >&2
    return 1
  }
  [[ "$build_output" == *'profile configuration error'* && "$build_output" == *'external_content_policy'* ]] || {
    echo "builder did not identify the missing clean-room policy as a profile configuration error: $build_output" >&2
    return 1
  }
  report="$scratch/.agents/audits/$name-build.json"
  if [[ -s "$report" ]]; then
    assert_json "$report" '.audit_pass != true' \
      'a clean-room configuration failure cannot produce a passing build report'
  fi
}

@test "L1: source and Codex severity doctrine matches profile WARN rules" {
  python3 - "$PROFILE" \
    "$REPO_ROOT/skills/heal-skill/scripts/audit.sh" \
    "$REPO_ROOT/skills-codex/heal-skill/scripts/audit.sh" <<'PY'
import re
import sys
from pathlib import Path

import yaml

profile_path = Path(sys.argv[1])
document = yaml.safe_load(profile_path.read_text(encoding="utf-8"))
profile = document["profiles"][document["default_profile"]]
severities = {
    rule_id: rule["severity"]
    for rule_id, rule in profile["rules"].items()
}
annotation = re.compile(
    r"^# Check \d+: (?P<rule>[a-z0-9-]+) \((?P<severity>WARN|FAIL) on miss"
)
mismatches = []
for raw_path in sys.argv[2:]:
    path = Path(raw_path)
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        match = annotation.match(line)
        if not match:
            continue
        rule_id = match.group("rule")
        declared = severities.get(rule_id)
        observed = match.group("severity")
        if declared is None:
            mismatches.append(f"{path}:{line_number}: unknown doctrine rule {rule_id}")
        elif observed != declared:
            mismatches.append(
                f"{path}:{line_number}: {rule_id} says {observed} on miss; "
                f"profile declares {declared}"
            )
if mismatches:
    raise SystemExit("\n".join(mismatches))
PY
}

@test "RESET RED: protected external description prose is rejected across scalar and target matrix" {
  local scratch="$BATS_TEST_TMPDIR/description-matrix-root"
  local copied_profile="$scratch/skills/skill-builder/references/skill-conformance-profiles.yaml"
  local verifier="$scratch/skills/skill-builder/scripts/conformance_profile.py"
  local failures=0
  local index case_name scalar_style target scalar_parts normalized
  local case_root external source_dir codex_dir source_status codex_status
  local source_output codex_output expected_source expected_codex
  local -a case_names=(
    inline-source
    inline-codex-only
    folded-source
    folded-codex
    literal-source
    literal-codex
  )
  local -a scalar_styles=(inline inline folded folded literal literal)
  local -a targets=(source codex source codex source codex)
  local -a descriptions=(
    'SAPPHIRE CLOCKWORK HERON marks protected external prose.'
    'COPPER CELESTIAL BADGER guards Codex-only protected prose.'
    'AMBER IBIS guards|semantic copy.'
    'TEAL PANTHER guards|Codex semantic copy.'
    'VIOLET OTTER guards|literal semantic copy.'
    'SILVER FALCON guards|Codex literal copy.'
  )

  prepare_builder_root "$scratch"
  set_protected_frontmatter_fields "$copied_profile" valid

  for index in "${!case_names[@]}"; do
    case_name="${case_names[$index]}"
    scalar_style="${scalar_styles[$index]}"
    target="${targets[$index]}"
    scalar_parts="${descriptions[$index]}"
    normalized="${scalar_parts//|/ }"
    case_root="$BATS_TEST_TMPDIR/description-$case_name"
    external="$case_root/external.md"
    source_dir="$case_root/skills/generated-skill"
    codex_dir="$case_root/skills-codex/generated-skill"
    mkdir -p "$source_dir" "$codex_dir"
    write_external_description_fixture "$external" "$scalar_style" "$scalar_parts"
    printf '%s\n' 'Independent source prose contains no protected phrase.' \
      >"$source_dir/SKILL.md"
    printf '%s\n' 'Independent Codex prose contains no protected phrase.' \
      >"$codex_dir/SKILL.md"
    if [[ "$target" == "source" ]]; then
      printf '%s\n' "$normalized" >"$source_dir/SKILL.md"
      expected_source=1
      expected_codex=0
    else
      printf '%s\n' "$normalized" >"$codex_dir/SKILL.md"
      expected_source=0
      expected_codex=1
    fi

    run python3 "$verifier" --repo-root "$scratch" --profile-id repo-runtime \
      --verify-clean-room "$external" --generated-dir "$source_dir"
    source_status="$status"
    source_output="$output"
    run python3 "$verifier" --repo-root "$scratch" --profile-id repo-runtime \
      --verify-clean-room "$external" --generated-dir "$codex_dir"
    codex_status="$status"
    codex_output="$output"

    printf 'DESCRIPTION_MATRIX case=%s style=%s target=%s source_status=%s codex_status=%s\n' \
      "$case_name" "$scalar_style" "$target" "$source_status" "$codex_status"
    if [[ "$expected_source" -eq 1 ]]; then
      if [[ "$source_status" -eq 0 || "$source_output" != *'clean-room violation'* || \
          "$source_output" != *'copied external'* ]]; then
        echo "source target did not reject copied description for $case_name: $source_output" >&2
        failures=$((failures + 1))
      fi
    elif [[ "$source_status" -ne 0 ]]; then
      echo "independent source target failed for $case_name: $source_output" >&2
      failures=$((failures + 1))
    fi
    if [[ "$expected_codex" -eq 1 ]]; then
      if [[ "$codex_status" -eq 0 || "$codex_output" != *'clean-room violation'* || \
          "$codex_output" != *'copied external'* ]]; then
        echo "Codex target did not reject copied description for $case_name: $codex_output" >&2
        failures=$((failures + 1))
      fi
    elif [[ "$codex_status" -ne 0 ]]; then
      echo "independent Codex target failed for $case_name: $codex_output" >&2
      failures=$((failures + 1))
    fi
  done

  [[ "$failures" -eq 0 ]] || {
    echo "description acceptance matrix failures: $failures/${#case_names[@]}" >&2
    return 1
  }
}

@test "RESET RED: protected-frontmatter profile mutations fail builder closed" {
  local scratch="$BATS_TEST_TMPDIR/profile-matrix-root"
  local copied_profile="$scratch/skills/skill-builder/references/skill-conformance-profiles.yaml"
  local baseline_profile="$BATS_TEST_TMPDIR/profile-with-protected-description.yaml"
  local external="$BATS_TEST_TMPDIR/profile-matrix-external.md"
  local failures=0
  local mutation name build_status build_output report profile_error audit_pass
  local -a mutations=(missing empty invalid duplicate unknown)

  prepare_builder_root "$scratch"
  set_protected_frontmatter_fields "$copied_profile" valid
  cp "$copied_profile" "$baseline_profile"
  write_external_description_fixture "$external" inline \
    'GOLDEN INDEPENDENT LYNX describes a foreign capability.'

  for mutation in "${mutations[@]}"; do
    cp "$baseline_profile" "$copied_profile"
    set_protected_frontmatter_fields "$copied_profile" "$mutation"
    name="profile-$mutation-description"
    run env SKILL_BUILDER_REPO_ROOT="$scratch" SKILL_TIER=execution \
      SKILL_INTENT_MODE=task bash "$scratch/skills/skill-builder/scripts/build.sh" \
      absorb-external "$name" --from "$external"
    build_status="$status"
    build_output="$output"
    report="$scratch/.agents/audits/$name-build.json"
    profile_error=no
    if [[ "$build_output" == *'profile configuration error'* && \
        "$build_output" == *'protected_frontmatter_fields'* ]]; then
      profile_error=yes
    fi
    audit_pass=missing
    if [[ -s "$report" ]]; then
      audit_pass="$(jq -r '.audit_pass // false' "$report")"
    fi
    printf 'PROFILE_MATRIX mutation=%s status=%s profile_error=%s audit_pass=%s\n' \
      "$mutation" "$build_status" "$profile_error" "$audit_pass"

    if [[ "$build_status" -eq 0 || "$profile_error" != yes || "$audit_pass" == true ]]; then
      echo "builder did not fail closed for $mutation protected fields" >&2
      failures=$((failures + 1))
    fi
  done

  [[ "$failures" -eq 0 ]] || {
    echo "protected-frontmatter configuration failures: $failures/${#mutations[@]}" >&2
    return 1
  }
}

@test "RESET GREEN: independently synthesized descriptions pass both generated targets" {
  local scratch="$BATS_TEST_TMPDIR/independent-description-root"
  local copied_profile="$scratch/skills/skill-builder/references/skill-conformance-profiles.yaml"
  local verifier="$scratch/skills/skill-builder/scripts/conformance_profile.py"
  local external="$BATS_TEST_TMPDIR/independent-description-external.md"
  local source_dir="$BATS_TEST_TMPDIR/independent-description/skills/generated-skill"
  local codex_dir="$BATS_TEST_TMPDIR/independent-description/skills-codex/generated-skill"

  prepare_builder_root "$scratch"
  set_protected_frontmatter_fields "$copied_profile" valid
  write_external_description_fixture "$external" folded \
    'CRIMSON FOREIGN MARMOT explains|a protected outside capability.'
  mkdir -p "$source_dir" "$codex_dir"
  printf '%s\n' 'AgentOps source synthesis validates a local workflow.' >"$source_dir/SKILL.md"
  printf '%s\n' 'Codex synthesis routes a separately authored local workflow.' >"$codex_dir/SKILL.md"

  run python3 "$verifier" --repo-root "$scratch" --profile-id repo-runtime \
    --verify-clean-room "$external" --generated-dir "$source_dir"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *'repo-runtime'* ]]
  run python3 "$verifier" --repo-root "$scratch" --profile-id repo-runtime \
    --verify-clean-room "$external" --generated-dir "$codex_dir"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *'repo-runtime'* ]]
}
