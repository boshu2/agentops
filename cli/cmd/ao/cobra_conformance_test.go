// practices: [pragmatic-programmer, twelve-factor-app]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

var commandHeadingPattern = regexp.MustCompile("(?m)^#{3,6} `(ao(?: [^`]+)+)`$")

func TestCobraConformance(t *testing.T) {
	// COMMANDS.md documents the DEFAULT (spine) build. The flywheel/legacy build
	// restores ADR-0012 archived commands, making the live tree a superset of the
	// generated docs by design — skip when any archive build tag is active.
	if len(archiveBuildTags) > 0 {
		t.Skipf("spine-conformance test: archive build tags active (%v); the restored build is a documented superset", archiveBuildTags)
	}
	removed := pruneToDefaultSpine(rootCmd)
	t.Cleanup(func() { restorePrunedCommands(rootCmd, removed) })
	rootCmd.InitDefaultHelpCmd()

	docsPath := filepath.Join("..", "..", "docs", "COMMANDS.md")
	data, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("read generated CLI reference: %v", err)
	}

	liveCommands := visibleCobraCommandPaths(rootCmd)
	documentedCommands := documentedCommandPaths(string(data))

	missingDocs := difference(liveCommands, documentedCommands)
	staleDocs := difference(documentedCommands, liveCommands)
	if len(missingDocs) > 0 || len(staleDocs) > 0 {
		t.Fatalf("cli/docs/COMMANDS.md is not in conformance with the live Cobra tree.\nMissing from docs:%s\nStale in docs:%s\nRun scripts/generate-cli-reference.sh to regenerate.",
			formatCommandList(missingDocs),
			formatCommandList(staleDocs),
		)
	}
}

func visibleCobraCommandPaths(root *cobra.Command) map[string]struct{} {
	paths := map[string]struct{}{}

	var walk func(*cobra.Command)
	walk = func(parent *cobra.Command) {
		for _, child := range parent.Commands() {
			if child.Hidden {
				continue
			}

			path := strings.TrimSpace(child.CommandPath())
			if path != "" {
				paths[path] = struct{}{}
			}

			walk(child)
		}
	}

	walk(root)
	return paths
}

func documentedCommandPaths(docs string) map[string]struct{} {
	paths := map[string]struct{}{}

	for _, match := range commandHeadingPattern.FindAllStringSubmatch(docs, -1) {
		paths[strings.TrimSpace(match[1])] = struct{}{}
	}

	return paths
}

func difference(left, right map[string]struct{}) []string {
	var diff []string

	for path := range left {
		if _, ok := right[path]; !ok {
			diff = append(diff, path)
		}
	}

	sort.Strings(diff)
	return diff
}

func formatCommandList(paths []string) string {
	if len(paths) == 0 {
		return "\n  - none"
	}

	var builder strings.Builder
	for _, path := range paths {
		fmt.Fprintf(&builder, "\n  - %s", path)
	}

	return builder.String()
}
