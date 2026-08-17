#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: doc" "grep -q '^name: doc' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions documentation generation" "grep -qi 'generate.*doc\|documentation' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions code-map" "grep -qi 'code-map\|code map' '$SKILL_DIR/SKILL.md'"
check "SKILL.md front-loads constraints" "grep -q '^## Constraints' '$SKILL_DIR/SKILL.md'"
check "SKILL.md declares output specification" "grep -q '^## Output Specification' '$SKILL_DIR/SKILL.md'"
check "output specification declares validation and handoff" "grep -qi 'validation command' '$SKILL_DIR/SKILL.md' && grep -qi 'downstream handoff' '$SKILL_DIR/SKILL.md'"
check "SKILL.md declares quality checklist" "grep -q '^## Quality Checklist' '$SKILL_DIR/SKILL.md'"
check "OSS scaffold is missing-only by default" "grep -Eqi 'OSS scaffold.*missing.*only by default' '$SKILL_DIR/SKILL.md'"
check "OSS existing-doc writes require explicit user confirmation" "grep -Eqi 'OSS.*existing doc.*explicit user confirmation' '$SKILL_DIR/SKILL.md'"
check "OSS output avoids broad create-or-update claim" "! grep -Eqi 'OSS mode (creates?|writes?) or updates?' '$SKILL_DIR/SKILL.md'"
check "OSS reference preserves explicit confirmation boundary" "grep -Eqi 'refresh.*explicit user confirmation' '$SKILL_DIR/references/oss-pack.md'"
check "OSS executable spec covers confirmed refresh" "grep -Eqi 'explicit user confirmation before updating or overwriting' '$SKILL_DIR/references/oss-docs.feature'"
check "scanner requires authorization and bounds" "grep -q -- '--authorization-id' '$SKILL_DIR/scripts/audit-oss-docs.sh' && grep -q 'scan entry ceiling exceeded' '$SKILL_DIR/scripts/audit-oss-docs.sh' && grep -q 'scan deadline exceeded' '$SKILL_DIR/scripts/audit-oss-docs.sh'"
check "live cluster verification is approval-gated and allowlisted" "grep -q 'Live verification is opt-in' '$SKILL_DIR/references/validation-rules.md' && grep -q 'secrets, ConfigMaps, logs' '$SKILL_DIR/references/validation-rules.md' && grep -q -- '--request-timeout=10s' '$SKILL_DIR/references/validation-rules.md'"

audit_tmp="$(mktemp -d)"
trap 'rm -rf -- "$audit_tmp"' EXIT
mkdir -p "$audit_tmp/src"
touch "$audit_tmp/LICENSE" "$audit_tmp/README.md" "$audit_tmp/src/one.py"

if bash "$SKILL_DIR/scripts/audit-oss-docs.sh" --root "$audit_tmp" --scan-path src --json; then
  echo "FAIL: scanner started without authorization" >&2
  FAIL=$((FAIL + 1))
else
  echo "PASS: scanner rejects missing authorization"
  PASS=$((PASS + 1))
fi

if bash "$SKILL_DIR/scripts/audit-oss-docs.sh" --authorization-id test:forbidden --root / --json; then
  echo "FAIL: scanner accepted broad root" >&2
  FAIL=$((FAIL + 1))
else
  echo "PASS: scanner rejects broad root"
  PASS=$((PASS + 1))
fi

if bash "$SKILL_DIR/scripts/audit-oss-docs.sh" --authorization-id test:path \
  --root "$audit_tmp" --scan-path ../outside --json; then
  echo "FAIL: scanner accepted a non-allowlisted path" >&2
  FAIL=$((FAIL + 1))
else
  echo "PASS: scanner rejects a non-allowlisted path"
  PASS=$((PASS + 1))
fi

escape_project="$audit_tmp/escape-project"
mkdir -p "$escape_project/src"
printf '%s\n' 'outside remains unchanged' > "$audit_tmp/outside.py"
ln -s "$audit_tmp/outside.py" "$escape_project/src/escape.py"
outside_digest="$(shasum -a 256 "$audit_tmp/outside.py" | awk '{print $1}')"
if bash "$SKILL_DIR/scripts/audit-oss-docs.sh" --authorization-id test:symlink \
  --root "$escape_project" --scan-path src --json; then
  echo "FAIL: scanner followed an escaping symlink" >&2
  FAIL=$((FAIL + 1))
else
  echo "PASS: scanner rejects an escaping symlink"
  PASS=$((PASS + 1))
fi
[[ "$(shasum -a 256 "$audit_tmp/outside.py" | awk '{print $1}')" == "$outside_digest" ]]

ceiling_project="$audit_tmp/ceiling-project"
mkdir -p "$ceiling_project/src"
touch "$ceiling_project/src/one.py" "$ceiling_project/src/two.py"
if bash "$SKILL_DIR/scripts/audit-oss-docs.sh" --authorization-id test:ceiling \
  --root "$ceiling_project" --scan-path src --max-files 1 --json; then
  echo "FAIL: scanner exceeded its entry ceiling" >&2
  FAIL=$((FAIL + 1))
else
  echo "PASS: scanner stops at its entry ceiling"
  PASS=$((PASS + 1))
fi

if bash "$SKILL_DIR/scripts/audit-oss-docs.sh" --authorization-id test:bounded \
  --root "$audit_tmp" --scan-path src --max-files 20 --deadline-seconds 5 --json \
  | jq -e '.authorization_id == "test:bounded" and .effects.writes == [] and .scan.source_files == 1'; then
  echo "PASS: bounded scanner returns declared read-only effects"
  PASS=$((PASS + 1))
else
  echo "FAIL: bounded scanner positive workflow" >&2
  FAIL=$((FAIL + 1))
fi

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
