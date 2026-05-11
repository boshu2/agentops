// practices: [agile-manifesto, dora-metrics]
package main

import (
	"fmt"
	"testing"
	"time"
)

// Tests for pure helper functions in rpi_loop_supervisor.go

func TestWrapSyncPushLandingFailure_NilError(t *testing.T) {
	got := wrapSyncPushLandingFailure(t.TempDir(), time.Second, "commit", nil)
	if got != nil {
		t.Errorf("expected nil error for nil input, got: %v", got)
	}
}

func TestWrapSyncPushLandingFailure_WrapsError(t *testing.T) {
	err := fmt.Errorf("push failed")
	dir := t.TempDir() // no git repo so rebase --abort won't exist
	got := wrapSyncPushLandingFailure(dir, time.Second, "push", err)
	if got == nil {
		t.Fatal("expected error, got nil")
	}
	msg := got.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestShouldRunBDSync_NeverPolicy(t *testing.T) {
	dir := t.TempDir()
	got, err := shouldRunBDSync(dir, loopBDSyncPolicyNever, "bd")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got {
		t.Error("expected false for 'never' policy")
	}
}

func TestShouldRunBDSync_UnsupportedPolicy(t *testing.T) {
	dir := t.TempDir()
	_, err := shouldRunBDSync(dir, "unsupported-policy", "bd")
	if err == nil {
		t.Error("expected error for unsupported policy")
	}
}

func TestNormalizeSearchRootPath(t *testing.T) {
	t.Run("existing directory normalized", func(t *testing.T) {
		dir := t.TempDir()
		got := normalizeSearchRootPath(dir)
		if got == "" {
			t.Error("expected non-empty normalized path")
		}
	})

	t.Run("non-existent path returns clean version", func(t *testing.T) {
		got := normalizeSearchRootPath("/tmp/definitely-does-not-exist-xyz")
		if got == "" {
			t.Error("expected non-empty result for non-existent path")
		}
	})
}
