package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultLearningMetadataFields(t *testing.T) {
	d := DefaultLearningMetadataFields()
	expectedKeys := []string{"utility", "maturity", "confidence", "reward_count", "helpful_count", "harmful_count"}
	for _, k := range expectedKeys {
		if _, ok := d[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if d["maturity"] != "provisional" {
		t.Errorf("default maturity = %q", d["maturity"])
	}
}

func TestParseFrontmatterFields(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "x.md")
	content := "---\nutility: 0.5\nmaturity: candidate\nquoted: \"value\"\n---\nbody\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ParseFrontmatterFields(path, "utility", "maturity", "quoted", "missing")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got["utility"] != "0.5" {
		t.Errorf("utility = %q", got["utility"])
	}
	if got["maturity"] != "candidate" {
		t.Errorf("maturity = %q", got["maturity"])
	}
	if got["quoted"] != "value" {
		t.Errorf("quotes should be stripped, got %q", got["quoted"])
	}
	if _, ok := got["missing"]; ok {
		t.Errorf("missing field should not be present")
	}
}

func TestIsLowSignalLearningBody(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"empty", "", true},
		{"too short", "short", true},
		{"starts with continuation", "and we did something important here today. totally.", true},
		{"no sentence enders", "a sufficient body of words but lacks punctuation endings here somewhere", true},
		{"good", "We discovered that goroutine leaks happen when channels aren't closed. Fix by deferring close.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsLowSignalLearningBody(tc.body); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStripLearningHeading(t *testing.T) {
	with := "# Title\n\nline one\nline two"
	if got := StripLearningHeading(with); got != "line one\nline two" {
		t.Errorf("got %q", got)
	}
	without := "line one\nline two"
	if got := StripLearningHeading(without); got != "line one\nline two" {
		t.Errorf("without heading: got %q", got)
	}
}

