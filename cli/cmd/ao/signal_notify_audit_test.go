// practices: [distributed-tracing, sre]
package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSignalNotifySitesPairWithSignalStop(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	baseDir := filepath.Dir(thisFile)

	targets := []string{
		filepath.Join(baseDir, "rpi_phased_setup.go"),
		filepath.Join(baseDir, "rpi_status.go"),
		filepath.Join(baseDir, "..", "..", "internal", "goals", "measure.go"),
	}

	for _, target := range targets {
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read %s: %v", target, err)
		}
		source := string(data)
		if strings.Contains(source, "signal.Notify(") && !strings.Contains(source, "signal.Stop(") {
			t.Fatalf("signal.Notify without signal.Stop in %s", target)
		}
	}
}
