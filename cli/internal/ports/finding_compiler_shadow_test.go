package ports

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInMemoryFindingCompiler_ShadowRequiresStoredPositivesAndNegativeControls(t *testing.T) {
	compiler := NewInMemoryFindingCompiler()
	withoutEvidence, err := compiler.Compile(context.Background(), FindingArtifact{
		ID:          "f-shadow",
		Frontmatter: mechanicalFM("constraint"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(withoutEvidence) != 0 {
		t.Fatalf("mechanical metadata without replay evidence must remain advisory, got %+v", withoutEvidence)
	}

	withEvidence, err := compiler.Compile(context.Background(), FindingArtifact{
		ID:               "f-shadow",
		Frontmatter:      mechanicalFM("constraint"),
		DetectorEvidence: loadDetectorEvidenceFixture(t, "replay-pass.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(withEvidence) != 1 || withEvidence[0].Kind != CompiledOutputConstraint {
		t.Fatalf("passing positive and negative replay must emit one shadow constraint, got %+v", withEvidence)
	}
}

func loadDetectorEvidenceFixture(t *testing.T, name string) *DetectorEvidence {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve fixture caller")
	}
	path := filepath.Join(filepath.Dir(here), "..", "..", "..", "tests", "fixtures", "constraint-shadow", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var evidence DetectorEvidence
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	return &evidence
}
