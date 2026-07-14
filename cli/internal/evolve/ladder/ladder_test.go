// practices: [dora-metrics, lean-startup]
package ladder

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/trackerexec"
	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

// fakeBeadRunner is a test double implementing BeadRunner.
type fakeBeadRunner struct {
	ReadyList      []Bead
	ReadyErr       error
	ReadyByTypeMap map[string][]Bead
	ReadyByTypeErr error
	ShowMap        map[string]Bead
	ShowErr        error
	InProgressList []Bead
	InProgressErr  error
}

func (f *fakeBeadRunner) Ready(ctx context.Context) ([]Bead, error) {
	return f.ReadyList, f.ReadyErr
}

func (f *fakeBeadRunner) ReadyByType(ctx context.Context, t string) ([]Bead, error) {
	return f.ReadyByTypeMap[t], f.ReadyByTypeErr
}

func (f *fakeBeadRunner) Show(ctx context.Context, id string) (Bead, error) {
	if f.ShowErr != nil {
		return Bead{}, f.ShowErr
	}
	b, ok := f.ShowMap[id]
	if !ok {
		return Bead{}, errors.New("not found: " + id)
	}
	return b, nil
}

func (f *fakeBeadRunner) InProgress(ctx context.Context) ([]Bead, error) {
	return f.InProgressList, f.InProgressErr
}

func TestEvolveNextWorkBDUsesResolvedTrackerContext(t *testing.T) {
	for _, tracker := range []string{trackerresolve.BR, trackerresolve.BD} {
		t.Run(tracker+" canonical process context and argv", func(t *testing.T) {
			root := t.TempDir()
			binary := filepath.Join(t.TempDir(), tracker)
			tracePath := filepath.Join(t.TempDir(), "trace")
			script := `#!/bin/sh
printf 'pwd=%s|beads=%s|argv=%s\n' "$PWD" "${BEADS_DIR-<unset>}" "$*" >> "$TRACKER_TRACE"
case "${1:-}" in
  show) printf '{"id":"ag-1"}\n' ;;
  *) printf '[]\n' ;;
esac
`
			if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			env := []string{
				"AGENTOPS_TRACKER=" + tracker,
				"BEADS_DIR=" + filepath.Join(t.TempDir(), "ambient-ledger"),
				"TRACKER_TRACE=" + tracePath,
			}
			resolution, err := trackerresolve.ResolveWithLookPath(root, env, func(string) (string, error) {
				return binary, nil
			})
			if err != nil {
				t.Fatalf("resolve tracker: %v", err)
			}
			runner := ExecBeadRunner{Resolution: resolution}
			if _, err := runner.Ready(context.Background()); err != nil {
				t.Fatalf("Ready: %v", err)
			}
			if _, err := runner.ReadyByType(context.Background(), "bug"); err != nil {
				t.Fatalf("ReadyByType: %v", err)
			}
			if _, err := runner.Show(context.Background(), "ag-1"); err != nil {
				t.Fatalf("Show: %v", err)
			}
			if _, err := runner.InProgress(context.Background()); err != nil {
				t.Fatalf("InProgress: %v", err)
			}

			physicalWorkDir, err := filepath.EvalSymlinks(resolution.WorkDir)
			if err != nil {
				t.Fatalf("physical work dir: %v", err)
			}
			wantBeads := "<unset>"
			for _, entry := range resolution.ChildEnv {
				if strings.HasPrefix(entry, "BEADS_DIR=") {
					wantBeads = strings.TrimPrefix(entry, "BEADS_DIR=")
				}
			}
			data, err := os.ReadFile(tracePath)
			if err != nil {
				t.Fatalf("read trace: %v", err)
			}
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			wantArgv := []string{
				"ready --json",
				"ready --json --type=bug",
				"show ag-1 --json",
				"list --status=in_progress --json",
			}
			if len(lines) != len(wantArgv) {
				t.Fatalf("trace lines = %q, want %d", lines, len(wantArgv))
			}
			for i, argv := range wantArgv {
				want := "pwd=" + physicalWorkDir + "|beads=" + wantBeads + "|argv=" + argv
				if lines[i] != want {
					t.Fatalf("trace[%d] = %q, want %q", i, lines[i], want)
				}
			}
		})
	}

	t.Run("typed exit", func(t *testing.T) {
		binary := filepath.Join(t.TempDir(), "bd")
		if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 23\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		resolution, err := trackerresolve.ResolveWithLookPath(
			t.TempDir(),
			[]string{"AGENTOPS_TRACKER=bd"},
			func(string) (string, error) { return binary, nil },
		)
		if err != nil {
			t.Fatal(err)
		}
		_, err = (ExecBeadRunner{Resolution: resolution}).Ready(context.Background())
		var exitErr *trackerexec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("Ready error = %T %v, want typed tracker exit", err, err)
		}
		if exitErr.ExitCode() != 23 {
			t.Fatalf("exit code = %d, want 23", exitErr.ExitCode())
		}
	})

	t.Run("pre-canceled context prevents launch", func(t *testing.T) {
		launchPath := filepath.Join(t.TempDir(), "launched")
		binary := filepath.Join(t.TempDir(), "br")
		script := "#!/bin/sh\nprintf launched > \"$LAUNCH_PATH\"\nprintf '[]\\n'\n"
		if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		resolution, err := trackerresolve.ResolveWithLookPath(
			t.TempDir(),
			[]string{"AGENTOPS_TRACKER=br", "LAUNCH_PATH=" + launchPath},
			func(string) (string, error) { return binary, nil },
		)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = (ExecBeadRunner{Resolution: resolution}).Ready(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Ready error = %T %v, want context cancellation", err, err)
		}
		if _, statErr := os.Stat(launchPath); !os.IsNotExist(statErr) {
			t.Fatalf("pre-canceled runner launched tracker: %v", statErr)
		}
	})
}

