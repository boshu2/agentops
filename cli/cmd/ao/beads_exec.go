// `ao beads exec [args...]` is the one tracker-agnostic CRUD passthrough.
// Most users track with bd; this repository tracks with br. The adapter keeps
// their intentional ledger, child-enumeration, and JSON-shape differences.
package main

import (
	"os"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
)

// These pure compatibility delegates remain until the yield family moves its
// tracker-child formatting onto the shared application policy.
func beadsExecChildEnv(res trackerResolution, _ string) []string {
	return beadsapp.ChildEnvironment(os.Environ(), beadsapp.TrackerResolution{Tracker: res.Tracker, LedgerDir: res.LedgerDir})
}

func beadsExecChildDir(res trackerResolution, cwd string) string {
	return beadsapp.ChildDirectory(beadsapp.TrackerResolution{Tracker: res.Tracker, LedgerDir: res.LedgerDir}, cwd)
}

// Kept as a package-main test seam until the legacy white-box tests move with
// their final owner.
func canonicalizeBDReadJSON(verb string, raw []byte) ([]byte, error) {
	return beadsapp.CanonicalizeBDReadJSON(verb, raw)
}
