#!/usr/bin/env bats
# skill-probe-i0.bats — the I0-INFORMATIONAL retrieval-probe receipt lane
# (scripts/skill-probe-i0.sh, ag-iyu4). It wraps the deterministic lexical
# trigger ranker (scan_descriptions.py --probe, ag-7led) so CI RUNS + REPORTS a
# per-skill JSON receipt artifact without it being a PR check.
#
# These specs prove the bead's acceptance properties:
#   1. the probe step PRODUCES the JSON-receipt artifact under
#      .agents/ao/skill-eval/<id>.json for every declared trigger phrase;
#   2. the lane is a no-op (exit 0, no receipts) when no skill opts in — so it
#      never blocks and is safe to wire before any skill declares a phrase;
#   3. a DELIBERATELY non-deterministic probe input is CAUGHT by the determinism
#      assertion (byte-diff across two runs) BEFORE it could become a gate.
#
# Hermetic: builds throwaway fixture skills trees (including a copy of the real
# scanner) under BATS_TEST_TMPDIR. No repo state, no network, no model.

setup() {
  ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  DRIVER="$ROOT/scripts/skill-probe-i0.sh"
  REAL_SCANNER="$ROOT/skills/skill-builder/scripts/scan_descriptions.py"
}

# Build a fixture skills/ tree with a real scanner and a declaring skill.
_mk_fixture() {
  local fix="$1"
  mkdir -p "$fix/skill-builder/scripts" "$fix/widget-builder" "$fix/widget-rival"
  cp "$REAL_SCANNER" "$fix/skill-builder/scripts/scan_descriptions.py"
  cat > "$fix/skill-builder/SKILL.md" <<'EOF'
---
name: skill-builder
description: scaffold skills
---
# Skill Builder
EOF
  cat > "$fix/widget-builder/SKILL.md" <<'EOF'
---
name: widget-builder
description: Build a custom widget on demand.
trigger_probes:
  - "build a custom widget"
---
# Widget Builder
EOF
  cat > "$fix/widget-rival/SKILL.md" <<'EOF'
---
name: widget-rival
description: Build a custom widget faster than anyone.
---
# Widget Rival
EOF
}

# Build a fixture whose declaring skill uses FLOW-form trigger_probes
# (`trigger_probes: ["a", "b"]`) instead of block form. Mirrors the scanner's
# parse_trigger_probes flow branch — flow-form skills must NOT be silently
# skipped by the wrapper's awk discovery (ag-iyu4 AMEND).
_mk_flow_fixture() {
  local fix="$1"
  mkdir -p "$fix/skill-builder/scripts" "$fix/alpha-skill" "$fix/beta-skill"
  cp "$REAL_SCANNER" "$fix/skill-builder/scripts/scan_descriptions.py"
  cat > "$fix/skill-builder/SKILL.md" <<'EOF'
---
name: skill-builder
description: scaffold skills
---
# Skill Builder
EOF
  # Two flow-form phrases on one inline YAML list line.
  cat > "$fix/alpha-skill/SKILL.md" <<'EOF'
---
name: alpha-skill
description: Handle the alpha phrase workflow uniquely.
trigger_probes: ["alpha phrase", "beta phrase"]
---
# Alpha Skill
EOF
  cat > "$fix/beta-skill/SKILL.md" <<'EOF'
---
name: beta-skill
description: A rival that mentions beta phrase too.
---
# Beta Skill
EOF
}

@test "i0-probe: flow-form trigger_probes are discovered, not silently skipped (ag-iyu4 AMEND)" {
  FIX="$BATS_TEST_TMPDIR/skills"
  RCPT="$BATS_TEST_TMPDIR/receipts"
  _mk_flow_fixture "$FIX"

  run "$DRIVER" "$FIX" "$RCPT"
  [ "$status" -eq 0 ]

  # A receipt is emitted (flow-form phrases were discovered, not skipped).
  [ -f "$RCPT/alpha-skill.json" ]

  # BOTH flow-form phrases produced a receipt line — proven via the wrapper's
  # per-phrase "wrote receipt ... (phrase: '...')" log.
  echo "$output" | grep -qF "(phrase: 'alpha phrase')"
  echo "$output" | grep -qF "(phrase: 'beta phrase')"
}

