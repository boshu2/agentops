package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAgentsCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"agents"})
	if err != nil {
		t.Fatalf("agents command not registered: %v", err)
	}
	if cmd.Name() != "agents" {
		t.Fatalf("found %q, want %q", cmd.Name(), "agents")
	}
}

func TestAgentsInspectCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"agents", "inspect"})
	if err != nil {
		t.Fatalf("agents inspect command not registered: %v", err)
	}
	if cmd.Name() != "inspect" {
		t.Fatalf("found %q, want %q", cmd.Name(), "inspect")
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Error("expected --json flag")
	}
	if cmd.Flags().Lookup("contract") == nil {
		t.Error("expected --contract flag")
	}
}

func TestParseAgentsAllowlist(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name: "empty content",
			want: []string{},
		},
		{
			name: "no markers",
			content: "ao\nlearnings\n",
			want: []string{},
		},
		{
			name: "basic allowlist",
			content: `# heading
<!-- BEGIN agents-write-surfaces-allowlist -->
ao
learnings
patterns
<!-- END agents-write-surfaces-allowlist -->
trailing
`,
			want: []string{"ao", "learnings", "patterns"},
		},
		{
			name: "inline comments and blanks",
			content: `<!-- BEGIN agents-write-surfaces-allowlist -->

# core
ao   # core runtime

# promoted
learnings   # promoted artifacts
<!-- END agents-write-surfaces-allowlist -->
`,
			want: []string{"ao", "learnings"},
		},
		{
			name: "duplicates dedup and sorted",
			content: `<!-- BEGIN agents-write-surfaces-allowlist -->
patterns
ao
learnings
ao
<!-- END agents-write-surfaces-allowlist -->
`,
			want: []string{"ao", "learnings", "patterns"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAgentsAllowlist(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseAgentsAllowlist() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiscoverActiveSkills(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(tmp, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("ok"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// dir without SKILL.md should be ignored
	if err := os.MkdirAll(filepath.Join(tmp, "no-skill-md"), 0o755); err != nil {
		t.Fatal(err)
	}
	// regular file should be ignored
	if err := os.WriteFile(filepath.Join(tmp, "stray.md"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := discoverActiveSkills(tmp)
	want := []string{"alpha", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("discoverActiveSkills() = %v, want %v", got, want)
	}
}

func TestDiscoverActiveSkills_MissingDir(t *testing.T) {
	got := discoverActiveSkills(filepath.Join(t.TempDir(), "nope"))
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

func TestRunAgentsInspect_TextOutput(t *testing.T) {
	tmp := t.TempDir()
	contract := filepath.Join(tmp, "contract.md")
	contractContent := `# Title
<!-- BEGIN agents-write-surfaces-allowlist -->
ao
learnings
<!-- END agents-write-surfaces-allowlist -->
`
	if err := os.WriteFile(contract, []byte(contractContent), 0o644); err != nil {
		t.Fatal(err)
	}

	origJSON := agentsInspectJSON
	origContract := agentsInspectContract
	t.Cleanup(func() {
		agentsInspectJSON = origJSON
		agentsInspectContract = origContract
	})
	agentsInspectJSON = false
	agentsInspectContract = contract

	var buf bytes.Buffer
	agentsInspectCmd.SetOut(&buf)
	t.Cleanup(func() { agentsInspectCmd.SetOut(nil) })

	if err := runAgentsInspect(agentsInspectCmd, nil); err != nil {
		t.Fatalf("runAgentsInspect: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Contract: " + contract,
		"Catalogued surfaces: 2",
		".agents/ao/",
		".agents/learnings/",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestRunAgentsInspect_JSONOutput(t *testing.T) {
	tmp := t.TempDir()
	contract := filepath.Join(tmp, "contract.md")
	contractContent := `<!-- BEGIN agents-write-surfaces-allowlist -->
ao
patterns
<!-- END agents-write-surfaces-allowlist -->
`
	if err := os.WriteFile(contract, []byte(contractContent), 0o644); err != nil {
		t.Fatal(err)
	}

	origJSON := agentsInspectJSON
	origContract := agentsInspectContract
	t.Cleanup(func() {
		agentsInspectJSON = origJSON
		agentsInspectContract = origContract
	})
	agentsInspectJSON = true
	agentsInspectContract = contract

	var buf bytes.Buffer
	agentsInspectCmd.SetOut(&buf)
	t.Cleanup(func() { agentsInspectCmd.SetOut(nil) })

	if err := runAgentsInspect(agentsInspectCmd, nil); err != nil {
		t.Fatalf("runAgentsInspect: %v", err)
	}

	var got AgentsInventory
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\nGot: %s", err, buf.String())
	}
	if got.Contract != contract {
		t.Errorf("Contract = %q, want %q", got.Contract, contract)
	}
	wantList := []string{"ao", "patterns"}
	if !reflect.DeepEqual(got.Allowlist, wantList) {
		t.Errorf("Allowlist = %v, want %v", got.Allowlist, wantList)
	}
}

func TestRunAgentsInspect_MissingContract(t *testing.T) {
	origJSON := agentsInspectJSON
	origContract := agentsInspectContract
	t.Cleanup(func() {
		agentsInspectJSON = origJSON
		agentsInspectContract = origContract
	})
	agentsInspectJSON = false
	agentsInspectContract = filepath.Join(t.TempDir(), "missing.md")

	var buf bytes.Buffer
	agentsInspectCmd.SetOut(&buf)
	t.Cleanup(func() { agentsInspectCmd.SetOut(nil) })

	err := runAgentsInspect(agentsInspectCmd, nil)
	if err == nil {
		t.Fatal("expected error for missing contract")
	}
	if !strings.Contains(err.Error(), "reading contract") {
		t.Errorf("error = %v, want one mentioning 'reading contract'", err)
	}
}
