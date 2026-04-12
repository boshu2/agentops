package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestKnowledgeActivateJSONRunsNativeActivationBuilders(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}

	repo := t.TempDir()
	writeKnowledgePacketBuilderFixtures(t, repo)

	origProjectDir := testProjectDir
	origActivateGoal := knowledgeActivateGoal
	origBriefGoal := knowledgeBriefGoal
	origIncludeThin := knowledgePlaybooksIncludeThin
	testProjectDir = repo
	defer func() {
		testProjectDir = origProjectDir
		knowledgeActivateGoal = origActivateGoal
		knowledgeBriefGoal = origBriefGoal
		knowledgePlaybooksIncludeThin = origIncludeThin
	}()

	out, err := executeCommand("knowledge", "activate", "--json", "--goal", "Fix auth startup")
	if err != nil {
		t.Fatalf("knowledge activate --json: %v\noutput: %s", err, out)
	}

	var result knowledgeActivateResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse knowledge activate json: %v\noutput: %s", err, out)
	}

	if len(result.Steps) != 7 {
		t.Fatalf("steps = %d, want 7", len(result.Steps))
	}
	if got := result.Steps[4].Implementation; got != knowledgeBuilderImplementationAONative {
		t.Fatalf("belief-book implementation = %q, want %q", got, knowledgeBuilderImplementationAONative)
	}
	if got := result.Steps[5].Implementation; got != knowledgeBuilderImplementationAONative {
		t.Fatalf("playbooks implementation = %q, want %q", got, knowledgeBuilderImplementationAONative)
	}
	if got := result.Steps[6].Implementation; got != knowledgeBuilderImplementationAONative {
		t.Fatalf("briefing implementation = %q, want %q", got, knowledgeBuilderImplementationAONative)
	}
	if result.BeliefBook == "" || !knowledgePathExists(result.BeliefBook) {
		t.Fatalf("belief book missing: %q", result.BeliefBook)
	}
	if result.PlaybooksIndex == "" || !knowledgePathExists(result.PlaybooksIndex) {
		t.Fatalf("playbooks index missing: %q", result.PlaybooksIndex)
	}
	if result.Briefing == "" || !knowledgePathExists(result.Briefing) {
		t.Fatalf("briefing missing: %q", result.Briefing)
	}
	if len(result.Gaps.ThinTopics) != 1 || result.Gaps.ThinTopics[0].ID != "thin-topic" {
		t.Fatalf("thin topics = %+v, want thin-topic only", result.Gaps.ThinTopics)
	}
	if len(result.Gaps.PromotionGaps) != 0 {
		t.Fatalf("promotion gaps = %+v, want none", result.Gaps.PromotionGaps)
	}
}

func TestKnowledgeBeliefsJSONUsesNativeBuilderWithoutScripts(t *testing.T) {
	repo := t.TempDir()
	writeKnowledgeCorpusFixtures(t, repo)

	origProjectDir := testProjectDir
	testProjectDir = repo
	defer func() { testProjectDir = origProjectDir }()

	out, err := executeCommand("knowledge", "beliefs", "--json")
	if err != nil {
		t.Fatalf("knowledge beliefs --json: %v\noutput: %s", err, out)
	}

	var result knowledgeBuilderResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse knowledge beliefs json: %v\noutput: %s", err, out)
	}
	if got := result.Step.Implementation; got != knowledgeBuilderImplementationAONative {
		t.Fatalf("implementation = %q, want %q", got, knowledgeBuilderImplementationAONative)
	}
	if result.OutputPath == "" || !knowledgePathExists(result.OutputPath) {
		t.Fatalf("belief book missing: %q", result.OutputPath)
	}

	data, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("read belief book: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "# Book Of Beliefs") {
		t.Fatal("expected belief book header")
	}
	if !strings.Contains(text, "`ao knowledge beliefs`") {
		t.Fatal("expected native refresh command in belief book")
	}
	if strings.Contains(text, ".agents/scripts") || strings.Contains(text, "python3") {
		t.Fatal("belief book should not reference workspace-local python builders")
	}
}

