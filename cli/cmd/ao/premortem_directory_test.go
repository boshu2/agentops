package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mortemDirectoryFixtureBytes(t *testing.T, rel string) []byte {
	t.Helper()
	root := os.Getenv("MORTEM_COMPAT_FIXTURES_DIR")
	if root == "" {
		root = filepath.Join("..", "..", "..", "tests", "fixtures", "mortem-compatibility")
	}
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read directory fixture %s: %v", rel, err)
	}
	return data
}

func writePremortemDirectoryFixture(t *testing.T, root, rel, content string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func premortemFinding(id, title string) string {
	return `---
id: "` + id + `"
title: "` + title + `"
status: "active"
applicable_when: ["task", "plan-shape"]
scope_tags: ["premortem", "directory", "compatibility"]
---
# Finding

Premortem directory compatibility must preserve compiled validation checks.
`
}

func premortemCheck(id, question string) string {
	return `---
id: "` + id + `"
---
# Premortem Check

- Ask: ` + question + `
`
}

func assemblePremortemDirectoryPacket(t *testing.T, root string) (StigmergicPacket, error) {
	t.Helper()
	writePremortemDirectoryFixture(t, root, filepath.Join(".agents", "rpi", "next-work.jsonl"), "")
	return assembleStigmergicPacket(root, StigmergicTarget{
		GoalText:  "premortem directory compatibility validation checks",
		IssueType: "task",
		Files:     []string{"skills/premortem/SKILL.md"},
		Repo:      "agentops",
		Limit:     10,
	})
}

func TestPremortemDirectoryReader_CanonicalFirstAndLegacyFill(t *testing.T) {
	root := t.TempDir()
	writePremortemDirectoryFixture(t, root, filepath.Join(".agents", "findings", "check-canonical.md"), premortemFinding("check-canonical", "Canonical premortem check"))
	writePremortemDirectoryFixture(t, root, filepath.Join(".agents", "findings", "check-legacy.md"), premortemFinding("check-legacy", "Legacy premortem check"))
	writePremortemDirectoryFixture(t, root, filepath.Join(".agents", "premortem-checks", "check-canonical.md"), premortemCheck("check-canonical", "canonical directory was read first"))
	writePremortemDirectoryFixture(t, root, filepath.Join(".agents", "pre-mortem-checks", "check-legacy.md"), premortemCheck("check-legacy", "legacy directory filled the missing ID"))

	packet, err := assemblePremortemDirectoryPacket(t, root)
	if err != nil {
		t.Fatalf("assembleStigmergicPacket: %v", err)
	}
	joined := strings.Join(packet.KnownRisks, "\n")
	for _, want := range []string{"check-canonical", "canonical directory was read first", "check-legacy", "legacy directory filled the missing ID"} {
		if !strings.Contains(joined, want) {
			t.Errorf("KnownRisks %q missing %q", joined, want)
		}
	}
}

func TestCanonicalPremortemReadDirectory_IsNotTheLegacyWriterDirectory(t *testing.T) {
	if canonicalPremortemReadDirectory != "premortem-checks" {
		t.Fatalf("canonicalPremortemReadDirectory = %q, want premortem-checks", canonicalPremortemReadDirectory)
	}
	if canonicalPremortemReadDirectory == "pre-mortem-checks" {
		t.Fatal("canonical read directory collapsed into the legacy writer directory")
	}
}

func TestPremortemDirectoryReader_EqualSameIDDeduplicates(t *testing.T) {
	root := t.TempDir()
	id := "check-equal"
	writePremortemDirectoryFixture(t, root, filepath.Join(".agents", "findings", id+".md"), premortemFinding(id, "Equal premortem check"))
	content := premortemCheck(id, "equal canonical and legacy content appears once")
	writePremortemDirectoryFixture(t, root, filepath.Join(".agents", "premortem-checks", id+".md"), content)
	writePremortemDirectoryFixture(t, root, filepath.Join(".agents", "pre-mortem-checks", id+".md"), content)

	packet, err := assemblePremortemDirectoryPacket(t, root)
	if err != nil {
		t.Fatalf("assembleStigmergicPacket: %v", err)
	}
	if got := len(packet.KnownRisks); got != 1 {
		t.Fatalf("KnownRisks = %v, want one deduplicated check", packet.KnownRisks)
	}
	if !strings.Contains(packet.KnownRisks[0], "equal canonical and legacy content appears once") {
		t.Errorf("KnownRisks[0] = %q, want equal check content", packet.KnownRisks[0])
	}
}

func TestPremortemDirectoryReader_ConflictFailsAndNamesBothPaths(t *testing.T) {
	root := t.TempDir()
	id := "check-conflict"
	writePremortemDirectoryFixture(t, root, filepath.Join(".agents", "findings", id+".md"), premortemFinding(id, "Conflicting premortem check"))
	canonical := writePremortemDirectoryFixture(t, root, filepath.Join(".agents", "premortem-checks", id+".md"), premortemCheck(id, "canonical content"))
	legacy := writePremortemDirectoryFixture(t, root, filepath.Join(".agents", "pre-mortem-checks", id+".md"), premortemCheck(id, "different legacy content"))

	_, err := assemblePremortemDirectoryPacket(t, root)
	if err == nil {
		t.Fatal("assembleStigmergicPacket accepted different canonical and legacy content for the same ID")
	}
	for _, path := range []string{canonical, legacy} {
		if !strings.Contains(err.Error(), path) {
			t.Errorf("conflict error %q does not name %s", err, path)
		}
	}
}

func TestMortemCompatibilityFixtures_DirectoryBytesReachProductionReader(t *testing.T) {
	t.Run("legacy fill", func(t *testing.T) {
		root := t.TempDir()
		id := "check-legacy"
		writePremortemDirectoryFixture(t, root, filepath.Join(".agents", "findings", id+".md"), premortemFinding(id, "Fixture legacy check"))
		path := filepath.Join(root, ".agents", "pre-mortem-checks", id+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, mortemDirectoryFixtureBytes(t, "legacy-directory/pre-mortem-check.json"), 0o644); err != nil {
			t.Fatal(err)
		}
		packet, err := assemblePremortemDirectoryPacket(t, root)
		if err != nil {
			t.Fatal(err)
		}
		if joined := strings.Join(packet.KnownRisks, "\n"); !strings.Contains(joined, "legacy-only content fills a missing canonical id") {
			t.Fatalf("production reader output %q omitted fixture bytes from legacy-directory/pre-mortem-check.json", joined)
		}
	})

	t.Run("conflict", func(t *testing.T) {
		root := t.TempDir()
		id := "check-conflict"
		writePremortemDirectoryFixture(t, root, filepath.Join(".agents", "findings", id+".md"), premortemFinding(id, "Fixture conflict"))
		canonical := filepath.Join(root, ".agents", "premortem-checks", id+".md")
		legacy := filepath.Join(root, ".agents", "pre-mortem-checks", id+".md")
		for path, rel := range map[string]string{
			canonical: "directory-conflict/premortem-check.json",
			legacy:    "directory-conflict/pre-mortem-check.json",
		} {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, mortemDirectoryFixtureBytes(t, rel), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, err := assemblePremortemDirectoryPacket(t, root)
		if err == nil {
			t.Fatal("production reader accepted conflicting directory fixture bytes")
		}
		for _, path := range []string{canonical, legacy} {
			if !strings.Contains(err.Error(), path) {
				t.Errorf("conflict error %q does not name %s", err, path)
			}
		}
	})
}
