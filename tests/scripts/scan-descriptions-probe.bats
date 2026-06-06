#!/usr/bin/env bats
# scan-descriptions-probe.bats — the deterministic lexical trigger-probe mode
# of skills/skill-builder/scripts/scan_descriptions.py (ag-7led).
#
# `--probe "<phrase>"` ranks every skill against the phrase using ONLY the
# in-script lexical ranker (no live model, no `claude -p`, no network) and
# asserts the skill that declares the phrase in its `trigger_probes:`
# frontmatter list ranks #1. This spec proves the two acceptance properties:
#   1. determinism — probe output is BYTE-IDENTICAL across two consecutive runs;
#   2. rank-drop — mutating the declaring skill's description so it no longer
#      matches the phrase drops it below rank #1.
#
# Hermetic: builds a throwaway fixture skills tree under BATS_TEST_TMPDIR and
# points the scanner at it via its positional SKILLS_DIR arg. No repo state,
# no network, no model.

setup() {
  ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$ROOT/skills/skill-builder/scripts/scan_descriptions.py"
  FIX="$BATS_TEST_TMPDIR/skills"
  mkdir -p "$FIX/widget-builder" "$FIX/widget-rival"

  # Declaring skill: lists the phrase in trigger_probes AND describes it, so it
  # earns rank #1 on pure lexical coverage.
  cat > "$FIX/widget-builder/SKILL.md" <<'EOF'
---
name: widget-builder
description: Build a custom widget on demand.
trigger_probes:
  - "build a custom widget"
---
# Widget Builder
EOF

  # Rival: mentions the same words but does NOT declare the phrase. It stays
  # rank #2 while the declarer describes the phrase, and overtakes it once the
  # declarer's description is mutated away.
  cat > "$FIX/widget-rival/SKILL.md" <<'EOF'
---
name: widget-rival
description: Build a custom widget faster and better than anyone.
---
# Widget Rival
EOF
}

@test "probe: declaring skill ranks #1 for its own trigger phrase (exit 0)" {
  run python3 "$SCRIPT" "$FIX" --probe "build a custom widget"
  [ "$status" -eq 0 ]
  # The first ranked row is the declaring skill, marked 'yes'.
  echo "$output" | grep -q "| 1 | \`widget-builder\` | yes |"
}

@test "probe: output is BYTE-IDENTICAL across two consecutive runs (determinism)" {
  python3 "$SCRIPT" "$FIX" --probe "build a custom widget" --json > "$BATS_TEST_TMPDIR/run1.json"
  python3 "$SCRIPT" "$FIX" --probe "build a custom widget" --json > "$BATS_TEST_TMPDIR/run2.json"
  run cmp "$BATS_TEST_TMPDIR/run1.json" "$BATS_TEST_TMPDIR/run2.json"
  [ "$status" -eq 0 ]
  # And the human render is byte-stable too.
  python3 "$SCRIPT" "$FIX" --probe "build a custom widget" > "$BATS_TEST_TMPDIR/h1.txt"
  python3 "$SCRIPT" "$FIX" --probe "build a custom widget" > "$BATS_TEST_TMPDIR/h2.txt"
  run cmp "$BATS_TEST_TMPDIR/h1.txt" "$BATS_TEST_TMPDIR/h2.txt"
  [ "$status" -eq 0 ]
}

@test "probe: mutating the declarer's description drops it below rank #1 (exit 1)" {
  # Sanity: the declarer wins before the mutation.
  run python3 "$SCRIPT" "$FIX" --probe "build a custom widget"
  [ "$status" -eq 0 ]

  # Mutate the declaring skill so its description no longer matches the phrase.
  # It still DECLARES the phrase in trigger_probes — proving the rank is earned
  # by lexical description match, not by the declaration itself.
  cat > "$FIX/widget-builder/SKILL.md" <<'EOF'
---
name: widget-builder
description: Does totally unrelated database things now.
trigger_probes:
  - "build a custom widget"
---
# Widget Builder
EOF

  run python3 "$SCRIPT" "$FIX" --probe "build a custom widget"
  [ "$status" -eq 1 ]
  # The rival is now rank #1; the declarer fell to #2.
  echo "$output" | grep -q "| 1 | \`widget-rival\` |"
  echo "$output" | grep -q "| 2 | \`widget-builder\` | yes |"
}

@test "probe: a phrase no skill declares is a usage error (exit 2)" {
  run python3 "$SCRIPT" "$FIX" --probe "nobody declares this phrase"
  [ "$status" -eq 2 ]
}
