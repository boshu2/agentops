#!/usr/bin/env bats
# age-0dq9.6 / audit M-2: check-path-containment.sh is a warn-only forward
# ratchet that surfaces filepath.Join on a high-confidence external-input segment
# for manual EvalSymlinks/filepath.Rel containment review. It is FAIL-CLOSED: it
# flags every such Join, contained or not (a proximity "looks contained" check
# would fail open). These cases prove the acceptance: an injected external-input
# Join is flagged (including multi-line and cobra args[0] forms), a nearby
# filepath.Rel does NOT suppress it, an env-var BASE dir with literal children is
# not flagged, internal-only Joins are not flagged, _test.go is excluded, and the
# gate is always warn-only (exit 0).

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/check-path-containment.sh"
  FIX="$(mktemp -d)"
}

teardown() {
  rm -rf "$FIX"
}

# Scan the fixture dir by pointing REPO_ROOT + SCAN_ROOT at it.
run_scan() {
  run env AGENTOPS_REPO_ROOT="$FIX" PATH_CONTAINMENT_SCAN_ROOT="." bash "$SCRIPT"
}

@test "path-containment: internal-only Join is NOT flagged (no false positive)" {
  cat > "$FIX/internal.go" <<'EOF'
package demo
import "path/filepath"
func internalPath(base, name string) string {
	return filepath.Join(base, "cache", name)
}
EOF
  run_scan
  [ "$status" -eq 0 ]
  [[ "$output" == *"0 external-input Join site(s)"* ]]
}

@test "path-containment: external-input Join WITHOUT containment IS flagged" {
  cat > "$FIX/unsafe.go" <<'EOF'
package demo
import (
	"os"
	"path/filepath"
)
func loadRemote(root string) string {
	remoteInputPath := os.Args[1]
	return filepath.Join(root, remoteInputPath)
}
EOF
  run_scan
  [ "$status" -eq 0 ]                       # warn-only
  [[ "$output" == *"WARN path-containment"* ]]
  [[ "$output" == *"unsafe.go"* ]]
  [[ "$output" != *"0 external-input Join site(s)"* ]]
}

@test "path-containment: fail-closed — a nearby filepath.Rel does NOT suppress the flag" {
  # An external-input Join with containment-looking code nearby is STILL flagged.
  # The gate deliberately does not auto-clear on proximity (an unrelated nearby
  # EvalSymlinks/filepath.Rel would otherwise fail open and hide a real escape);
  # a human confirms the containment is real.
  cat > "$FIX/contained.go" <<'EOF'
package demo
import (
	"os"
	"path/filepath"
)
func loadRemoteSafe(root string) (string, error) {
	remoteInputPath := os.Args[1]
	joined := filepath.Join(root, remoteInputPath)
	rel, err := filepath.Rel(root, joined)
	if err != nil || rel == ".." {
		return "", err
	}
	return joined, nil
}
EOF
  run_scan
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN path-containment"* ]]
  [[ "$output" == *"contained.go"* ]]
}

@test "path-containment: multi-line Join with the tainted arg on a later line IS flagged" {
  # The external input is several lines into the Join call — a fixed line window
  # would miss it (fail open). Balanced-paren reading must still catch it.
  cat > "$FIX/multiline.go" <<'EOF'
package demo
import (
	"os"
	"path/filepath"
)
func build(root string) string {
	return filepath.Join(
		root,
		"static",
		"more",
		os.Args[1],
	)
}
EOF
  run_scan
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN path-containment"* ]]
  [[ "$output" == *"multiline.go"* ]]
}

@test "path-containment: cobra positional args[0] Join IS flagged" {
  cat > "$FIX/cobra.go" <<'EOF'
package demo
import "path/filepath"
func runCmd(root string, args []string) string {
	return filepath.Join(root, args[0])
}
EOF
  run_scan
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN path-containment"* ]]
  [[ "$output" == *"cobra.go"* ]]
}

@test "path-containment: env var as BASE dir with literal children is NOT flagged" {
  # filepath.Join(os.Getenv("HOME"), ".agentops", "config.yaml") — safe base+literals,
  # the false-positive shape we intentionally do not flag.
  cat > "$FIX/base.go" <<'EOF'
package demo
import (
	"os"
	"path/filepath"
)
func homeConfig() string {
	return filepath.Join(os.Getenv("HOME"), ".agentops", "config.yaml")
}
EOF
  run_scan
  [ "$status" -eq 0 ]
  [[ "$output" == *"0 external-input Join site(s)"* ]]
}

@test "path-containment: _test.go files are excluded from scanning" {
  cat > "$FIX/thing_test.go" <<'EOF'
package demo
import (
	"os"
	"path/filepath"
)
func helper(root string) string {
	userInputPath := os.Args[1]
	return filepath.Join(root, userInputPath)
}
EOF
  run_scan
  [ "$status" -eq 0 ]
  [[ "$output" == *"0 external-input Join site(s)"* ]]
}

@test "path-containment: gate is warn-only — exit 0 even with a flagged site" {
  cat > "$FIX/unsafe.go" <<'EOF'
package demo
import (
	"os"
	"path/filepath"
)
func f(root string) string { return filepath.Join(root, os.Args[1]) }
EOF
  run_scan
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN path-containment"* ]]
}
