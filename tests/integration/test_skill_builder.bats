#!/usr/bin/env bats

setup_file() {
  REAL_REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../.." && pwd)"
  SCRATCH_ROOT="$BATS_FILE_TMPDIR/repo"
  mkdir -p "$SCRATCH_ROOT"
  SCRATCH_ROOT="$(cd "$SCRATCH_ROOT" && pwd -P)"
  if [[ -z "${AO_SKILL_BUILDER_BIN:-}" ]]; then
    AO_SKILL_BUILDER_BIN="$BATS_FILE_TMPDIR/ao"
    (cd "$REAL_REPO_ROOT/cli" && go build -o "$AO_SKILL_BUILDER_BIN" ./cmd/ao)
  fi
  export AO_SKILL_BUILDER_BIN
  mkdir -p "$BATS_FILE_TMPDIR/reports"
  REPORT_DIR="$(cd "$BATS_FILE_TMPDIR/reports" && pwd -P)"
  export REPORT_DIR
  cp -R "$REAL_REPO_ROOT/skills" "$SCRATCH_ROOT/skills"
  cp -R "$REAL_REPO_ROOT/scripts" "$SCRATCH_ROOT/scripts"
  cp -R "$REAL_REPO_ROOT/docs" "$SCRATCH_ROOT/docs"
  cp -R "$REAL_REPO_ROOT/skills-codex" "$SCRATCH_ROOT/skills-codex"
  cp -R "$REAL_REPO_ROOT/skills-codex-overrides" "$SCRATCH_ROOT/skills-codex-overrides"
  cp -R "$REAL_REPO_ROOT/images" "$SCRATCH_ROOT/images"
  cp -R "$REAL_REPO_ROOT/.claude-plugin" "$SCRATCH_ROOT/.claude-plugin"
  cp "$REAL_REPO_ROOT/registry.json" "$SCRATCH_ROOT/registry.json"
  export REAL_REPO_ROOT SCRATCH_ROOT
  export SKILL_BUILDER_REPO_ROOT="$SCRATCH_ROOT"
  BUILD_SH="$REAL_REPO_ROOT/skills/skill-builder/scripts/build.sh"
  INIT_SH="$REAL_REPO_ROOT/skills/skill-builder/scripts/init.sh"
  export BUILD_SH INIT_SH
}

@test "builder rejects missing and unknown modes" {
  run bash "$BUILD_SH"
  [ "$status" -eq 2 ]

  run bash "$BUILD_SH" removed-mode example
  [ "$status" -eq 2 ]
}

@test "one invocation creates metadata source and derived projections" {
  name="builder-contract-test"
  run env \
    SKILL_TIER=execution \
    SKILL_DEPENDENCIES='[]' \
    SKILL_EFFECTS='[]' \
    bash "$BUILD_SH" from-scratch "$name" --report "$REPORT_DIR/${name}-build.json"
  [ "$status" -eq 0 ]

  source="$SCRATCH_ROOT/skills/$name/SKILL.md"
  [ -f "$source" ]
  [ ! -e "$SCRATCH_ROOT/skills/$name/scripts" ]
  grep -q '"authoring_state": "scaffold"' "$REPORT_DIR/${name}-build.json"
  run env HEAL_REPO_ROOT="$SCRATCH_ROOT" bash "$REAL_REPO_ROOT/skills/skill-builder/scripts/heal.sh" --check --strict "$SCRATCH_ROOT/skills/$name"
  [ "$status" -eq 1 ]
  [[ "$output" == *INCOMPLETE_SCAFFOLD* ]]
  grep -q '^  canonical_status: canonical$' "$source"
  grep -q '^  disposition: keep_specialist$' "$source"
  grep -q '^  capabilities: ' "$source"
  grep -q 'builder_contract_test' "$source"
  grep -q '^practices: \[\]$' "$source"
  grep -q '^user-invocable: true$' "$source"

  [ -f "$SCRATCH_ROOT/skills-codex/$name/SKILL.md" ]
  [ -f "$SCRATCH_ROOT/skills-codex/$name/prompt.md" ]
  grep -q "\"name\": \"$name\"" "$SCRATCH_ROOT/skills/catalog.json"
  grep -q '"structure_check_pass": true' \
    "$REPORT_DIR/${name}-build.json"
}

