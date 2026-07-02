//go:build !legacy

// practices: [tdd]
package main

import "testing"

// snapshotArchivedCommandGlobals / resetArchivedCommandGlobals are no-ops in the
// spine (default) test build: the codex + autodev cobra-flag globals they manage
// exist only in the //go:build legacy archive (age-h4y3). The tagged twin
// (testutil_archived_globals_legacy_test.go) does the real save/restore/reset.
func snapshotArchivedCommandGlobals() func() { return func() {} }

func resetArchivedCommandGlobals(_ *testing.T) {}
