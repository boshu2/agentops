package knowledge

import (
	"strings"
	"testing"
)

func TestTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "basic tokenization",
			text: "Hello World Testing",
			want: []string{"hello", "world", "testing"},
		},
		{
			name: "short tokens dropped",
			text: "go is a lang",
			want: []string{"lang"},
		},
		{
			name: "deduplicates",
			text: "hello hello world",
			want: []string{"hello", "world"},
		},
		{
			name: "strips non-alphanumeric",
			text: "foo-bar_baz (qux)",
			want: []string{"foo", "bar", "baz", "qux"},
		},
		{
			name: "empty input",
			text: "",
			want: []string{},
		},
		{
			name: "numeric tokens",
			text: "abc 123 def456",
			want: []string{"abc", "123", "def456"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Tokens(tt.text)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i, g := range got {
				if g != tt.want[i] {
					t.Errorf("token %d: got %q, want %q", i, g, tt.want[i])
				}
			}
		})
	}
}

func TestHealthRank(t *testing.T) {
	tests := []struct {
		health string
		want   int
	}{
		{"healthy", 2},
		{"Healthy", 2},
		{"thin", 1},
		{"THIN", 1},
		{"unknown", 0},
		{"", 0},
	}
	for _, tt := range tests {
		t.Run(tt.health, func(t *testing.T) {
			got := HealthRank(tt.health)
			if got != tt.want {
				t.Errorf("HealthRank(%q) = %d, want %d", tt.health, got, tt.want)
			}
		})
	}
}

func TestChunkRank(t *testing.T) {
	tests := []struct {
		chunkType string
		want      int
	}{
		{"decision", 3},
		{"Decision", 3},
		{"pattern", 2},
		{"overview", 1},
		{"other", 0},
		{"", 0},
	}
	for _, tt := range tests {
		t.Run(tt.chunkType, func(t *testing.T) {
			got := ChunkRank(tt.chunkType)
			if got != tt.want {
				t.Errorf("ChunkRank(%q) = %d, want %d", tt.chunkType, got, tt.want)
			}
		})
	}
}

func TestAppendCandidate(t *testing.T) {
	tests := []struct {
		name      string
		items     []string
		candidate string
		wantLen   int
	}{
		{"adds new", []string{"a"}, "b", 2},
		{"skips duplicate", []string{"a", "b"}, "b", 2},
		{"normalizes whitespace dedup", []string{"hello world"}, "hello  world", 1},
		{"normalizes whitespace new", []string{"other"}, "hello  world", 2},
		{"skips empty", []string{"a"}, "", 1},
		{"skips whitespace-only", []string{"a"}, "   ", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AppendCandidate(tt.items, tt.candidate)
			if len(got) != tt.wantLen {
				t.Errorf("got len %d, want %d: %v", len(got), tt.wantLen, got)
			}
		})
	}
}

func TestContainsTopic(t *testing.T) {
	topics := []TopicDetail{
		{TopicState: TopicState{ID: "alpha"}},
		{TopicState: TopicState{ID: "beta"}},
	}
	if !ContainsTopic(topics, "alpha") {
		t.Error("expected true for existing topic")
	}
	if ContainsTopic(topics, "gamma") {
		t.Error("expected false for missing topic")
	}
	if ContainsTopic(nil, "alpha") {
		t.Error("expected false for nil topics")
	}
}

