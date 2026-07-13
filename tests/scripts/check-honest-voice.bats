#!/usr/bin/env bats
# age-5qjyn (FU3): check-honest-voice.sh gates user-facing CLI strings + seed/
# template assets against docs/contracts/forbidden-claims.yaml — claims of
# proven/automatic knowledge compounding (unproven: ADR-0004, ADR-0011) and
# hookless-3.0-violating "session hooks" (docs/3.0.md, ADR-0009) must not regrow.
#
# The gate scans a fixed tree ($HONEST_VOICE_ROOT, default repo root). These L2
# cases build a throwaway fixture tree and point the scanner at it while reusing
# the REAL lexicon, plus one case proving the current real tree is clean.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/check-honest-voice.sh"
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  LEXICON="$REPO_ROOT/docs/contracts/forbidden-claims.yaml"
  FIX="$(mktemp -d)"
  mkdir -p "$FIX/cli/cmd/ao" "$FIX/cli/internal/lifecycle"
}

teardown() {
  rm -rf "$FIX"
}

@test "FAILS naming phrase + file + line + rationale when a CLI string claims automatic compounding" {
  cat > "$FIX/cli/cmd/ao/seed.go" <<'EOF'
package main

func seedNext() {
	// user-facing next-steps string
	println("Knowledge compounds automatically — MEMORY.md updates after each session")
}
EOF
  run env HONEST_VOICE_ROOT="$FIX" HONEST_VOICE_LEXICON="$LEXICON" bash "$SCRIPT"
  [ "$status" -eq 1 ]
  # names the phrase's file+line
  [[ "$output" == *"cli/cmd/ao/seed.go:5"* ]]
  # names the offending phrase
  [[ "$output" == *"compounds automatically"* ]]
  # names the rationale (cites the unproven-compounding ADRs)
  [[ "$output" == *"rationale:"* ]]
  [[ "$output" == *"ADR-0011"* ]]
}

@test "FAILS on hookless-3.0-violating 'session hooks' in a CLI string" {
  cat > "$FIX/cli/cmd/ao/init.go" <<'EOF'
package main

func initNext() {
	println("  ao init --hooks    # Register session hooks")
}
EOF
  run env HONEST_VOICE_ROOT="$FIX" HONEST_VOICE_LEXICON="$LEXICON" bash "$SCRIPT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"session-hooks"* ]]
  [[ "$output" == *"hookless"* ]]
}

@test "PASSES the honest conditional escape-velocity phrasing (flywheel NOT banned)" {
  cat > "$FIX/cli/cmd/ao/metrics.go" <<'EOF'
package main

func metricsHelp() {
	// conditional model + subsystem label are honest, must not trip the gate
	println("Operational escape velocity: σ × ρ > δ/100 → Knowledge compounds")
	println("Flywheel Health: pass")
}
EOF
  run env HONEST_VOICE_ROOT="$FIX" HONEST_VOICE_LEXICON="$LEXICON" bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"ok"* ]]
}

@test "PASSES when the offending line carries a honest-voice:allow suppression" {
  cat > "$FIX/cli/cmd/ao/seed.go" <<'EOF'
package main

func seedNext() {
	println("Knowledge compounds automatically — reviewed exception") // honest-voice:allow
}
EOF
  run env HONEST_VOICE_ROOT="$FIX" HONEST_VOICE_LEXICON="$LEXICON" bash "$SCRIPT"
  [ "$status" -eq 0 ]
}

@test "IGNORES *_test.go (tests assert absence and may name a phrase)" {
  cat > "$FIX/cli/cmd/ao/seed_test.go" <<'EOF'
package main

import "testing"

func TestNoClaim(t *testing.T) {
	if want := "Knowledge compounds automatically"; want == "" {
		t.Fatal(want)
	}
}
EOF
  run env HONEST_VOICE_ROOT="$FIX" HONEST_VOICE_LEXICON="$LEXICON" bash "$SCRIPT"
  [ "$status" -eq 0 ]
}

@test "PASSES on the current real tree (sweep completed, gate green)" {
  run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"ok"* ]]
}

@test "catches a forbidden claim split across Go string concatenation" {
  fixture="$BATS_TEST_TMPDIR/root7"
  mkdir -p "$fixture/cli/cmd/ao"
  cat > "$fixture/cli/cmd/ao/evade.go" <<'GO'
package main

var msg = "Knowledge compounds " +
	"automatically across sessions"
GO
  run env HONEST_VOICE_ROOT="$fixture" HONEST_VOICE_LEXICON="$LEXICON" \
    bash "$REPO_ROOT/scripts/check-honest-voice.sh"
  [ "$status" -eq 1 ]
  [[ "$output" == *"fused string concatenation"* ]]
}

@test "catches a forbidden claim wrapped inside a multiline Go raw string" {
  fixture="$BATS_TEST_TMPDIR/root8"
  mkdir -p "$fixture/cli/cmd/ao"
  cat > "$fixture/cli/cmd/ao/rawevade.go" <<'GO'
package main

var banner = `Welcome!
Knowledge compounds
automatically across sessions.`
GO
  run env HONEST_VOICE_ROOT="$fixture" HONEST_VOICE_LEXICON="$LEXICON" \
    bash "$REPO_ROOT/scripts/check-honest-voice.sh"
  [ "$status" -eq 1 ]
  [[ "$output" == *"multiline raw string"* ]]
}
