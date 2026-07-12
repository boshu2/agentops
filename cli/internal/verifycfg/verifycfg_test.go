package verifycfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// allEnvVars is every canonical env var the resolver reads. Tests clear these
// up front so a runner that happens to export a PAWL_* var cannot poison an
// assertion about default/file provenance.
var allEnvVars = []string{
	"PAWL_REVIEWER_CHAIN", "PAWL_REVIEW_TIMEOUT", "PAWL_STRICT",
	"PAWL_SMOKE_CMD", "PAWL_AUTOBIND", "PAWL_AUTHOR_FAMILY",
}

// isolate unsets every canonical env var and restores the prior values on
// cleanup, giving each test a hermetic environment.
func isolate(t *testing.T) {
	t.Helper()
	for _, name := range allEnvVars {
		t.Setenv(name, "") // snapshots + auto-restores the prior set/unset state
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}
}

// mkRepo creates a temp dir with a ".git" directory marker and, when yamlContent
// is non-empty, an .aoverify.yaml. It returns the repo root.
func mkRepo(t *testing.T, yamlContent string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if yamlContent != "" {
		writeConfig(t, root, yamlContent)
	}
	return root
}

func writeConfig(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestLoadDir_AbsentFileUsesDefaults(t *testing.T) {
	isolate(t)
	root := mkRepo(t, "")
	got := LoadDir(root)

	if got.FileFound {
		t.Errorf("FileFound = true, want false (no config file)")
	}
	if got.ConfigPath != "" {
		t.Errorf("ConfigPath = %q, want empty", got.ConfigPath)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", got.Warnings)
	}
	// Exact defaults.
	if got.ReviewerChain != "" {
		t.Errorf("ReviewerChain = %q, want empty", got.ReviewerChain)
	}
	if got.ReviewTimeout != 300 {
		t.Errorf("ReviewTimeout = %d, want 300", got.ReviewTimeout)
	}
	if got.Strict {
		t.Errorf("Strict = true, want false")
	}
	if got.Smoke != "" {
		t.Errorf("Smoke = %q, want empty", got.Smoke)
	}
	if !got.Autobind {
		t.Errorf("Autobind = false, want true (default ON)")
	}
	if got.AuthorFamily != "claude" {
		t.Errorf("AuthorFamily = %q, want claude", got.AuthorFamily)
	}
	for _, e := range got.Entries() {
		if e.Source != SourceDefault {
			t.Errorf("Entry %s source = %s, want default", e.Key, e.Source)
		}
	}
	// Byte-identical zero-config lock: the shell bridge emits nothing.
	if ee := got.ExportEnv(); ee != "" {
		t.Errorf("ExportEnv() = %q, want empty for zero-config", ee)
	}
}

func TestLoadDir_ValidFile_AllFromFile(t *testing.T) {
	isolate(t)
	root := mkRepo(t, `reviewer_chain: "gpt,claude"
review_timeout: 600
strict: true
smoke: "make test"
autobind: false
author_family: gpt5
`)
	got := LoadDir(root)

	if !got.FileFound {
		t.Fatalf("FileFound = false, want true")
	}
	if want := filepath.Join(root, ConfigFileName); got.ConfigPath != want {
		t.Errorf("ConfigPath = %q, want %q", got.ConfigPath, want)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", got.Warnings)
	}
	checks := []struct {
		key    string
		gotVal string
		want   string
	}{
		{"reviewer_chain", got.ReviewerChain, "gpt,claude"},
		{"review_timeout", got.valueString("review_timeout"), "600"},
		{"strict", got.valueString("strict"), "true"},
		{"smoke", got.Smoke, "make test"},
		{"autobind", got.valueString("autobind"), "false"},
		{"author_family", got.AuthorFamily, "gpt5"},
	}
	for _, c := range checks {
		if c.gotVal != c.want {
			t.Errorf("%s = %q, want %q", c.key, c.gotVal, c.want)
		}
		if got.Source(c.key) != SourceFile {
			t.Errorf("%s source = %s, want file", c.key, got.Source(c.key))
		}
	}
}

func TestLoadDir_UnknownKey_WarnsOnceNeverFails(t *testing.T) {
	isolate(t)
	root := mkRepo(t, `review_timeout: 600
zzz_unknown: 1
aaa_unknown: hello
`)
	got := LoadDir(root)

	// Known key still applied.
	if got.ReviewTimeout != 600 || got.Source("review_timeout") != SourceFile {
		t.Errorf("review_timeout = %d (%s), want 600 (file)", got.ReviewTimeout, got.Source("review_timeout"))
	}
	// Exactly one warning, listing both unknown keys sorted.
	if len(got.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want exactly 1", got.Warnings)
	}
	want := got.ConfigPath + ": unknown key(s) ignored: aaa_unknown, zzz_unknown"
	if got.Warnings[0] != want {
		t.Errorf("warning = %q, want %q", got.Warnings[0], want)
	}
}

func TestLoadDir_EnvOverridesFile(t *testing.T) {
	isolate(t)
	root := mkRepo(t, "review_timeout: 600\nautobind: false\n")
	t.Setenv("PAWL_REVIEW_TIMEOUT", "999")
	got := LoadDir(root)

	if got.ReviewTimeout != 999 {
		t.Errorf("ReviewTimeout = %d, want 999 (env wins)", got.ReviewTimeout)
	}
	if got.Source("review_timeout") != SourceEnv {
		t.Errorf("review_timeout source = %s, want env", got.Source("review_timeout"))
	}
	// A key set only in the file keeps its file provenance.
	if got.Autobind {
		t.Errorf("Autobind = true, want false (from file)")
	}
	if got.Source("autobind") != SourceFile {
		t.Errorf("autobind source = %s, want file", got.Source("autobind"))
	}
}

func TestLoadDir_EnvOverridesDefault(t *testing.T) {
	isolate(t)
	root := mkRepo(t, "") // no file
	t.Setenv("PAWL_AUTOBIND", "false")
	got := LoadDir(root)

	if got.Autobind {
		t.Errorf("Autobind = true, want false (env over default)")
	}
	if got.Source("autobind") != SourceEnv {
		t.Errorf("autobind source = %s, want env", got.Source("autobind"))
	}
	if got.Source("review_timeout") != SourceDefault {
		t.Errorf("review_timeout source = %s, want default", got.Source("review_timeout"))
	}
}

// TestLoadDir_StrangerRepoRootFromSubdir proves the STRANGER repo's config is
// read (root walk-up), not some ancestor's — the load-bearing multi-repo case.
func TestLoadDir_StrangerRepoRootFromSubdir(t *testing.T) {
	isolate(t)
	repoA := mkRepo(t, "review_timeout: 111\nauthor_family: aaa\n")
	repoB := mkRepo(t, "review_timeout: 222\nauthor_family: bbb\n")
	subB := filepath.Join(repoB, "deep", "nested")
	if err := os.MkdirAll(subB, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}

	got := LoadDir(subB)

	if got.ReviewTimeout != 222 || got.AuthorFamily != "bbb" {
		t.Errorf("read wrong repo: review_timeout=%d author_family=%q, want 222/bbb", got.ReviewTimeout, got.AuthorFamily)
	}
	if want := filepath.Join(repoB, ConfigFileName); got.ConfigPath != want {
		t.Errorf("ConfigPath = %q, want %q (repoB, not repoA %q)", got.ConfigPath, want, repoA)
	}
}

// TestLoadDir_GitFileWorktree covers a linked worktree where ".git" is a FILE.
func TestLoadDir_GitFileWorktree(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}
	writeConfig(t, root, "review_timeout: 42\n")
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}

	got := LoadDir(sub)
	if got.ReviewTimeout != 42 {
		t.Errorf("ReviewTimeout = %d, want 42 (root via .git file)", got.ReviewTimeout)
	}
	if want := filepath.Join(root, ConfigFileName); got.ConfigPath != want {
		t.Errorf("ConfigPath = %q, want %q", got.ConfigPath, want)
	}
}

