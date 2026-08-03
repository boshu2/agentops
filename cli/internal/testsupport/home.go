package testsupport

import "os"

// RunTestMainWithIsolatedHome runs a package's tests with HOME rooted in a
// temporary directory. Call it from TestMain: HOME is established before
// m.Run starts and restored only after every package test has finished, so
// parallel tests never observe a package-level environment transition.
func RunTestMainWithIsolatedHome(run func() int) (code int) {
	testHome, err := os.MkdirTemp("", "agentops-test-home-*")
	if err != nil {
		panic("testsupport: create isolated HOME: " + err.Error())
	}

	previousHome, hadHome := os.LookupEnv("HOME")
	if err := os.Setenv("HOME", testHome); err != nil {
		_ = os.RemoveAll(testHome)
		panic("testsupport: set isolated HOME: " + err.Error())
	}
	defer func() {
		if hadHome {
			_ = os.Setenv("HOME", previousHome)
		} else {
			_ = os.Unsetenv("HOME")
		}
		_ = os.RemoveAll(testHome)
	}()

	return run()
}