// fakeGrep returns canned hits per pattern.
type fakeGrep struct {
	Hits map[string][]string
}

func (g fakeGrep) Grep(ctx context.Context, pattern string, roots []string) ([]string, error) {
	return g.Hits[pattern], nil
}

// TestStep1ShapeFilter exercises the operator-shape skip behavior.
func TestStep1ShapeFilter(t *testing.T) {
	tests := []struct {
		name       string
		beads      []Bead
		includeOps bool
		wantID     string
		wantAlts   []string
	}{
		{
			name: "filters operator-shape by default",
			beads: []Bead{
				{ID: "soc-a", Labels: []string{"operator-shape"}},
				{ID: "soc-b"},
				{ID: "soc-c"},
			},
			wantID:   "soc-b",
			wantAlts: []string{"soc-c"},
		},
		{
			name: "includes operator-shape when flag set",
			beads: []Bead{
				{ID: "soc-a", Labels: []string{"operator-shape"}},
				{ID: "soc-b"},
			},
			includeOps: true,
			wantID:     "soc-a",
			wantAlts:   []string{"soc-b"},
		},
		{
			name:   "empty when nothing ready",
			beads:  nil,
			wantID: "",
		},
		{
			name: "all filtered returns empty",
			beads: []Bead{
				{ID: "soc-a", Labels: []string{"operator-shape"}},
				{ID: "soc-b", Labels: []string{"meta-runtime"}},
			},
			wantID: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			br := &fakeBeadRunner{ReadyList: tc.beads}
			got, alts, err := Step1ShapeFilter(context.Background(), br, Config{IncludeOperatorShape: tc.includeOps})
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got.ID != tc.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tc.wantID)
			}
			if len(tc.wantAlts) > 0 && !reflect.DeepEqual(alts, tc.wantAlts) {
				t.Errorf("alts = %v, want %v", alts, tc.wantAlts)
			}
		})
	}
}

