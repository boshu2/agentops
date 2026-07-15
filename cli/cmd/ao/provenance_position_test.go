package main

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

func TestBuildPositionReportIsGenericLedgerTip(t *testing.T) {
	empty := buildPositionReport(nil)
	if empty.TotalEdges != 0 || empty.TipHash != "" || empty.Latest != nil {
		t.Fatalf("unexpected empty report: %+v", empty)
	}

	edges := []provenancegraph.Edge{
		{FromID: "intent-1", ToID: "artifact-1", Relation: "wasGeneratedBy", Hash: "aaa"},
		{FromID: "artifact-1", ToID: "learning-1", Relation: "wasInformedBy", Hash: "bbb"},
	}
	report := buildPositionReport(edges)
	if report.TotalEdges != 2 || report.TipHash != "bbb" || report.Latest == nil {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Latest.FromID != "artifact-1" || report.Latest.ToID != "learning-1" {
		t.Fatalf("unexpected latest edge: %+v", report.Latest)
	}
}
