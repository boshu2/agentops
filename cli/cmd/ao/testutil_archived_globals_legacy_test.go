//go:build legacy

// practices: [tdd]
package main

import "testing"

// snapshotArchivedCommandGlobals saves the cobra-flag globals owned by commands
// archived behind //go:build legacy (the codex lifecycle command + autodev),
// resets them to their flag defaults, and returns a restore closure. It lives in
// the tagged test build so the spine (default) test build never references
// archived symbols (age-h4y3). Use it directly with `defer` (executeCommand) or
// via resetArchivedCommandGlobals for the t.Cleanup shape (resetGlobalFlags).
// Mirrors the save/restore/reset discipline in .claude/rules/go.md "Test isolation".
func snapshotArchivedCommandGlobals() func() {
	origCodexStartLimit := codexStartLimit
	origCodexStartQuery := codexStartQuery
	origCodexStartNoMaintenance := codexStartNoMaintenance
	origCodexStopSessionID := codexStopSessionID
	origCodexStopTranscriptPath := codexStopTranscriptPath
	origCodexStopAutoExtract := codexStopAutoExtract
	origCodexStopNoHistoryFallback := codexStopNoHistoryFallback
	origCodexStopCloseLoop := codexStopCloseLoop
	origCodexStopNoCloseLoop := codexStopNoCloseLoop
	origCodexStatusDays := codexStatusDays
	origCodexDispatchPacketPath := codexDispatchPacketPath
	origAutodevFile := autodevFile
	origAutodevForce := autodevForce

	restore := func() {
		codexStartLimit = origCodexStartLimit
		codexStartQuery = origCodexStartQuery
		codexStartNoMaintenance = origCodexStartNoMaintenance
		codexStopSessionID = origCodexStopSessionID
		codexStopTranscriptPath = origCodexStopTranscriptPath
		codexStopAutoExtract = origCodexStopAutoExtract
		codexStopNoHistoryFallback = origCodexStopNoHistoryFallback
		codexStopCloseLoop = origCodexStopCloseLoop
		codexStopNoCloseLoop = origCodexStopNoCloseLoop
		codexStatusDays = origCodexStatusDays
		codexDispatchPacketPath = origCodexDispatchPacketPath
		autodevFile = origAutodevFile
		autodevForce = origAutodevForce
	}

	// Reset to flag defaults.
	codexStartLimit = 3
	codexStartQuery = ""
	codexStartNoMaintenance = false
	codexStopSessionID = ""
	codexStopTranscriptPath = ""
	codexStopAutoExtract = true
	codexStopNoHistoryFallback = false
	codexStopCloseLoop = false
	codexStopNoCloseLoop = false
	codexStatusDays = 7
	codexDispatchPacketPath = ""
	autodevFile = ""
	autodevForce = false

	return restore
}

// resetArchivedCommandGlobals is the t.Cleanup shape over
// snapshotArchivedCommandGlobals, for helpers that own state via testing.T.
func resetArchivedCommandGlobals(t *testing.T) {
	t.Helper()
	t.Cleanup(snapshotArchivedCommandGlobals())
}
