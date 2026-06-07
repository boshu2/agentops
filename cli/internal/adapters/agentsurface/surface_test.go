// practices: [wiki-knowledge-surface, design-by-contract]
package agentsurface

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseAllowlist(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name: "empty content",
			want: []string{},
		},
		{
			name:    "no markers",
			content: "ao\nlearnings\n",
			want:    []string{},
		},
		{
			name: "basic allowlist",
			content: `# heading
<!-- BEGIN agents-write-surfaces-allowlist -->
ao
learnings
patterns
<!-- END agents-write-surfaces-allowlist -->
trailing
`,
			want: []string{"ao", "learnings", "patterns"},
		},
		{
			name: "inline comments and blanks",
			content: `<!-- BEGIN agents-write-surfaces-allowlist -->

# core
ao   # core runtime

# promoted
learnings   # promoted artifacts
<!-- END agents-write-surfaces-allowlist -->
`,
			want: []string{"ao", "learnings"},
		},
		{
			name: "duplicates dedup and sorted",
			content: `<!-- BEGIN agents-write-surfaces-allowlist -->
patterns
ao
learnings
ao
<!-- END agents-write-surfaces-allowlist -->
`,
			want: []string{"ao", "learnings", "patterns"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAllowlist(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseAllowlist() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseAllowlist_ShellParity locks parity between the Go parser and the
// shell parser pipeline embedded in scripts/check-agents-write-surfaces.sh.
func TestParseAllowlist_ShellParity(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if _, err := exec.LookPath("awk"); err != nil {
		t.Skip("awk not available")
	}

	fixture := `# AgentOps Write Surface Contract

Some doc prose.

<!-- BEGIN agents-write-surfaces-allowlist -->
ao
learnings   # promoted artifacts
patterns
ao

# core runtime
findings
overnight   # dream output
sessions
<!-- END agents-write-surfaces-allowlist -->

trailing prose.
`

	goOut := ParseAllowlist(fixture)

	pipeline := `awk '
  /^[[:space:]]*<!-- BEGIN agents-write-surfaces-allowlist -->[[:space:]]*$/ { inside=1; next }
  /^[[:space:]]*<!-- END agents-write-surfaces-allowlist -->[[:space:]]*$/   { inside=0; next }
  inside { print }
' \
  | sed -E 's/[[:space:]]+#.*$//' \
  | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' \
  | awk 'NF && $1 !~ /^#/' \
  | sort -u`

	cmd := exec.Command("bash", "-c", pipeline)
	cmd.Stdin = strings.NewReader(fixture)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("shell pipeline failed: %v\nstderr: %s", err, stderr.String())
	}

	rawShell := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	shellOut := []string{}
	for _, l := range rawShell {
		if l != "" {
			shellOut = append(shellOut, l)
		}
	}

	if !reflect.DeepEqual(goOut, shellOut) {
		t.Errorf("shell-parser/Go-parser allowlist drift:\n  go:    %v\n  shell: %v", goOut, shellOut)
	}

	want := []string{"ao", "findings", "learnings", "overnight", "patterns", "sessions"}
	if !reflect.DeepEqual(goOut, want) {
		t.Errorf("Go parser fixture-lock failure: got %v, want %v", goOut, want)
	}
}

func TestDiscoverActiveSkills(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(tmp, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("ok"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(tmp, "no-skill-md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "stray.md"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := DiscoverActiveSkills(tmp)
	want := []string{"alpha", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DiscoverActiveSkills() = %v, want %v", got, want)
	}
}

func TestDiscoverActiveSkills_MissingDir(t *testing.T) {
	got := DiscoverActiveSkills(filepath.Join(t.TempDir(), "nope"))
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}
