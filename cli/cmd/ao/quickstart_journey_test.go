package main

import (
	"regexp"
	"strings"
	"testing"
)

var renderedAOCommand = regexp.MustCompile(`(?m)^\s*\$ (ao(?: [^\n]+)?)\s*$`)

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

func TestQuickStartNextSteps_AllAOCommandsResolve(t *testing.T) {
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

		matches := renderedAOCommand.FindAllStringSubmatch(out, -1)
		if len(matches) == 0 {
			t.Fatalf("tracked=%v: quick-start emitted no executable ao route", tracked)
		}
		for _, match := range matches {
			line := strings.TrimSpace(match[1])
			args := strings.Fields(strings.TrimPrefix(line, "ao "))
			resolved, _, err := rootCmd.Find(args)
			if err != nil || resolved == rootCmd {
				t.Errorf("tracked=%v: quick-start emitted unresolved route %q: %v", tracked, line, err)
			}
		}
	}
}
