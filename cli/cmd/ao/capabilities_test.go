// practices: [ai-assisted-dev, pragmatic-programmer]
package main

import (
	"encoding/json"
	"strconv"
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

// TestCapabilities_CommandExitCodesPublished proves audit A7: the machine
// contract now enumerates the per-command typed exit codes an agent must
// interpret (pawl/plan-pawl/governor emit 3/4), and does NOT leak legacy or
// flywheel commands absent from the shipping binary.
func TestCapabilities_CommandExitCodesPublished(t *testing.T) {
	doc := buildCapabilitiesDoc()
	if len(doc.CommandExitCodes) == 0 {
		t.Fatal("command_exit_codes is empty; an agent cannot interpret codes 3-4")
	}
	if doc.CommandExitCodes["pawl review"]["3"] == "" {
		t.Error("pawl review exit 3 (REFUTED) not documented")
	}
	for _, absent := range []string{"tick", "corpus-scan", "evolve", "orchestrate"} {
		for cmd := range doc.CommandExitCodes {
			if strings.HasPrefix(cmd, absent) {
				t.Errorf("command_exit_codes lists %q — a command absent from the default build must not appear", cmd)
			}
		}
	}
}

// TestCapabilitiesCommandExitCodesMatchConstants keeps the published per-command
// codes in lockstep with the code's own exit-code constants, so a constant change
// cannot silently drift the contract (the exact drift class audit A7 closes).
func TestCapabilitiesCommandExitCodesMatchConstants(t *testing.T) {
	planPawl := capabilitiesCommandExitCodes["plan-pawl decide"]
	for code, name := range map[int]string{
		planPawlExitPass:    "PASS",
		planPawlExitUsage:   "usage",
		planPawlExitRedo:    "REDO",
		planPawlExitBlocked: "BLOCKED",
	} {
		if _, ok := planPawl[strconv.Itoa(code)]; !ok {
			t.Errorf("plan-pawl decide: exit %d (%s) is a defined const but missing from capabilities", code, name)
		}
	}
	if _, ok := capabilitiesCommandExitCodes["governor budget"][strconv.Itoa(hardenExitCode)]; !ok {
		t.Errorf("governor budget: hardenExitCode=%d missing from capabilities", hardenExitCode)
	}
	// pawl review codes come from scripts/pawl-review.sh (the pawl.go:29 contract): 0/1/2/3/4.
	pawl := capabilitiesCommandExitCodes["pawl review"]
	for _, code := range []string{"0", "1", "2", "3", "4"} {
		if pawl[code] == "" {
			t.Errorf("pawl review: documented contract code %s missing from capabilities", code)
		}
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
