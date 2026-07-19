// Package testsupport holds process-level isolation helpers shared by
// TestMain functions across packages. It is imported only from _test.go
// files and must stay free of production dependencies.
package testsupport

import "os"

// GitDiscoveryEnvVars are the repository-discovery variables git injects
// into hook processes (and that shells can leak into any child). When one
// of these points at a real repository, every fixture `git init` /
// `git config` a test runs is silently redirected there — the mechanism
// behind the age-cmdao-core-bare-pollution-ek8v corruption, where a
// worktree-origin hook run re-initialized the shared .git as bare
// (core.bare=true), bricking git operations repo-wide.
var GitDiscoveryEnvVars = []string{
	"GIT_DIR",
	"GIT_WORK_TREE",
	"GIT_INDEX_FILE",
	"GIT_PREFIX",
	"GIT_OBJECT_DIRECTORY",
	"GIT_COMMON_DIR",
	"GIT_NAMESPACE",
}

// ScrubGitDiscoveryEnv unsets GitDiscoveryEnvVars for the whole test
// process. Call it from TestMain, before m.Run(), in any package whose
// tests shell out to git (directly or through production code) so fixture
// git operations can only ever act on the directories the tests create.
// No restore is needed: the test process owns its environment and exits
// after m.Run().
func ScrubGitDiscoveryEnv() {
	for _, key := range GitDiscoveryEnvVars {
		_ = os.Unsetenv(key)
	}
}