func TestLoadDir_NoRepoUsesStartDir(t *testing.T) {
	isolate(t)
	// A dir with a config but NO .git anywhere up the tree resolves to start dir.
	root := t.TempDir()
	writeConfig(t, root, "review_timeout: 7\n")
	got := LoadDir(root)
	if got.ReviewTimeout != 7 || !got.FileFound {
		t.Errorf("ReviewTimeout = %d FileFound=%v, want 7/true (start-dir fallback)", got.ReviewTimeout, got.FileFound)
	}
}

func TestLoadDir_MalformedYamlHoldsVerification(t *testing.T) {
	isolate(t)
	root := mkRepo(t, "- not\n- a\n- mapping\n")
	got := LoadDir(root)

	if !got.FileFound {
		t.Errorf("FileFound = false, want true (file exists though malformed)")
	}
	if !got.Strict {
		t.Errorf("Strict = false, want safe HOLD posture after parse failure")
	}
	if got.Autobind {
		t.Errorf("Autobind = true, want disabled after parse failure")
	}
	if len(got.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want exactly 1", got.Warnings)
	}
	if got, want := got.Warnings[0], "cannot parse"; !strings.Contains(got, want) {
		t.Errorf("warning = %q, want it to contain %q", got, want)
	}
}

func TestLoadDir_BadTypedCommittedFieldHoldsVerification(t *testing.T) {
	for _, tt := range []struct {
		name    string
		content string
		field   string
	}{
		{name: "strict", content: "strict: definitely-not-a-bool\n", field: "strict"},
		{name: "autobind", content: "autobind: definitely-not-a-bool\n", field: "autobind"},
		{name: "review timeout", content: "review_timeout: not_an_int\n", field: "review_timeout"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			isolate(t)
			root := mkRepo(t, tt.content)
			got := LoadDir(root)

			if err := got.ValidationError(); err == nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("ValidationError() = %v, want invalid committed %s policy", err, tt.field)
			}
			if !got.Strict {
				t.Error("Strict = false, want safe HOLD posture for invalid committed typed field")
			}
			if got.Autobind {
				t.Error("Autobind = true, want disabled for invalid committed typed field")
			}
			if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], tt.field) {
				t.Errorf("Warnings = %v, want one warning naming %s", got.Warnings, tt.field)
			}
		})
	}
}

