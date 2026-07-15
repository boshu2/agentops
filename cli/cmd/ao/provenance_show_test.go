package main

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

func TestBuildShowReportShowsGenericInboundAndOutboundEdges(t *testing.T) {
	edges := []provenancegraph.Edge{
		{FromID: "intent-1", FromType: "decision", ToID: "artifact-1", ToType: "artifact", Relation: "wasGeneratedBy", TrustTier: "authored", Hash: "a"},
		{FromID: "artifact-1", FromType: "artifact", ToID: "learning-1", ToType: "learning", Relation: "wasInformedBy", TrustTier: "mined", Hash: "b"},
	}
	report, err := buildShowReport(edges, "artifact-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Relationships) != 2 {
		t.Fatalf("relationships = %d, want 2", len(report.Relationships))
	}
	if report.Relationships[0].Direction != "inbound" || report.Relationships[1].Direction != "outbound" {
		t.Fatalf("unexpected directions: %+v", report.Relationships)
	}
}

func TestBuildShowReportRejectsMissingNode(t *testing.T) {
	if _, err := buildShowReport(nil, "missing"); err == nil {
		t.Fatal("expected missing node error")
	}
}
