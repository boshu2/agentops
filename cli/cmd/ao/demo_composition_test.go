package main

import (
	"bytes"
	"testing"
)

// TestAdvertisedCommandsResolveInDemoOutput runs every demo mode (a pure print
// command) and verifies that each "ao ..." string in its printed output
// resolves against the live command tree, so the demo never teaches a retired
// or unknown command.
func TestAdvertisedCommandsResolveInDemoOutput(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "quick", args: nil},
		{name: "concepts", args: []string{"--concepts"}},
		{name: "rpi", args: []string{"--rpi"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newDemoCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tt.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("demo %v failed: %v", tt.args, err)
			}
			if out.Len() == 0 {
				t.Fatalf("demo %v printed no output", tt.args)
			}
			extractAndCheck(t, "demo "+tt.name, "output", out.String(), true)
		})
	}
}