// TestStep2GrepSiblings exercises sibling pattern enrichment.
func TestStep2GrepSiblings(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		hits     map[string][]string
		want     []string
	}{
		{
			name:     "no patterns yields no hits",
			patterns: nil,
			want:     nil,
		},
		{
			name:     "merges hits across patterns",
			patterns: []string{"wiring", "sibling pattern"},
			hits: map[string][]string{
				"wiring":          {"cli/foo.go:10", "skills/bar/SKILL.md:5"},
				"sibling pattern": {"docs/x.md:1"},
			},
			want: []string{"cli/foo.go:10", "skills/bar/SKILL.md:5", "docs/x.md:1"},
		},
		{
			name:     "dedupes across patterns",
			patterns: []string{"wiring", "sibling pattern"},
			hits: map[string][]string{
				"wiring":          {"a:1", "b:2", "c:3", "d:4"},
				"sibling pattern": {"a:1"},
			},
			want: []string{"a:1", "b:2", "c:3"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Step2GrepSiblings(context.Background(), fakeGrep{Hits: tc.hits}, "/repo", tc.patterns)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("hits = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestStep3PrimitiveTest covers the 3-question gating.
func TestStep3PrimitiveTest(t *testing.T) {
	tests := []struct {
		name      string
		bead      Bead
		wantPass  bool
		wantMissL int
	}{
		{
			name: "all three pass",
			bead: Bead{
				Title:       "implement foo",
				Description: "Edit cli/foo.go and skills/bar.md. ## Scenarios when X then Y. Follows soc-1234.",
			},
			wantPass: true,
		},
		{
			name: "one miss is acceptable",
			bead: Bead{
				Title:       "x",
				Description: "Edit cli/foo.go. when X then Y.",
			},
			wantPass: true,
		},
		{
			name: "two misses fails",
			bead: Bead{
				Title:       "vague work",
				Description: "make it better",
			},
			wantPass:  false,
			wantMissL: 3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pass, summary := Step3PrimitiveTest(tc.bead)
			if pass != tc.wantPass {
				t.Errorf("pass = %v, want %v (summary=%s)", pass, tc.wantPass, summary)
			}
			if !pass && summary == "" {
				t.Errorf("expected failure summary for failed primitive test")
			}
		})
	}
}

// TestStep4CrossHopPickup exercises sibling traversal from in-progress beads.
func TestStep4CrossHopPickup(t *testing.T) {
	br := &fakeBeadRunner{
		InProgressList: []Bead{{ID: "soc-ip1"}},
		ShowMap: map[string]Bead{
			"soc-ip1": {
				ID: "soc-ip1",
				Dependencies: []Dependency{
					{Type: "discovered-from", IssueID: "soc-sib1", Relation: "discovered-from"},
					{Type: "blocks", IssueID: "soc-sib2"}, // wrong relation
				},
			},
			"soc-sib1": {ID: "soc-sib1", Status: "ready"},
			"soc-sib2": {ID: "soc-sib2", Status: "ready"},
		},
	}
	got, _, err := Step4CrossHopPickup(context.Background(), br)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.ID != "soc-sib1" {
		t.Errorf("ID = %q, want soc-sib1", got.ID)
	}
}

// TestStep4CrossHopPickup_SkipsNonReady confirms non-ready siblings are excluded.
func TestStep4CrossHopPickup_SkipsNonReady(t *testing.T) {
	br := &fakeBeadRunner{
		InProgressList: []Bead{{ID: "soc-ip1"}},
		ShowMap: map[string]Bead{
			"soc-ip1": {
				ID: "soc-ip1",
				Dependencies: []Dependency{
					{Type: "discovered-from", IssueID: "soc-closed", Relation: "discovered-from"},
				},
			},
			"soc-closed": {ID: "soc-closed", Status: "closed"},
		},
	}
	got, _, err := Step4CrossHopPickup(context.Background(), br)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.ID != "" {
		t.Errorf("expected no candidate, got %q", got.ID)
	}
}

// TestStep5BugFallback orders bugs by surface area.
func TestStep5BugFallback(t *testing.T) {
	br := &fakeBeadRunner{
		ReadyByTypeMap: map[string][]Bead{
			"bug": {
				{ID: "big", Description: "Edit cli/a.go and cli/b.go and cli/c.go"},
				{ID: "small", Description: "Edit cli/x.go only"},
				{ID: "mid", Description: "Edit a.go and b.go"},
			},
		},
	}
	got, alts, err := Step5BugFallback(context.Background(), br)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.ID != "small" {
		t.Errorf("smallest bug = %q, want small", got.ID)
	}
	if len(alts) == 0 {
		t.Errorf("expected alternatives")
	}
}

// TestRun_ExhaustionEmitsBlockedHint covers the terminal recommendation.
func TestRun_ExhaustionEmitsBlockedHint(t *testing.T) {
	br := &fakeBeadRunner{
		ReadyByTypeMap: map[string][]Bead{},
	}
	got, err := Run(context.Background(), br, nil, Config{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.RecommendedBead != "" {
		t.Errorf("bead = %q, want empty", got.RecommendedBead)
	}
	if !strings.Contains(got.Rationale, "ladder exhausted") {
		t.Errorf("rationale: %q", got.Rationale)
	}
	if !strings.Contains(got.Rationale, "ao loop blocked") {
		t.Errorf("rationale missing blocked hint: %q", got.Rationale)
	}
}

// TestRun_Step1_PrimitivePass returns step 1 when bead passes primitive test.
func TestRun_Step1_PrimitivePass(t *testing.T) {
	br := &fakeBeadRunner{
		ReadyList: []Bead{
			{
				ID:          "soc-x",
				Title:       "do work",
				Description: "Edit cli/x.go. ## Scenarios when X then Y. Follows soc-prev.",
			},
		},
	}
	got, err := Run(context.Background(), br, nil, Config{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.RecommendedBead != "soc-x" || got.LadderStepMatched != 1 {
		t.Errorf("got=%+v, want bead=soc-x step=1", got)
	}
}

// TestRun_Step3_DecompositionRecommendation covers the scout-mode path.
func TestRun_Step3_DecompositionRecommendation(t *testing.T) {
	br := &fakeBeadRunner{
		ReadyList: []Bead{
			{ID: "soc-vague", Title: "improve things", Description: "make it better"},
		},
	}
	got, err := Run(context.Background(), br, nil, Config{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.LadderStepMatched != 3 {
		t.Errorf("step = %d, want 3", got.LadderStepMatched)
	}
	if !strings.Contains(got.Rationale, "scout-mode") {
		t.Errorf("rationale: %q", got.Rationale)
	}
}

// TestRun_Step5_BugFallbackChosen exercises the bug-fallback hop.
func TestRun_Step5_BugFallbackChosen(t *testing.T) {
	br := &fakeBeadRunner{
		ReadyList: nil,
		ReadyByTypeMap: map[string][]Bead{
			"bug": {
				{ID: "buggy", Description: "Edit cli/a.go to fix Y"},
			},
		},
	}
	got, err := Run(context.Background(), br, nil, Config{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.LadderStepMatched != 5 || got.RecommendedBead != "buggy" {
		t.Errorf("got=%+v, want bead=buggy step=5", got)
	}
}

// TestSiblingPatterns covers the trigger-phrase extraction.
func TestSiblingPatterns(t *testing.T) {
	b := Bead{
		Title:       "wiring up X",
		Description: "Uses Hop C shape and With_X builder pattern; see also sibling pattern from before.",
	}
	got := siblingPatterns(b)
	want := []string{"wiring", "with_x builder", "hop c shape", "sibling pattern"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("siblingPatterns = %v, want %v", got, want)
	}
}

// TestDecodeBeadList covers wrap vs raw shapes.
func TestDecodeBeadList(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{`[{"id":"a"},{"id":"b"}]`, 2},
		{`{"issues":[{"id":"a"}]}`, 1},
		{`{"items":[{"id":"a"},{"id":"b"},{"id":"c"}]}`, 3},
		{``, 0},
	}
	for _, tc := range cases {
		got, err := decodeBeadList([]byte(tc.in))
		if err != nil {
			t.Errorf("decode %q: %v", tc.in, err)
			continue
		}
		if len(got) != tc.want {
			t.Errorf("decode %q: got %d, want %d", tc.in, len(got), tc.want)
		}
	}
}
