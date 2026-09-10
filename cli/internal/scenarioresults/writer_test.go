package scenarioresults

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriter_RejectsInvalidJudgedAtWithoutWriting(t *testing.T) {
	for _, source := range []string{"incoming", "existing"} {
		t.Run(source, func(t *testing.T) {
			root := t.TempDir()
			stageFixture(t, root, "has-failing.json")
			loaded, err := Load(root, true)
			if err != nil {
				t.Fatal(err)
			}
			invalid := loaded.Artifact.Results[1]
			invalid.JudgedAt, invalid.Verdict, invalid.Score = "not-a-time", VerdictPass, 1
			incoming := []ScenarioResult{invalid}
			if source == "existing" {
				loaded.Artifact.Results = append(loaded.Artifact.Results, invalid)
				payload, err := json.Marshal(loaded.Artifact)
				if err != nil {
					t.Fatal(err)
				}
				writeArtifactBytes(t, root, payload)
				incoming = nil
			}
			path := filepath.Join(root, ArtifactRelPath)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := (Writer{}).Append(root, "next-run", 1, incoming); err == nil {
				t.Error("writer accepted invalid judged_at")
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Fatalf("rejection changed the existing artifact: %v", err)
			}
			if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
				t.Fatalf("rejection left a temporary artifact: %v", err)
			}
		})
	}
}

func TestWriter_ValidJudgedAtOrdersOffsetsAndFractions(t *testing.T) {
	root := t.TempDir()
	stageFixture(t, root, "has-failing.json")
	loaded, err := Load(root, true)
	if err != nil {
		t.Fatal(err)
	}
	failure := loaded.Artifact.Results[1]
	failure.JudgedAt = "2026-09-09T12:00:00.5Z"
	if _, err := (Writer{}).Append(root, "run", 1, []ScenarioResult{failure}); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []struct{ timestamp, want string }{
		{"2026-09-09T08:00:00.4-04:00", VerdictFail},
		{"2026-09-09T08:00:00.6-04:00", VerdictPass},
	} {
		pass := failure
		pass.JudgedAt, pass.Verdict, pass.Score = candidate.timestamp, VerdictPass, 1
		if _, err := (Writer{}).Append(root, "run", 2, []ScenarioResult{pass}); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load(root, true)
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.Artifact.Results[1].Verdict; got != candidate.want {
			t.Fatalf("timestamp %s: verdict = %s, want %s", candidate.timestamp, got, candidate.want)
		}
	}
}
