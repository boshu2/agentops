package provenance

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/evidence"
)

func TestEvidenceOrphansCommand(t *testing.T) {
	root := t.TempDir()
	artifact := filepath.Join(root, "docs/evals/scorecards/sc.json")
	if err := os.MkdirAll(filepath.Dir(artifact), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, []byte(`{"evaluator":{"h":{"path":"missing","sha256":"old"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	command := NewModule(clicontract.HostOptions{}).Command()
	var out, stderr bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&stderr)
	command.SetArgs([]string{"evidence-orphans", "--root", root, "--changed", "missing", "--changed", "missing"})
	if err := command.Execute(); err != nil {
		t.Fatalf("scan: %v %s", err, stderr.String())
	}
	var result evidence.OrphanReceipt
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.BindingCount != 1 || result.ArtifactCount != 1 || result.Orphaned[0].Cause != "both" || !reflect.DeepEqual(result.Changed, []string{"missing", "missing"}) {
		t.Fatalf("wrong receipt: %+v", result)
	}
	leaf, _, err := command.Find([]string{"evidence-orphans"})
	if err != nil {
		t.Fatal(err)
	}
	contract, ok := clicontract.ContractFor(leaf)
	if !ok || contract.ID != "ao.provenance.evidence-orphans" || contract.Effects != clicontract.EffectFilesystem || contract.Output != clicontract.OutputStructured || !reflect.DeepEqual(contract.ExitClasses, map[int]clicontract.ExitClass{0: clicontract.ExitSuccess, 2: clicontract.ExitFailure}) {
		t.Fatalf("wrong native contract: %+v", contract)
	}
	if leaf.Flags().Lookup("strict") != nil || leaf.Flags().Lookup("graph") != nil {
		t.Fatal("scanner acquired graph/gate behavior")
	}
}

func TestEvidenceOrphansIncompleteAndFormats(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		badJSON bool
	}{
		{"missing root", nil, false}, {"positional", []string{"--root", "ROOT", "extra"}, false},
		{"unknown flag", []string{"--root", "ROOT", "--text"}, false}, {"missing changed value", []string{"--root", "ROOT", "--changed"}, false},
		{"malformed scan", []string{"--root", "ROOT"}, true},
		{"explicit table", []string{"--root", "ROOT", "--output", "table"}, false},
		{"explicit yaml", []string{"--root", "ROOT", "--output", "yaml"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.badJSON {
				dir := filepath.Join(root, "docs/evals/scorecards")
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			output := "table"
			command := NewModule(clicontract.HostOptions{OutputMode: func() string { return output }}).Command()
			command.PersistentFlags().StringVar(&output, "output", "table", "format")
			args := append([]string{"evidence-orphans"}, tc.args...)
			for n, x := range args {
				if x == "ROOT" {
					args[n] = root
				}
			}
			var out, stderr bytes.Buffer
			command.SetOut(&out)
			command.SetErr(&stderr)
			command.SetArgs(args)
			err := command.Execute()
			var failure *clicontract.ExitError
			if !errors.As(err, &failure) || failure.Code != 2 || out.Len() != 0 {
				t.Fatalf("expected incomplete exit2/no JSON, got %v out=%q stderr=%q", err, out.String(), stderr.String())
			}
		})
	}
}
