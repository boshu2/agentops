package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
)

// withProductionSpine narrows the test binary's fully-registered command tree
// to the ADR-0012 default spine, so advertised-command validation matches what
// a fresh-install `ao` binary actually serves (the test binary deliberately
// keeps archived registrations; see zzz_default_spine.go).
func withProductionSpine(t *testing.T) {
	t.Helper()
	removed := pruneToDefaultSpine(rootCmd)
	t.Cleanup(func() { restorePrunedCommands(rootCmd, removed) })
}

// advertisedProseStopwords are English words that can follow "ao" in help
// prose ("ao is the CLI for ...") without naming a subcommand. Extraction
// matches them; the sweep skips them instead of failing. Keep this list to
// function words only — a removed command name must never be added here.
var advertisedProseStopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "by": true,
	"can": true, "command": true, "commands": true, "does": true, "for": true,
	"if": true, "in": true, "is": true, "of": true, "on": true, "or": true,
	"that": true, "the": true, "this": true, "to": true, "was": true, "with": true,
}

// TestExtractAdvertisedAoInvocations pins the extraction contract: command
// runs stop at placeholders, punctuation, and quotes, and prose without a
// following command word never matches.
func TestExtractAdvertisedAoInvocations(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"backtick fenced", "run `ao session bootstrap` first", []string{"session bootstrap"}},
		{"placeholder stops the run", "ao verify <change-slug> # review", []string{"verify"}},
		{"flags included", "ao lookup --query \"<topic>\"", []string{"lookup --query"}},
		{"flag with value and trailing word", "ao gate check --fast --scope head", []string{"gate check --fast --scope head"}},
		{"quoted single command", "the same check as 'ao doctor' runs", []string{"doctor"}},
		{"no match inside word", "ciao status", nil},
		{"no match without subcommand", "the ao CLI", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAdvertisedAoInvocations(tt.text)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractAdvertisedAoInvocations(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// TestValidateAdvertisedAoInvocation pins the resolution contract against the
// live command tree: removed commands fail, group commands reject positional
// args masquerading as subcommands, and unknown flags fail.
func TestValidateAdvertisedAoInvocation(t *testing.T) {
	withProductionSpine(t)
	valid := []string{
		"status",
		"session bootstrap",
		"quick-start --no-beads",
		"beads exec ready", // exec is runnable; "ready" is a forwarded arg
		"verify my-first-change",
	}
	for _, inv := range valid {
		if err := validateAdvertisedAoInvocation(rootCmd, inv); err != nil {
			t.Errorf("ao %s should resolve: %v", inv, err)
		}
	}

	invalid := []string{
		"factory start --goal",  // removed (ADR-0012)
		"orchestrate status",    // removed (ADR-0012)
		"autodev init",          // removed (ADR-0012)
		"flywheel status",       // archived behind the flywheel build tag
		"beads ready",           // beads is a group; ready is not a subcommand
		"init --with-schedule",  // flag never existed on ao init
		"quick-start --no-such", // unknown flag
	}
	for _, inv := range invalid {
		if err := validateAdvertisedAoInvocation(rootCmd, inv); err == nil {
			t.Errorf("ao %s should NOT resolve in the default build", inv)
		}
	}
}

// TestUserFacingOutputAdvertisesOnlyLiveCommands is the fresh-install UX
// guard (the "Stale References" doctor check missed these): every `ao ...`
// string emitted by repo readiness, the quick-start golden paths, the seeded
// CLAUDE.md sections, and the starter knowledge pack must parse against the
// actual cobra command tree, so a removed command can never be advertised to
// a new user again.
func TestUserFacingOutputAdvertisesOnlyLiveCommands(t *testing.T) {
	withProductionSpine(t)
	sources := map[string]string{
		"root help":          rootCmd.Long,
		"quick-start help":   quickstartCmd.Long,
		"claude-md seed":     lifecycle.ClaudeMDSeedSection,
		"first-verdict step": firstVerdictCommand,
	}

	// Repo readiness actions (what quick-start / ao init print as "next: ...").
	report, err := lifecycle.InspectRepoReadiness(t.TempDir(), lifecycle.ReadinessOptions{})
	if err != nil {
		t.Fatalf("InspectRepoReadiness: %v", err)
	}
	for _, item := range report.Items {
		sources["readiness action for "+item.Name] = item.Action
	}

	// Quick-start LIVE PATH journey, both tracked and untracked.
	for _, tracked := range []bool{false, true} {
		for _, step := range quickstartJourney(tracked) {
			for _, command := range step.Commands {
				sources["journey step "+step.Title] = command
			}
		}
	}

	// The terminal first-verdict output, both reviewer-reachable and
	// install-needed variants.
	for name, info := range map[string]*firstVerdictInfo{
		"first-verdict reachable": {LedgerReady: true, ReviewerLive: []string{"codex"}, NextCommand: firstVerdictCommand},
		"first-verdict install":   {LedgerReady: true, ReviewerInstall: []string{"codex: npm i -g @openai/codex"}},
	} {
		out, _ := captureStdout(t, func() error {
			printFirstVerdictStep(info)
			return nil
		})
		sources[name] = out
	}

	// The CLAUDE.md quick-start writes into user repos, and the starter
	// knowledge pack files.
	seedDir := t.TempDir()
	if err := createProjectClaudeMd(seedDir); err != nil {
		t.Fatalf("createProjectClaudeMd: %v", err)
	}
	claudeMD, err := os.ReadFile(filepath.Join(seedDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read seeded CLAUDE.md: %v", err)
	}
	sources["seeded CLAUDE.md"] = string(claudeMD)

	if _, err := captureStdout(t, func() error { return createStarterPack(seedDir) }); err != nil {
		t.Fatalf("createStarterPack: %v", err)
	}
	for _, rel := range []string{
		".agents/patterns/context-boundaries.md",
		".agents/patterns/pre-mortem-first.md",
		".agents/learnings/session-hygiene.md",
	} {
		data, err := os.ReadFile(filepath.Join(seedDir, rel))
		if err != nil {
			t.Fatalf("read starter pack %s: %v", rel, err)
		}
		sources["starter pack "+rel] = string(data)
	}

	total := 0
	for name, text := range sources {
		for _, invocation := range extractAdvertisedAoInvocations(text) {
			if advertisedProseStopwords[strings.Fields(invocation)[0]] {
				continue
			}
			total++
			if err := validateAdvertisedAoInvocation(rootCmd, invocation); err != nil {
				t.Errorf("%s advertises `ao %s`, which does not resolve in the default build: %v", name, invocation, err)
			}
		}
	}
	// The sweep must actually see commands — an empty extraction means the
	// regex or a source regressed, not that everything is clean.
	if total < 10 {
		t.Fatalf("expected the sweep to find at least 10 advertised ao invocations, found %d", total)
	}
}