func TestKnowledgePlaybooksJSONUsesNativeBuilderWithoutScripts(t *testing.T) {
	repo := t.TempDir()
	writeKnowledgeCorpusFixtures(t, repo)

	origProjectDir := testProjectDir
	testProjectDir = repo
	defer func() { testProjectDir = origProjectDir }()

	out, err := executeCommand("knowledge", "playbooks", "--json")
	if err != nil {
		t.Fatalf("knowledge playbooks --json: %v\noutput: %s", err, out)
	}

	var result knowledgeBuilderResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse knowledge playbooks json: %v\noutput: %s", err, out)
	}
	if got := result.Step.Implementation; got != knowledgeBuilderImplementationAONative {
		t.Fatalf("implementation = %q, want %q", got, knowledgeBuilderImplementationAONative)
	}
	if result.OutputPath == "" || !knowledgePathExists(result.OutputPath) {
		t.Fatalf("playbooks index missing: %q", result.OutputPath)
	}
	if !knowledgePathExists(filepath.Join(repo, ".agents", "playbooks", "healthy-topic.md")) {
		t.Fatal("expected healthy-topic playbook to exist")
	}
	if knowledgePathExists(filepath.Join(repo, ".agents", "playbooks", "thin-topic.md")) {
		t.Fatal("did not expect thin-topic playbook without --include-thin")
	}
}

func TestKnowledgeBriefJSONUsesNativeBuilderWithoutScripts(t *testing.T) {
	repo := t.TempDir()
	writeKnowledgeCorpusFixtures(t, repo)

	origProjectDir := testProjectDir
	testProjectDir = repo
	defer func() { testProjectDir = origProjectDir }()

	out, err := executeCommand("knowledge", "brief", "--json", "--goal", "Healthy topic rollout")
	if err != nil {
		t.Fatalf("knowledge brief --json: %v\noutput: %s", err, out)
	}

	var result knowledgeBuilderResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse knowledge brief json: %v\noutput: %s", err, out)
	}
	if got := result.Step.Implementation; got != knowledgeBuilderImplementationAONative {
		t.Fatalf("implementation = %q, want %q", got, knowledgeBuilderImplementationAONative)
	}
	if result.OutputPath == "" || !knowledgePathExists(result.OutputPath) {
		t.Fatalf("briefing missing: %q", result.OutputPath)
	}

	data, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("read briefing: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "# Briefing: Healthy topic rollout") {
		t.Fatal("expected briefing header")
	}
	if !strings.Contains(text, "`ao knowledge brief --goal \"Healthy topic rollout\"`") {
		t.Fatal("expected native refresh command in briefing")
	}
	if strings.Contains(text, ".agents/scripts") || strings.Contains(text, "python3") {
		t.Fatal("briefing should not reference workspace-local python builders")
	}
}

func TestSelectKnowledgeBriefTopicsSuppressesThinCandidate(t *testing.T) {
	topics := []knowledgeTopicDetail{
		{TopicState: knowledgeTopicState{ID: "thin-topic", Title: "Thin Topic", Health: "thin"}},
		{TopicState: knowledgeTopicState{ID: "healthy-topic", Title: "Healthy Topic", Health: "healthy"}},
	}

	selected, suppressed := selectKnowledgeBriefTopics("thin topic rollout", topics, t.TempDir())
	if len(selected) != 1 || selected[0].ID != "healthy-topic" {
		t.Fatalf("selected = %+v, want healthy-topic only", selected)
	}
	if len(suppressed) != 1 || suppressed[0] != "thin-topic" {
		t.Fatalf("suppressed = %+v, want thin-topic", suppressed)
	}
}