func TestApplyInt_BadEnvFallsThrough(t *testing.T) {
	isolate(t)
	root := mkRepo(t, "review_timeout: 600\n")
	t.Setenv("PAWL_REVIEW_TIMEOUT", "notanint")
	got := LoadDir(root)

	if got.ReviewTimeout != 600 || got.Source("review_timeout") != SourceFile {
		t.Errorf("review_timeout = %d (%s), want 600 (file, env ignored)", got.ReviewTimeout, got.Source("review_timeout"))
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "not an integer") {
		t.Errorf("Warnings = %v, want one 'not an integer'", got.Warnings)
	}
}

func TestApplyBool_BadEnvFallsThrough(t *testing.T) {
	isolate(t)
	root := mkRepo(t, "") // no file -> default true
	t.Setenv("PAWL_AUTOBIND", "maybe")
	got := LoadDir(root)

	if !got.Autobind || got.Source("autobind") != SourceDefault {
		t.Errorf("autobind = %v (%s), want true (default, env ignored)", got.Autobind, got.Source("autobind"))
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "not a boolean") {
		t.Errorf("Warnings = %v, want one 'not a boolean'", got.Warnings)
	}
}

func TestParseBool(t *testing.T) {
	truthy := []string{"1", "true", "TRUE", "yes", "on", " On "}
	falsy := []string{"0", "false", "FALSE", "no", "off", " Off "}
	bad := []string{"", "maybe", "2", "y"}
	for _, s := range truthy {
		if v, ok := parseBool(s); !ok || !v {
			t.Errorf("parseBool(%q) = (%v,%v), want (true,true)", s, v, ok)
		}
	}
	for _, s := range falsy {
		if v, ok := parseBool(s); !ok || v {
			t.Errorf("parseBool(%q) = (%v,%v), want (false,true)", s, v, ok)
		}
	}
	for _, s := range bad {
		if _, ok := parseBool(s); ok {
			t.Errorf("parseBool(%q) ok = true, want false", s)
		}
	}
}

func TestExportEnv_OnlyNonDefault_ShellForm(t *testing.T) {
	isolate(t)
	root := mkRepo(t, "review_timeout: 600\nautobind: false\nauthor_family: gpt5\n")
	got := LoadDir(root).ExportEnv()

	// keyOrder order; defaults (reviewer_chain, strict, smoke) omitted; bool as 0/1.
	want := "export PAWL_REVIEW_TIMEOUT='600'\n" +
		"export PAWL_AUTOBIND='0'\n" +
		"export PAWL_AUTHOR_FAMILY='gpt5'\n"
	if got != want {
		t.Errorf("ExportEnv() =\n%q\nwant\n%q", got, want)
	}
}

func TestExportEnv_EnvSourcedReemitted(t *testing.T) {
	isolate(t)
	root := mkRepo(t, "") // no file
	t.Setenv("PAWL_REVIEW_TIMEOUT", "888")
	got := LoadDir(root).ExportEnv()
	want := "export PAWL_REVIEW_TIMEOUT='888'\n"
	if got != want {
		t.Errorf("ExportEnv() = %q, want %q", got, want)
	}
}

func TestExportEnv_ShellQuoteEscapesSingleQuote(t *testing.T) {
	isolate(t)
	root := mkRepo(t, "smoke: \"it's ok\"\n")
	got := LoadDir(root).ExportEnv()
	// it's ok  ->  'it'\''s ok'
	want := `export PAWL_SMOKE_CMD='it'\''s ok'` + "\n"
	if got != want {
		t.Errorf("ExportEnv() = %q, want %q", got, want)
	}
}

func TestEntries_OrderAndSources(t *testing.T) {
	isolate(t)
	root := mkRepo(t, "review_timeout: 600\n")
	t.Setenv("PAWL_AUTHOR_FAMILY", "envfam")
	entries := LoadDir(root).Entries()

	wantKeys := []string{"reviewer_chain", "review_timeout", "strict", "smoke", "autobind", "author_family"}
	if len(entries) != len(wantKeys) {
		t.Fatalf("Entries len = %d, want %d", len(entries), len(wantKeys))
	}
	for i, e := range entries {
		if e.Key != wantKeys[i] {
			t.Errorf("Entries[%d].Key = %q, want %q", i, e.Key, wantKeys[i])
		}
	}
	bySource := map[string]Source{}
	for _, e := range entries {
		bySource[e.Key] = e.Source
	}
	if bySource["review_timeout"] != SourceFile {
		t.Errorf("review_timeout source = %s, want file", bySource["review_timeout"])
	}
	if bySource["author_family"] != SourceEnv {
		t.Errorf("author_family source = %s, want env", bySource["author_family"])
	}
	if bySource["strict"] != SourceDefault {
		t.Errorf("strict source = %s, want default", bySource["strict"])
	}
}
