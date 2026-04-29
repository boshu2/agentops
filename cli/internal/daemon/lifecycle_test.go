package daemon

import "testing"

func TestLifecycleDryRunPlan(t *testing.T) {
	plan := BuildServiceInstallPlan("/repo", "/usr/local/bin/ao", "127.0.0.1:9876", true)
	if plan.ServiceName != "agentopsd" {
		t.Fatalf("ServiceName = %q, want agentopsd", plan.ServiceName)
	}
	if !plan.DryRun {
		t.Fatal("DryRun = false, want true")
	}
	if plan.Executable != "/usr/local/bin/ao" || plan.Address != "127.0.0.1:9876" {
		t.Fatalf("plan executable/address = %#v", plan)
	}
	if len(plan.Args) == 0 || plan.Args[0] != "daemon" {
		t.Fatalf("plan args = %#v, want daemon run args", plan.Args)
	}
	if plan.UnitPath == "" {
		t.Fatalf("UnitPath empty in plan: %#v", plan)
	}
}
