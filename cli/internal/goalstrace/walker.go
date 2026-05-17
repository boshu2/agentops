package goalstrace

import (
	"os"
	"path/filepath"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/boshu2/agentops/cli/internal/scenarioresults"
)

// Options configures a walk.
type Options struct {
	// ProjectRoot is the repository root the walker resolves all paths against.
	ProjectRoot string
	// GoalsPath optionally overrides the GOALS.md location; empty uses the
	// patcher's default resolution under ProjectRoot.
	GoalsPath string
	// Beads is the bead querier; when nil, NewExecBeadQuerier() is used.
	Beads BeadQuerier
}

// Walk builds the read-only executable-spec trace graph for every directive in
// GOALS.md, following ADR-0005. It never mutates any file or external state.
//
// Graceful degradation: a missing GOALS.md returns an empty graph with a
// diagnostic; an unavailable `bd` skips bead/artifact-via-bead edges with a
// diagnostic; a missing docs/learnings/ directory skips learning edges with a
// diagnostic. None of these are fatal errors.
func Walk(opts Options) (Graph, error) {
	b := newBuilder()
	directives, ok := loadDirectives(opts, b)
	if !ok {
		return b.graph(), nil
	}
	scenarios := loadAllScenarios(opts.ProjectRoot)
	known := knownDirectiveIDs(directives)

	walkDirectiveScenarios(b, opts.ProjectRoot, directives, scenarios)
	walkScenarioResults(b, opts.ProjectRoot)
	beadIDs := walkBeadEdges(b, opts)
	artifacts := walkBeadArtifacts(b, opts.ProjectRoot, beadIDs)
	walkLearnings(b, opts.ProjectRoot, known)
	walkArtifactCitations(b, opts.ProjectRoot, artifacts)
	return b.graph(), nil
}

// loadDirectives loads GOALS.md directives and registers a node per directive.
// It returns false (with a diagnostic recorded) when GOALS.md is unreadable.
func loadDirectives(opts Options, b *builder) ([]goals.ParsedDirective, bool) {
	path := opts.GoalsPath
	if path == "" {
		path = goals.ResolveGoalsPath(filepath.Join(opts.ProjectRoot, "GOALS.md"))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		b.addDiag("GOALS.md not readable (" + err.Error() + "); empty trace graph")
		return nil, false
	}
	directives, perr := goals.ParseDirectiveBlocks(data)
	if perr != nil {
		b.addDiag("GOALS.md parse failed (" + perr.Error() + "); empty trace graph")
		return nil, false
	}
	rel := relPath(opts.ProjectRoot, path)
	for _, d := range directives {
		if d.StableID == "" {
			continue
		}
		b.addNode(Node{ID: d.StableID, Type: NodeDirective, Label: d.Title, Path: rel})
	}
	return directives, true
}

// knownDirectiveIDs returns the set of stable directive IDs declared in GOALS.md.
func knownDirectiveIDs(directives []goals.ParsedDirective) map[string]bool {
	out := map[string]bool{}
	for _, d := range directives {
		if d.StableID != "" {
			out[d.StableID] = true
		}
	}
	return out
}

// walkDirectiveScenarios emits directive_has_scenario edges for the forward
// (GOALS.md Scenarios:) and reverse (scenario.directive_id) links per
// ADR-0005 §2.1.
func walkDirectiveScenarios(b *builder, root string, directives []goals.ParsedDirective, scenarios []scenarioFile) {
	declared := map[string]bool{} // directiveID|scenarioID pairs already emitted
	for _, d := range directives {
		if d.StableID == "" {
			continue
		}
		emitDirectiveNoScenarioWarning(b, d, root)
		for _, sid := range d.Scenarios {
			emitForwardScenarioEdge(b, root, d.StableID, sid)
			declared[d.StableID+"|"+sid] = true
		}
	}
	emitReverseScenarioEdges(b, root, scenarios, knownDirectiveIDs(directives), declared)
}

// emitDirectiveNoScenarioWarning attaches a directive_no_scenarios warning to a
// directive node when it declares no scenarios.
func emitDirectiveNoScenarioWarning(b *builder, d goals.ParsedDirective, root string) {
	if len(d.Scenarios) > 0 {
		return
	}
	b.addEdge(Edge{
		Type:       EdgeDirectiveHasScenario,
		FromID:     d.StableID,
		Confidence: ConfidenceLow,
		Defects: []Defect{{
			Code:     DefectDirectiveNoScenario,
			Severity: SeverityWarning,
			Detail:   "directive declares no Scenarios attribute",
		}},
	})
}

