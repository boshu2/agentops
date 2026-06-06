package bench

import (
	"math"
	"testing"
)

func TestNormalizeSplit(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Train", "train"},
		{"  TEST  ", "test"},
		{"val", "val"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := NormalizeSplit(tt.input); got != tt.want {
			t.Errorf("NormalizeSplit(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeSection(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"## Summary", "summary"},
		{"### Detailed Analysis", "detailed analysis"},
		{"  # Title  ", "# title"},
		{"## Plain", "plain"},
		{"No Hash", "no hash"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := NormalizeSection(tt.input); got != tt.want {
			t.Errorf("NormalizeSection(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStripFrontMatter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips frontmatter",
			input: "---\ntitle: Hello\n---\nBody content",
			want:  "Body content",
		},
		{
			name:  "no frontmatter",
			input: "Just content\nMore content",
			want:  "Just content\nMore content",
		},
		{
			name:  "unclosed frontmatter",
			input: "---\ntitle: Hello\nno closing delimiter",
			want:  "---\ntitle: Hello\nno closing delimiter",
		},
		{
			name:  "empty content",
			input: "",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripFrontMatter(tt.input); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSectionHeading(t *testing.T) {
	tests := []struct {
		name    string
		section string
		want    string
	}{
		{"h2 heading", "## Summary\n\nContent here", "Summary"},
		{"h1 heading", "# Title\n\nBody", "Title"},
		{"no heading", "Just text\nMore text", ""},
		{"heading after blank", "\n\n## Found It", "Found It"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SectionHeading(tt.section); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestScoreResults(t *testing.T) {
	expected := map[string]bool{"a": true, "b": true, "c": true}

	tests := []struct {
		name     string
		results  []string
		expected map[string]bool
		bestID   string
		k        int
		wantPAtK float64
		wantMRR  float64
	}{
		{
			name:     "perfect precision",
			results:  []string{"a", "b", "c"},
			expected: expected,
			bestID:   "a",
			k:        3,
			wantPAtK: 1.0,
			wantMRR:  1.0,
		},
		{
			name:     "half precision",
			results:  []string{"a", "x", "b", "y"},
			expected: expected,
			bestID:   "a",
			k:        4,
			wantPAtK: 0.5,
			wantMRR:  1.0,
		},
		{
			name:     "best at position 3",
			results:  []string{"x", "y", "a"},
			expected: expected,
			bestID:   "a",
			k:        3,
			wantPAtK: 1.0 / 3.0,
			wantMRR:  1.0 / 3.0,
		},
		{
			name:     "k=0 returns zero",
			results:  []string{"a"},
			expected: expected,
			bestID:   "a",
			k:        0,
			wantPAtK: 0,
			wantMRR:  0,
		},
		{
			name:     "empty results",
			results:  nil,
			expected: expected,
			bestID:   "a",
			k:        5,
			wantPAtK: 0,
			wantMRR:  0,
		},
		{
			name:     "k larger than results",
			results:  []string{"a"},
			expected: expected,
			bestID:   "a",
			k:        5,
			wantPAtK: 0.2,
			wantMRR:  1.0,
		},
		{
			name:     "best not in results",
			results:  []string{"x", "y"},
			expected: expected,
			bestID:   "a",
			k:        2,
			wantPAtK: 0,
			wantMRR:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pAtK, mrr := ScoreResults(tt.results, tt.expected, tt.bestID, tt.k)
			if math.Abs(pAtK-tt.wantPAtK) > 1e-9 {
				t.Errorf("P@K: got %f, want %f", pAtK, tt.wantPAtK)
			}
			if math.Abs(mrr-tt.wantMRR) > 1e-9 {
				t.Errorf("MRR: got %f, want %f", mrr, tt.wantMRR)
			}
		})
	}
}
