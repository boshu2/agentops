package gcmaintainer

import (
	"os"
	"testing"

	"github.com/boshu2/agentops/cli/internal/testsupport"
)

// TestMain prevents any fallback through os.UserHomeDir from reaching the
// operator's real home when a test omits an explicit CODEX_HOME or GC_HOME.
func TestMain(m *testing.M) {
	os.Exit(testsupport.RunTestMainWithIsolatedHome(m.Run))
}