// emitForwardScenarioEdge emits one directive->scenario edge for a forward
// (GOALS.md Scenarios:) declaration, classifying confidence and defects.
func emitForwardScenarioEdge(b *builder, root, directiveID, scenarioID string) {
	res := resolveScenario(root, scenarioID)
	if !res.Found {
		b.addEdge(Edge{
			Type:       EdgeDirectiveHasScenario,
			FromID:     directiveID,
			ToID:       scenarioID,
			Confidence: ConfidenceLow,
			Defects: []Defect{{
				Code:     DefectBrokenScenarioRef,
				Severity: SeverityError,
				Detail:   "no spec/scenarios or .agents/holdout file for " + scenarioID,
			}},
		})
		return
	}
	sf := res.File
	b.addNode(Node{ID: sf.ID, Type: NodeScenario, Label: sf.Goal, Path: relPath(root, sf.path)})
	edge := Edge{
		Type:     EdgeDirectiveHasScenario,
		FromID:   directiveID,
		ToID:     scenarioID,
		Evidence: relPath(root, sf.path),
	}
	if sf.DirectiveID == directiveID {
		edge.Confidence = ConfidenceHigh
	} else {
		edge.Confidence = ConfidenceLow
	}
	if !sf.promoted {
		edge.Defects = append(edge.Defects, Defect{
			Code:     DefectScenarioNotPromoted,
			Severity: SeverityWarning,
			Detail:   "scenario resolves only in .agents/holdout/, not spec/scenarios/",
		})
	}
	b.addEdge(edge)
}

// emitReverseScenarioEdges emits directive_has_scenario edges discovered from a
// scenario file's directive_id back-reference that the forward pass missed.
func emitReverseScenarioEdges(b *builder, root string, scenarios []scenarioFile, known map[string]bool, declared map[string]bool) {
	for i := range scenarios {
		sf := scenarios[i]
		if sf.DirectiveID == "" || declared[sf.DirectiveID+"|"+sf.ID] {
			continue
		}
		b.addNode(Node{ID: sf.ID, Type: NodeScenario, Label: sf.Goal, Path: relPath(root, sf.path)})
		edge := Edge{
			Type:       EdgeDirectiveHasScenario,
			FromID:     sf.DirectiveID,
			ToID:       sf.ID,
			Confidence: ConfidenceLow,
			Evidence:   relPath(root, sf.path),
		}
		if !known[sf.DirectiveID] {
			edge.Defects = append(edge.Defects, Defect{
				Code:     DefectBrokenDirectiveBackref,
				Severity: SeverityError,
				Detail:   "scenario directive_id " + sf.DirectiveID + " matches no GOALS.md directive",
			})
		}
		b.addEdge(edge)
	}
}

// walkScenarioResults emits scenario_result edges from the scenario-results
// artifact (F2.0). A missing artifact degrades gracefully with a diagnostic.
func walkScenarioResults(b *builder, root string) {
	res, err := scenarioresults.Load(root, false)
	if err != nil || res.Status != scenarioresults.StatusOK || res.Artifact == nil {
		if res.Warning != "" {
			b.addDiag(res.Warning)
		}
		return
	}
	evidence := scenarioresults.ArtifactRelPath
	for _, r := range res.Artifact.Results {
		artifactID := "result:" + r.ScenarioID
		b.addNode(Node{ID: artifactID, Type: NodeArtifact, Label: "verdict " + r.Verdict, Path: evidence})
		b.addEdge(Edge{
			Type:       EdgeScenarioResult,
			FromID:     r.ScenarioID,
			ToID:       artifactID,
			Confidence: ConfidenceHigh,
			Evidence:   evidence,
		})
	}
}