func TestKnowledgeGapsJSONSurfacesThinTopicsAndPromotionGaps(t *testing.T) {
	repo := t.TempDir()
	writeKnowledgeCorpusFixtures(t, repo)
	if err := os.Remove(filepath.Join(repo, ".agents", "packets", "promoted", "healthy-topic.md")); err != nil {
		t.Fatalf("remove promoted packet: %v", err)
	}
	if err := os.Remove(filepath.Join(repo, ".agents", "playbooks", "index.md")); err == nil {
		t.Fatal("playbooks index should not exist before building playbooks")
	}

	origProjectDir := testProjectDir
	testProjectDir = repo
	defer func() { testProjectDir = origProjectDir }()

	out, err := executeCommand("knowledge", "gaps", "--json")
	if err != nil {
		t.Fatalf("knowledge gaps --json: %v\noutput: %s", err, out)
	}

	var result knowledgeGapSummary
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse knowledge gaps json: %v\noutput: %s", err, out)
	}

	if len(result.ThinTopics) != 1 || result.ThinTopics[0].ID != "thin-topic" {
		t.Fatalf("thin topics = %+v, want thin-topic only", result.ThinTopics)
	}
	if len(result.PromotionGaps) != 1 || result.PromotionGaps[0].ID != "healthy-topic" {
		t.Fatalf("promotion gaps = %+v, want healthy-topic only", result.PromotionGaps)
	}
	if got := strings.Join(result.PromotionGaps[0].Missing, ","); got != "promoted-packet,playbook" {
		t.Fatalf("promotion gap missing = %q, want promoted-packet,playbook", got)
	}
	if len(result.WeakClaims) != 1 || !strings.Contains(result.WeakClaims[0].Reason, "threshold") {
		t.Fatalf("weak claims = %+v, want threshold warning", result.WeakClaims)
	}
	if len(result.NextRecommendedWork) == 0 {
		t.Fatal("expected next recommended work suggestions")
	}
}

func writeKnowledgeCorpusFixtures(t *testing.T, repo string) {
	t.Helper()

	agentsRoot := filepath.Join(repo, ".agents")
	mkdirAll := func(rel string) string {
		path := filepath.Join(agentsRoot, rel)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		return path
	}

	topicsRoot := mkdirAll("topics")
	promotedRoot := mkdirAll(filepath.Join("packets", "promoted"))
	chunksRoot := mkdirAll(filepath.Join("packets", "chunks"))
	packetsRoot := mkdirAll("packets")

	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	write(filepath.Join(packetsRoot, "index.md"), "# Packet registry\n")
	write(filepath.Join(chunksRoot, "index.md"), "# Chunk index\n")
	write(filepath.Join(topicsRoot, "index.md"), "# Topic index\n")

	write(filepath.Join(topicsRoot, "healthy-topic.md"), `
---
topic_id: healthy-topic
title: Healthy Topic
health_state: healthy
aliases:
  - healthy rollout
  - stable activation
query_seeds:
  - healthy topic
  - stable activation
consumer_surfaces:
  - .agents/topics/index.md
evidence_counts:
  conversations: 4
  artifacts: 8
  verified_hits: 2
---

# Topic Packet: Healthy Topic

## Summary

Healthy Topic currently resolves to 4 primary conversation-backed session(s) and 8 linked artifact(s). The packet is stable enough to drive operator surfaces.

## Consumers

- .agents/topics/index.md
- discovery and research reference surface

## Key Decisions

- Keep activation surfaces deterministic across unchanged inputs.
- Prefer native Go writers for stable operator outputs.

## Repeated Patterns

- Healthy topics should generate canonical playbooks.
- Briefings should stay citation-backed and bounded.

## Open Gaps

- No open gaps recorded.
`)

	write(filepath.Join(topicsRoot, "thin-topic.md"), `
---
topic_id: thin-topic
title: Thin Topic
health_state: thin
aliases:
  - thin caution
query_seeds:
  - thin topic
consumer_surfaces:
  - .agents/topics/index.md
evidence_counts:
  conversations: 1
  artifacts: 3
  verified_hits: 1
---

# Topic Packet: Thin Topic

## Summary

Thin Topic is only partially grounded and should stay discovery-only until more evidence is gathered.

## Key Decisions

- Thin topics should never silently become canonical outputs.

## Repeated Patterns

- Thin topics need more evidence before promotion.

## Open Gaps

- Primary conversations below threshold (1/3).
`)

	write(filepath.Join(promotedRoot, "healthy-topic.md"), `
---
source_topic: healthy-topic
---

# Promoted Pattern Packet: Healthy Topic

## Primary Claims

- Activation surfaces should stay deterministic across unchanged inputs.
- Stable operator outputs belong in the ao binary, not workspace-local prototypes.
`)

	write(filepath.Join(chunksRoot, "healthy-topic.md"), `
---
topic_id: healthy-topic
title: Healthy Topic
promoted_packet_path: `+filepath.Join(promotedRoot, "healthy-topic.md")+`
---

# Historical Chunk Bundle: Healthy Topic

## Knowledge Chunks

### Healthy Topic Decision

- Chunk ID: healthy-topic-decision-01
- Type: decision
- Confidence: topic
- Claim: Activation surfaces should stay deterministic across unchanged inputs.

### Healthy Topic Pattern

- Chunk ID: healthy-topic-pattern-01
- Type: pattern
- Confidence: topic
- Claim: Stable operator outputs belong in the ao binary, not workspace-local prototypes.
`)
}

