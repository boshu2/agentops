package testsupport

import (
	"os"
	"testing"
)

func TestScrubGitDiscoveryEnv_UnsetsEveryDiscoveryVar(t *testing.T) {
	for _, key := range GitDiscoveryEnvVars {
		t.Setenv(key, "/tmp/decoy-repo/.git")
	}

	ScrubGitDiscoveryEnv()

	for _, key := range GitDiscoveryEnvVars {
		if got, ok := os.LookupEnv(key); ok {
			t.Errorf("expected %s to be unset after scrub, still set to %q", key, got)
		}
	}
}

func TestGitDiscoveryEnvVars_CoverHookInjectedSet(t *testing.T) {
	// Guard the list itself: these are the vars git sets when running hooks
	// (githooks(5) / git(1) ENVIRONMENT). Dropping one silently reopens the
	// ek8v redirection hole, so removal must be a deliberate edit here.
	want := map[string]bool{
		"GIT_DIR":              true,
		"GIT_WORK_TREE":        true,
		"GIT_INDEX_FILE":       true,
		"GIT_PREFIX":           true,
		"GIT_OBJECT_DIRECTORY": true,
		"GIT_COMMON_DIR":       true,
		"GIT_NAMESPACE":        true,
	}
	if len(GitDiscoveryEnvVars) != len(want) {
		t.Fatalf("GitDiscoveryEnvVars has %d entries, want %d", len(GitDiscoveryEnvVars), len(want))
	}
	for _, key := range GitDiscoveryEnvVars {
		if !want[key] {
			t.Errorf("unexpected var %q in GitDiscoveryEnvVars", key)
		}
	}
}
