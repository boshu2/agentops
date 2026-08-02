package gcmaintainer

import (
	"os"
	"testing"
)

// TestMain prevents any fallback through os.UserHomeDir from reaching the
// operator's real home when a test omits an explicit CODEX_HOME or GC_HOME.
func TestMain(m *testing.M) {
	testHome, err := os.MkdirTemp("", "gcmaintainer-test-home-*")
	if err != nil {
		panic("gcmaintainer TestMain: create isolated HOME: " + err.Error())
	}
	previousHome, hadHome := os.LookupEnv("HOME")
	if err := os.Setenv("HOME", testHome); err != nil {
		_ = os.RemoveAll(testHome)
		panic("gcmaintainer TestMain: set isolated HOME: " + err.Error())
	}

	code := m.Run()

	if hadHome {
		_ = os.Setenv("HOME", previousHome)
	} else {
		_ = os.Unsetenv("HOME")
	}
	_ = os.RemoveAll(testHome)
	os.Exit(code)
}
