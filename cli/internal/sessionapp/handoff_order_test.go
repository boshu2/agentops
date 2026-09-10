package sessionapp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRehydrateOrdersFractionalTimestamps(t *testing.T) {
	for _, tc := range []struct {
		name, firstFraction, secondFraction string
		firstRoot, secondRoot, want         int
	}{
		{"same-root", "9", "99", 0, 0, 1},
		{"newer-legacy", "9", "99", 0, 1, 1},
		{"newer-canonical", "99", "9", 0, 1, 0},
		{"equal-canonical", "9", "90", 0, 1, 0},
		{"sub-nanosecond", "0000000001", "0000000002", 0, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			roots := []string{filepath.Join(dir, ".agents", "ao", "handoff"), filepath.Join(dir, ".agents", "handoff")}
			ids := []string{"handoff-20260909T120000." + tc.firstFraction + "Z", "handoff-20260909T120000." + tc.secondFraction + "Z"}
			for i, index := range []int{tc.firstRoot, tc.secondRoot} {
				if err := os.MkdirAll(roots[index], 0o700); err != nil {
					t.Fatal(err)
				}
				data, err := json.Marshal(map[string]any{"schema_version": 1, "id": ids[i], "created_at": "2026-09-09T12:00:00Z"})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(roots[index], ids[i]+".json"), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, stderr bytes.Buffer
			if err := Rehydrate(RehydrateOptions{JSON: true, Stdout: &out, Stderr: &stderr}); err != nil {
				t.Fatal(err)
			}
			var result storedHandoff
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.ID == nil || *result.ID != ids[tc.want] {
				t.Fatalf("selected %s, want %s", out.Bytes(), ids[tc.want])
			}
		})
	}
}
