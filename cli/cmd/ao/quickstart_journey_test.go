package main

import (
	"strings"
	"testing"
)

func TestQuickstartJourney_AllRenderedAORoutesAreLiveAndVerdictIsTerminal(t *testing.T) {
	for _, tracked := range []bool{false, true} {
		out, _ := captureStdout(t, func() error {
			showNextSteps(tracked)
			printFirstVerdictStep(&firstVerdictInfo{
				LedgerReady:  true,
				ReviewerLive: []string{"codex"},
				NextCommand:  firstVerdictCommand,
			})
			return nil
		})
		for _, tombstone := range []string{"ao factory", "ao orchestrate", "ao codex", "/rpi", "/validation"} {
			if strings.Contains(out, tombstone) {
				t.Fatalf("tracked=%v: rendered removed route %q:\n%s", tracked, tombstone, out)
			}
		}
		if got := strings.LastIndex(out, "ao "); got < 0 || !strings.HasPrefix(out[got:], firstVerdictCommand) {
			t.Fatalf("tracked=%v: final ao command must be %q:\n%s", tracked, firstVerdictCommand, out)
		}
	}
}
