// practices: [cmm-process-maturity, dora-metrics]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// writeSizedLearningJSONL creates a JSONL learning file with the given metadata
// and pads its body to approximately sizeBytes total file size. The padding is
// appended on subsequent lines so it does not interfere with the metadata
// parser (which only reads the first line).
func writeSizedLearningJSONL(t *testing.T, dir, name string, utility, confidence float64, maturity string, sizeBytes int) string {
	t.Helper()
	first := fmt.Sprintf(`{"id":%q,"utility":%g,"confidence":%g,"maturity":%q}`+"\n",
		name, utility, confidence, maturity)
	pad := sizeBytes - len(first)
	if pad < 0 {
		pad = 0
	}
	body := make([]byte, 0, len(first)+pad)
	body = append(body, first...)
	if pad > 0 {
		filler := make([]byte, pad)
		for i := range filler {
			filler[i] = 'x'
		}
		body = append(body, filler...)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// resetMaturityFlags returns a cleanup function that restores all maturity
// flags this test suite mutates.
func resetMaturityFlags(t *testing.T) func() {
	t.Helper()
	prevTarget := maturityTargetSize
	prevEvict := maturityEvict
	prevArchive := maturityArchive
	prevDry := dryRun
	return func() {
		maturityTargetSize = prevTarget
		maturityEvict = prevEvict
		maturityArchive = prevArchive
		dryRun = prevDry
	}
}

func TestRunMaturityEvict_TargetSize_BelowBudget_NoOp(t *testing.T) {
	defer resetMaturityFlags(t)()

	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// ~10 KB total hub (5 files * 2 KB).
	for i := 0; i < 5; i++ {
		writeSizedLearningJSONL(t, learningsDir,
			fmt.Sprintf("low-%d.jsonl", i), 0.1, 0.1, "provisional", 2048)
	}

	t.Chdir(tmp)

	maturityEvict = true
	maturityArchive = true
	maturityTargetSize = "1M" // budget far above current ~10 KB
	dryRun = false

	if err := runMaturityEvict(nil); err != nil {
		t.Fatalf("runMaturityEvict: %v", err)
	}

	// All files must remain in learnings dir; archive dir should be empty/missing.
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("low-%d.jsonl", i)
		if _, err := os.Stat(filepath.Join(learningsDir, name)); os.IsNotExist(err) {
			t.Errorf("file %s should not have been evicted (under budget)", name)
		}
	}
	archiveDir := filepath.Join(tmp, ".agents", "archive", "learnings")
	if entries, err := os.ReadDir(archiveDir); err == nil && len(entries) > 0 {
		t.Errorf("archive dir should be empty under budget, got %d entries", len(entries))
	}
}

func TestRunMaturityEvict_TargetSize_OverBudget_EvictsLowestUtilityFirst(t *testing.T) {
	defer resetMaturityFlags(t)()

	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 4 files * 10 KB = 40 KB total. Target 25 KB. Need to evict 2 files
	// (lowest utility first: u=0.1, then u=0.2).
	utilities := []float64{0.1, 0.2, 0.25, 0.28}
	const fileSize = 10240
	for _, u := range utilities {
		// confidence and maturity must keep them eligible (utility < 0.3,
		// confidence < 0.3, not established).
		writeSizedLearningJSONL(t, learningsDir,
			fmt.Sprintf("u%02d.jsonl", int(u*100)), u, 0.1, "provisional", fileSize)
	}

	t.Chdir(tmp)

	maturityEvict = true
	maturityArchive = true
	maturityTargetSize = "25K"
	dryRun = false

	if err := runMaturityEvict(nil); err != nil {
		t.Fatalf("runMaturityEvict: %v", err)
	}

	archiveDir := filepath.Join(tmp, ".agents", "archive", "learnings")
	got := map[string]bool{}
	if entries, err := os.ReadDir(archiveDir); err == nil {
		for _, e := range entries {
			got[e.Name()] = true
		}
	}

	wantEvicted := []string{"u10.jsonl", "u20.jsonl"}
	wantKept := []string{"u25.jsonl", "u28.jsonl"}

	for _, name := range wantEvicted {
		if !got[name] {
			t.Errorf("expected %s to be evicted", name)
		}
		if _, err := os.Stat(filepath.Join(learningsDir, name)); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed from learnings", name)
		}
	}
	for _, name := range wantKept {
		if got[name] {
			t.Errorf("did not expect %s to be evicted", name)
		}
		if _, err := os.Stat(filepath.Join(learningsDir, name)); os.IsNotExist(err) {
			t.Errorf("expected %s to remain in learnings", name)
		}
	}

	// Sanity: exactly two evicted, no more, no less.
	gotKeys := make([]string, 0, len(got))
	for k := range got {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	if len(gotKeys) != 2 {
		t.Errorf("expected 2 evicted files, got %d (%v)", len(gotKeys), gotKeys)
	}
}

func TestRunMaturityEvict_TargetSize_RespectsExistingEligibility(t *testing.T) {
	defer resetMaturityFlags(t)()

	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	const fileSize = 10240

	// File 1: utility=0.05 BUT maturity="established" -> ineligible.
	writeSizedLearningJSONL(t, learningsDir, "established-low.jsonl", 0.05, 0.05, "established", fileSize)
	// File 2: utility=0.05 BUT confidence=0.5 -> ineligible (confidence >= 0.3).
	writeSizedLearningJSONL(t, learningsDir, "highconf-low.jsonl", 0.05, 0.5, "provisional", fileSize)
	// File 3: utility=0.1 AND eligible -> the only legitimate target.
	writeSizedLearningJSONL(t, learningsDir, "eligible-low.jsonl", 0.1, 0.1, "provisional", fileSize)

	t.Chdir(tmp)

	maturityEvict = true
	maturityArchive = true
	// 30 KB total -> set target to 5 KB; only one file is eligible, so even
	// though budget would prefer two, eligibility wins.
	maturityTargetSize = "5K"
	dryRun = false

	if err := runMaturityEvict(nil); err != nil {
		t.Fatalf("runMaturityEvict: %v", err)
	}

	archiveDir := filepath.Join(tmp, ".agents", "archive", "learnings")

	// established + high-confidence should still be present.
	for _, name := range []string{"established-low.jsonl", "highconf-low.jsonl"} {
		if _, err := os.Stat(filepath.Join(learningsDir, name)); os.IsNotExist(err) {
			t.Errorf("ineligible file %s must NOT be evicted", name)
		}
		if _, err := os.Stat(filepath.Join(archiveDir, name)); err == nil {
			t.Errorf("ineligible file %s must NOT be archived", name)
		}
	}

	// eligible-low should be evicted.
	if _, err := os.Stat(filepath.Join(archiveDir, "eligible-low.jsonl")); os.IsNotExist(err) {
		t.Errorf("eligible-low.jsonl should have been archived")
	}
	if _, err := os.Stat(filepath.Join(learningsDir, "eligible-low.jsonl")); !os.IsNotExist(err) {
		t.Errorf("eligible-low.jsonl should have been removed from learnings")
	}
}
