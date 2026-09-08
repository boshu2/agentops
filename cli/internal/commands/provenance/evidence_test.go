package provenance

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

func TestEvidenceSnapshotExternal(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "intent")
	payload := []byte("Independent factual support acceptance\n")
	if err := os.WriteFile(source, payload, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := NewModule(clicontract.HostOptions{}).Command()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"snapshot-intent", "--source", source, "--evidence-root", root, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("snapshot-intent: %v; %s", err, out.String())
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	if !strings.Contains(out.String(), digest) {
		t.Fatalf("missing exact-byte digest: %s", out.String())
	}
	stored, err := os.ReadFile(filepath.Join(root, "intents", "sha256", digest+".intent"))
	if err != nil || !bytes.Equal(stored, payload) {
		t.Fatalf("external snapshot: %q, %v", stored, err)
	}
}

func TestEvidenceVersionDryRunAndRequiredFlags(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		dry  bool
	}{
		{"version", []string{"snapshot-intent", "--source", "-", "--evidence-root", "ROOT", "--helper-version", "999"}, false},
		{"missing root", []string{"snapshot-intent", "--source", "-"}, false},
		{"dry run", []string{"snapshot-intent", "--source", "-", "--evidence-root", "ROOT"}, true},
		{"invalid scope", []string{"store-verdict", "--root", "ROOT", "--evidence-root", "ROOT", "--draft", "missing", "--intent-source", "missing", "--subject-manifest", "missing", "--author-context-id", "a", "--validator-context-id", "b", "--freshness-source", "runtime", "--freshness-attester-id", "r", "--scope-result", "unknown"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			args := append([]string(nil), test.args...)
			for i := range args {
				if args[i] == "ROOT" {
					args[i] = root
				}
			}
			cmd := NewModule(clicontract.HostOptions{DryRun: func() bool { return test.dry }}).Command()
			cmd.SetArgs(args)
			cmd.SetIn(strings.NewReader("intent"))
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			if err := cmd.Execute(); err == nil {
				t.Fatal("expected pre-mutation rejection")
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("output mutated: %v %v", entries, err)
			}
		})
	}
}

func TestEvidenceGitStorageCapabilities(t *testing.T) {
	root := NewModule(clicontract.HostOptions{}).Command()
	family, ok := clicontract.ContractFor(root)
	if !ok || family.Effects.String() != "filesystem,environment,clock" {
		t.Fatalf("family contract: %+v", family)
	}
	for _, command := range root.Commands() {
		switch command.Name() {
		case "snapshot-intent", "manifest", "store-verdict":
			c, ok := clicontract.ContractFor(command)
			if !ok || c.Effects.String() != "filesystem,environment" || command.Flags().Lookup("exclude-git-root") == nil {
				t.Fatalf("missing actual storage boundary contract: %+v", c)
			}
		}
	}
}