@test "initializer accepts template and external creation modes with defaults" {
  run bash "$INIT_SH" --template builder-template-test --like plan
  [ "$status" -eq 0 ]
  grep -q 'builder_template_test' "$SCRATCH_ROOT/skills/builder-template-test/SKILL.md"

  external="$BATS_TEST_TMPDIR/external-skill.md"
  printf '%s\n' '# External skill source' > "$external"
  run bash "$INIT_SH" --external builder-external-test --from "$external"
  [ "$status" -eq 0 ]
  grep -q 'builder_external_test' "$SCRATCH_ROOT/skills/builder-external-test/SKILL.md"
}

@test "builder does not create lifecycle ledgers or touch the real repository" {
  [ ! -e "$SCRATCH_ROOT/docs/contracts/skill-dispositions.yaml" ]
  [ ! -e "$REAL_REPO_ROOT/skills/builder-contract-test" ]
  [ ! -e "$REAL_REPO_ROOT/skills-codex/builder-contract-test" ]
  [ ! -e "$REAL_REPO_ROOT/.agents/audits/builder-contract-test-build.json" ]
}

@test "completed concise adapter checks projects and audits without helper or heading ceremony" {
  name=builder-contract-test
  source="$SCRATCH_ROOT/skills/$name/SKILL.md"
  python3 - "$source" <<'PYCODE'
from pathlib import Path
import sys
p=Path(sys.argv[1]); fm=p.read_text().split('---',2)[1]
fm=fm.replace("description: 'TODO: state when this behavior applies.'", "description: 'Inspect current Git changes in the caller-selected repository.'")
fm=fm.replace('  authoring_state: scaffold\n','')
p.write_text('---'+fm+'---\nFor a request to inspect current Git changes, run `git status --short` in the caller-selected repository. Report changed paths inline. Do not alter files or Git state. Finish after the command succeeds and the paths are reported; if Git fails or the directory is not a repository, report that error and stop.\n')
PYCODE
  run env HEAL_REPO_ROOT="$SCRATCH_ROOT" bash "$SCRATCH_ROOT/skills/skill-builder/scripts/heal.sh" --check --strict "$SCRATCH_ROOT/skills/$name"
  [ "$status" -eq 0 ]
  cp -R "$SCRATCH_ROOT/skills-codex/plan" "$BATS_TEST_TMPDIR/sibling-before"
  run env HEAL_REPO_ROOT="$SCRATCH_ROOT" bash "$SCRATCH_ROOT/skills/skill-builder/scripts/heal.sh" --fix "$SCRATCH_ROOT/skills/$name/"
  [ "$status" -eq 0 ]
  diff -r "$BATS_TEST_TMPDIR/sibling-before" "$SCRATCH_ROOT/skills-codex/plan"
  # Equivalent relative spelling must retain this skill and sibling boundary.
  cd "$SCRATCH_ROOT"
  run env HEAL_REPO_ROOT="$SCRATCH_ROOT" bash "$SCRATCH_ROOT/skills/skill-builder/scripts/heal.sh" --fix "skills/$name/"
  [ "$status" -eq 0 ]
  diff -r "$BATS_TEST_TMPDIR/sibling-before" "$SCRATCH_ROOT/skills-codex/plan"
  [ ! -e "$SCRATCH_ROOT/skills-codex/$name/scripts" ]
  grep -q 'Report changed paths inline' "$SCRATCH_ROOT/skills-codex/$name/SKILL.md"
  run bash "$SCRATCH_ROOT/skills/skill-builder/scripts/audit.sh" --strict "$SCRATCH_ROOT/skills/$name"
  [ "$status" -eq 0 ]
  [[ "$output" == *NOT_PROVEN* ]]
  printf '\n## Result to caller\nThe answer is inline.\n' >> "$source"
  run bash "$SCRATCH_ROOT/skills/skill-builder/scripts/audit.sh" --strict "$SCRATCH_ROOT/skills/$name"
  [ "$status" -eq 0 ]
  printf '\n[required instructions](references/missing.md)\n' >> "$source"
  run bash "$SCRATCH_ROOT/skills/skill-builder/scripts/audit.sh" --strict "$SCRATCH_ROOT/skills/$name"
  [ "$status" -eq 1 ]
  [[ "$output" == *DEAD_REF* ]]
}

