// practices: [agent-ergonomics]
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCapabilities_EmitsValidJSON(t *testing.T) {
	out, err := executeCommand("capabilities")
	if err != nil {
		t.Fatalf("ao capabilities returned error: %v", err)
	}
	var doc capabilitiesDoc
	if jerr := json.Unmarshal([]byte(out), &doc); jerr != nil {
		t.Fatalf("ao capabilities output is not valid JSON: %v\noutput: %s", jerr, out)
	}
	if doc.Tool != "ao" {
		t.Errorf("tool = %q, want %q", doc.Tool, "ao")
	}
	if doc.ContractVersion != capabilitiesContractVersion {
		t.Errorf("contract_version = %q, want %q", doc.ContractVersion, capabilitiesContractVersion)
	}
	if len(doc.CommandGroups) == 0 {
		t.Error("command_groups is empty; expected the live command tree")
	}
	if doc.ExitCodes["0"] != "success" {
		t.Errorf("exit_codes[0] = %q, want %q", doc.ExitCodes["0"], "success")
	}
	if _, ok := doc.RobotSurfaces["robot_docs"]; !ok {
		t.Error("robot_surfaces missing robot_docs pointer")
	}
}

func TestCapabilities_ListsRegisteredCommands(t *testing.T) {
	doc := buildCapabilitiesDoc()
	seen := map[string]bool{}
	for _, g := range doc.CommandGroups {
		for _, c := range g.Commands {
			seen[c.Name] = true
		}
	}
	// capabilities and robot-docs must appear in their own contract.
	for _, want := range []string{"capabilities", "robot-docs", "status", "doctor"} {
		if !seen[want] {
			t.Errorf("capabilities contract missing command %q", want)
		}
	}
}

func TestCapabilities_GlobalFlagsIncludeJSON(t *testing.T) {
	doc := buildCapabilitiesDoc()
	found := false
	for _, f := range doc.GlobalFlags {
		if f.Name == "json" {
			found = true
		}
	}
	if !found {
		t.Error("global_flags should include the --json flag")
	}
}

func TestCapabilities_RegisteredOnRoot(t *testing.T) {
	if capabilitiesCmd.GroupID != "core" {
		t.Errorf("capabilitiesCmd.GroupID = %q, want core", capabilitiesCmd.GroupID)
	}
	for _, c := range rootCmd.Commands() {
		if c.Name() == "capabilities" {
			return
		}
	}
	t.Error("capabilities command not registered on rootCmd")
}

func TestRobotDocs_ContainsContractSections(t *testing.T) {
	out, err := executeCommand("robot-docs")
	if err != nil {
		t.Fatalf("ao robot-docs returned error: %v", err)
	}
	for _, want := range []string{
		"# ao — Agent Handbook",
		"## Output contract",
		"## Exit codes",
		"ao capabilities",
		"## Command surface",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("robot-docs output missing %q", want)
		}
	}
}
