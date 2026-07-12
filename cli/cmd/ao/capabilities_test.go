// practices: [ai-assisted-dev, pragmatic-programmer]
package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	capabilitiesapp "github.com/boshu2/agentops/cli/internal/capabilities"
	"github.com/spf13/cobra"
)

func TestCapabilitiesEnvironmentInputsMatchExecutableOwners(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve capabilities test path")
	}
	cliRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	cmd := exec.Command("go", "list", "-json", "./cmd/ao", "./internal/...")
	cmd.Dir = cliRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list default-build owners: %v\n%s", err, stderr.String())
	}

	type listedPackage struct {
		Dir     string
		GoFiles []string
	}
	want := map[string]bool{"NO_COLOR": true}
	pattern := regexp.MustCompile(`\b(?:AGENTOPS|AO|PAWL)_[A-Z0-9_]+\b`)
	decoder := json.NewDecoder(&stdout)
	for {
		var pkg listedPackage
		if err := decoder.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode go list output: %v", err)
		}
		for _, name := range pkg.GoFiles {
			path := filepath.Join(pkg.Dir, name)
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			ast.Inspect(file, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(literal.Value)
				if err != nil {
					return true
				}
				for _, input := range pattern.FindAllString(value, -1) {
					want[input] = true
				}
				if strings.Contains(value, "NO_COLOR") {
					want["NO_COLOR"] = true
				}
				return true
			})
		}
	}

	doc := readCapabilitiesDocument(t)
	var missing, empty []string
	for input := range want {
		description, ok := doc.EnvVars[input]
		if !ok {
			missing = append(missing, input)
		} else if strings.TrimSpace(description) == "" {
			empty = append(empty, input)
		}
	}
	sort.Strings(missing)
	sort.Strings(empty)
	if len(missing) > 0 || len(empty) > 0 {
		t.Fatalf("capabilities environment projection drift: missing=%v empty=%v", missing, empty)
	}

	var stale []string
	for input := range doc.EnvVars {
		if !want[input] {
			stale = append(stale, input)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Fatalf("capabilities publishes non-default or stale environment inputs: %v", stale)
	}
}

func TestCapabilities_EmitsValidJSON(t *testing.T) {
	doc := readCapabilitiesDocument(t)
	if doc.Tool != "ao" {
		t.Errorf("tool = %q, want %q", doc.Tool, "ao")
	}
	if doc.ContractVersion != capabilitiesapp.ContractVersion {
		t.Errorf("contract_version = %q, want %q", doc.ContractVersion, capabilitiesapp.ContractVersion)
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
	doc := readCapabilitiesDocument(t)
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
	doc := readCapabilitiesDocument(t)
	planPawl := doc.CommandExitCodes["plan-pawl decide"]
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
	if _, ok := doc.CommandExitCodes["governor budget"][strconv.Itoa(hardenExitCode)]; !ok {
		t.Errorf("governor budget: hardenExitCode=%d missing from capabilities", hardenExitCode)
	}
	// pawl review codes come from scripts/pawl-review.sh (the pawl.go contract):
	// 0/1/2/3/4 plus strict HOLD/UNAVAILABLE at 5.
	pawl := doc.CommandExitCodes["pawl review"]
	for _, code := range []string{"0", "1", "2", "3", "4", "5"} {
		if pawl[code] == "" {
			t.Errorf("pawl review: documented contract code %s missing from capabilities", code)
		}
	}
	verify := doc.CommandExitCodes["verify"]
	for _, code := range []string{"0", "1", "2", "3", "4", "5"} {
		if verify[code] == "" {
			t.Errorf("verify: executable contract code %s missing from capabilities", code)
		}
	}
}

func TestCapabilitiesRecursiveCommandsPublishExecutableHoldExits(t *testing.T) {
	clearVerifyCfgEnv(t)
	marker := writeVerifyTestRepo(t, 0)
	repo := testProjectDir
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".aoverify.yaml"), []byte("- not\n- a\n- mapping\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	verifyInvocation := &cobra.Command{}
	verifyInvocation.SetErr(io.Discard)
	verifyCode, verifyIsVerdict := verdictShape(runVerify(verifyInvocation, []string{"age-capabilities-oracle"}))
	if !verifyIsVerdict || verifyCode != 5 {
		t.Fatalf("malformed committed policy returned code=%d verdict=%v, want executable HOLD exit 5", verifyCode, verifyIsVerdict)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("verify HOLD must precede review execution; marker stat=%v", err)
	}

	script := filepath.Join(repo, "scripts", "pawl-review.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\nexit 5\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	pawlCode, pawlIsVerdict := verdictShape(runPawlReview(pawlReviewCmd, []string{"age-capabilities-oracle", "--strict"}))
	if !pawlIsVerdict || pawlCode != 5 {
		t.Fatalf("pawl strict HOLD returned code=%d verdict=%v, want executable exit 5", pawlCode, pawlIsVerdict)
	}

	doc := readCapabilitiesDocument(t)
	want := map[string]string{"ao.verify": "HOLD", "ao.pawl.review": "HOLD"}
	for id, meaning := range want {
		var exits map[string]string
		for _, command := range doc.Commands {
			if command.ID == id {
				exits = command.ExitCodes
				break
			}
		}
		if exits == nil {
			t.Errorf("recursive capabilities contract missing command %s", id)
			continue
		}
		if !strings.Contains(strings.ToUpper(exits["5"]), meaning) {
			t.Errorf("%s recursive exit_codes[5] = %q, want executable %s contract", id, exits["5"], meaning)
		}
	}
}

func TestCapabilities_ListsRegisteredCommands(t *testing.T) {
	doc := readCapabilitiesDocument(t)
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
	doc := readCapabilitiesDocument(t)
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
	for _, c := range rootCmd.Commands() {
		if c.Name() == "capabilities" {
			if c.GroupID != "core" {
				t.Errorf("capabilities GroupID = %q, want core", c.GroupID)
			}
			return
		}
	}
	t.Error("capabilities command not registered on rootCmd")
}

func readCapabilitiesDocument(t *testing.T) capabilitiesapp.Document {
	t.Helper()
	out, err := executeCommand("capabilities")
	if err != nil {
		t.Fatalf("ao capabilities returned error: %v", err)
	}
	var document capabilitiesapp.Document
	if err := json.Unmarshal([]byte(out), &document); err != nil {
		t.Fatalf("ao capabilities output is not valid JSON: %v\noutput: %s", err, out)
	}
	return document
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
