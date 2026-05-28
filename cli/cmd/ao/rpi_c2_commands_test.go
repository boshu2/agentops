// practices: [agile-manifesto, dora-metrics]
package main

import "testing"

func TestRPIC2CommandAppendAndLoad(t *testing.T) {
	root := t.TempDir()
	runID := "run-cmd-001"

	record, err := appendRPIC2Command(root, rpiC2CommandInput{
		RunID:   runID,
		Phase:   1,
		Kind:    "nudge",
		Targets: []string{"w1", "w2", "w1"},
		Message: "change direction",
		Metadata: map[string]any{
			"source": "test",
		},
	})
	if err != nil {
		t.Fatalf("appendRPIC2Command: %v", err)
	}
	if record.CommandID == "" {
		t.Fatal("expected command_id")
	}
	if len(record.Targets) != 2 {
		t.Fatalf("targets len = %d, want 2", len(record.Targets))
	}

	records, err := loadRPIC2Commands(root, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Commands: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0].Kind != "nudge" {
		t.Fatalf("kind = %q", records[0].Kind)
	}
}

func TestRPIC2Command_RequiresFields(t *testing.T) {
	root := t.TempDir()
	if _, err := appendRPIC2Command(root, rpiC2CommandInput{Kind: "nudge", Targets: []string{"a"}}); err == nil {
		t.Fatal("expected missing run_id error")
	}
	if _, err := appendRPIC2Command(root, rpiC2CommandInput{RunID: "run", Targets: []string{"a"}}); err == nil {
		t.Fatal("expected missing kind error")
	}
	if _, err := appendRPIC2Command(root, rpiC2CommandInput{RunID: "run", Kind: "nudge"}); err == nil {
		t.Fatal("expected missing targets error")
	}
}
