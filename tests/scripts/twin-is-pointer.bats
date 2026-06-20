#!/usr/bin/env bats
# Regression test for the twin_is_pointer frontmatter detection in
# scripts/validate-codex-generated-artifacts.sh (age-k2ag). It proves the marker
# is recognized ONLY as `parity_policy: pointer` in the FIRST frontmatter block —
# never in the body, never with another value, never absent. (The full gate-skip
# integration — pointer twin exempt, full-mirror twin still enforced — is covered
# by age-backfill-uco, which marks the real pointer twins.)

setup() {
  WORK="$(mktemp -d "${TMPDIR:-/tmp}/twin-pointer-XXXXXX")"
  # The exact detection logic copied verbatim from twin_is_pointer() in
  # scripts/validate-codex-generated-artifacts.sh. A bats guard test that
  # paraphrases the production awk would not catch a real divergence — keep this
  # byte-identical to the source.
  is_pointer() {
    awk 'NR==1 && /^---/{f=1; next} f && /^---/{exit} f && /^parity_policy:[[:space:]]*pointer([[:space:]]+#.*|[[:space:]]*)$/{found=1} END{exit !found}' "$1"
  }
}

teardown() { rm -rf "$WORK"; }

@test "frontmatter with parity_policy: pointer -> detected (exit 0)" {
  printf -- '---\nname: foo\nparity_policy: pointer\n---\nBody.\n' > "$WORK/p.md"
  run is_pointer "$WORK/p.md"
  [ "$status" -eq 0 ]
}

@test "frontmatter without the marker -> not a pointer (exit 1)" {
  printf -- '---\nname: foo\ndescription: bar\n---\nBody.\n' > "$WORK/p.md"
  run is_pointer "$WORK/p.md"
  [ "$status" -eq 1 ]
}

@test "marker in the BODY (after frontmatter close) -> not a pointer (exit 1)" {
  printf -- '---\nname: foo\n---\nparity_policy: pointer\n' > "$WORK/p.md"
  run is_pointer "$WORK/p.md"
  [ "$status" -eq 1 ]
}

@test "wrong value (parity_policy: full) -> not a pointer (exit 1)" {
  printf -- '---\nname: foo\nparity_policy: full\n---\nBody.\n' > "$WORK/p.md"
  run is_pointer "$WORK/p.md"
  [ "$status" -eq 1 ]
}

@test "no frontmatter at all -> not a pointer (exit 1)" {
  printf -- 'just a body, no frontmatter\nparity_policy: pointer\n' > "$WORK/p.md"
  run is_pointer "$WORK/p.md"
  [ "$status" -eq 1 ]
}

@test "trailing whitespace after pointer is tolerated (exit 0)" {
  printf -- '---\nname: foo\nparity_policy: pointer   \n---\nBody.\n' > "$WORK/p.md"
  run is_pointer "$WORK/p.md"
  [ "$status" -eq 0 ]
}

@test "inline YAML comment after pointer is tolerated (matches the documented form)" {
  printf -- '---\nname: foo\nparity_policy: pointer   # defers to source\n---\nBody.\n' > "$WORK/p.md"
  run is_pointer "$WORK/p.md"
  [ "$status" -eq 0 ]
}

@test "a non-pointer value with a pointer-ish suffix is NOT matched (no false positive)" {
  printf -- '---\nname: foo\nparity_policy: pointerish\n---\nBody.\n' > "$WORK/p.md"
  run is_pointer "$WORK/p.md"
  [ "$status" -eq 1 ]
}

@test "no space before # is part of the value, not a comment (YAML semantics) -> NOT matched" {
  # In YAML 'pointer# x' has no inline comment (a comment needs leading whitespace),
  # so the value is 'pointer# x', not the 'pointer' marker.
  printf -- '---\nname: foo\nparity_policy: pointer# x\n---\nBody.\n' > "$WORK/p.md"
  run is_pointer "$WORK/p.md"
  [ "$status" -eq 1 ]
}

@test "production script still has the byte-identical awk (guard against drift)" {
  GATE="$BATS_TEST_DIRNAME/../../scripts/validate-codex-generated-artifacts.sh"
  grep -qF "awk 'NR==1 && /^---/{f=1; next} f && /^---/{exit} f && /^parity_policy:[[:space:]]*pointer([[:space:]]+#.*|[[:space:]]*)\$/{found=1} END{exit !found}'" "$GATE"
}