func writeKnowledgePacketBuilderFixtures(t *testing.T, repo string) {
	t.Helper()

	scriptsRoot := filepath.Join(repo, ".agents", "scripts")
	if err := os.MkdirAll(scriptsRoot, 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}

	fixtures := map[string]string{
		"source_manifest_build.py": `
from pathlib import Path
root = Path.cwd().parent
packets = root / "packets"
packets.mkdir(parents=True, exist_ok=True)
(packets / "index.md").write_text("# Packet registry\n", encoding="utf-8")
print("source_manifests=1")
`,
		"topic_packet_build.py": `
from pathlib import Path
root = Path.cwd().parent
topics = root / "topics"
topics.mkdir(parents=True, exist_ok=True)
(topics / "index.md").write_text("# Topic index\n", encoding="utf-8")
(topics / "healthy-topic.md").write_text("""---
topic_id: healthy-topic
title: Healthy Topic
health_state: healthy
aliases:
  - healthy rollout
  - stable activation
query_seeds:
  - healthy topic
  - stable activation
consumer_surfaces:
  - .agents/topics/index.md
evidence_counts:
  conversations: 4
  artifacts: 8
  verified_hits: 2
---

# Topic Packet: Healthy Topic

## Summary

Healthy Topic currently resolves to 4 primary conversation-backed session(s) and 8 linked artifact(s). The packet is stable enough to drive operator surfaces.

## Consumers

- .agents/topics/index.md
- discovery and research reference surface

## Key Decisions

- Keep activation surfaces deterministic across unchanged inputs.
- Prefer native Go writers for stable operator outputs.

## Repeated Patterns

- Healthy topics should generate canonical playbooks.
- Briefings should stay citation-backed and bounded.

## Open Gaps

- No open gaps recorded.
""", encoding="utf-8")
(topics / "thin-topic.md").write_text("""---
topic_id: thin-topic
title: Thin Topic
health_state: thin
aliases:
  - thin caution
query_seeds:
  - thin topic
consumer_surfaces:
  - .agents/topics/index.md
evidence_counts:
  conversations: 1
  artifacts: 3
  verified_hits: 1
---

# Topic Packet: Thin Topic

## Summary

Thin Topic is only partially grounded and should stay discovery-only until more evidence is gathered.

## Key Decisions

- Thin topics should never silently become canonical outputs.

## Repeated Patterns

- Thin topics need more evidence before promotion.

## Open Gaps

- Primary conversations below threshold (1/3).
""", encoding="utf-8")
print("topic_packets=2")
`,
		"corpus_packet_promote.py": `
from pathlib import Path
root = Path.cwd().parent
promoted = root / "packets" / "promoted"
promoted.mkdir(parents=True, exist_ok=True)
(promoted / "healthy-topic.md").write_text("""---
source_topic: healthy-topic
---

# Promoted Pattern Packet: Healthy Topic

## Primary Claims

- Activation surfaces should stay deterministic across unchanged inputs.
- Stable operator outputs belong in the ao binary, not workspace-local prototypes.
""", encoding="utf-8")
print("promoted_packets=1")
`,
		"knowledge_chunk_build.py": `
from pathlib import Path
root = Path.cwd().parent
chunks = root / "packets" / "chunks"
chunks.mkdir(parents=True, exist_ok=True)
(chunks / "index.md").write_text("# Chunk index\n", encoding="utf-8")
promoted = root / "packets" / "promoted" / "healthy-topic.md"
(chunks / "healthy-topic.md").write_text(f"""---
topic_id: healthy-topic
title: Healthy Topic
promoted_packet_path: {promoted.as_posix()}
---

# Historical Chunk Bundle: Healthy Topic

## Knowledge Chunks

### Healthy Topic Decision

- Chunk ID: healthy-topic-decision-01
- Type: decision
- Confidence: topic
- Claim: Activation surfaces should stay deterministic across unchanged inputs.

### Healthy Topic Pattern

- Chunk ID: healthy-topic-pattern-01
- Type: pattern
- Confidence: topic
- Claim: Stable operator outputs belong in the ao binary, not workspace-local prototypes.
""", encoding="utf-8")
print("chunk_bundles=1")
`,
	}

	for name, body := range fixtures {
		path := filepath.Join(scriptsRoot, name)
		content := "#!/usr/bin/env python3\n" + strings.TrimLeft(body, "\n")
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

// ---------------------------------------------------------------------------
// outputKnowledgeGapSummary
// ---------------------------------------------------------------------------

func TestOutputKnowledgeGapSummary_Human_Empty(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	summary := knowledgeGapSummary{
		Workspace: "/tmp/repo",
	}

	out, err := captureStdout(t, func() error {
		return outputKnowledgeGapSummary(summary)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Knowledge gaps for /tmp/repo") {
		t.Errorf("missing header, got: %q", out)
	}
	if !strings.Contains(out, "- None surfaced") {
		t.Errorf("missing 'None surfaced' for empty gaps, got: %q", out)
	}
	if !strings.Contains(out, "- No follow-up work surfaced") {
		t.Errorf("missing no follow-up work, got: %q", out)
	}
}

func TestOutputKnowledgeGapSummary_Human_Populated(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	summary := knowledgeGapSummary{
		Workspace: "/tmp/repo",
		ThinTopics: []knowledgeTopicGap{
			{Title: "Auth", Health: "thin", OpenGaps: []string{"missing evidence"}},
			{Title: "Logging", Health: "sparse"},
		},
		PromotionGaps: []knowledgePromotionGap{
			{Title: "Security pattern", Missing: []string{"proof", "citation"}},
		},
		WeakClaims: []knowledgeWeakClaim{
			{Title: "Perf claim", Reason: "no benchmark data"},
		},
		NextRecommendedWork: []string{"Run security audit", "Add benchmarks"},
	}

	out, err := captureStdout(t, func() error {
		return outputKnowledgeGapSummary(summary)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "- Auth: missing evidence") {
		t.Errorf("missing thin topic with open gap, got: %q", out)
	}
	if !strings.Contains(out, "- Logging: sparse") {
		t.Errorf("missing thin topic with health fallback, got: %q", out)
	}
	if !strings.Contains(out, "- Security pattern: missing proof, citation") {
		t.Errorf("missing promotion gap, got: %q", out)
	}
	if !strings.Contains(out, "- Perf claim: no benchmark data") {
		t.Errorf("missing weak claim, got: %q", out)
	}
	if !strings.Contains(out, "- Run security audit") {
		t.Errorf("missing recommended work, got: %q", out)
	}
}

func TestOutputKnowledgeGapSummary_JSON(t *testing.T) {
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	summary := knowledgeGapSummary{
		Workspace:  "/tmp/repo",
		ThinTopics: []knowledgeTopicGap{{Title: "Test"}},
	}

	out, err := captureStdout(t, func() error {
		return outputKnowledgeGapSummary(summary)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed knowledgeGapSummary
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out)
	}
	if parsed.Workspace != "/tmp/repo" {
		t.Errorf("Workspace = %q, want %q", parsed.Workspace, "/tmp/repo")
	}
	if len(parsed.ThinTopics) != 1 {
		t.Errorf("ThinTopics len = %d, want 1", len(parsed.ThinTopics))
	}
}

// ---------------------------------------------------------------------------
// outputKnowledgeActivateResult
// ---------------------------------------------------------------------------

func TestOutputKnowledgeActivateResult_Human(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	result := knowledgeActivateResult{
		Workspace:      "/tmp/repo",
		BeliefBook:     "/tmp/repo/.agents/beliefs.md",
		PlaybooksIndex: "/tmp/repo/.agents/playbooks/index.md",
		Briefing:       "/tmp/repo/.agents/briefing.md",
		Steps: []knowledgeBuilderRun{
			{knowledgeBuilderInvocation: knowledgeBuilderInvocation{Step: "consolidate"}, Path: "/tmp/repo/.agents/consolidated.md"},
			{knowledgeBuilderInvocation: knowledgeBuilderInvocation{Step: "promote"}},
		},
		Gaps: knowledgeGapSummary{
			ThinTopics:    []knowledgeTopicGap{{Title: "a"}, {Title: "b"}},
			PromotionGaps: []knowledgePromotionGap{{Title: "c"}},
		},
	}

	out, err := captureStdout(t, func() error {
		return outputKnowledgeActivateResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Knowledge activation target: /tmp/repo") {
		t.Errorf("missing header, got: %q", out)
	}
	if !strings.Contains(out, "- consolidate: consolidated.md") {
		t.Errorf("missing step with path, got: %q", out)
	}
	if !strings.Contains(out, "Belief book:") {
		t.Errorf("missing belief book, got: %q", out)
	}
	if !strings.Contains(out, "Playbooks index:") {
		t.Errorf("missing playbooks index, got: %q", out)
	}
	if !strings.Contains(out, "Briefing:") {
		t.Errorf("missing briefing, got: %q", out)
	}
	if !strings.Contains(out, "Thin topics: 2") {
		t.Errorf("missing thin topics count, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// outputKnowledgeBuilderResult
// ---------------------------------------------------------------------------

func TestOutputKnowledgeBuilderResult_Human(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	result := knowledgeBuilderResult{
		Workspace: "/tmp/repo",
		Step: knowledgeBuilderRun{
			knowledgeBuilderInvocation: knowledgeBuilderInvocation{
				Step:           "consolidate",
				Implementation: "python3",
			},
			Output: "Consolidated 5 artifacts",
		},
		OutputPath: "/tmp/repo/.agents/consolidated.md",
	}

	out, err := captureStdout(t, func() error {
		return outputKnowledgeBuilderResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Knowledge builder: consolidate") {
		t.Errorf("missing step, got: %q", out)
	}
	if !strings.Contains(out, "Workspace: /tmp/repo") {
		t.Errorf("missing workspace, got: %q", out)
	}
	if !strings.Contains(out, "Implementation: python3") {
		t.Errorf("missing implementation, got: %q", out)
	}
	if !strings.Contains(out, "Output: /tmp/repo/.agents/consolidated.md") {
		t.Errorf("missing output path, got: %q", out)
	}
	if !strings.Contains(out, "Consolidated 5 artifacts") {
		t.Errorf("missing builder output, got: %q", out)
	}
}

func TestOutputKnowledgeBuilderResult_JSON(t *testing.T) {
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	result := knowledgeBuilderResult{
		Workspace: "/tmp/repo",
		Step: knowledgeBuilderRun{
			knowledgeBuilderInvocation: knowledgeBuilderInvocation{Step: "test"},
		},
	}

	out, err := captureStdout(t, func() error {
		return outputKnowledgeBuilderResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed knowledgeBuilderResult
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Workspace != "/tmp/repo" {
		t.Errorf("Workspace = %q, want %q", parsed.Workspace, "/tmp/repo")
	}
}

// ---------------------------------------------------------------------------
// knowledgeBuilderDisplayTarget
// ---------------------------------------------------------------------------

func TestKnowledgeBuilderDisplayTarget(t *testing.T) {
	tests := []struct {
		name string
		step knowledgeBuilderRun
		want string
	}{
		{
			name: "path takes priority",
			step: knowledgeBuilderRun{
				knowledgeBuilderInvocation: knowledgeBuilderInvocation{Step: "consolidate", Script: "run.py", Implementation: "python3"},
				Path:                       "/tmp/output/result.md",
			},
			want: "result.md",
		},
		{
			name: "script fallback",
			step: knowledgeBuilderRun{
				knowledgeBuilderInvocation: knowledgeBuilderInvocation{Step: "consolidate", Script: "run.py", Implementation: "python3"},
			},
			want: "run.py",
		},
		{
			name: "implementation fallback",
			step: knowledgeBuilderRun{
				knowledgeBuilderInvocation: knowledgeBuilderInvocation{Step: "consolidate", Implementation: "python3"},
			},
			want: "python3",
		},
		{
			name: "step fallback",
			step: knowledgeBuilderRun{
				knowledgeBuilderInvocation: knowledgeBuilderInvocation{Step: "consolidate"},
			},
			want: "consolidate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := knowledgeBuilderDisplayTarget(tt.step)
			if got != tt.want {
				t.Errorf("knowledgeBuilderDisplayTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}
