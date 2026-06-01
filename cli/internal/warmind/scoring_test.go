package warmind

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTierFromScore(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		expected Tier
	}{
		{"gold high", 0.95, TierGold},
		{"gold boundary", 0.86, TierGold},
		{"silver high", 0.85, TierSilver},
		{"silver mid", 0.75, TierSilver},
		{"silver boundary", 0.70, TierSilver},
		{"bronze high", 0.69, TierBronze},
		{"bronze mid", 0.60, TierBronze},
		{"bronze boundary", 0.50, TierBronze},
		{"discard high", 0.49, TierDiscard},
		{"discard low", 0.10, TierDiscard},
		{"discard zero", 0.0, TierDiscard},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TierFromScore(tt.score)
			if got != tt.expected {
				t.Errorf("TierFromScore(%v) = %v, want %v", tt.score, got, tt.expected)
			}
		})
	}
}

func TestScoreSpecificity(t *testing.T) {
	cfg := DefaultConfig().Scoring
	scorer := NewScorer(cfg, "")

	tests := []struct {
		name        string
		content     string
		minExpected float64
		maxExpected float64
	}{
		{
			name:        "code block increases score",
			content:     "Use this code:\n```go\nfmt.Println(\"hello\")\n```",
			minExpected: 0.6,
			maxExpected: 1.0,
		},
		{
			name:        "file path increases score",
			content:     "Edit the file at /etc/config.yaml to change settings.",
			minExpected: 0.55,
			maxExpected: 1.0,
		},
		{
			name:        "vague language decreases score",
			content:     "This might work, maybe try something like possibly doing it.",
			minExpected: 0.0,
			maxExpected: 0.45,
		},
		{
			name:        "specific values increase score",
			content:     "Set MAX_CONNECTIONS=100 and timeout: 30",
			minExpected: 0.55,
			maxExpected: 1.0,
		},
		{
			name:        "concrete example increases score",
			content:     "For example, if you have a list of users, you can filter them.",
			minExpected: 0.55,
			maxExpected: 1.0,
		},
		{
			name:        "neutral content",
			content:     "This is some content about programming.",
			minExpected: 0.4,
			maxExpected: 0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := Candidate{Content: tt.content}
			got := scorer.scoreSpecificity(candidate)
			if got < tt.minExpected || got > tt.maxExpected {
				t.Errorf("scoreSpecificity() = %v, want between %v and %v", got, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestScoreActionability(t *testing.T) {
	cfg := DefaultConfig().Scoring
	scorer := NewScorer(cfg, "")

	tests := []struct {
		name        string
		content     string
		minExpected float64
		maxExpected float64
	}{
		{
			name:        "numbered steps",
			content:     "1. First step\n2. Second step\n3. Third step",
			minExpected: 0.5,
			maxExpected: 1.0,
		},
		{
			name:        "bullet points",
			content:     "- Do this\n- Then that\n- Finally this",
			minExpected: 0.45,
			maxExpected: 1.0,
		},
		{
			name:        "action verbs",
			content:     "Use the tool to create a config and run the command.",
			minExpected: 0.5,
			maxExpected: 1.0,
		},
		{
			name:        "do and dont guidance",
			content:     "Always check the return value. Don't ignore errors.",
			minExpected: 0.5,
			maxExpected: 1.0,
		},
		{
			name:        "passive content",
			content:     "The system was designed with certain principles in mind.",
			minExpected: 0.3,
			maxExpected: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := Candidate{Content: tt.content}
			got := scorer.scoreActionability(candidate)
			if got < tt.minExpected || got > tt.maxExpected {
				t.Errorf("scoreActionability() = %v, want between %v and %v", got, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestScoreConfidence(t *testing.T) {
	cfg := DefaultConfig().Scoring
	scorer := NewScorer(cfg, "")

	tests := []struct {
		name       string
		confidence float64
		expected   float64
	}{
		{"high confidence", 0.9, 0.9},
		{"medium confidence", 0.6, 0.6},
		{"low confidence floors to 0.3", 0.1, 0.3},
		{"zero confidence floors to 0.3", 0.0, 0.3},
		{"exactly at floor", 0.3, 0.3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := Candidate{Confidence: tt.confidence}
			got := scorer.scoreConfidence(candidate)
			if got != tt.expected {
				t.Errorf("scoreConfidence() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestScoreNovelty(t *testing.T) {
	cfg := DefaultConfig().Scoring
	scorer := NewScorer(cfg, "")

	t.Run("first learning gets 0.85", func(t *testing.T) {
		candidate := Candidate{Content: "This is new content about testing."}
		existing := []Learning{}
		got := scorer.scoreNovelty(candidate, existing)
		if got != 0.85 {
			t.Errorf("scoreNovelty() with no existing = %v, want 0.85", got)
		}
	})

	t.Run("similar content gets low novelty", func(t *testing.T) {
		candidate := Candidate{Content: "How to configure kubernetes pods for deployment."}
		existing := []Learning{
			{Content: "Configuring kubernetes pods for deployment is important."},
		}
		got := scorer.scoreNovelty(candidate, existing)
		if got > 0.5 {
			t.Errorf("scoreNovelty() with similar content = %v, want < 0.5", got)
		}
	})

	t.Run("different content gets high novelty", func(t *testing.T) {
		candidate := Candidate{Content: "Python decorators enable metaprogramming patterns."}
		existing := []Learning{
			{Content: "Kubernetes pods should have resource limits configured."},
		}
		got := scorer.scoreNovelty(candidate, existing)
		if got < 0.5 {
			t.Errorf("scoreNovelty() with different content = %v, want > 0.5", got)
		}
	})

	t.Run("empty content gets 0.5", func(t *testing.T) {
		candidate := Candidate{Content: ""}
		existing := []Learning{{Content: "Some existing content."}}
		got := scorer.scoreNovelty(candidate, existing)
		if got != 0.5 {
			t.Errorf("scoreNovelty() with empty content = %v, want 0.5", got)
		}
	})
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		expectedPresent []string
		expectedAbsent  []string
	}{
		{
			name:            "extracts meaningful words",
			content:         "Kubernetes deployments require careful configuration.",
			expectedPresent: []string{"kubernetes", "deployments", "require", "careful", "configuration"},
			expectedAbsent:  []string{"the", "a", "is"},
		},
		{
			name:            "removes code blocks",
			content:         "Use this:\n```go\nfmt.Println()\n```\nFor printing.",
			expectedPresent: []string{"use", "printing"},
			expectedAbsent:  []string{"fmt", "println"},
		},
		{
			name:            "filters short words",
			content:         "Do it as we go on by",
			expectedPresent: []string{},
			expectedAbsent:  []string{"do", "it", "as", "we", "go", "on", "by"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractKeywords(tt.content)
			for _, word := range tt.expectedPresent {
				if !got[word] {
					t.Errorf("extractKeywords() missing expected word %q", word)
				}
			}
			for _, word := range tt.expectedAbsent {
				if got[word] {
					t.Errorf("extractKeywords() should not contain %q", word)
				}
			}
		})
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        map[string]bool
		b        map[string]bool
		expected float64
	}{
		{
			name:     "identical sets",
			a:        map[string]bool{"foo": true, "bar": true},
			b:        map[string]bool{"foo": true, "bar": true},
			expected: 1.0,
		},
		{
			name:     "disjoint sets",
			a:        map[string]bool{"foo": true, "bar": true},
			b:        map[string]bool{"baz": true, "qux": true},
			expected: 0.0,
		},
		{
			name:     "partial overlap",
			a:        map[string]bool{"foo": true, "bar": true},
			b:        map[string]bool{"bar": true, "baz": true},
			expected: 1.0 / 3.0, // intersection=1, union=3
		},
		{
			name:     "empty sets",
			a:        map[string]bool{},
			b:        map[string]bool{},
			expected: 1.0,
		},
		{
			name:     "one empty set",
			a:        map[string]bool{"foo": true},
			b:        map[string]bool{},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jaccardSimilarity(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("jaccardSimilarity() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestHasSpecificValues(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{"version number", "Use version 1.2.3", true},
		{"environment variable", "Set MAX_RETRIES=5", true},
		{"json key", `"timeout": 30`, true},
		{"cli flag", "Run with --verbose", true},
		{"snake_case", "Use my_variable_name", true},
		{"camelCase", "Call myFunction here", true},
		{"no specific values", "This is generic text.", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSpecificValues(tt.content)
			if got != tt.expected {
				t.Errorf("hasSpecificValues(%q) = %v, want %v", tt.content, got, tt.expected)
			}
		})
	}
}

func TestHasConcreteExamples(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{"for example", "For example, you can use curl.", true},
		{"e.g.", "Use a tool (e.g. wget) to download.", true},
		{"such as", "Languages such as Go and Rust.", true},
		{"example:", "Example: curl localhost", true},
		{"output:", "Output: success", true},
		{"no examples", "This is some content.", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasConcreteExamples(tt.content)
			if got != tt.expected {
				t.Errorf("hasConcreteExamples(%q) = %v, want %v", tt.content, got, tt.expected)
			}
		})
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		name     string
		x        float64
		min      float64
		max      float64
		expected float64
	}{
		{"within range", 0.5, 0.0, 1.0, 0.5},
		{"below min", -0.5, 0.0, 1.0, 0.0},
		{"above max", 1.5, 0.0, 1.0, 1.0},
		{"at min", 0.0, 0.0, 1.0, 0.0},
		{"at max", 1.0, 0.0, 1.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clamp(tt.x, tt.min, tt.max)
			if got != tt.expected {
				t.Errorf("clamp(%v, %v, %v) = %v, want %v", tt.x, tt.min, tt.max, got, tt.expected)
			}
		})
	}
}

func TestScorerIntegration(t *testing.T) {
	// Create temp directory for test learnings
	tmpDir, err := os.MkdirTemp("", "warmind-scoring-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig().Scoring
	scorer := NewScorer(cfg, tmpDir)

	t.Run("high quality learning scores gold", func(t *testing.T) {
		candidate := Candidate{
			Title:      "Kubernetes Pod Resource Limits",
			Content:    "Always set resource limits on pods.\n\n```yaml\nresources:\n  limits:\n    memory: 128Mi\n    cpu: 500m\n```\n\n1. First, check current usage\n2. Set limits based on observed usage\n3. Monitor for OOMKills\n\nDon't set limits too low or pods will be killed.",
			Confidence: 0.9,
		}

		result := scorer.Score(candidate)

		if result.Tier != TierGold && result.Tier != TierSilver {
			t.Errorf("High quality learning got tier %v, want Gold or Silver", result.Tier)
		}
		if result.CompositeScore < 0.7 {
			t.Errorf("High quality learning got score %v, want >= 0.7", result.CompositeScore)
		}
	})

	t.Run("low quality learning scores discard", func(t *testing.T) {
		candidate := Candidate{
			Title:      "Maybe something",
			Content:    "This might work possibly.",
			Confidence: 0.2,
		}

		result := scorer.Score(candidate)

		if result.Tier != TierDiscard && result.Tier != TierBronze {
			t.Errorf("Low quality learning got tier %v, want Discard or Bronze", result.Tier)
		}
		if result.CompositeScore > 0.6 {
			t.Errorf("Low quality learning got score %v, want <= 0.6", result.CompositeScore)
		}
	})
}

func TestScorerWithExistingLearnings(t *testing.T) {
	// Create temp directory with existing learnings
	tmpDir, err := os.MkdirTemp("", "warmind-scoring-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create existing learning
	existingContent := `---
id: existing-001
---
# Kubernetes Debugging

When debugging Kubernetes issues, check pod logs first.
`
	err = os.WriteFile(filepath.Join(tmpDir, "existing-001.md"), []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write existing learning: %v", err)
	}

	cfg := DefaultConfig().Scoring
	scorer := NewScorer(cfg, tmpDir)

	t.Run("similar content gets lower novelty", func(t *testing.T) {
		candidate := Candidate{
			Content:    "When debugging Kubernetes, always check the pod logs.",
			Confidence: 0.8,
		}

		result := scorer.Score(candidate)

		if result.Novelty > 0.5 {
			t.Errorf("Similar content got novelty %v, want < 0.5", result.Novelty)
		}
	})

	t.Run("different content gets higher novelty", func(t *testing.T) {
		candidate := Candidate{
			Content:    "Python asyncio enables concurrent programming without threads.",
			Confidence: 0.8,
		}

		result := scorer.Score(candidate)

		if result.Novelty < 0.5 {
			t.Errorf("Different content got novelty %v, want > 0.5", result.Novelty)
		}
	})
}
