package skills

import "testing"

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
