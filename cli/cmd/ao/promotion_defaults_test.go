// practices: [continuous-delivery, dora-metrics]
package main

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestPromotionDefaults_AutoPromoteThresholdParseable(t *testing.T) {
	// Verify the default constant is a valid Go duration string.
	d, err := time.ParseDuration(defaultAutoPromoteThreshold)
	if err != nil {
		t.Fatalf("defaultAutoPromoteThreshold %q is not a valid duration: %v", defaultAutoPromoteThreshold, err)
	}
	if d <= 0 {
		t.Errorf("defaultAutoPromoteThreshold duration = %v, want positive", d)
	}
}

func TestPromotionDefaults_AutoPromoteThresholdIs24h(t *testing.T) {
	if defaultAutoPromoteThreshold != "24h" {
		t.Errorf("defaultAutoPromoteThreshold = %q, want %q", defaultAutoPromoteThreshold, "24h")
	}
}

func TestPromotionDefaults_ParsesTo24Hours(t *testing.T) {
	d, err := time.ParseDuration(defaultAutoPromoteThreshold)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if d != 24*time.Hour {
		t.Errorf("defaultAutoPromoteThreshold parses to %v, want %v", d, 24*time.Hour)
	}
}

func TestPromotionDefaults_FlywheelCloseLoopFlagDefault(t *testing.T) {
	// Verify the flywheel close-loop command's --threshold flag uses the shared constant.
	flag := flywheelCloseLoopCmd.Flags().Lookup("threshold")
	if flag == nil {
		t.Fatal("flywheelCloseLoopCmd should have a --threshold flag")
	}
	if flag.DefValue != defaultAutoPromoteThreshold {
		t.Errorf("flywheelCloseLoopCmd --threshold default = %q, want %q", flag.DefValue, defaultAutoPromoteThreshold)
	}
}

func TestPromotionDefaults_PoolAutoPromoteFlagDefault(t *testing.T) {
	// Verify the pool auto-promote command's --threshold flag uses the shared constant.
	flag := poolAutoPromoteCmd.Flags().Lookup("threshold")
	if flag == nil {
		t.Fatal("poolAutoPromoteCmd should have a --threshold flag")
	}
	if flag.DefValue != defaultAutoPromoteThreshold {
		t.Errorf("poolAutoPromoteCmd --threshold default = %q, want %q", flag.DefValue, defaultAutoPromoteThreshold)
	}
}

func TestPromotionDefaults_FlagDefaultIsParseable(t *testing.T) {
	// Simulate what resolveAutoPromoteThreshold does with the default value:
	// create a command with a flag using the default, don't change it, and verify
	// it parses correctly.
	var threshold string
	cmd := &cobra.Command{Use: "test-defaults"}
	cmd.Flags().StringVar(&threshold, "threshold", defaultAutoPromoteThreshold, "")

	// Flag not changed — threshold should still be the default and parseable.
	d, err := time.ParseDuration(threshold)
	if err != nil {
		t.Fatalf("default threshold %q should be parseable: %v", threshold, err)
	}
	if d != 24*time.Hour {
		t.Errorf("default threshold parsed to %v, want 24h", d)
	}
}

func TestPromotionDefaults_ConsistentAcrossEntrypoints(t *testing.T) {
	// Both commands that use defaultAutoPromoteThreshold should have the same
	// flag default, ensuring the shared constant achieves its stated purpose.
	fwFlag := flywheelCloseLoopCmd.Flags().Lookup("threshold")
	poolFlag := poolAutoPromoteCmd.Flags().Lookup("threshold")

	if fwFlag == nil {
		t.Fatal("flywheelCloseLoopCmd missing --threshold flag")
	}
	if poolFlag == nil {
		t.Fatal("poolAutoPromoteCmd missing --threshold flag")
	}
	if fwFlag.DefValue != poolFlag.DefValue {
		t.Errorf("threshold defaults diverge: flywheel=%q pool=%q", fwFlag.DefValue, poolFlag.DefValue)
	}
}
