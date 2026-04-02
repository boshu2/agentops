package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestKnowledgeActivateJSONRunsWorkspaceBuilders(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}

	repo := t.TempDir()
	writeKnowledgeBuilderFixtures(t, repo)

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

func TestKnowledgeGapsJSONSurfacesThinTopicsAndPromotionGaps(t *testing.T) {
	repo := t.TempDir()
	agentsRoot := filepath.Join(repo, ".agents")
	topicsRoot := filepath.Join(agentsRoot, "topics")
	if err := os.MkdirAll(topicsRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	healthyTopic := `---
topic_id: healthy-topic
title: Healthy Topic
health_state: healthy
---

# Topic Packet: Healthy Topic

## Open Gaps

- No open gaps recorded.
`
	if err := os.WriteFile(filepath.Join(topicsRoot, "healthy-topic.md"), []byte(healthyTopic), 0o644); err != nil {
		t.Fatal(err)
	}
	thinTopic := `---
topic_id: thin-topic
title: Thin Topic
health_state: thin
---

# Topic Packet: Thin Topic

## Open Gaps

- Primary conversations below threshold (1/3).
`
	if err := os.WriteFile(filepath.Join(topicsRoot, "thin-topic.md"), []byte(thinTopic), 0o644); err != nil {
		t.Fatal(err)
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

func writeKnowledgeBuilderFixtures(t *testing.T, repo string) {
	t.Helper()

	scriptsRoot := filepath.Join(repo, ".agents", "scripts")
	if err := os.MkdirAll(scriptsRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	fixtures := map[string]string{
		"source_manifest_build.py": `
from pathlib import Path
root = Path.cwd().parent
out = root / "packets" / "source-manifests"
out.mkdir(parents=True, exist_ok=True)
(out / "index.md").write_text("# Source manifests\n", encoding="utf-8")
print("source_manifests=2")
`,
		"topic_packet_build.py": `
from pathlib import Path
root = Path.cwd().parent
topics = root / "topics"
topics.mkdir(parents=True, exist_ok=True)
(topics / "healthy-topic.md").write_text("""---
topic_id: healthy-topic
title: Healthy Topic
health_state: healthy
---

# Topic Packet: Healthy Topic

## Open Gaps

- No open gaps recorded.
""", encoding="utf-8")
(topics / "thin-topic.md").write_text("""---
topic_id: thin-topic
title: Thin Topic
health_state: thin
---

# Topic Packet: Thin Topic

## Open Gaps

- Primary conversations below threshold (1/3).
""", encoding="utf-8")
(topics / "index.md").write_text("# Topic index\n", encoding="utf-8")
print("healthy-topic: health=healthy conversations=4 artifacts=8 verified=2")
print("thin-topic: health=thin conversations=1 artifacts=3 verified=1")
`,
		"corpus_packet_promote.py": `
from pathlib import Path
root = Path.cwd().parent
promoted = root / "packets" / "promoted"
promoted.mkdir(parents=True, exist_ok=True)
(promoted / "healthy-topic.md").write_text("# Promoted packet\n", encoding="utf-8")
(promoted / "index.md").write_text("# Promoted index\n", encoding="utf-8")
(root / "packets").mkdir(parents=True, exist_ok=True)
((root / "packets") / "index.md").write_text("# Packet registry\n", encoding="utf-8")
print("promoted_packets=1")
`,
		"knowledge_chunk_build.py": `
from pathlib import Path
root = Path.cwd().parent
chunks = root / "packets" / "chunks"
chunks.mkdir(parents=True, exist_ok=True)
(chunks / "healthy-topic.md").write_text("# Chunk bundle\n", encoding="utf-8")
(chunks / "catalog.jsonl").write_text("{\"chunk_id\":\"healthy-topic-overview-01\"}\n", encoding="utf-8")
(chunks / "index.md").write_text("# Chunks index\n", encoding="utf-8")
print("chunk_bundles=1")
print("chunk_records=1")
`,
		"book_of_beliefs_build.py": `
from pathlib import Path
root = Path.cwd().parent
out = root / "knowledge"
out.mkdir(parents=True, exist_ok=True)
path = out / "book-of-beliefs.md"
path.write_text("# Book Of Beliefs\n", encoding="utf-8")
print(f"belief_book={path.as_posix()}")
`,
		"playbook_build.py": `
from pathlib import Path
root = Path.cwd().parent
out = root / "playbooks"
out.mkdir(parents=True, exist_ok=True)
(out / "healthy-topic.md").write_text("# Playbook Candidate\n", encoding="utf-8")
(out / "index.md").write_text("# Playbook Candidates\n", encoding="utf-8")
print("playbooks=1")
`,
		"briefing_build.py": `
from pathlib import Path
import re
import sys
root = Path.cwd().parent
goal = "unknown-goal"
if "--goal" in sys.argv:
    goal = sys.argv[sys.argv.index("--goal") + 1]
slug = re.sub(r"[^a-z0-9]+", "-", goal.lower()).strip("-") or "briefing"
out = root / "briefings"
out.mkdir(parents=True, exist_ok=True)
path = out / f"2026-04-01-{slug}.md"
path.write_text(f"# Briefing: {goal}\n", encoding="utf-8")
print(f"briefing={path.as_posix()}")
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