# Build a fixture whose declaring skill uses a flow-form phrase containing a
# QUOTED comma (`trigger_probes: ["alpha, beta"]`). The wrapper must treat this
# as the ONE phrase `alpha, beta`, not split it into two (`alpha` + `beta`).
# This is the AMEND2 regression: the old awk discovery did a naive comma split
# and broke quoted commas; reusing the scanner's parse_trigger_probes parser
# (via --list-probes) preserves them.
_mk_quoted_comma_fixture() {
  local fix="$1"
  mkdir -p "$fix/skill-builder/scripts" "$fix/phrase-skill"
  cp "$REAL_SCANNER" "$fix/skill-builder/scripts/scan_descriptions.py"
  cat > "$fix/skill-builder/SKILL.md" <<'EOF'
---
name: skill-builder
description: scaffold skills
---
# Skill Builder
EOF
  cat > "$fix/phrase-skill/SKILL.md" <<'EOF'
---
name: phrase-skill
description: Handle the alpha beta combined phrase distinctly.
trigger_probes: ["alpha, beta"]
---
# Phrase Skill
EOF
}

@test "i0-probe: a QUOTED comma is ONE phrase, not two (ag-iyu4 AMEND2)" {
  FIX="$BATS_TEST_TMPDIR/skills"
  RCPT="$BATS_TEST_TMPDIR/receipts"
  _mk_quoted_comma_fixture "$FIX"

  run "$DRIVER" "$FIX" "$RCPT"
  [ "$status" -eq 0 ]

  # Exactly ONE receipt line for the single phrase `alpha, beta`.
  echo "$output" | grep -qF "(phrase: 'alpha, beta')"

  # The naive-split bug would have probed 'alpha' and 'beta' separately — assert
  # those split fragments were NEVER probed as standalone phrases.
  ! echo "$output" | grep -qF "(phrase: 'alpha')"
  ! echo "$output" | grep -qF "(phrase: 'beta')"

  # Exactly one "wrote receipt" line total (one phrase => one receipt write).
  run bash -c "$DRIVER '$FIX' '$RCPT' | grep -c 'wrote receipt'"
  [ "$output" = "1" ]
}

@test "i0-probe: produces a per-skill JSON receipt for each declared phrase (exit 0)" {
  FIX="$BATS_TEST_TMPDIR/skills"
  RCPT="$BATS_TEST_TMPDIR/receipts"
  _mk_fixture "$FIX"

  run "$DRIVER" "$FIX" "$RCPT"
  [ "$status" -eq 0 ]

  # The receipt artifact exists and is valid JSON naming the declaring skill #1.
  [ -f "$RCPT/widget-builder.json" ]
  run python3 -c "import json,sys; d=json.load(open('$RCPT/widget-builder.json')); print(d['top'], d['declarer_is_top'])"
  [ "$status" -eq 0 ]
  [ "$output" = "widget-builder True" ]
}

@test "i0-probe: no trigger_probes declared anywhere is a clean no-op (exit 0, no receipts)" {
  FIX="$BATS_TEST_TMPDIR/skills"
  RCPT="$BATS_TEST_TMPDIR/receipts"
  mkdir -p "$FIX/skill-builder/scripts" "$FIX/plain"
  cp "$REAL_SCANNER" "$FIX/skill-builder/scripts/scan_descriptions.py"
  cat > "$FIX/skill-builder/SKILL.md" <<'EOF'
---
name: skill-builder
description: scaffold skills
---
# x
EOF
  cat > "$FIX/plain/SKILL.md" <<'EOF'
---
name: plain
description: does a plain thing
---
# Plain
EOF

  run "$DRIVER" "$FIX" "$RCPT"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "no .*trigger_probes.* phrases declared"
  # No receipts written.
  run bash -c "ls '$RCPT'/*.json 2>/dev/null | wc -l"
  [ "$output" = "0" ]
}

@test "i0-probe: a NON-DETERMINISTIC probe is caught by the determinism assertion (warning, non-zero exit)" {
  FIX="$BATS_TEST_TMPDIR/skills"
  RCPT="$BATS_TEST_TMPDIR/receipts"
  _mk_fixture "$FIX"

  # Replace the scanner with a non-deterministic stub that ignores its args and
  # emits a fresh nonce per invocation — simulating a probe whose output is not
  # byte-stable across runs (the exact failure the determinism assertion guards
  # against before any gate promotion).
  cat > "$FIX/skill-builder/scripts/scan_descriptions.py" <<'PY'
import sys, time, random
# Emit different bytes every call regardless of --probe args.
sys.stdout.write('{"nonce": "%d-%d"}\n' % (time.time_ns(), random.randint(0, 1 << 30)))
PY

  run "$DRIVER" "$FIX" "$RCPT"
  # I0 lane surfaces non-determinism via non-zero exit (the CI step pins
  # continue-on-error, so it still never blocks a merge).
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "NON-DETERMINISTIC"
}
