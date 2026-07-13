package ports

import (
	"context"
	"testing"
)

func TestInMemoryFindingCompiler_PremortemAliasStillWritesOneLegacyArtifactThroughS7(t *testing.T) {
	out, err := NewInMemoryFindingCompiler().Compile(context.Background(), FindingArtifact{
		ID: "finding-mortem-writer",
		Frontmatter: map[string]string{
			"compiler_targets": "premortem,pre-mortem,pre_mortem",
		},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("mortem aliases emitted %d artifacts, want exactly one", len(out))
	}
	if out[0].Path != ".agents/pre-mortem-checks/finding-mortem-writer.md" {
		t.Fatalf("writer path = %q, want legacy path through S7", out[0].Path)
	}
}
