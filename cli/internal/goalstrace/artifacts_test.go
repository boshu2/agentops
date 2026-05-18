package goalstrace

import (
	"path/filepath"
	"testing"
)

// rpiArtifactRel is the fixture verdict artifact path produced by bead
// soc-58nt.2.6, in slash form relative to the fixture project root.
func rpiArtifactRel() string {
	return filepath.ToSlash(filepath.Join(".agents", "rpi", "runs",
		"2026-05-17-soc-58nt.2.6", "verdict.md"))
}

// orphanArtifactRel is the fixture artifact whose declared bead_id resolves to
// no bead the fixture querier serves.
func orphanArtifactRel() string {
	return filepath.ToSlash(filepath.Join(".agents", "rpi", "runs",
		"2026-05-17-soc-58nt.9.9", "plan.md"))
}

func TestParseRPIArtifact_ExtractsBeadID(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "frontmatter bead_id is extracted",
			content: "---\nbead_id: soc-58nt.2.6\nrun_id: r1\n---\nbody",
			want:    "soc-58nt.2.6",
		},
		{
			name:    "quoted bead_id is trimmed",
			content: "---\nbead_id: \"soc-58nt.4.2\"\n---\nbody",
			want:    "soc-58nt.4.2",
		},
		{
			name:    "no frontmatter yields empty bead_id",
			content: "# plain artifact\nno frontmatter here",
			want:    "",
		},
		{
			name:    "unterminated frontmatter yields empty bead_id",
			content: "---\nbead_id: soc-58nt.2.6\nno closing fence",
			want:    "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := parseRPIArtifact("artifact.md", c.content)
			if a.beadID != c.want {
				t.Errorf("beadID = %q, want %q", a.beadID, c.want)
			}
		})
	}
}

func TestArtifactBeadLink_ConfidenceClassification(t *testing.T) {
	cases := []struct {
		name     string
		artifact rpiArtifact
		wantBead string
		wantConf Confidence
		wantPath bool
	}{
		{
			name:     "frontmatter bead_id is high confidence",
			artifact: rpiArtifact{relPath: "x/verdict.md", beadID: "soc-58nt.2.6"},
			wantBead: "soc-58nt.2.6", wantConf: ConfidenceHigh, wantPath: false,
		},
		{
			name:     "bead ID in path is high confidence via path",
			artifact: rpiArtifact{relPath: ".agents/rpi/runs/2026-05-17-soc-58nt.4.8/plan.md"},
			wantBead: "soc-58nt.4.8", wantConf: ConfidenceHigh, wantPath: true,
		},
		{
			name:     "bead token in body only is low confidence",
			artifact: rpiArtifact{relPath: "plain/doc.md", body: "mentions soc-58nt.3.1 in prose"},
			wantBead: "soc-58nt.3.1", wantConf: ConfidenceLow, wantPath: false,
		},
		{
			name:     "no bead signal yields empty bead",
			artifact: rpiArtifact{relPath: "plain/doc.md", body: "no bead tokens here"},
			wantBead: "", wantConf: ConfidenceLow, wantPath: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bead, conf, viaPath := artifactBeadLink(c.artifact)
			if bead != c.wantBead {
				t.Errorf("beadID = %q, want %q", bead, c.wantBead)
			}
			if conf != c.wantConf {
				t.Errorf("confidence = %q, want %q", conf, c.wantConf)
			}
			if viaPath != c.wantPath {
				t.Errorf("viaPath = %v, want %v", viaPath, c.wantPath)
			}
		})
	}
}

func TestLearningCitesArtifact_ConfidenceClassification(t *testing.T) {
	artifact := ".agents/rpi/runs/2026-05-17-soc-58nt.2.6/verdict.md"
	cases := []struct {
		name      string
		learning  learningFile
		wantCited bool
		wantConf  Confidence
	}{
		{
			name:      "frontmatter source exact match is high",
			learning:  learningFile{fm: frontmatterFields{source: artifact}},
			wantCited: true, wantConf: ConfidenceHigh,
		},
		{
			name:      "frontmatter source with leading ./ still matches high",
			learning:  learningFile{fm: frontmatterFields{source: "./" + artifact}},
			wantCited: true, wantConf: ConfidenceHigh,
		},
		{
			name:      "verbatim path in body is high",
			learning:  learningFile{body: "see " + artifact + " for details"},
			wantCited: true, wantConf: ConfidenceHigh,
		},
		{
			name:      "bare filename in body is low",
			learning:  learningFile{body: "the verdict.md says it passed"},
			wantCited: true, wantConf: ConfidenceLow,
		},
		{
			name:      "no citation is not cited",
			learning:  learningFile{body: "unrelated learning text"},
			wantCited: false, wantConf: ConfidenceLow,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cited, conf := learningCitesArtifact(c.learning, artifact)
			if cited != c.wantCited {
				t.Errorf("cited = %v, want %v", cited, c.wantCited)
			}
			if cited && conf != c.wantConf {
				t.Errorf("confidence = %q, want %q", conf, c.wantConf)
			}
		})
	}
}

