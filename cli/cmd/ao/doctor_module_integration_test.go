package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/quality"
)

func TestDoctorModuleTableOutput(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(root, ".agents", "ao"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := doctorModule.Command()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	_ = command.Execute()
	if !strings.Contains(output.String(), "ao doctor") {
		t.Fatalf("output=%q", output.String())
	}
}

func TestDoctorModuleJSONOutputIsSingleHealthReport(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("HOME", t.TempDir())
	command := doctorModule.Command()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"--json"})
	_ = command.Execute()
	var report quality.DoctorOutput
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("json=%q err=%v", output.String(), err)
	}
	if len(report.Checks) == 0 || report.Result == "" {
		t.Fatalf("report=%+v", report)
	}
}
