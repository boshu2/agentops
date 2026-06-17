package claimpolicy

import "testing"

func TestSurfaceAllowed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		patterns []string
		surface  string
		want     bool
	}{
		{name: "all", patterns: []string{"**"}, surface: "PRODUCT.md", want: true},
		{name: "exact", patterns: []string{"GOALS.md"}, surface: "GOALS.md", want: true},
		{name: "prefix glob", patterns: []string{"docs/**"}, surface: "docs/evals/x.md", want: true},
		{name: "nested prefix glob", patterns: []string{"docs/comparisons/**"}, surface: "docs/comparisons/vs.md", want: true},
		{name: "shell glob", patterns: []string{"docs/*.md"}, surface: "docs/index.md", want: true},
		{name: "not allowed", patterns: []string{"docs/comparisons/**"}, surface: "README.md", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := SurfaceAllowed(tc.patterns, tc.surface); got != tc.want {
				t.Fatalf("SurfaceAllowed(%v, %q) = %v, want %v", tc.patterns, tc.surface, got, tc.want)
			}
		})
	}
}
