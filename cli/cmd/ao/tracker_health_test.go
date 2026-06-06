// practices: [dora-metrics, distributed-tracing]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTrackerFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bd-fixture.sh")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write tracker fixture: %v", err)
	}
	return path
}

func TestDetectTrackerHealth_Healthy(t *testing.T) {
	command := writeTrackerFixture(t, `#!/bin/sh
set -eu
case "${1:-}" in
  ready)
    printf '[]\n'
    ;;
  list)
    printf '[{"id":"ag-123"}]\n'
    ;;
  *)
    printf '[]\n'
    ;;
esac
`)

	health := detectTrackerHealth(command, nil)
	if !health.Healthy {
		t.Fatalf("healthy = false, want true: %+v", health)
	}
	if health.Mode != "beads" {
		t.Fatalf("mode = %q, want beads", health.Mode)
	}
	if !strings.Contains(health.Reason, "succeeded") {
		t.Fatalf("reason = %q, want probe-success hint", health.Reason)
	}
}

func TestDetectTrackerHealth_Degraded(t *testing.T) {
	command := writeTrackerFixture(t, `#!/bin/sh
set -eu
printf 'column "crystallizes" could not be found in any table in scope\n' >&2
exit 1
`)

	health := detectTrackerHealth(command, nil)
	if health.Healthy {
		t.Fatalf("healthy = true, want false: %+v", health)
	}
	if health.Mode != "tasklist" {
		t.Fatalf("mode = %q, want tasklist", health.Mode)
	}
	if !strings.Contains(health.Error, "crystallizes") {
		t.Fatalf("error = %q, want beads schema failure", health.Error)
	}
}

