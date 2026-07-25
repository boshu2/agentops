//go:build windows

package subprocess

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestTaskkillTargetAbsentClassification(t *testing.T) {
	absentErr := windowsExitError(t, 128)
	if !taskkillTargetAbsent(absentErr) {
		t.Fatalf("taskkillTargetAbsent(%v) = false, want true", absentErr)
	}
	if !taskkillTargetAbsent(fmt.Errorf("wrapped: %w", absentErr)) {
		t.Fatal("wrapped taskkill exit 128 was not classified as target absent")
	}
	if taskkillTargetAbsent(windowsExitError(t, 1)) {
		t.Fatal("taskkill exit 1 was classified as target absent")
	}
}

func windowsExitError(t *testing.T, code int) error {
	t.Helper()
	err := exec.Command("cmd", "/C", fmt.Sprintf("exit /b %d", code)).Run()
	if err == nil {
		t.Fatalf("cmd exit %d returned nil", code)
	}
	return err
}
