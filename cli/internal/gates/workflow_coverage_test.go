package gates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryWorkflowCoverageReportsMissingAndRegistryOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	workflow := []byte(`name: Validate
jobs:
  one:
    runs-on: ubuntu-latest
    steps:
      - name: covered
        run: bash scripts/check-covered.sh
      - name: missing
        run: ./scripts/check-missing.sh
      - name: advisory
        continue-on-error: true
        run: |
          chmod +x scripts/check-advisory.sh
          echo "run scripts/check-echo-only.sh if this fails"
          ./scripts/check-advisory.sh
      - name: deferred
        run: scripts/skill-probe-i0.sh skills .agents/ao/skill-eval
`)
	if err := os.WriteFile(filepath.Join(root, ".github", "workflows", "validate.yml"), workflow, 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry()
	if err := reg.Add(Check{ID: "covered", Tiers: Full, Blocking: true, Backing: "check-covered.sh"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Add(Check{ID: "extra", Tiers: Full, Blocking: true, Backing: "check-extra.sh"}); err != nil {
		t.Fatal(err)
	}

	got, err := RegistryWorkflowCoverage(reg, root, ".github/workflows/validate.yml")
	if err != nil {
		t.Fatalf("RegistryWorkflowCoverage: %v", err)
	}
	if got.WorkflowScriptCount != 4 {
		t.Fatalf("WorkflowScriptCount = %d, want 4", got.WorkflowScriptCount)
	}
	if got.RegistryScriptCount != 2 {
		t.Fatalf("RegistryScriptCount = %d, want 2", got.RegistryScriptCount)
	}
	if got.MissingScriptCount != 3 {
		t.Fatalf("MissingScripts = %+v, want 3 total missing", got.MissingScripts)
	}
	if got.MissingBlockingCount != 1 || got.MissingBlockingScripts[0] != "scripts/check-missing.sh" {
		t.Fatalf("MissingBlockingScripts = %+v, want check-missing", got.MissingBlockingScripts)
	}
	if got.MissingAdvisoryCount != 1 || got.MissingAdvisoryScripts[0] != "scripts/check-advisory.sh" {
		t.Fatalf("MissingAdvisoryScripts = %+v, want check-advisory", got.MissingAdvisoryScripts)
	}
	if got.MissingDeferredCount != 1 || got.MissingDeferredScripts[0].Script != "scripts/skill-probe-i0.sh" {
		t.Fatalf("MissingDeferredScripts = %+v, want skill-probe-i0", got.MissingDeferredScripts)
	}
	if got.MissingScripts[0] != "scripts/check-advisory.sh" ||
		got.MissingScripts[1] != "scripts/check-missing.sh" ||
		got.MissingScripts[2] != "scripts/skill-probe-i0.sh" {
		t.Fatalf("MissingScripts = %+v, want check-missing", got.MissingScripts)
	}
	if got.RegistryOnlyScriptCount != 1 || got.RegistryOnlyScripts[0] != "scripts/check-extra.sh" {
		t.Fatalf("RegistryOnlyScripts = %+v, want check-extra", got.RegistryOnlyScripts)
	}
}
