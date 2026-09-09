package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/evidence"
	"gopkg.in/yaml.v3"
)

func TestStatusExplicitEvidenceRootComposed(t *testing.T) {
	bin, root, cwd := aoBinary(t), t.TempDir(), t.TempDir()
	if _, err := evidence.SnapshotIntent(root, []byte("selected evidence")); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(config, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTOPS_CONFIG", config)
	t.Setenv("AGENTOPS_OUTPUT", "table")
	for _, format := range []string{"text", "json", "yaml"} {
		args := []string{"status", "--evidence-root", root}
		if format != "text" {
			args = append(args, "-o", format)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("status %s: %v\n%s", format, err, out)
		}
		if format == "text" {
			if !strings.Contains(string(out), "Artifacts: 1 intents, 0 verdicts") {
				t.Fatalf("text selected wrong evidence: %s", out)
			}
			continue
		}
		var report map[string]map[string]any
		if format == "json" {
			err = json.Unmarshal(out, &report)
		} else {
			err = yaml.Unmarshal(out, &report)
		}
		if err != nil {
			t.Fatal(err)
		}
		loop := report["loop_evidence"]
		want := any(float64(1))
		if format == "yaml" {
			want = 1
		}
		if loop["intent_artifacts"] != want || loop["state"] != "intent_is_latest_evidence" {
			t.Fatalf("%s selected wrong evidence: %s", format, out)
		}
	}
}

func TestStatusExplicitEvidenceRootCapability(t *testing.T) {
	entry := capabilityEntry(t, "ao status")
	for _, flag := range entry.Flags {
		if flag.Name == "evidence-root" {
			if flag.Required || flag.Origin != "local" || entry.Effects != "filesystem,environment,clock" {
				t.Fatalf("status root contract: %+v, %+v", flag, entry)
			}
			return
		}
	}
	t.Fatal("status capabilities omit --evidence-root")
}
