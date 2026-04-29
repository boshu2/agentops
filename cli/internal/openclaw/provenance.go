package openclaw

import "strings"

func WithProvenance(resource ResourceSummary) ResourceSummary {
	resource.Provenance = BuildProvenanceLinks(resource)
	return resource
}

func BuildProvenanceLinks(resource ResourceSummary) []ProvenanceLink {
	links := append([]ProvenanceLink{}, resource.Provenance...)
	if strings.TrimSpace(resource.LastEventID) != "" {
		links = append(links, ProvenanceLink{
			Rel:     "source-event",
			Kind:    "daemon-ledger-event",
			EventID: resource.LastEventID,
		})
	}
	if strings.TrimSpace(resource.JobID) != "" {
		links = append(links, ProvenanceLink{
			Rel:   "source-job",
			Kind:  "daemon-job",
			JobID: resource.JobID,
		})
	}
	if strings.TrimSpace(resource.RunID) != "" {
		links = append(links, ProvenanceLink{
			Rel:   "source-run",
			Kind:  "daemon-run",
			RunID: resource.RunID,
			JobID: resource.JobID,
		})
	}
	for name, target := range resource.Artifacts {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		link := ProvenanceLink{
			Rel:      "artifact",
			Kind:     "artifact",
			Artifact: name,
			JobID:    resource.JobID,
			RunID:    resource.RunID,
		}
		if strings.Contains(target, "://") {
			link.URI = target
		} else {
			link.Path = target
		}
		links = append(links, link)
	}
	return dedupeProvenanceLinks(links)
}

func dedupeProvenanceLinks(links []ProvenanceLink) []ProvenanceLink {
	out := make([]ProvenanceLink, 0, len(links))
	seen := map[ProvenanceLink]struct{}{}
	for _, link := range links {
		if _, ok := seen[link]; ok {
			continue
		}
		seen[link] = struct{}{}
		out = append(out, link)
	}
	if out == nil {
		return []ProvenanceLink{}
	}
	return out
}
