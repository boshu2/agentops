package ports

import (
	"context"
	"testing"
)

func TestInMemoryFindingCompiler_PremortemAliasesWriteOneCanonicalArtifact(t *testing.T) {
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
	if out[0].Path != ".agents/premortem-checks/finding-mortem-writer.md" {
		t.Fatalf("writer path = %q, want canonical premortem path", out[0].Path)
	}
}