func TestWalk_EmitsBeadProducedArtifactEdge(t *testing.T) {
	g, err := Walk(Options{
		ProjectRoot: fixtureRoot(t),
		Beads:       NewStaticBeadQuerier(true, fixtureBeads()),
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}
	e, ok := findEdge(g, EdgeBeadProducedArtifact, "soc-58nt.2.6", rpiArtifactRel())
	if !ok {
		t.Fatalf("missing bead_produced_artifact edge soc-58nt.2.6->%s; edges=%+v",
			rpiArtifactRel(), g.Edges)
	}
	if e.Confidence != ConfidenceHigh {
		t.Errorf("confidence = %q, want high", e.Confidence)
	}
	if len(e.Defects) != 0 {
		t.Errorf("known-bead artifact must have no defects, got %+v", e.Defects)
	}
	if !hasNode(g, rpiArtifactRel(), NodeArtifact) {
		t.Errorf("missing artifact node %s", rpiArtifactRel())
	}
}

func TestWalk_BrokenArtifactBeadRefIsError(t *testing.T) {
	g, err := Walk(Options{
		ProjectRoot: fixtureRoot(t),
		Beads:       NewStaticBeadQuerier(true, fixtureBeads()),
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}
	e, ok := findEdge(g, EdgeBeadProducedArtifact, "soc-58nt.9.9", orphanArtifactRel())
	if !ok {
		t.Fatalf("missing bead_produced_artifact edge for orphan artifact")
	}
	if len(e.Defects) != 1 {
		t.Fatalf("want exactly 1 defect, got %d", len(e.Defects))
	}
	if e.Defects[0].Code != DefectBrokenArtifactBeadRef {
		t.Errorf("defect code = %q, want %q", e.Defects[0].Code, DefectBrokenArtifactBeadRef)
	}
	if e.Defects[0].Severity != SeverityError {
		t.Errorf("severity = %q, want error", e.Defects[0].Severity)
	}
}

func TestWalk_EmitsArtifactCitedByLearningEdge(t *testing.T) {
	g, err := Walk(Options{
		ProjectRoot: fixtureRoot(t),
		Beads:       NewStaticBeadQuerier(true, fixtureBeads()),
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}
	learningRel := filepath.ToSlash(filepath.Join("docs", "learnings", "2026-05-17-trace-chain.md"))
	e, ok := findEdge(g, EdgeArtifactCitedByLearning, rpiArtifactRel(), learningRel)
	if !ok {
		t.Fatalf("missing artifact_cited_by_learning edge %s->%s; edges=%+v",
			rpiArtifactRel(), learningRel, g.Edges)
	}
	if e.Confidence != ConfidenceHigh {
		t.Errorf("confidence = %q, want high (exact source-path match)", e.Confidence)
	}
}

func TestWalk_MissingRPIRunsDirDegradesGracefully(t *testing.T) {
	// no-learnings fixture has no .agents/rpi/runs/ tree.
	root := filepath.Join(fixtureRoot(t), "no-learnings")
	g, err := Walk(Options{
		ProjectRoot: root,
		Beads:       NewStaticBeadQuerier(false, nil),
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}
	found := false
	for _, d := range g.Diagnostics {
		if contains(d, ".agents/rpi/runs/") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an .agents/rpi/runs/ diagnostic, got %v", g.Diagnostics)
	}
	for _, e := range g.Edges {
		if e.Type == EdgeBeadProducedArtifact {
			t.Errorf("no bead_produced_artifact edge expected, got %+v", e)
		}
	}
}