func TestFieldValue(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"standard field", "- Type: decision", "decision"},
		{"backtick value", "- Status: `draft`", "draft"},
		{"no colon", "no-field-here", ""},
		{"empty value", "- Key: ", ""},
		{"value with colon", "- URL: http://example.com", "http://example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FieldValue(tt.line)
			if got != tt.want {
				t.Errorf("FieldValue(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestBuilderGoal(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"found", []string{"--verbose", "--goal", "test coverage"}, "test coverage"},
		{"not present", []string{"--verbose"}, ""},
		{"at end without value", []string{"--goal"}, ""},
		{"empty args", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuilderGoal(tt.args)
			if got != tt.want {
				t.Errorf("BuilderGoal(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestSlicesContain(t *testing.T) {
	items := []string{"apple", "banana", "cherry"}
	if !SlicesContain(items, "banana") {
		t.Error("expected true for existing item")
	}
	if SlicesContain(items, "Banana") {
		t.Error("expected false for case mismatch")
	}
	if SlicesContain(nil, "apple") {
		t.Error("expected false for nil slice")
	}
}

func TestStringSliceContainsFold(t *testing.T) {
	items := []string{"Apple", "Banana"}
	if !StringSliceContainsFold(items, "apple") {
		t.Error("expected true for case-insensitive match")
	}
	if StringSliceContainsFold(items, "cherry") {
		t.Error("expected false for missing item")
	}
}

func TestYesNo(t *testing.T) {
	if YesNo(true) != "yes" {
		t.Error("expected yes for true")
	}
	if YesNo(false) != "no" {
		t.Error("expected no for false")
	}
}

func TestSectionText(t *testing.T) {
	doc := `## Summary

This is the summary
with multiple lines.

## Next

Other content here.
`
	got := SectionText(doc, "## Summary")
	if !strings.Contains(got, "summary") {
		t.Errorf("expected summary text, got %q", got)
	}
	if strings.Contains(got, "Other") {
		t.Error("should not include next section")
	}

	empty := SectionText(doc, "## Missing")
	if empty != "" {
		t.Errorf("expected empty for missing heading, got %q", empty)
	}
}

func TestFrontmatterStringList(t *testing.T) {
	tests := []struct {
		name string
		fm   map[string]any
		key  string
		want []string
	}{
		{
			name: "string slice",
			fm:   map[string]any{"tags": []string{"a", "b", "a"}},
			key:  "tags",
			want: []string{"a", "b"},
		},
		{
			name: "any slice",
			fm:   map[string]any{"items": []any{"x", "y"}},
			key:  "items",
			want: []string{"x", "y"},
		},
		{
			name: "scalar fallback",
			fm:   map[string]any{"single": "only"},
			key:  "single",
			want: []string{"only"},
		},
		{
			name: "missing key",
			fm:   map[string]any{},
			key:  "nope",
			want: nil,
		},
		{
			name: "nil map",
			fm:   nil,
			key:  "any",
			want: nil,
		},
		{
			name: "nil value in any slice",
			fm:   map[string]any{"items": []any{nil, "real"}},
			key:  "items",
			want: []string{"real"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FrontmatterStringList(tt.fm, tt.key)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i, g := range got {
				if g != tt.want[i] {
					t.Errorf("item %d: got %q, want %q", i, g, tt.want[i])
				}
			}
		})
	}
}

func TestFrontmatterNestedInt(t *testing.T) {
	fm := map[string]any{
		"stats": map[string]any{
			"count":   42,
			"float":   3.14,
			"text":    "7",
			"invalid": "abc",
		},
	}
	tests := []struct {
		name   string
		fm     map[string]any
		parent string
		key    string
		want   int
	}{
		{"int value", fm, "stats", "count", 42},
		{"float truncated", fm, "stats", "float", 3},
		{"string parsed", fm, "stats", "text", 7},
		{"invalid string", fm, "stats", "invalid", 0},
		{"missing parent", fm, "nope", "count", 0},
		{"missing key", fm, "stats", "nope", 0},
		{"nil map", nil, "stats", "count", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FrontmatterNestedInt(tt.fm, tt.parent, tt.key)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseChunks(t *testing.T) {
	doc := `## Knowledge Chunks

### Chunk 1

- Chunk ID: chunk-001
- Type: decision
- Confidence: high
- Claim: Always test your code

### Chunk 2

- Chunk ID: chunk-002
- Type: pattern
- Confidence: medium
- Claim: Table-driven tests are best

## Next Section
`
	chunks := ParseChunks(doc)
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if chunks[0].ID != "chunk-001" {
		t.Errorf("chunk 0 ID: got %q", chunks[0].ID)
	}
	if chunks[0].Type != "decision" {
		t.Errorf("chunk 0 Type: got %q", chunks[0].Type)
	}
	if chunks[0].Confidence != "high" {
		t.Errorf("chunk 0 Confidence: got %q", chunks[0].Confidence)
	}
	if chunks[0].Claim != "Always test your code" {
		t.Errorf("chunk 0 Claim: got %q", chunks[0].Claim)
	}
	if chunks[1].ID != "chunk-002" {
		t.Errorf("chunk 1 ID: got %q", chunks[1].ID)
	}

	empty := ParseChunks("no chunks here")
	if len(empty) != 0 {
		t.Errorf("expected 0 chunks for non-chunk doc, got %d", len(empty))
	}
}

func TestWhenToUse(t *testing.T) {
	topic := TopicDetail{
		TopicState: TopicState{Title: "Testing"},
		Aliases:    []string{"Quality Assurance", "Validation"},
	}
	got := WhenToUse(topic)
	if !strings.Contains(got, "quality assurance") {
		t.Errorf("expected alias in output, got %q", got)
	}
	if !strings.Contains(got, "bounded operator loop") {
		t.Errorf("expected template text, got %q", got)
	}

	noAliases := TopicDetail{TopicState: TopicState{Title: "Deployment"}}
	got2 := WhenToUse(noAliases)
	if !strings.Contains(got2, "deployment") {
		t.Errorf("expected title fallback, got %q", got2)
	}
}

func TestPrimitivesForTopic(t *testing.T) {
	topic := TopicDetail{
		TopicState: TopicState{Title: "CI Gate Validation"},
		Summary:    "Runs acceptance tests and validates gate policies for releases",
	}
	primitives := PrimitivesForTopic(topic)
	if len(primitives) < 2 {
		t.Fatalf("expected at least 2 primitives, got %d: %v", len(primitives), primitives)
	}
	if primitives[0] != "stateful environment" {
		t.Errorf("first primitive should always be 'stateful environment', got %q", primitives[0])
	}
}

func TestRenderPlaybooksIndex(t *testing.T) {
	rows := []PlaybookRow{
		{Topic: "Beta", Path: "beta.md", Health: "thin", Canonical: false},
		{Topic: "Alpha", Path: "alpha.md", Health: "healthy", Canonical: true},
	}
	got := RenderPlaybooksIndex(rows)
	if !strings.Contains(got, "# Playbook Candidates") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "| [Alpha](alpha.md)") {
		t.Error("missing alpha row")
	}
	alphaIdx := strings.Index(got, "Alpha")
	betaIdx := strings.Index(got, "Beta")
	if alphaIdx > betaIdx {
		t.Error("healthy topics should sort before thin")
	}
}

func TestRenderBeliefBook(t *testing.T) {
	got := RenderBeliefBook(
		"/tmp/beliefs.md",
		"/tmp/source",
		[]string{"Belief 1", "Belief 2"},
		[]string{"Principle A"},
		[]string{"Thin Topic X"},
		[]string{".agents/topics/"},
	)
	if !strings.Contains(got, "# Book Of Beliefs") {
		t.Error("missing title")
	}
	if !strings.Contains(got, "Belief 1") {
		t.Error("missing belief")
	}
	if !strings.Contains(got, "Principle A") {
		t.Error("missing principle")
	}
	if !strings.Contains(got, "Thin Topic X") {
		t.Error("missing thin topic")
	}
	if !strings.Contains(got, "type: principle-book") {
		t.Error("missing frontmatter type")
	}

	empty := RenderBeliefBook("/tmp/b.md", "", nil, nil, nil, nil)
	if !strings.Contains(empty, "No operating principles") {
		t.Error("expected empty-state message for principles")
	}
	if !strings.Contains(empty, "None surfaced") {
		t.Error("expected empty-state message for thin topics")
	}
}
