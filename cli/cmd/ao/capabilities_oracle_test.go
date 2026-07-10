package main

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

type oracleCapability struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

func TestCapabilitiesOracleRecursivelyMatchesPublicTree(t *testing.T) {
	out, err := executeCommand("capabilities")
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Commands []oracleCapability `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out), &wire); err != nil {
		t.Fatal(err)
	}
	want := independentPublicCommandPaths(rootCmd)
	got := make([]string, 0, len(wire.Commands))
	ids := map[string]bool{}
	for _, command := range wire.Commands {
		if command.ID == "" || ids[command.ID] {
			t.Fatalf("capability has empty or duplicate stable id %q", command.ID)
		}
		ids[command.ID] = true
		got = append(got, command.Path)
	}
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("recursive capability surface drift:\n got %d commands\nwant %d commands\nfirst expected: %s", len(got), len(want), want[0])
	}
}

func TestCapabilitiesExitOracleIncludesDefinedCodeFive(t *testing.T) {
	if got := capabilitiesCommandExitCodes["plan-pawl decide"]["5"]; got == "" {
		t.Fatal("plan-pawl decide defines exit 5 (DEGRADED) but capabilities omits it")
	}
}

func independentPublicCommandPaths(root *cobra.Command) []string {
	var paths []string
	var walk func(*cobra.Command)
	walk = func(parent *cobra.Command) {
		for _, child := range parent.Commands() {
			if child.Hidden || child.Name() == "help" || child.Name() == "completion" {
				continue
			}
			paths = append(paths, child.CommandPath())
			walk(child)
		}
	}
	walk(root)
	sort.Strings(paths)
	return paths
}
