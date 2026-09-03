package skills

import (
	"strings"
	"testing"
)

// fixtureSkills is a fixed in-memory catalog so ranking assertions are stable
// and independent of the live skills/ tree (which churns).
func fixtureSkills() []SkillMeta {
	return []SkillMeta{
		{Name: "evolve", Description: "Run autonomous improvement loops that close the loop on agent work", Path: "skills/evolve/SKILL.md"},
		{Name: "rpi", Description: "Run discovery, crank, validation — one turn of the operating loop", Path: "skills/rpi/SKILL.md"},
		{Name: "release", Description: "Run release validation and cut a versioned release", Path: "skills/release/SKILL.md"},
		{Name: "crank", Description: "Execute epics through waves of parallel workers", Triggers: []string{"loop", "wave"}, Path: "skills/crank/SKILL.md"},
	}
}

func TestScore_RanksByIntent(t *testing.T) {
	matches := Score("close the loop", fixtureSkills())
	if len(matches) != 4 {
		t.Fatalf("expected 4 matches (one per skill), got %d", len(matches))
	}

	// evolve hits both "close" and "loop" in its description; it must rank top.
	if matches[0].Name != "evolve" {
		t.Errorf("expected evolve ranked first, got %q (scores: %+v)", matches[0].Name, matches)
	}
	// release matches neither "close" nor "loop" -> must rank last with score 0.
	last := matches[len(matches)-1]
	if last.Name != "release" {
		t.Errorf("expected release ranked last, got %q", last.Name)
	}
	if last.Score != 0 {
		t.Errorf("expected release score 0 (no token overlap), got %v", last.Score)
	}
	// Scores must be sorted descending.
	for i := 1; i < len(matches); i++ {
		if matches[i-1].Score < matches[i].Score {
			t.Errorf("results not sorted descending at %d: %v < %v", i, matches[i-1].Score, matches[i].Score)
		}
	}
}

func TestScore_NormalizedToUnitRange(t *testing.T) {
	// A query whose every token is a skill name token scores exactly 1.0.
	metas := []SkillMeta{{Name: "evolve loop", Description: "x"}}
	got := Score("evolve loop", metas)
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0].Score != 1.0 {
		t.Errorf("expected full name match to score 1.0, got %v", got[0].Score)
	}
	for _, m := range Score("close the loop", fixtureSkills()) {
		if m.Score < 0 || m.Score > 1 {
			t.Errorf("score %v for %q out of [0,1]", m.Score, m.Name)
		}
	}
}

func TestScore_TriggerBeatsDescription(t *testing.T) {
	metas := []SkillMeta{
		{Name: "alpha", Description: "mentions wave somewhere"},
		{Name: "beta", Triggers: []string{"wave"}, Description: "unrelated text here"},
	}
	got := Score("wave", metas)
	if got[0].Name != "beta" {
		t.Errorf("expected trigger hit (beta) to outrank description hit (alpha), got %q", got[0].Name)
	}
	if !(got[0].Score > got[1].Score) {
		t.Errorf("expected beta score (%v) > alpha score (%v)", got[0].Score, got[1].Score)
	}
}

func TestScore_NoMatchIsZero(t *testing.T) {
	got := Score("zzqqxx-nonsense", fixtureSkills())
	for _, m := range got {
		if m.Score != 0 {
			t.Errorf("expected score 0 for nonsense query, got %v for %q", m.Score, m.Name)
		}
	}
}

func TestScore_EmptyQueryYieldsZeroScores(t *testing.T) {
	got := Score("", fixtureSkills())
	if len(got) != 4 {
		t.Fatalf("expected all skills returned for empty query, got %d", len(got))
	}
	for _, m := range got {
		if m.Score != 0 {
			t.Errorf("expected zero score for empty query, got %v for %q", m.Score, m.Name)
		}
	}
}

func TestScore_TieBrokenByNameAscending(t *testing.T) {
	// Two skills with identical signal for "test" must order by name.
	metas := []SkillMeta{
		{Name: "zeta", Description: "test harness"},
		{Name: "alpha", Description: "test harness"},
	}
	got := Score("test", metas)
	if got[0].Name != "alpha" || got[1].Name != "zeta" {
		t.Errorf("expected tie broken alpha before zeta, got %q then %q", got[0].Name, got[1].Name)
	}
}

