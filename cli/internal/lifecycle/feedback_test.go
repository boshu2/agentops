package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveReward(t *testing.T) {
	cases := []struct {
		name    string
		helpful bool
		harmful bool
		reward  float64
		alpha   float64
		want    float64
		wantErr bool
	}{
		{"both flags error", true, true, -1, 0.5, 0, true},
		{"helpful sets to 1.0", true, false, -1, 0.5, 1.0, false},
		{"harmful sets to 0.0", false, true, -1, 0.5, 0.0, false},
		{"negative reward errors when no shortcut", false, false, -1, 0.5, 0, true},
		{"reward > 1 errors", false, false, 1.5, 0.5, 0, true},
		{"alpha 0 errors", false, false, 0.5, 0, 0, true},
		{"alpha > 1 errors", false, false, 0.5, 1.5, 0, true},
		{"valid custom reward", false, false, 0.6, 0.3, 0.6, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveReward(tc.helpful, tc.harmful, tc.reward, tc.alpha)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClassifyFeedbackType(t *testing.T) {
	if ClassifyFeedbackType(true, false) != "helpful" {
		t.Error("helpful")
	}
	if ClassifyFeedbackType(false, true) != "harmful" {
		t.Error("harmful")
	}
	if ClassifyFeedbackType(false, false) != "custom" {
		t.Error("custom")
	}
}

func TestCounterDirectionFromFeedback(t *testing.T) {
	cases := []struct {
		name           string
		reward         float64
		explicitHelp   bool
		explicitHarm   bool
		wantHelpful    bool
		wantHarmful    bool
	}{
		{"explicit helpful wins", 0.1, true, false, true, false},
		{"explicit harmful wins", 0.9, false, true, false, true},
		{"high reward implies helpful", 0.9, false, false, true, false},
		{"low reward implies harmful", 0.1, false, false, false, true},
		{"neutral middle is neither", 0.5, false, false, false, false},
		{"boundary 0.8 is helpful", 0.8, false, false, true, false},
		{"boundary 0.2 is harmful", 0.2, false, false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, harm := CounterDirectionFromFeedback(tc.reward, tc.explicitHelp, tc.explicitHarm)
			if h != tc.wantHelpful || harm != tc.wantHarmful {
				t.Errorf("got helpful=%v harmful=%v; want helpful=%v harmful=%v", h, harm, tc.wantHelpful, tc.wantHarmful)
			}
		})
	}
}

func TestParseJSONLFirstLine(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ok.jsonl")
	_ = os.WriteFile(path, []byte(`{"utility":0.5,"maturity":"candidate"}`+"\n"), 0o600)

	lines, data, err := ParseJSONLFirstLine(path)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(lines) < 1 {
		t.Error("should return lines")
	}
	if data["utility"] != 0.5 {
		t.Errorf("utility = %v", data["utility"])
	}

	// Empty file
	empty := filepath.Join(tmp, "empty.jsonl")
	_ = os.WriteFile(empty, []byte(""), 0o600)
	if _, _, err := ParseJSONLFirstLine(empty); err == nil {
		t.Error("empty file should error")
	}

	// Invalid JSON
	bad := filepath.Join(tmp, "bad.jsonl")
	_ = os.WriteFile(bad, []byte("not json\n"), 0o600)
	if _, _, err := ParseJSONLFirstLine(bad); err == nil {
		t.Error("invalid json should error")
	}

	// Missing file
	if _, _, err := ParseJSONLFirstLine(filepath.Join(tmp, "missing.jsonl")); err == nil {
		t.Error("missing file should error")
	}
}

func TestApplyJSONLRewardFields(t *testing.T) {
	data := map[string]any{
		"utility":       0.5,
		"reward_count":  float64(2),
		"helpful_count": float64(1),
		"harmful_count": float64(0),
	}
	ApplyJSONLRewardFields(data, 0.5, 0.7, 1.0, true, false)

	if data["utility"] != 0.7 {
		t.Errorf("utility = %v", data["utility"])
	}
	if data["last_reward"] != 1.0 {
		t.Errorf("last_reward = %v", data["last_reward"])
	}
	if data["reward_count"] != 3 {
		t.Errorf("reward_count = %v", data["reward_count"])
	}
	if data["helpful_count"] != 2 {
		t.Errorf("helpful_count = %v (should increment from 1)", data["helpful_count"])
	}
	if _, ok := data["confidence"]; !ok {
		t.Error("confidence should be set")
	}
	if _, ok := data["last_reward_at"]; !ok {
		t.Error("last_reward_at should be set")
	}
}

func TestApplyJSONLRewardFields_HarmfulPath(t *testing.T) {
	data := map[string]any{"utility": 0.3}
	ApplyJSONLRewardFields(data, 0.3, 0.2, 0.0, false, true)
	if data["harmful_count"] != 1 {
		t.Errorf("harmful_count should be 1, got %v", data["harmful_count"])
	}
}

func TestParseFrontMatterUtility(t *testing.T) {
	lines := []string{"---", "utility: 0.75", "maturity: candidate", "---", "body"}
	endIdx, u, err := ParseFrontMatterUtility(lines)
	if err != nil {
		t.Fatal(err)
	}
	if endIdx != 3 {
		t.Errorf("endIdx = %d", endIdx)
	}
	if u != 0.75 {
		t.Errorf("utility = %v", u)
	}

	// Missing closing ---
	lines2 := []string{"---", "utility: 0.5"}
	if _, _, err := ParseFrontMatterUtility(lines2); err == nil {
		t.Error("missing close should error")
	}
}

func TestRebuildWithFrontMatter(t *testing.T) {
	fm := []string{"utility: 0.5", "maturity: candidate"}
	body := []string{"line one", "line two"}
	got := RebuildWithFrontMatter(fm, body)
	if !strings.HasPrefix(got, "---\n") {
		t.Error("should start with ---")
	}
	if !strings.Contains(got, "utility: 0.5") {
		t.Error("should contain fm fields")
	}
	if !strings.Contains(got, "line one") {
		t.Error("should contain body")
	}
}

func TestUpdateFrontMatterFields_ReplacesAndAppends(t *testing.T) {
	existing := []string{"utility: 0.3", "maturity: provisional"}
	result := UpdateFrontMatterFields(existing, map[string]string{
		"utility":  "0.9",
		"new_key":  "new_val",
	})
	joined := strings.Join(result, "\n")
	if !strings.Contains(joined, "utility: 0.9") {
		t.Errorf("should have replaced utility, got %v", result)
	}
	if strings.Contains(joined, "utility: 0.3") {
		t.Errorf("old utility should not remain")
	}
	if !strings.Contains(joined, "new_key: new_val") {
		t.Errorf("missing field should be appended")
	}
	if !strings.Contains(joined, "maturity: provisional") {
		t.Errorf("unrelated fields should be preserved")
	}
}

func TestIncrementRewardCount(t *testing.T) {
	got := IncrementRewardCount([]string{"reward_count: 4"})
	if got != "5" {
		t.Errorf("got %q, want 5", got)
	}

	// Missing field starts at 0
	if got := IncrementRewardCount([]string{"utility: 0.5"}); got != "1" {
		t.Errorf("got %q, want 1", got)
	}
}

func TestParseFrontMatterInt(t *testing.T) {
	lines := []string{"reward_count: 7", "helpful_count: 3"}
	if got := ParseFrontMatterInt(lines, "reward_count"); got != 7 {
		t.Errorf("got %d", got)
	}
	if got := ParseFrontMatterInt(lines, "missing"); got != 0 {
		t.Errorf("missing should default to 0, got %d", got)
	}
}

func TestIncrementFMCount(t *testing.T) {
	lines := []string{"helpful_count: 2"}
	if got := IncrementFMCount(lines, "helpful_count"); got != "3" {
		t.Errorf("got %q", got)
	}
}
