package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Given an installed binary and an empty home/consumer checkout, the default
// front doors need neither skills nor setup and leave both directories empty.
func TestNativeOnboardingWithoutSkillsOrSetupWrites(t *testing.T) {
	bin := aoBinary(t)
	for _, args := range [][]string{{"--help"}, {"quick-start"}, {"quick-start", "--help"}, {"demo"}, {"demo", "--quick"}, {"demo", "--concepts"}, {"demo", "--help"}, {"init", "--help"}, {"robot-docs"}, {"robot-docs", "--help"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			home, consumer := t.TempDir(), t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, args...)
			cmd.Dir = consumer
			cmd.Env = []string{"HOME=" + home, "PATH=" + home, "CODEX_HOME=" + home}
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("native command: %v\n%s", err, output)
			}
			text := string(output)
			for _, want := range []string{"native", "optional"} {
				if !strings.Contains(strings.ToLower(text), want) {
					t.Errorf("native entry missing %q:\n%s", want, text)
				}
			}
			for _, forbidden := range []string{"invoke the Validate skill", "Run the RPI skill", "run this first", "RPI -> Plan -> Implement", "Semantic judgment belongs to the Validate skill"} {
				if strings.Contains(text, forbidden) {
					t.Errorf("default entry requires workflow %q:\n%s", forbidden, text)
				}
			}
			for _, dir := range []string{home, consumer} {
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 0 {
					t.Fatalf("native entry changed %s: %v (err %v)", dir, entries, err)
				}
			}
		})
	}
}

func TestExplicitRPIDemoRemainsAvailable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, aoBinary(t), "demo", "--rpi").CombinedOutput()
	if err != nil || !strings.Contains(string(output), "AGENTOPS RPI DEMO") {
		t.Fatalf("explicit full workflow unavailable: %v\n%s", err, output)
	}
}
