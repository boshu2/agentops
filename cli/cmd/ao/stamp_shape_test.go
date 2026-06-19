package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/orchestration"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

func TestExecuteStampShapeUsesCanonicalDefaultPacketPath(t *testing.T) {
	t.Setenv("RPI_RUN_ID", "")

	root := t.TempDir()
	stateDir := filepath.Join(root, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}

	packet := []byte(`{"run_id":"run-123","orchestration_decision":{"chosen_shape":"single-agent"}}`)
	aliasPath := filepath.Join(stateDir, cliRPI.ExecutionPacketFile)
	if err := os.WriteFile(aliasPath, packet, 0o600); err != nil {
		t.Fatal(err)
	}

	archiveDir := cliRPI.RPIRunRegistryDir(root, "run-123")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(archiveDir, cliRPI.ExecutionPacketFile)
	if err := os.WriteFile(archivePath, packet, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Chdir(root)

	var out bytes.Buffer
	if err := executeStampShape(stampShapeOptions{
		NoAM: true,
		Out:  &out,
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "shape="+orchestration.ShapeSingleAgent) {
		t.Fatalf("output %q does not report single-agent shape", out.String())
	}

	for _, path := range []string{aliasPath, archivePath} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var stamped struct {
			OrchestrationDecision struct {
				ChosenShape     string   `json:"chosen_shape"`
				PredicatesFired []string `json:"predicates_fired"`
			} `json:"orchestration_decision"`
		}
		if err := json.Unmarshal(data, &stamped); err != nil {
			t.Fatal(err)
		}
		if stamped.OrchestrationDecision.ChosenShape != orchestration.ShapeSingleAgent {
			t.Fatalf("%s chosen_shape = %q, want %q", path, stamped.OrchestrationDecision.ChosenShape, orchestration.ShapeSingleAgent)
		}
		if len(stamped.OrchestrationDecision.PredicatesFired) != 0 {
			t.Fatalf("%s predicates_fired = %v, want empty", path, stamped.OrchestrationDecision.PredicatesFired)
		}
	}
}
