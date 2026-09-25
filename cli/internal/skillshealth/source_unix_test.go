//go:build darwin || linux

package skillshealth

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestSourceRejectsFIFOWithoutReading(t *testing.T) {
	if root := os.Getenv("AO_SOURCE_FIFO_ROOT"); root != "" {
		findings, err := CheckSource(root, []string{"skills/sample"}, false)
		if err != nil || len(findings) != 1 || findings[0].Code != "MISSING_SKILL" {
			t.Fatalf("FIFO must be rejected before reading: findings=%v error=%v", findings, err)
		}
		return
	}
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "skills", "sample")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(target, "SKILL.md")
	if err := syscall.Mkfifo(skill, 0600); err != nil {
		t.Fatal(err)
	}
	// A separate bounded process detects a blocked FIFO open without hanging the suite.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSourceRejectsFIFOWithoutReading$")
	cmd.Env = append(os.Environ(), "AO_SOURCE_FIFO_ROOT="+root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bounded FIFO reader: %v; output: %s", err, output)
	}
	info, err := os.Lstat(skill)
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("FIFO changed: %v %v", info, err)
	}
}