// walkBeadEdges emits scenario_claimed_by_bead edges from bead Scenarios:
// claims and returns the set of bead IDs the querier could see (used by
// walkBeadArtifacts to classify broken_artifact_bead_ref). A nil set means the
// querier was unavailable; an empty non-nil set means bd had zero beads.
// A missing bd binary degrades gracefully with a diagnostic.
func walkBeadEdges(b *builder, opts Options) map[string]bool {
	q := opts.Beads
	if q == nil {
		q = NewExecBeadQuerier()
	}
	if !q.Available() {
		b.addDiag("bd not available; scenario_claimed_by_bead edges skipped (graceful degradation)")
		return nil
	}
	beads, err := q.Beads()
	if err != nil {
		b.addDiag("bd query failed (" + err.Error() + "); scenario_claimed_by_bead edges skipped")
		return nil
	}
	ids := map[string]bool{}
	for _, bead := range beads {
		ids[bead.ID] = true
		emitBeadClaimEdges(b, opts.ProjectRoot, bead)
	}
	return ids
}

// walkBeadArtifacts emits bead_produced_artifact edges for every RPI run
// artifact traceable to a bead (ADR-0005 §2.4). A missing .agents/rpi/runs/
// directory degrades gracefully with a diagnostic. knownBeads is the bead-ID
// set from walkBeadEdges; when nil (bd unavailable) the broken-ref check is
// skipped. It returns every discovered artifact for the citation pass.
func walkBeadArtifacts(b *builder, root string, knownBeads map[string]bool) []rpiArtifact {
	artifacts, ok := loadRPIArtifacts(root)
	if !ok {
		b.addDiag(".agents/rpi/runs/ not present; bead_produced_artifact edges skipped (graceful degradation)")
		return nil
	}
	for i := range artifacts {
		emitBeadArtifactEdge(b, artifacts[i], knownBeads)
	}
	return artifacts
}

// emitBeadArtifactEdge emits one bead_produced_artifact edge for an artifact
// whose producing bead is traceable, attaching a broken_artifact_bead_ref
// error when an explicit bead_id frontmatter field names an unknown bead.
func emitBeadArtifactEdge(b *builder, a rpiArtifact, knownBeads map[string]bool) {
	beadID, conf, viaPath := artifactBeadLink(a)
	if beadID == "" {
		return
	}
	b.addNode(Node{ID: a.relPath, Type: NodeArtifact, Label: filepath.Base(a.relPath), Path: a.relPath})
	b.addNode(Node{ID: beadID, Type: NodeBead})
	edge := Edge{
		Type:       EdgeBeadProducedArtifact,
		FromID:     beadID,
		ToID:       a.relPath,
		Confidence: conf,
		Evidence:   a.relPath,
	}
	// A frontmatter bead_id naming an unknown bead is a broken explicit link.
	// A bead ID discovered only via the path is not an explicit declaration.
	if !viaPath && conf == ConfidenceHigh && a.beadID != "" && knownBeads != nil && !knownBeads[beadID] {
		edge.Defects = append(edge.Defects, Defect{
			Code:     DefectBrokenArtifactBeadRef,
			Severity: SeverityError,
			Detail:   "artifact bead_id " + beadID + " matches no known bead",
		})
	}
	b.addEdge(edge)
}

// walkArtifactCitations emits artifact_cited_by_learning edges for every
// (artifact, learning) pair where the learning cites the artifact (ADR-0005
// §2.5). Both inputs are already loaded; a missing learnings dir or empty
// artifact set simply yields no edges.
func walkArtifactCitations(b *builder, root string, artifacts []rpiArtifact) {
	if len(artifacts) == 0 {
		return
	}
	learnings, ok := loadLearnings(root)
	if !ok {
		return
	}
	for li := range learnings {
		lf := learnings[li]
		learningRel := relPath(root, lf.path)
		for ai := range artifacts {
			emitArtifactCitationEdge(b, artifacts[ai], lf, learningRel)
		}
	}
}

// emitArtifactCitationEdge emits one artifact_cited_by_learning edge when a
// learning cites an artifact, classifying citation confidence per §2.5.
func emitArtifactCitationEdge(b *builder, a rpiArtifact, lf learningFile, learningRel string) {
	cited, conf := learningCitesArtifact(lf, a.relPath)
	if !cited {
		return
	}
	b.addNode(Node{ID: a.relPath, Type: NodeArtifact, Label: filepath.Base(a.relPath), Path: a.relPath})
	b.addNode(Node{ID: learningRel, Type: NodeLearning, Label: filepath.Base(lf.path), Path: learningRel})
	b.addEdge(Edge{
		Type:       EdgeArtifactCitedByLearning,
		FromID:     a.relPath,
		ToID:       learningRel,
		Confidence: conf,
		Evidence:   learningRel,
	})
}