func TestIsEvictionEligible(t *testing.T) {
	cases := []struct {
		name       string
		utility    float64
		confidence float64
		maturity   string
		want       bool
	}{
		{"established is sacred", 0.0, 0.0, "established", false},
		{"high utility not eligible", 0.5, 0.0, "provisional", false},
		{"high confidence not eligible", 0.0, 0.5, "provisional", false},
		{"low everything is eligible", 0.1, 0.1, "provisional", true},
		{"boundary utility 0.3", 0.3, 0.0, "provisional", false},
		{"boundary confidence 0.3", 0.0, 0.3, "provisional", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsEvictionEligible(tc.utility, tc.confidence, tc.maturity); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFloatValueFromData(t *testing.T) {
	data := map[string]any{"u": 0.75, "s": "str"}
	if got := FloatValueFromData(data, "u", 0.0); got != 0.75 {
		t.Errorf("got %v", got)
	}
	if got := FloatValueFromData(data, "missing", 0.42); got != 0.42 {
		t.Errorf("missing default: got %v", got)
	}
	if got := FloatValueFromData(data, "s", 0.42); got != 0.42 {
		t.Errorf("wrong type should default: got %v", got)
	}
}

func TestNonEmptyStringFromData(t *testing.T) {
	data := map[string]any{"a": "hello", "b": ""}
	if got := NonEmptyStringFromData(data, "a", "d"); got != "hello" {
		t.Errorf("got %q", got)
	}
	if got := NonEmptyStringFromData(data, "b", "d"); got != "d" {
		t.Errorf("empty should fall back, got %q", got)
	}
	if got := NonEmptyStringFromData(data, "missing", "d"); got != "d" {
		t.Errorf("missing should fall back, got %q", got)
	}
}

func TestClassifyExpiryFields(t *testing.T) {
	now := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		fields map[string]string
		want   ExpiryClassification
	}{
		{"archived", map[string]string{"expiry_status": "archived"}, ExpiryAlreadyArchived},
		{"no expiry", map[string]string{}, ExpiryNeverExpiring},
		{"empty expiry", map[string]string{"valid_until": ""}, ExpiryNeverExpiring},
		{"bad format", map[string]string{"valid_until": "not-a-date"}, ExpiryNeverExpiring},
		{"still active", map[string]string{"valid_until": "2026-05-22"}, ExpiryActive},
		{"expired", map[string]string{"valid_until": "2026-03-22"}, ExpiryNewlyExpired},
		{"rfc3339 active", map[string]string{"valid_until": "2026-05-22T00:00:00Z"}, ExpiryActive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyExpiryFields(tc.fields, now); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEvictionCitationStatus(t *testing.T) {
	cutoff := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	old := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	// Never cited
	status, evictable := EvictionCitationStatus("f", map[string]time.Time{}, cutoff)
	if !evictable {
		t.Error("uncited should be evictable")
	}
	if status != "never" {
		t.Errorf("status = %q, want 'never'", status)
	}

	// Recently cited -> not evictable
	_, ev := EvictionCitationStatus("f", map[string]time.Time{"f": recent}, cutoff)
	if ev {
		t.Error("recently-cited should not be evictable")
	}

	// Old citation -> evictable with date
	status2, ev2 := EvictionCitationStatus("f", map[string]time.Time{"f": old}, cutoff)
	if !ev2 {
		t.Error("old citation should be evictable")
	}
	if status2 != "2026-03-01" {
		t.Errorf("status = %q", status2)
	}
}

func TestShouldArchiveUncitedLearning(t *testing.T) {
	cutoff := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	old := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	// Established never archived
	if ShouldArchiveUncitedLearning("established", old, false, cutoff) {
		t.Error("established should not be archived")
	}
	// Anti-pattern never archived
	if ShouldArchiveUncitedLearning("anti-pattern", old, false, cutoff) {
		t.Error("anti-pattern should not be archived")
	}
	// Recent mod time -> not archived
	if ShouldArchiveUncitedLearning("provisional", recent, false, cutoff) {
		t.Error("recently-modified should not be archived")
	}
	// Old + cited -> not archived
	if ShouldArchiveUncitedLearning("provisional", old, true, cutoff) {
		t.Error("cited learning should not be archived")
	}
	// Old + uncited + provisional -> archive
	if !ShouldArchiveUncitedLearning("provisional", old, false, cutoff) {
		t.Error("old+uncited+provisional should be archived")
	}
}

func TestFormatLastCited(t *testing.T) {
	when := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	if got := FormatLastCited(when, true); got != "2026-04-22" {
		t.Errorf("got %q", got)
	}
	if got := FormatLastCited(when, false); got != "never" {
		t.Errorf("got %q", got)
	}
}

func TestParseFrontmatterFloats(t *testing.T) {
	in := map[string]string{
		"utility":  "0.5",
		"maturity": "candidate",
		"count":    "42",
	}
	got := ParseFrontmatterFloats(in)
	if f, ok := got["utility"].(float64); !ok || f != 0.5 {
		t.Errorf("utility = %v", got["utility"])
	}
	if s, ok := got["maturity"].(string); !ok || s != "candidate" {
		t.Errorf("maturity = %v", got["maturity"])
	}
	if f, ok := got["count"].(float64); !ok || f != 42 {
		t.Errorf("count = %v", got["count"])
	}
}

func TestApplyJSONLDefaults_FillsMissing(t *testing.T) {
	data := map[string]any{"utility": 0.8}
	changed := ApplyJSONLDefaults(data)
	if !changed {
		t.Error("should report changed when filling defaults")
	}
	if data["utility"] != 0.8 {
		t.Errorf("should preserve existing utility, got %v", data["utility"])
	}
	if data["maturity"] != "provisional" {
		t.Errorf("should fill maturity default, got %v", data["maturity"])
	}
}

func TestApplyJSONLDefaults_NoChangesWhenAllPresent(t *testing.T) {
	data := map[string]any{
		"utility": 0.8, "maturity": "candidate",
		"confidence": 0.5, "reward_count": 1,
		"helpful_count": 2, "harmful_count": 0,
	}
	if ApplyJSONLDefaults(data) {
		t.Error("should not report changed when all present")
	}
}

func TestMissingMetadataFields(t *testing.T) {
	existing := map[string]string{"utility": "0.5", "maturity": ""}
	missing := MissingMetadataFields(existing)
	if _, ok := missing["utility"]; ok {
		t.Error("utility should not be missing")
	}
	if _, ok := missing["maturity"]; !ok {
		t.Error("maturity should be reported missing")
	}
}

func TestLearningMetadataFieldOrder(t *testing.T) {
	order := LearningMetadataFieldOrder()
	if len(order) != 6 {
		t.Errorf("expected 6 fields, got %d", len(order))
	}
	if order[0] != "utility" {
		t.Errorf("utility should come first, got %v", order)
	}
}

func TestBuildMarkdownFrontmatterPrefix(t *testing.T) {
	got := BuildMarkdownFrontmatterPrefix()
	if !strings.HasPrefix(got, "---\n") {
		t.Errorf("should start with ---")
	}
	if !strings.HasSuffix(got, "---\n") {
		t.Errorf("should end with ---")
	}
	for _, field := range LearningMetadataFieldOrder() {
		if !strings.Contains(got, field+":") {
			t.Errorf("missing field %q in output: %s", field, got)
		}
	}
}

func TestFindFrontmatterEnd(t *testing.T) {
	lines := []string{"---", "key: value", "---", "body"}
	if got := FindFrontmatterEnd(lines); got != 2 {
		t.Errorf("got %d, want 2", got)
	}

	unclosed := []string{"---", "key: value", "body"}
	if got := FindFrontmatterEnd(unclosed); got != -1 {
		t.Errorf("unclosed should return -1, got %d", got)
	}
}

func TestHasYAMLFrontmatter(t *testing.T) {
	if !HasYAMLFrontmatter("---\nkey: val") {
		t.Error("should detect frontmatter")
	}
	if HasYAMLFrontmatter("no frontmatter") {
		t.Error("false positive")
	}
}

func TestNormalizeJSONLLine(t *testing.T) {
	// Missing fields -> normalize
	out, changed, err := NormalizeJSONLLine(`{"utility":0.7}`)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("should be changed")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["utility"] != 0.7 {
		t.Errorf("utility should be preserved, got %v", parsed["utility"])
	}
	if parsed["maturity"] != "provisional" {
		t.Errorf("maturity should be provisional, got %v", parsed["maturity"])
	}

	// Empty line -> no change
	_, changed2, err := NormalizeJSONLLine("")
	if err != nil {
		t.Fatal(err)
	}
	if changed2 {
		t.Error("empty line should not be changed")
	}

	// Invalid JSON -> error
	_, _, err = NormalizeJSONLLine("not json")
	if err == nil {
		t.Error("invalid json should error")
	}
}

func TestReadLearningJSONLData(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "a.jsonl")
	_ = os.WriteFile(path, []byte(`{"utility":0.9}`+"\n"), 0o600)

	data, ok := ReadLearningJSONLData(path)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if data["utility"] != 0.9 {
		t.Errorf("utility = %v", data["utility"])
	}

	// Missing file
	if _, ok := ReadLearningJSONLData(filepath.Join(tmp, "missing.jsonl")); ok {
		t.Error("missing file should return ok=false")
	}
}
