package provenance

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/okfprofile"
	"gopkg.in/yaml.v3"
)

func TestCheckOKFCLIReadOnlyAndNoSourceFetch(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	fixture, err := os.ReadFile("../../okfprofile/testdata/maintained-reference.md")
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(strings.Replace(string(fixture), "https://example.invalid/declared-source", server.URL+"/source-locator-canary", 1))
	dir := t.TempDir()
	file := filepath.Join(dir, "input.md")
	if err := os.WriteFile(file, payload, 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"", "json", "yaml"} {
		t.Run("format-"+mode, func(t *testing.T) {
			cmd := NewModule(clicontract.HostOptions{OutputMode: func() string { return mode }, DryRun: func() bool { return true }}).Command()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"check-okf", "--file", file})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if mode == "yaml" {
				err = yaml.Unmarshal(out.Bytes(), &result)
			} else {
				err = json.Unmarshal(out.Bytes(), &result)
			}
			if err != nil || result["structurally_valid"] != true || result["assurance"] != okfprofile.Assurance {
				t.Fatalf("result: %v %v", result, err)
			}
			if _, exists := result["verdict"]; exists {
				t.Fatal("structural checker emitted semantic verdict")
			}
			if strings.Contains(out.String(), "source-locator-canary") || strings.Contains(out.String(), "human:example") || strings.Contains(out.String(), "Synthetic request behavior") {
				t.Fatal("candidate text echoed")
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatal("checker fetched a source")
	}
	after, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(after, payload) {
		t.Fatal("checker changed candidate")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("unexpected file effects: %v %v", entries, err)
	}
}

func TestCheckOKFCLIFindingsAndInvalidRequests(t *testing.T) {
	fixture, err := os.ReadFile("../../okfprofile/testdata/maintained-reference.md")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "input.md")
	if err := os.WriteFile(file, []byte(strings.Replace(string(fixture), "status: stable\n", "", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		args   []string
		report bool
	}{
		{"missing status", []string{"--file", file}, true},
		{"missing file flag", nil, false},
		{"unknown profile before read", []string{"--file", "not-present.md", "--profile", "future/2"}, false},
		{"empty profile", []string{"--file", file, "--profile", ""}, false},
		{"positional argument", []string{"--file", file, "unexpected"}, false},
		{"unavailable file", []string{"--file", "not-present.md"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := execProv(t, "must-not-resolve-ledger", append([]string{"check-okf"}, tc.args...)...)
			if err == nil {
				t.Fatal("invalid request succeeded")
			}
			if tc.report {
				var result okfprofile.Result
				if err := json.Unmarshal([]byte(out), &result); err != nil {
					t.Fatal(err)
				}
				if result.StructurallyValid || len(result.Issues) != 1 || result.Issues[0].Field != "status" {
					t.Fatalf("missing explicit status: %+v", result)
				}
			} else if out != "" {
				t.Fatalf("invalid operation emitted misleading report: %s", out)
			}
		})
	}
	if err := os.WriteFile(file, []byte("---\ntype: [private-content-canary\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := execProv(t, "unused", "check-okf", "--file", file)
	if err == nil || strings.Contains(out+err.Error(), "private-content-canary") {
		t.Fatal("parser error exposed content or succeeded")
	}
}

func TestCheckOKFCapabilityContract(t *testing.T) {
	cmd := NewModule(clicontract.HostOptions{}).Command()
	leaf, _, err := cmd.Find([]string{"check-okf"})
	if err != nil {
		t.Fatal(err)
	}
	contract, ok := clicontract.ContractFor(leaf)
	if !ok || contract.ID != "ao.provenance.check-okf" || contract.Effects != clicontract.EffectFilesystem || contract.Output != clicontract.OutputStructured || contract.Args.Name != "no-args" {
		t.Fatalf("incorrect contract: %+v", contract)
	}
	if len(contract.ExitClasses) != 2 || contract.ExitClasses[0] != clicontract.ExitSuccess || contract.ExitClasses[1] != clicontract.ExitFailure {
		t.Fatalf("incorrect exits: %+v", contract.ExitClasses)
	}
	if leaf.Flags().Lookup("profile").DefValue != okfprofile.Profile {
		t.Fatal("profile default does not match implementation pin")
	}
}