// emitBeadClaimEdges emits one scenario_claimed_by_bead edge per scenario a
// bead claims, classifying explicit vs heuristic claims per ADR-0005 §2.3.
func emitBeadClaimEdges(b *builder, root string, bead beadRecord) {
	explicit, heuristic := claimedScenarios(bead)
	if len(explicit) == 0 && len(heuristic) == 0 {
		return
	}
	b.addNode(Node{ID: bead.ID, Type: NodeBead, Label: bead.Title})
	for _, sid := range explicit {
		b.addEdge(beadClaimEdge(root, bead.ID, sid, ConfidenceHigh))
	}
	for _, sid := range heuristic {
		b.addEdge(beadClaimEdge(root, bead.ID, sid, ConfidenceLow))
	}
}

// beadClaimEdge builds one scenario_claimed_by_bead edge, attaching a
// broken_bead_scenario_claim defect when the claimed scenario does not resolve.
//
// Per ADR-0005 §4.1, the defect is an error only when an explicit link is
// broken — i.e. the claim came from an explicit "Scenarios:" line
// (conf == ConfidenceHigh). A heuristic free-text match (conf == ConfidenceLow)
// can never constitute an explicit declaration, so an unresolvable heuristic
// token is downgraded to a warning rather than an error.
func beadClaimEdge(root, beadID, scenarioID string, conf Confidence) Edge {
	e := Edge{
		Type:       EdgeScenarioClaimedByBead,
		FromID:     scenarioID,
		ToID:       beadID,
		Confidence: conf,
	}
	if !resolveScenario(root, scenarioID).Found {
		sev := SeverityError
		if conf == ConfidenceLow {
			// Heuristic (non-explicit) claims are not broken explicit links;
			// downgrade to warning so they don't gate a clean run.
			sev = SeverityWarning
		}
		e.Defects = append(e.Defects, Defect{
			Code:     DefectBrokenBeadScenarioClaim,
			Severity: sev,
			Detail:   "bead claims scenario " + scenarioID + " which does not resolve",
		})
	}
	return e
}

// walkLearnings emits directive_has_learning edges from learning frontmatter.
// A missing docs/learnings/ directory degrades gracefully with a diagnostic.
func walkLearnings(b *builder, root string, known map[string]bool) {
	learnings, ok := loadLearnings(root)
	if !ok {
		b.addDiag("docs/learnings/ not present; directive_has_learning edges skipped (graceful degradation)")
		return
	}
	for i := range learnings {
		emitLearningEdge(b, root, learnings[i], known)
	}
}

// emitLearningEdge emits a directive_has_learning edge for a learning that
// declares a directive_id, classifying confidence per ADR-0005 §2.6.
func emitLearningEdge(b *builder, root string, lf learningFile, known map[string]bool) {
	did, conf := learningDirective(lf, known)
	if did == "" {
		return
	}
	rel := relPath(root, lf.path)
	b.addNode(Node{ID: rel, Type: NodeLearning, Label: filepath.Base(lf.path), Path: rel})
	edge := Edge{
		Type:       EdgeDirectiveHasLearning,
		FromID:     did,
		ToID:       rel,
		Confidence: conf,
		Evidence:   rel,
	}
	if !known[did] {
		edge.Defects = append(edge.Defects, Defect{
			Code:     DefectBrokenLearningDirRef,
			Severity: SeverityError,
			Detail:   "learning directive_id " + did + " matches no GOALS.md directive",
		})
	}
	b.addEdge(edge)
}

// learningDirective resolves the directive a learning links to and the
// confidence of that link. Frontmatter directive_id is high confidence; a
// directive ID token in the body is high when known, low when unknown.
func learningDirective(lf learningFile, known map[string]bool) (string, Confidence) {
	if lf.fm.directiveID != "" {
		return lf.fm.directiveID, ConfidenceHigh
	}
	if m := directiveTokenRe.FindString(lf.body); m != "" {
		if known[m] {
			return m, ConfidenceHigh
		}
		return m, ConfidenceLow
	}
	return "", ConfidenceLow
}

// relPath returns path relative to root with forward slashes; on failure it
// returns the cleaned input path so the walker never panics on odd inputs.
func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(filepath.Clean(path))
	}
	return filepath.ToSlash(rel)
}
