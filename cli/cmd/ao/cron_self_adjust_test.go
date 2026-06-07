// practices: [dora-metrics, lean-startup]
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCronSelfAdjust_RoutesToMTO(t *testing.T) {
	out, err := executeCommand(
		"cron", "self-adjust",
		"--on", "cycle-close",
		"--template", ".agents/evolve/cron-template.md",
		"--shipped", "abc123:cp-x",
		"--next", "cp-y",
		"--sub-beads", "cp-a,cp-b",
		"--tests-delta", "+3 passing",
	)
	if err != nil {
		t.Fatalf("err: %v\nout=%s", err, out)
	}

	var notice cronSelfAdjustRelocationNotice
	if err := json.Unmarshal([]byte(out), &notice); err != nil {
		t.Fatalf("decode notice: %v\nout=%s", err, out)
	}
	if notice.Status != "relocated" {
		t.Fatalf("status = %q, want relocated", notice.Status)
	}
	if notice.Route != "mto-fleet" {
		t.Fatalf("route = %q, want mto-fleet", notice.Route)
	}
	if notice.Replacement == "" {
		t.Fatal("replacement should be populated")
	}
	if !strings.Contains(notice.Message, "MTO/factory boundary") {
		t.Fatalf("message did not explain route: %q", notice.Message)
	}
	if notice.Accepted["next"] != "cp-y" {
		t.Fatalf("accepted next = %q, want cp-y", notice.Accepted["next"])
	}
}

func TestCronSelfAdjust_RegisteredOnCron(t *testing.T) {
	var found bool
	for _, sub := range cronCmd.Commands() {
		if sub.Name() == "self-adjust" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("cron self-adjust should be registered on cronCmd")
	}
}

func TestCron_RegisteredOnRoot(t *testing.T) {
	var found bool
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "cron" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("cron should be registered on rootCmd")
	}
}