@test "Go creation input failures migrate explicitly to exit one without mutation" {
  run bash "$INIT_SH" --scratch ../invalid
  [ "$status" -eq 1 ]
  [[ "$output" == *lowercase-hyphen* ]]
  run bash "$INIT_SH" --external missing-external --from /nonexistent/skill.md
  [ "$status" -eq 1 ]
  [ ! -e "$SCRATCH_ROOT/skills/missing-external" ]
}

@test "development launcher preserves caller-relative external input paths" {
  mkdir -p "$BATS_TEST_TMPDIR/caller directory"
  cd "$BATS_TEST_TMPDIR/caller directory"
  printf '%s\n' '# External source stays outside the generated skill' > 'input file.md'
  run env -u AO_SKILL_BUILDER_BIN bash "$BUILD_SH" absorb-external caller-relative-test --from 'input file.md' --init-only
  [ "$status" -eq 0 ]
  [ -f "$SCRATCH_ROOT/skills/caller-relative-test/SKILL.md" ]
  [[ "$output" == *'"source_hint":"input file.md"'* ]]
  ! grep -q 'External source stays outside' "$SCRATCH_ROOT/skills/caller-relative-test/SKILL.md"
}

@test "projection failure retains source and failed report for remaining-stage recovery" {
  name=builder-recovery-test
  obstruction="$SCRATCH_ROOT/skills-codex/$name"
  printf '%s\n' 'injected projection obstruction' > "$obstruction"
  run bash "$BUILD_SH" from-scratch "$name" --report "$REPORT_DIR/${name}-failed.json"
  [ "$status" -eq 1 ]
  [[ "$output" == *"projection incomplete"* ]]
  source="$SCRATCH_ROOT/skills/$name/SKILL.md"
  [ -f "$source" ]
  grep -q '"structure_check_pass": false' "$REPORT_DIR/${name}-failed.json"
  cp "$REPORT_DIR/${name}-failed.json" "$BATS_TEST_TMPDIR/failed-original.json"
  printf '\nRetained caller note: do not recreate.\n' >> "$source"
  run bash "$BUILD_SH" from-scratch "$name"
  [ "$status" -eq 1 ]
  grep -q 'Retained caller note' "$source"

  # Remove only this test-owned obstruction and complete the retained source.
  rm -- "$obstruction"
  python3 - "$source" <<'PYCODE'
from pathlib import Path
import sys
p=Path(sys.argv[1]);s=p.read_text()
s=s.replace("description: 'TODO: state when this behavior applies.'", "description: 'Inspect current Git changes in a selected repository.'")
s=s.replace('  authoring_state: scaffold\n','')
a=s.index('TODO: State the applicable request');b=s.index('Retained caller note:')
s=s[:a]+'Run `git status --short` in the caller-selected repository and report paths inline. Do not alter Git or files. Stop after reporting the actual result or error.\n\n'+s[b:]
p.write_text(s)
PYCODE
  run env HEAL_REPO_ROOT="$SCRATCH_ROOT" bash "$SCRATCH_ROOT/skills/skill-builder/scripts/heal.sh" --check --strict "$SCRATCH_ROOT/skills/$name"
  [ "$status" -eq 0 ]
  run env HEAL_REPO_ROOT="$SCRATCH_ROOT" bash "$SCRATCH_ROOT/skills/skill-builder/scripts/heal.sh" --fix "$SCRATCH_ROOT/skills/$name"
  [ "$status" -eq 0 ]
  run bash "$SCRATCH_ROOT/scripts/regen-codex-hashes.sh" --only "$name"
  [ "$status" -eq 0 ]
  grep -q 'Retained caller note' "$SCRATCH_ROOT/skills-codex/$name/SKILL.md"
  run bash "$SCRATCH_ROOT/skills/skill-builder/scripts/audit.sh" --strict "$SCRATCH_ROOT/skills/$name"
  [ "$status" -eq 0 ]
  [[ "$output" == *NOT_PROVEN* ]]
  cmp "$REPORT_DIR/${name}-failed.json" "$BATS_TEST_TMPDIR/failed-original.json"
}