func TestTokenize_DropsStopwordsAndShortTokens(t *testing.T) {
	got := tokenize("Close THE a loop-validation")
	want := []string{"close", "loop", "validation"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestScore_ExclusionSentenceRoutesAway(t *testing.T) {
	metas := []SkillMeta{
		{Name: "premortem", Description: "Fresh-judge a frozen plan. Not for a live decision's reversibility; that is one-way-door. Triggers: \"premortem\", \"challenge this plan\"."},
		{Name: "one-way-door", Description: "Classify a pending decision as reversible or irreversible; route irreversible ones to the caller. Not for challenging a frozen plan; that is premortem. Triggers: \"is this a one-way door\"."},
	}

	got := Score("is this live decision reversible", metas)
	if got[0].Name != "one-way-door" {
		t.Fatalf("reversibility query: want one-way-door first, got %q (%+v)", got[0].Name, got)
	}
	if got[1].Name != "premortem" || got[1].Score != 0 {
		t.Errorf("premortem's exclusion vocabulary must earn nothing: got %+v", got[1])
	}

	// Holding the sentence out must never cost the skill its own query: an
	// explicit name, or the skill's own words beside one exclusion word.
	got = Score("premortem the plan for this live rollout", metas)
	if got[0].Name != "premortem" {
		t.Fatalf("named query: want premortem first, got %q (%+v)", got[0].Name, got)
	}

	got = Score("challenge this frozen plan", metas)
	if got[0].Name != "premortem" {
		t.Fatalf("plan-challenge query: want premortem first, got %q (%+v)", got[0].Name, got)
	}
	if got[1].Name != "one-way-door" || got[1].Score != 0 {
		t.Errorf("one-way-door's exclusion vocabulary must earn nothing: got %+v", got[1])
	}
}

func TestSplitExclusion(t *testing.T) {
	cases := []struct {
		desc, wantPositive, wantExclusion string
	}{
		{"Trace a repo. Not for a bounded question; that is research. Triggers: \"recon\".", "Trace a repo. Triggers: \"recon\".", "Not for a bounded question; that is research."},
		{"Trace a repo. Triggers: \"recon\".", "Trace a repo. Triggers: \"recon\".", ""},
		{"Answer one question. Not for dissecting a codebase; that is codebase-recon or reverse-engineer.", "Answer one question.", "Not for dissecting a codebase; that is codebase-recon or reverse-engineer."},
	}
	for _, c := range cases {
		pos, excl := splitExclusion(c.desc)
		if pos != c.wantPositive || excl != c.wantExclusion {
			t.Errorf("splitExclusion(%q) = (%q, %q); want (%q, %q)", c.desc, pos, excl, c.wantPositive, c.wantExclusion)
		}
	}
}

// TestScore_LiveCatalogOwnVocabulary runs the shipped catalog, not a fixture:
// a skill's own words beside one word from its exclusion sentence must still
// route to it. Each query regressed once when the exclusion sentence was
// penalised instead of held out (round 3/4 of the 2026-09-03 Train 2 review).
func TestScore_LiveCatalogOwnVocabulary(t *testing.T) {
	root := repoSkillsDir(t)
	if root == "" {
		t.Skip("skills/ not found relative to test working dir")
	}
	metas, err := Load(root)
	if err != nil {
		t.Fatalf("Load(%s): %v", root, err)
	}
	cases := map[string]string{
		"validate the claim that tests pass":       "validate",
		"validate this change and check the tests": "validate",
		"premortem the plan for this live rollout": "premortem",
		"challenge this plan with one judge":       "premortem",
		"is this live decision reversible":         "one-way-door",
		// The two triggers added in the 2026-09-03 repair, quoted verbatim:
		// a declared phrase must beat a sibling that owns one of its words as
		// a name token (reality-check, one-way-door).
		"check this change": "validate",
		"one judge":         "premortem",
		// Phrase matching reads the query in order: "one council judge"
		// must not collapse into premortem's "one judge".
		"ask the council whether one council judge is enough": "council",
	}
	for q, want := range cases {
		got := Score(q, metas)
		if len(got) == 0 || got[0].Name != want {
			top := ""
			if len(got) > 0 {
				top = got[0].Name
			}
			t.Errorf("%q: want %s first, got %q (top: %+v)", q, want, top, got[:min(3, len(got))])
		}
	}
}

func TestTriggerPhrases_ParsesDescriptionAndFrontmatter(t *testing.T) {
	m := SkillMeta{
		Name:        "validate",
		Triggers:    []string{"prove it"},
		Description: `Freshly judge a change. Not for claim-vs-tree checks; that is reality-check. Triggers: "validate", "is this proven", "check this change".`,
	}
	got := triggerPhrases(m)
	want := [][]string{{"prove"}, {"validate"}, {"proven"}, {"check", "change"}}
	if len(got) != len(want) {
		t.Fatalf("triggerPhrases: got %v, want %v", got, want)
	}
	for i := range want {
		if strings.Join(got[i], " ") != strings.Join(want[i], " ") {
			t.Errorf("phrase %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestScore_DeclaredPhraseBeatsSiblingNameToken(t *testing.T) {
	metas := []SkillMeta{
		{Name: "validate", Description: `Freshly judge a finished change. Triggers: "validate", "check this change".`},
		{Name: "reality-check", Description: `Compare a claimed state with the tree. Triggers: "reality check".`},
	}
	got := Score("check this change", metas)
	if got[0].Name != "validate" {
		t.Fatalf("want validate first, got %+v", got)
	}
	if got[0].Score > 1 {
		t.Errorf("score must stay clamped at 1, got %v", got[0].Score)
	}
	// A single-word trigger earns no phrase bonus; single-word behaviour is
	// unchanged from the name/trigger/description weights.
	got = Score("validate", metas)
	if got[0].Score != 1 {
		t.Errorf("single name token: want score 1, got %v", got[0].Score)
	}
}

func TestTriggerPhrases_DedupesAndStopsAtSentenceEnd(t *testing.T) {
	m := SkillMeta{
		Name:        "demo",
		Triggers:    []string{"check this change"},
		Description: `Do the job. Triggers: "check this change", "run the demo". Not for "other things".`,
	}
	got := triggerPhrases(m)
	want := [][]string{{"check", "change"}, {"run", "demo"}}
	if len(got) != len(want) {
		t.Fatalf("triggerPhrases: got %v, want %v", got, want)
	}
	for i := range want {
		if strings.Join(got[i], " ") != strings.Join(want[i], " ") {
			t.Errorf("phrase %d: got %v, want %v", i, got[i], want[i])
		}
	}
	// A phrase declared in both frontmatter and the description scores once.
	// Both fixtures carry the same tokens at the same weights; only the
	// number of declarations differs. The query is long enough that a
	// double bonus (0.75) would not hide behind the clamp (0.5 expected).
	once := []SkillMeta{{Name: "demo", Triggers: []string{"check this change"}, Description: `Do the job: check this change. Triggers: "run the demo".`}}
	twice := []SkillMeta{m}
	a, b := Score("please check this change today", once)[0].Score, Score("please check this change today", twice)[0].Score
	if a != 0.5 || b != 0.5 {
		t.Errorf("duplicate declaration must score identically at 0.5: once=%v twice=%v", a, b)
	}
}

func TestScore_PhraseBonusClampsAtOne(t *testing.T) {
	metas := []SkillMeta{{Name: "skill-builder", Description: `Build a skill. Triggers: "new skill".`}}
	// "new skill": skill hits the name (3) and new the description (1), plus
	// the whole phrase (3) = 7 over a ceiling of 6; the score reports 1.
	got := Score("new skill", metas)
	if got[0].Score != 1 {
		t.Errorf("want clamped score 1, got %v", got[0].Score)
	}
}

func TestScore_PhraseMatchesInOrderWithoutDedup(t *testing.T) {
	metas := []SkillMeta{
		{Name: "premortem", Description: `Fresh-judge a frozen plan. Triggers: "premortem", "one judge".`},
		{Name: "council", Description: `Gather independent views on a high-stakes judgment. Triggers: "council", "multi-judge review".`},
	}
	got := Score("ask the council whether one council judge is enough", metas)
	if got[0].Name != "council" {
		t.Fatalf("want council first, got %+v", got)
	}
}
