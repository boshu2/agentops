#!/usr/bin/env bats
# Regression test for the static reference-counterpart assertion in
# scripts/validate-codex-generated-artifacts.sh (age-odv): every source
# skills/<s>/references/*.md must have a twin counterpart for PARITY twins;
# BESPOKE twins (age-0js4) and parity_policy:pointer twins (age-k2ag) are exempt.
#
# The full script requires a valid manifest fixture (computed hashes) to reach
# this assertion, so this test exercises a byte-faithful copy of the assertion
# loop against directory fixtures, plus a drift guard (last test) that pins the
# production wiring. Keep ref_violations() identical to the production loop.

setup() {
  WORK="$(mktemp -d "${TMPDIR:-/tmp}/ref-counterpart-XXXXXX")"
  ROOT="$WORK/repo"
  SKILLS_ROOT="$ROOT/skills-codex"
  mkdir -p "$ROOT/skills" "$SKILLS_ROOT"

  # Byte-faithful copy of the production assertion loop body (see the drift guard
  # in the final test). BESPOKE is a newline-separated name list.
  ref_violations() {
    local bespoke="${1:-}"
    is_bespoke() { grep -qxF "$1" <<<"$bespoke"; }
    twin_is_pointer() {
      local twin="$SKILLS_ROOT/$1/SKILL.md"
      [[ -f "$twin" ]] || return 1
      awk 'NR==1 && /^---/{f=1; next} f && /^---/{exit} f && /^parity_policy:[[:space:]]*pointer([[:space:]]+#.*|[[:space:]]*)$/{found=1} END{exit !found}' "$twin"
    }
    while IFS= read -r src_ref; do
      [[ -n "$src_ref" ]] || continue
      local rel="${src_ref#"$ROOT"/skills/}"
      local ref_skill="${rel%%/*}"
      local ref_rel="${rel#*/}"
      is_bespoke "$ref_skill" && continue
      [[ -d "$SKILLS_ROOT/$ref_skill" ]] || continue
      twin_is_pointer "$ref_skill" && continue
      [[ -f "$SKILLS_ROOT/$ref_skill/$ref_rel" ]] || echo "$ref_skill/$ref_rel"
    done < <(find "$ROOT/skills" -mindepth 3 -path '*/references/*' -type f -name '*.md' 2>/dev/null)
  }

  mk_source_ref() { # <skill> <relpath-under-references>
    mkdir -p "$ROOT/skills/$1/references/$(dirname "$2")"
    printf 'ref body\n' > "$ROOT/skills/$1/references/$2"
    printf -- '---\nname: %s\ndescription: d\n---\nBody\n' "$1" > "$ROOT/skills/$1/SKILL.md"
  }
  mk_twin() { # <skill> [frontmatter-extra-line]
    mkdir -p "$SKILLS_ROOT/$1"
    if [[ -n "${2:-}" ]]; then
      printf -- '---\nname: %s\ndescription: d\n%s\n---\nBody\n' "$1" "$2" > "$SKILLS_ROOT/$1/SKILL.md"
    else
      printf -- '---\nname: %s\ndescription: d\n---\nBody\n' "$1" > "$SKILLS_ROOT/$1/SKILL.md"
    fi
  }
  mk_twin_ref() { mkdir -p "$SKILLS_ROOT/$1/references/$(dirname "$2")"; printf 'ref body\n' > "$SKILLS_ROOT/$1/references/$2"; }
}

teardown() { rm -rf "$WORK"; }

@test "parity twin missing a source reference -> violation" {
  mk_source_ref foo deep-dive.md
  mk_twin foo            # twin exists but has NO references/deep-dive.md
  run ref_violations ""
  [ "$status" -eq 0 ]
  [[ "$output" == *"foo/references/deep-dive.md"* ]]
}

@test "parity twin WITH the counterpart -> no violation" {
  mk_source_ref foo deep-dive.md
  mk_twin foo
  mk_twin_ref foo deep-dive.md
  run ref_violations ""
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "bespoke skill missing a source reference -> exempt (no violation)" {
  mk_source_ref foo deep-dive.md
  mk_twin foo
  run ref_violations $'foo'   # foo is bespoke
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "parity_policy:pointer twin missing a source reference -> exempt (no violation)" {
  mk_source_ref foo deep-dive.md
  mk_twin foo 'parity_policy: pointer'
  run ref_violations ""
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "twin directory absent -> not flagged here (source->codex existence check's job)" {
  mk_source_ref foo deep-dive.md
  # no twin dir created for foo
  run ref_violations ""
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "nested references/sub/file.md is checked with the full relative path" {
  mk_source_ref foo sub/nested.md
  mk_twin foo
  run ref_violations ""
  [ "$status" -eq 0 ]
  [[ "$output" == *"foo/references/sub/nested.md"* ]]
}

@test "production script wires the assertion: find loop + exemptions + fail message (drift guard)" {
  GATE="$BATS_TEST_DIRNAME/../../scripts/validate-codex-generated-artifacts.sh"
  grep -qF 'find "$ROOT/skills" -mindepth 3 -path '"'"'*/references/*'"'"' -type f -name '"'"'*.md'"'"'' "$GATE"
  grep -qF 'Codex twin missing source reference:' "$GATE"
  # exemptions present in the same assertion
  grep -qF 'is_bespoke "$ref_skill" && continue' "$GATE"
  grep -qF 'twin_is_pointer "$ref_skill" && continue' "$GATE"
}
