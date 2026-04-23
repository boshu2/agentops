package quality

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestComputeResult_Healthy(t *testing.T) {
	t.Parallel()
	out := ComputeResult([]Check{{Name: "a", Status: "pass", Required: true}, {Name: "b", Status: "pass"}})
	if out.Result != "HEALTHY" {
		t.Errorf("Result = %q, want HEALTHY", out.Result)
	}
}

func TestHealStrictTimeout_DefaultsTo120s(t *testing.T) {
	t.Setenv("AO_DOCTOR_HEAL_TIMEOUT", "")
	got := healStrictTimeout()
	if got != 120*time.Second {
		t.Errorf("healStrictTimeout() default = %s, want 2m0s", got)
	}
}

func TestHealStrictTimeout_HonorsEnvOverride(t *testing.T) {
	t.Setenv("AO_DOCTOR_HEAL_TIMEOUT", "3m")
	got := healStrictTimeout()
	if got != 3*time.Minute {
		t.Errorf("healStrictTimeout() with AO_DOCTOR_HEAL_TIMEOUT=3m = %s, want 3m0s", got)
	}
}

func TestHealStrictTimeout_IgnoresInvalidOverride(t *testing.T) {
	// Garbage values fall back to the default so doctor never runs with a
	// zero-length timeout and instantly times out every check.
	for _, bad := range []string{"not-a-duration", "0", "-5s", "   "} {
		t.Setenv("AO_DOCTOR_HEAL_TIMEOUT", bad)
		got := healStrictTimeout()
		if got != 120*time.Second {
			t.Errorf("healStrictTimeout() with AO_DOCTOR_HEAL_TIMEOUT=%q = %s, want 2m0s", bad, got)
		}
	}
}

func TestComputeResult_Degraded(t *testing.T) {
	t.Parallel()
	out := ComputeResult([]Check{{Status: "pass"}, {Status: "warn"}})
	if out.Result != "DEGRADED" {
		t.Errorf("Result = %q, want DEGRADED", out.Result)
	}
}

func TestComputeResult_Unhealthy(t *testing.T) {
	t.Parallel()
	out := ComputeResult([]Check{{Status: "fail", Required: true}})
	if out.Result != "UNHEALTHY" {
		t.Errorf("Result = %q, want UNHEALTHY", out.Result)
	}
}

func TestHasRequiredFailure(t *testing.T) {
	t.Parallel()
	if HasRequiredFailure([]Check{{Status: "pass", Required: true}}) {
		t.Error("should not fail for passing required")
	}
	if HasRequiredFailure([]Check{{Status: "fail", Required: false}}) {
		t.Error("should not fail for optional failure")
	}
	if !HasRequiredFailure([]Check{{Status: "fail", Required: true}}) {
		t.Error("should fail for required failure")
	}
}

func TestStatusIcon(t *testing.T) {
	t.Parallel()
	if StatusIcon("pass") != "\u2713" {
		t.Error("pass icon wrong")
	}
	if StatusIcon("fail") != "\u2717" {
		t.Error("fail icon wrong")
	}
	if StatusIcon("warn") != "!" {
		t.Error("warn icon wrong")
	}
}

func TestFormatVersion(t *testing.T) {
	t.Parallel()
	if FormatVersion("1.0") != "v1.0" {
		t.Error("missing v prefix")
	}
	if FormatVersion("v1.0") != "v1.0" {
		t.Error("double v prefix")
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{3 * time.Hour, "3h"},
		{48 * time.Hour, "2d"},
	}
	for _, tt := range tests {
		if got := FormatDuration(tt.d); got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	t.Parallel()
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"}, {999, "999"}, {1000, "1,000"}, {1247, "1,247"}, {1000000, "1,000,000"},
	}
	for _, tt := range tests {
		if got := FormatNumber(tt.n); got != tt.want {
			t.Errorf("FormatNumber(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestCountFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if CountFiles(dir) != 0 {
		t.Error("empty dir should be 0")
	}
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	if CountFiles(dir) != 1 {
		t.Error("should count 1 file, not directory")
	}
}

func TestRunDoctor_JSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := RunDoctor(DoctorOptions{JSON: true, Checks: []Check{{Name: "test", Status: "pass", Required: true}}, Stdout: &buf})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("HEALTHY")) {
		t.Error("expected HEALTHY in JSON")
	}
}

func TestRunDoctor_FailsOnRequired(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := RunDoctor(DoctorOptions{Checks: []Check{{Name: "broken", Status: "fail", Required: true}}, Stdout: &buf})
	if err == nil {
		t.Error("expected error")
	}
}

func TestCheckKnowledgeBase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if c := CheckKnowledgeBase(dir); c.Status != "pass" {
		t.Errorf("existing dir: %q", c.Status)
	}
	if c := CheckKnowledgeBase(filepath.Join(dir, "nope")); c.Status != "fail" {
		t.Errorf("missing dir: %q", c.Status)
	}
}

func TestCheckFlywheelHealth(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if c := CheckFlywheelHealth(dir); c.Status != "warn" {
		t.Errorf("empty: %q", c.Status)
	}
	os.MkdirAll(filepath.Join(dir, "learnings"), 0o755)
	os.WriteFile(filepath.Join(dir, "learnings", "test.md"), []byte("x"), 0o644)
	if c := CheckFlywheelHealth(dir); c.Status != "pass" {
		t.Errorf("with learnings: %q", c.Status)
	}
}

func TestCheckSearchIndex(t *testing.T) {
	t.Parallel()
	if c := CheckSearchIndex("/nonexistent"); c.Status != "warn" {
		t.Errorf("missing: %q", c.Status)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "idx.jsonl")
	os.WriteFile(p, []byte(""), 0o644)
	if c := CheckSearchIndex(p); c.Status != "warn" {
		t.Errorf("empty: %q", c.Status)
	}
	os.WriteFile(p, []byte("term1\nterm2\n"), 0o644)
	if c := CheckSearchIndex(p); c.Status != "pass" {
		t.Errorf("with content: %q", c.Status)
	}
}
