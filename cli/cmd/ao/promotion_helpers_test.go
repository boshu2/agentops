package main

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveAutoPromoteThreshold_UsesConfigWhenFlagNotSet(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Setenv("HOME", tmp)
	t.Setenv("AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD", "36h")

	var threshold string
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringVar(&threshold, "threshold", defaultAutoPromoteThreshold, "")

	d, raw, err := resolveAutoPromoteThreshold(cmd, "threshold", threshold)
	if err != nil {
		t.Fatalf("resolve threshold: %v", err)
	}
	if raw != "36h" {
		t.Fatalf("expected raw threshold from config/env, got %q", raw)
	}
	if d.String() != "36h0m0s" {
		t.Fatalf("expected parsed 36h duration, got %s", d)
	}
}

func TestResolveAutoPromoteThreshold_FlagWinsOverConfig(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Setenv("HOME", tmp)
	t.Setenv("AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD", "36h")

	var threshold string
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringVar(&threshold, "threshold", defaultAutoPromoteThreshold, "")
	if err := cmd.Flags().Set("threshold", "12h"); err != nil {
		t.Fatalf("set threshold flag: %v", err)
	}

	d, raw, err := resolveAutoPromoteThreshold(cmd, "threshold", threshold)
	if err != nil {
		t.Fatalf("resolve threshold: %v", err)
	}
	if raw != "12h" {
		t.Fatalf("expected raw threshold from explicit flag, got %q", raw)
	}
	if d.String() != "12h0m0s" {
		t.Fatalf("expected parsed 12h duration, got %s", d)
	}
}

func TestResolveAutoPromoteThreshold_InvalidConfigValue(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Setenv("HOME", tmp)
	t.Setenv("AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD", "not-a-duration")

	var threshold string
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringVar(&threshold, "threshold", defaultAutoPromoteThreshold, "")

	if _, _, err := resolveAutoPromoteThreshold(cmd, "threshold", threshold); err == nil {
		t.Fatal("expected invalid config duration to return error")
	}
}
