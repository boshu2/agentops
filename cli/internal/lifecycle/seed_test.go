package lifecycle

import (
	"testing"
	"testing/fstest"
)

func TestValidateTemplateMapEntries_AllPresent(t *testing.T) {
	fsys := fstest.MapFS{
		"templates/go-cli.yaml":     &fstest.MapFile{Data: []byte("x")},
		"templates/python-lib.yaml": &fstest.MapFile{Data: []byte("x")},
		"templates/generic.yaml":    &fstest.MapFile{Data: []byte("x")},
	}
	templates := map[string]bool{"go-cli": true, "python-lib": true, "generic": true, "rust-cli": false}
	if err := ValidateTemplateMapEntries(templates, fsys); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateTemplateMapEntries_MissingFile(t *testing.T) {
	fsys := fstest.MapFS{"templates/go-cli.yaml": &fstest.MapFile{Data: []byte("x")}}
	templates := map[string]bool{"go-cli": true, "web-app": true}
	err := ValidateTemplateMapEntries(templates, fsys)
	if err == nil {
		t.Fatalf("expected error for missing web-app.yaml")
	}
	if !contains(err.Error(), "web-app") {
		t.Errorf("error should mention missing template name: %v", err)
	}
}

func TestBuildSeedGoalFile_KnownTemplate(t *testing.T) {
	g := BuildSeedGoalFile("/tmp/myproj", "go-cli")
	if g == nil {
		t.Fatal("expected non-nil GoalFile")
	}
	if g.Version != 4 {
		t.Errorf("Version = %d, want 4", g.Version)
	}
	if g.Format != "md" {
		t.Errorf("Format = %q, want md", g.Format)
	}
	if !contains(g.Mission, "myproj") {
		t.Errorf("Mission should contain project basename, got %q", g.Mission)
	}
	if !contains(g.Mission, "(Go CLI)") {
		t.Errorf("Mission should contain suffix, got %q", g.Mission)
	}
	if len(g.NorthStars) == 0 {
		t.Error("expected NorthStars populated")
	}
	if len(g.Directives) == 0 {
		t.Error("expected Directives populated")
	}
}

func TestBuildSeedGoalFile_UnknownTemplateFallsBackToGeneric(t *testing.T) {
	g := BuildSeedGoalFile("/x/proj", "no-such-template")
	generic := TemplateConfigs["generic"]
	if len(g.Directives) != len(generic.Directives) {
		t.Errorf("expected generic directives (%d), got %d", len(generic.Directives), len(g.Directives))
	}
}

func TestValidTemplates_ContainsExpectedSet(t *testing.T) {
	expected := []string{"go-cli", "python-lib", "web-app", "rust-cli", "generic"}
	for _, name := range expected {
		if !ValidTemplates[name] {
			t.Errorf("ValidTemplates missing %q", name)
		}
	}
	if ValidTemplates["fake"] {
		t.Error("ValidTemplates should not contain fake")
	}
}

func TestHasSeedMarker(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"empty", "", false},
		{"current marker", "before\n## AgentOps Knowledge Flywheel\nafter", true},
		{"legacy marker", "before\n## AgentOps Session Protocol\nafter", true},
		{"no marker", "some random content", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasSeedMarker(tc.content); got != tc.want {
				t.Errorf("HasSeedMarker(%q) = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}

func TestFindSeedMarker(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"empty returns empty", "", ""},
		{"current marker", "x\n## AgentOps Knowledge Flywheel\ny", ClaudeMDSeedMarker},
		{"legacy marker only", "x\n## AgentOps Session Protocol\ny", ClaudeMDSeedMarkerLegacy},
		{"current wins over legacy when both present", "## AgentOps Knowledge Flywheel\n## AgentOps Session Protocol", ClaudeMDSeedMarker},
		{"no marker", "nothing here", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FindSeedMarker(tc.content); got != tc.want {
				t.Errorf("FindSeedMarker = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTemplateConfigs_AllTemplatesHaveDirectives(t *testing.T) {
	for name, cfg := range TemplateConfigs {
		if len(cfg.Directives) == 0 {
			t.Errorf("template %q has no directives", name)
		}
		if len(cfg.NorthStars) == 0 {
			t.Errorf("template %q has no north stars", name)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
