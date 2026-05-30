#!/usr/bin/env bats
# ag-ekyq: skill-builder init.sh must regenerate registry.json (the SKU catalog)
# as part of new-skill scaffolding — the 5th one-shot-green surface ag-cw2y missed.
# A stale registry.json trips contracts-sync ("registry.json is stale") AND
# correctness(ubuntu) ("SKU_CATALOG: DRIFT") together; it cost /burndown #600 a
# 2nd fix-and-repush. generate-registry.sh scans the whole skills/ tree (not
# repo-root-injectable), so the contract we lock here is the WIRING + ORDERING:
# init.sh must invoke the canonical generator, AFTER the skeleton + the other
# three plumbing surfaces exist on disk.

INIT="$BATS_TEST_DIRNAME/../../skills/skill-builder/scripts/init.sh"

@test "init.sh invokes the canonical generate-registry.sh during scaffolding" {
  run grep -E 'scripts/generate-registry\.sh' "$INIT"
  [ "$status" -eq 0 ]
}

@test "the registry regen is guarded with a WARN fallback like its sibling steps" {
  # Must not hard-fail the scaffold if regen has trouble — same || echo WARN shape
  # as the dispositions / counts / override-catalog steps.
  run grep -E 'generate-registry\.sh.*>/dev/null|WARN could not regen registry' "$INIT"
  [ "$status" -eq 0 ]
}

@test "registry regen runs AFTER the codex override-catalog step (skeleton must exist first)" {
  override_line=$(grep -n 'append-codex-override-entry\.sh' "$INIT" | head -1 | cut -d: -f1)
  registry_line=$(grep -n 'generate-registry\.sh' "$INIT" | head -1 | cut -d: -f1)
  [ -n "$override_line" ]
  [ -n "$registry_line" ]
  # registry regen must come later in the file (it scans the whole tree, so every
  # other artifact — skill dir, dispositions, counts, codex catalog — must be in
  # place first).
  [ "$registry_line" -gt "$override_line" ]
}

@test "registry regen runs BEFORE the final 'created skill skeleton' echo (inside the plumbing block)" {
  registry_line=$(grep -n 'generate-registry\.sh' "$INIT" | head -1 | cut -d: -f1)
  echo_line=$(grep -n 'created skill skeleton at' "$INIT" | head -1 | cut -d: -f1)
  [ -n "$registry_line" ]
  [ -n "$echo_line" ]
  [ "$registry_line" -lt "$echo_line" ]
}
