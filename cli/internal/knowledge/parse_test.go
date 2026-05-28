package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBuilderMetadata(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   map[string]string
		isNil  bool
	}{
		{
			name:  "key=value pairs",
			input: "output_path=/tmp/out.md\nstatus=ok\n",
			want:  map[string]string{"output_path": "/tmp/out.md", "status": "ok"},
		},
		{
			name:  "skips lines without equals",
			input: "some log line\nkey=val\nanother line\n",
			want:  map[string]string{"key": "val"},
		},
		{
			name:  "trims whitespace",
			input: "  key = value  \n",
			want:  map[string]string{"key": "value"},
		},
		{
			name:  "empty input returns nil",
			input: "",
			isNil: true,
		},
		{
			name:  "no valid pairs returns nil",
			input: "just text\nmore text\n",
			isNil: true,
		},
		{
			name:  "empty key skipped",
			input: "=value\nreal=data\n",
			want:  map[string]string{"real": "data"},
		},
		{
			name:  "empty value skipped",
			input: "key=\nreal=data\n",
			want:  map[string]string{"real": "data"},
		},
		{
			name:  "value with equals sign preserved",
			input: "expr=a=b=c\n",
			want:  map[string]string{"expr": "a=b=c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseBuilderMetadata(tt.input)
			if tt.isNil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("length mismatch: got %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for k, wantV := range tt.want {
				if got[k] != wantV {
					t.Errorf("key %q: got %q, want %q", k, got[k], wantV)
				}
			}
		})
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		isNil bool
		check func(t *testing.T, m map[string]any)
	}{
		{
			name:  "valid frontmatter",
			input: "---\ntitle: Hello\nstatus: draft\n---\n\nBody text",
			check: func(t *testing.T, m map[string]any) {
				if m["title"] != "Hello" {
					t.Errorf("title: got %v", m["title"])
				}
				if m["status"] != "draft" {
					t.Errorf("status: got %v", m["status"])
				}
			},
		},
		{
			name:  "no frontmatter returns nil",
			input: "Just a markdown document\n\nWith paragraphs.",
			isNil: true,
		},
		{
			name:  "empty string returns nil",
			input: "",
			isNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFrontmatter(tt.input)
			if tt.isNil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil map")
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestFrontmatterString(t *testing.T) {
	fm := map[string]any{
		"title":  "Hello",
		"empty":  "",
		"nilval": nil,
	}
	tests := []struct {
		name     string
		fm       map[string]any
		key      string
		fallback string
		want     string
	}{
		{"existing key", fm, "title", "default", "Hello"},
		{"missing key uses fallback", fm, "missing", "default", "default"},
		{"empty value uses fallback", fm, "empty", "default", "default"},
		{"nil map uses fallback", nil, "title", "default", "default"},
		{"nil value uses fallback", fm, "nilval", "default", "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FrontmatterString(tt.fm, tt.key, tt.fallback)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractBullets(t *testing.T) {
	doc := `## Open Gaps

- Gap one
- Gap two

## Next Section

- Not this one
`
	tests := []struct {
		name    string
		text    string
		heading string
		want    []string
	}{
		{
			name:    "extracts bullets under heading",
			text:    doc,
			heading: "## Open Gaps",
			want:    []string{"Gap one", "Gap two"},
		},
		{
			name:    "stops at next heading",
			text:    doc,
			heading: "## Next Section",
			want:    []string{"Not this one"},
		},
		{
			name:    "missing heading returns empty",
			text:    doc,
			heading: "## Nonexistent",
			want:    []string{},
		},
		{
			name:    "non-bullet lines skipped",
			text:    "## Section\n\nParagraph text\n- Bullet\nMore text\n",
			heading: "## Section",
			want:    []string{"Bullet"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractBullets(tt.text, tt.heading)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d items, want %d: %v", len(got), len(tt.want), got)
			}
			for i, g := range got {
				if g != tt.want[i] {
					t.Errorf("item %d: got %q, want %q", i, g, tt.want[i])
				}
			}
		})
	}
}

func TestFilterOpenGaps(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "removes sentinel",
			input: []string{"No open gaps recorded.", "Real gap"},
			want:  []string{"Real gap"},
		},
		{
			name:  "case-insensitive sentinel",
			input: []string{"no open gaps recorded.", "Real gap"},
			want:  []string{"Real gap"},
		},
		{
			name:  "no sentinel passes through",
			input: []string{"Gap A", "Gap B"},
			want:  []string{"Gap A", "Gap B"},
		},
		{
			name:  "empty input",
			input: []string{},
			want:  []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterOpenGaps(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d, want %d: %v", len(got), len(tt.want), got)
			}
			for i, g := range got {
				if g != tt.want[i] {
					t.Errorf("item %d: got %q, want %q", i, g, tt.want[i])
				}
			}
		})
	}
}

func TestDedupeStrings(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"removes duplicates", []string{"a", "b", "a", "c"}, []string{"a", "b", "c"}},
		{"trims whitespace", []string{" a ", "a"}, []string{"a"}},
		{"drops empty strings", []string{"", "a", "", "b"}, []string{"a", "b"}},
		{"empty input", []string{}, []string{}},
		{"nil input", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DedupeStrings(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d, want %d: %v", len(got), len(tt.want), got)
			}
			for i, g := range got {
				if g != tt.want[i] {
					t.Errorf("item %d: got %q, want %q", i, g, tt.want[i])
				}
			}
		})
	}
}

func TestPathExists(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(existing, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !PathExists(existing) {
		t.Error("expected true for existing file")
	}
	if PathExists(filepath.Join(dir, "nope.txt")) {
		t.Error("expected false for missing file")
	}
}
