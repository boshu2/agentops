package okfprofile

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// All examples are synthetic; no private source text belongs in this corpus.
const observation = `---
type: Observation
title: Synthetic timeout observation
description: One synthetic request succeeded after a bounded retry.
status: draft
knowledge_use: promotion-candidate
sources:
  - id: episode
    resource: references/synthetic-episode.md
generated: {by: example/1, at: 2026-09-08T12:00:00Z}
---

## Applicability
The synthetic one-request experiment only.
## Claim
The second request succeeded.[^episode]
## Limitations
One observation does not establish a general retry policy.
## Consumer
The caller reviewing the synthetic request experiment.
## Retirement condition
Review when the request behavior changes; retire if the source is withdrawn.

[^episode]: Synthetic episode.
`

func TestProfileExamples(t *testing.T) {
	for _, tc := range []struct {
		name, before, after, issue string
	}{
		{name: "narrow observation"},
		{"maintained reference", "knowledge_use: promotion-candidate", "knowledge_use: maintained-reference", ""},
		{"unknown concept type", "type: Observation", "type: Locally defined kind", ""},
		{"bare verifier", "status: draft", "status: stable\nverified: {by: human:example, at: 2026-09-08T13:00:00-04:00}", ""},
		{"verifier list", "status: draft", "status: deprecated\nverified:\n  - {by: process:example, at: 2026-09-08T13:00:00Z}\n  - {by: human:example, at: 2026-09-08T14:00:00Z}", ""},
		{"generated time optional", ", at: 2026-09-08T12:00:00Z", "", ""},
		{"source scope descriptor", "references/synthetic-episode.md", "all synthetic requests in the declared experiment", ""},
		{"unknown extensions", "status: draft", "status: draft\nowner_label: illustrative-only\ncustom: {nested: [one, two]}", ""},
		{"no implicit stable", "status: draft\n", "", "status"},
		{"empty status", "status: draft", "status:", "status"},
		{"unsupported status", "status: draft", "status: approved", "status"},
		{"missing sources", "sources:\n  - id: episode\n    resource: references/synthetic-episode.md\n", "", "sources"},
		{"empty sources", "sources:\n  - id: episode\n    resource: references/synthetic-episode.md", "sources: []", "sources"},
		{"source string is not entry", "  - id: episode\n    resource: references/synthetic-episode.md", "  - references/synthetic-episode.md", "sources[0]"},
		{"missing source resource", "    resource: references/synthetic-episode.md\n", "", "sources[0].resource"},
		{"missing applicability", "## Applicability\nThe synthetic one-request experiment only.\n", "", "applicability"},
		{"empty claim", "The second request succeeded.[^episode]", "", "claim/action"},
		{"missing limitations", "## Limitations\nOne observation does not establish a general retry policy.\n", "", "limitations"},
		{"missing consumer", "## Consumer\nThe caller reviewing the synthetic request experiment.\n", "", "consumer"},
		{"missing retirement", "## Retirement condition\nReview when the request behavior changes; retire if the source is withdrawn.\n", "", "retirement_condition"},
		{"missing use", "knowledge_use: promotion-candidate\n", "", "knowledge_use"},
		{"unknown use", "knowledge_use: promotion-candidate", "knowledge_use: universal-rule", "knowledge_use"},
		{"unknown page profile", "status: draft", "status: draft\nagentops_profile: unknown/99", "agentops_profile"},
		{"incompatible version", "status: draft", "status: draft\nokf_version: '0.1'", "okf_version"},
		{"malformed YAML", "title: Synthetic timeout observation", "title: [secret-unclosed", "frontmatter"},
		{"duplicate status", "status: draft", "status: draft\nstatus: stable", "frontmatter"},
		{"duplicate nested field", "    resource: references/synthetic-episode.md", "    resource: references/synthetic-episode.md\n    resource: something", "frontmatter"},
		{"no YAML alias inference", "status: draft", "status: &s draft\ncopy: *s", "frontmatter"},
		{"string type enforced", "title: Synthetic timeout observation", "title: 42", "title"},
		{"generated actor required", "generated: {by: example/1, at: 2026-09-08T12:00:00Z}", "generated: {at: 2026-09-08T12:00:00Z}", "generated.by"},
		{"invalid actor", "by: example/1", "by: human:", "generated.by"},
		{"missing verifier timestamp", "status: draft", "status: draft\nverified: {by: human:example}", "verified[0].at"},
		{"no timezone", "2026-09-08T12:00:00Z", "2026-09-08T12:00:00", "generated.at"},
		{"fenced headings are examples", "## Applicability\nThe synthetic one-request experiment only.", "~~~md\n## Applicability\nThe synthetic one-request experiment only.\n~~~", "applicability"},
		{"commented headings are absent", "## Applicability\nThe synthetic one-request experiment only.", "<!--\n## Applicability\nThe synthetic one-request experiment only.\n-->", "applicability"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := []byte(strings.Replace(observation, tc.before, tc.after, 1))
			result, err := Check(payload, Profile)
			if err != nil {
				t.Fatal(err)
			}
			if result.StructurallyValid != (tc.issue == "") {
				t.Fatalf("valid=%v, issues=%+v", result.StructurallyValid, result.Issues)
			}
			if tc.issue != "" {
				found := false
				for _, issue := range result.Issues {
					found = found || issue.Field == tc.issue
				}
				if !found {
					t.Fatalf("missing %s: %+v", tc.issue, result.Issues)
				}
			}
			if result.ContentSHA256 != fmt.Sprintf("%x", sha256.Sum256(payload)) {
				t.Fatal("receipt is not bound to exact original bytes")
			}
		})
	}
}

func TestProfileRequiredStringsAndMetadataSections(t *testing.T) {
	for _, key := range []string{"type", "title", "description"} {
		lines := strings.Split(observation, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, key+":") {
				lines[i] = key + ": ''"
			}
		}
		result, err := Check([]byte(strings.Join(lines, "\n")), Profile)
		if err != nil || result.StructurallyValid {
			t.Fatalf("empty %s accepted: %v", key, err)
		}
	}
	page := strings.Split(observation, "\n---\n")[0] + `
applicability: The one declared experiment.
action: Read the referenced observation.
limitations: No demonstrated general benefit.
consumer: The caller of this experiment.
retirement_condition: Retire when withdrawn.
---
`
	result, err := Check([]byte(page), Profile)
	if err != nil || !result.StructurallyValid {
		t.Fatalf("metadata sections: %+v %v", result, err)
	}
}

func TestProfileBoundsAndVersion(t *testing.T) {
	if _, err := Check([]byte(observation), "unknown/1"); err == nil {
		t.Fatal("unknown profile accepted")
	}
	for _, payload := range [][]byte{
		[]byte(strings.Repeat("x", MaxPageBytes+1)),
		[]byte("---\ncustom: " + strings.Repeat("x", MaxFrontmatterBytes) + "\n---\n"),
		[]byte("---\ntype: [\xff]\n---\n"),
		[]byte("---\ntype: Observation\n"),
		[]byte("not frontmatter\n"),
	} {
		result, err := Check(payload, Profile)
		if err != nil || result.StructurallyValid {
			t.Fatalf("invalid bounded input accepted: %+v %v", result, err)
		}
	}
}
