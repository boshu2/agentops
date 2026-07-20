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

// withOutputMode pins the package-global output flag the doctor host seam reads
// (GetOutput) for the duration of a test, isolating it from -shuffle leakage.
func withOutputMode(t *testing.T, mode string) {
	t.Helper()
	original := output
	output = mode
	t.Cleanup(func() { output = original })
}

func TestDoctorModuleTableOutput(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("HOME", t.TempDir())
	withOutputMode(t, "table")
	if err := os.MkdirAll(filepath.Join(root, ".agents", "ao"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := newDoctorCommand()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	_ = command.Execute()
	if !strings.Contains(out.String(), "ao doctor") {
		t.Fatalf("output=%q", out.String())
	}
}

func TestDoctorModuleJSONOutputIsSingleHealthReport(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("HOME", t.TempDir())
	withOutputMode(t, "table")
	command := newDoctorCommand()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetArgs([]string{"--json"})
	_ = command.Execute()
	var report quality.DoctorOutput
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json=%q err=%v", out.String(), err)
	}
	if len(report.Checks) == 0 || report.Result == "" {
		t.Fatalf("report=%+v", report)
	}
}
