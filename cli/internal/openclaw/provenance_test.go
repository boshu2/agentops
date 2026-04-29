package openclaw

import "testing"

func TestBuildProvenanceLinksIncludesEventJobRunAndArtifacts(t *testing.T) {
	links := BuildProvenanceLinks(ResourceSummary{
		JobID:       "job-rpi-1",
		RunID:       "run-rpi-1",
		LastEventID: "evt-rpi-1",
		Artifacts: map[string]string{
			"summary": ".agents/rpi/runs/run-rpi-1/summary.md",
			"remote":  "gascity://city/session/artifact",
		},
	})

	assertProvenanceLink(t, links, "source-event", "daemon-ledger-event", "evt-rpi-1", "", "", "")
	assertProvenanceLink(t, links, "source-job", "daemon-job", "", "job-rpi-1", "", "")
	assertProvenanceLink(t, links, "source-run", "daemon-run", "", "job-rpi-1", "run-rpi-1", "")
	assertProvenanceLink(t, links, "artifact", "artifact", "", "job-rpi-1", "run-rpi-1", "summary")
	assertProvenanceLink(t, links, "artifact", "artifact", "", "job-rpi-1", "run-rpi-1", "remote")
}

func TestWithProvenanceDedupesExistingLinks(t *testing.T) {
	resource := WithProvenance(ResourceSummary{
		JobID:       "job-1",
		LastEventID: "evt-1",
		Provenance: []ProvenanceLink{{
			Rel:     "source-event",
			Kind:    "daemon-ledger-event",
			EventID: "evt-1",
		}},
	})
	var eventLinks int
	for _, link := range resource.Provenance {
		if link.Rel == "source-event" && link.EventID == "evt-1" {
			eventLinks++
		}
	}
	if eventLinks != 1 {
		t.Fatalf("source-event links = %d, want 1: %#v", eventLinks, resource.Provenance)
	}
}

func assertProvenanceLink(t *testing.T, links []ProvenanceLink, rel, kind, eventID, jobID, runID, artifact string) {
	t.Helper()
	for _, link := range links {
		if link.Rel != rel || link.Kind != kind {
			continue
		}
		if eventID != "" && link.EventID != eventID {
			continue
		}
		if jobID != "" && link.JobID != jobID {
			continue
		}
		if runID != "" && link.RunID != runID {
			continue
		}
		if artifact != "" && link.Artifact != artifact {
			continue
		}
		return
	}
	t.Fatalf("missing provenance link rel=%s kind=%s event=%s job=%s run=%s artifact=%s in %#v", rel, kind, eventID, jobID, runID, artifact, links)
}
