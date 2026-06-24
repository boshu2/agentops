package vibecheck

import (
	"fmt"
	"testing"
	"time"
)

// helper to create a TimelineEvent with minimal fields.
func makeTestEvent(sha, message string, ts time.Time) TimelineEvent {
	return TimelineEvent{
		SHA:          sha,
		Message:      message,
		Timestamp:    ts,
		Author:       "test-author",
		FilesChanged: 1,
		Insertions:   10,
		Deletions:    2,
	}
}

func baseTime() time.Time {
	return time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
}

// --- Velocity Tests ---

func TestMetricVelocity_Empty(t *testing.T) {
	m := MetricVelocity(nil)
	if m.Value != 0 {
		t.Errorf("expected 0 for empty events, got %f", m.Value)
	}
	if m.Passed {
		t.Error("expected not passed for empty events")
	}
}

func TestMetricVelocity_HighPace(t *testing.T) {
	base := baseTime()
	events := []TimelineEvent{
		makeTestEvent("a1", "feat: one", base),
		makeTestEvent("a2", "feat: two", base.Add(1*time.Hour)),
		makeTestEvent("a3", "feat: three", base.Add(2*time.Hour)),
		makeTestEvent("a4", "feat: four", base.Add(3*time.Hour)),
		makeTestEvent("a5", "feat: five", base.Add(4*time.Hour)),
	}
	m := MetricVelocity(events)
	if !m.Passed {
		t.Errorf("expected passed for 5 commits in one day, got value=%f", m.Value)
	}
	if m.Value < 3 {
		t.Errorf("expected velocity >= 3, got %f", m.Value)
	}
}

func TestMetricVelocity_LowPace(t *testing.T) {
	base := baseTime()
	events := []TimelineEvent{
		makeTestEvent("a1", "feat: one", base),
		makeTestEvent("a2", "feat: two", base.Add(48*time.Hour)),
	}
	m := MetricVelocity(events)
	if m.Passed {
		t.Errorf("expected not passed for 2 commits over 2 days, got value=%f", m.Value)
	}
}

// --- Rework Tests ---

func TestMetricRework_Empty(t *testing.T) {
	m := MetricRework(nil)
	if m.Value != 0 {
		t.Errorf("expected 0, got %f", m.Value)
	}
	if !m.Passed {
		t.Error("expected passed for empty events (0% rework)")
	}
}

func TestMetricRework_LowRework(t *testing.T) {
	base := baseTime()
	events := []TimelineEvent{
		makeTestEvent("a1", "feat: add login", base),
		makeTestEvent("a2", "feat: add dashboard", base.Add(time.Hour)),
		makeTestEvent("a3", "feat: add settings", base.Add(2*time.Hour)),
		makeTestEvent("a4", "fix: login edge case", base.Add(3*time.Hour)),
	}
	m := MetricRework(events)
	if !m.Passed {
		t.Errorf("expected passed for 25%% rework, got value=%f", m.Value)
	}
}

func TestMetricRework_HighRework(t *testing.T) {
	base := baseTime()
	events := []TimelineEvent{
		makeTestEvent("a1", "fix: broken auth", base),
		makeTestEvent("a2", "fix: broken auth again", base.Add(time.Hour)),
		makeTestEvent("a3", "fix: still broken", base.Add(2*time.Hour)),
		makeTestEvent("a4", "feat: finally works", base.Add(3*time.Hour)),
	}
	m := MetricRework(events)
	if m.Passed {
		t.Errorf("expected not passed for 75%% rework, got value=%f", m.Value)
	}
}

// --- Trust Tests ---

func TestMetricTrust_Empty(t *testing.T) {
	m := MetricTrust(nil)
	if m.Value != 0 {
		t.Errorf("expected 0, got %f", m.Value)
	}
	if m.Passed {
		t.Error("expected not passed for empty events")
	}
}

func TestMetricTrust_WithTests(t *testing.T) {
	base := baseTime()
	events := []TimelineEvent{
		makeTestEvent("a1", "feat: add login", base),
		makeTestEvent("a2", "test: add login tests", base.Add(time.Hour)),
		makeTestEvent("a3", "feat: add dashboard", base.Add(2*time.Hour)),
		makeTestEvent("a4", "test: add dashboard tests", base.Add(3*time.Hour)),
	}
	m := MetricTrust(events)
	// 2 test commits / 2 code commits = 1.0 ratio
	if !m.Passed {
		t.Errorf("expected passed with ratio=1.0, got value=%f", m.Value)
	}
}

func TestMetricTrust_CountsTestFilesInFeatCommits(t *testing.T) {
	// AgentOps TDD: the test ships in the feat:/fix: commit, not a separate test:
	// commit. A feat commit that TOUCHES a _test.go file must count toward trust
	// (age-mr1c — the prior message-prefix-only heuristic scored TDD work ~0).
	base := baseTime()
	events := []TimelineEvent{
		{SHA: "a1", Message: "feat: add login", Timestamp: base, Files: []string{"cli/auth.go", "cli/auth_test.go"}},
		{SHA: "a2", Message: "feat: add dashboard", Timestamp: base.Add(time.Hour), Files: []string{"cli/dash.go"}},
	}
	m := MetricTrust(events)
	// a1 is test-bearing (touches auth_test.go); a2 is code-only -> ratio 1/1 = 1.0.
	if !m.Passed {
		t.Errorf("expected passed: a feat commit touching _test.go must count toward trust, got value=%f", m.Value)
	}
	if m.Value != 1.0 {
		t.Errorf("expected trust 1.0 (1 test-bearing / 1 code-only), got %f", m.Value)
	}
}

func TestMetricTrust_NoTests(t *testing.T) {
	base := baseTime()
	events := []TimelineEvent{
		makeTestEvent("a1", "feat: add login", base),
		makeTestEvent("a2", "feat: add dashboard", base.Add(time.Hour)),
		makeTestEvent("a3", "feat: add settings", base.Add(2*time.Hour)),
	}
	m := MetricTrust(events)
	if m.Passed {
		t.Errorf("expected not passed with 0 test commits, got value=%f", m.Value)
	}
	if m.Value != 0 {
		t.Errorf("expected value=0, got %f", m.Value)
	}
}

// --- Spirals Tests ---

func TestMetricSpirals_Empty(t *testing.T) {
	m := MetricSpirals(nil)
	if m.Value != 0 {
		t.Errorf("expected 0, got %f", m.Value)
	}
	if !m.Passed {
		t.Error("expected passed for empty events")
	}
}

func TestMetricSpirals_NoSpirals(t *testing.T) {
	base := baseTime()
	events := []TimelineEvent{
		makeTestEvent("a1", "feat: add login", base),
		makeTestEvent("a2", "fix: typo in readme", base.Add(time.Hour)),
		makeTestEvent("a3", "feat: add dashboard", base.Add(2*time.Hour)),
		makeTestEvent("a4", "fix: dashboard color", base.Add(3*time.Hour)),
	}
	m := MetricSpirals(events)
	if !m.Passed {
		t.Errorf("expected passed with no spirals, got value=%f", m.Value)
	}
}

func TestMetricSpirals_WithSpiral(t *testing.T) {
	base := baseTime()
	events := []TimelineEvent{
		makeTestEvent("a1", "fix(auth): wrong token", base),
		makeTestEvent("a2", "fix(auth): still wrong", base.Add(10*time.Minute)),
		makeTestEvent("a3", "fix(auth): try again", base.Add(20*time.Minute)),
	}
	m := MetricSpirals(events)
	if m.Passed {
		t.Errorf("expected not passed with spiral, got value=%f", m.Value)
	}
	if m.Value < 1 {
		t.Errorf("expected at least 1 spiral, got %f", m.Value)
	}
}

// --- Flow Tests ---

func TestMetricFlow_Empty(t *testing.T) {
	m := MetricFlow(nil)
	if m.Value != 0 {
		t.Errorf("expected 0, got %f", m.Value)
	}
	if m.Passed {
		t.Error("expected not passed for empty events")
	}
}

func TestMetricFlow_EvenDistribution(t *testing.T) {
	base := baseTime()
	events := []TimelineEvent{
		makeTestEvent("a1", "feat: day1", base),
		makeTestEvent("a2", "feat: day1b", base.Add(2*time.Hour)),
		makeTestEvent("a3", "feat: day2", base.Add(24*time.Hour)),
		makeTestEvent("a4", "feat: day2b", base.Add(26*time.Hour)),
		makeTestEvent("a5", "feat: day3", base.Add(48*time.Hour)),
		makeTestEvent("a6", "feat: day3b", base.Add(50*time.Hour)),
	}
	m := MetricFlow(events)
	if !m.Passed {
		t.Errorf("expected passed for even distribution, got value=%f", m.Value)
	}
}

func TestMetricFlow_SpikyDistribution(t *testing.T) {
	base := baseTime()
	// 10 commits on day 1, 0 on day 2, 0 on day 3
	var events []TimelineEvent
	for i := range 10 {
		events = append(events, makeTestEvent("a"+string(rune('0'+i)), "feat: work",
			base.Add(time.Duration(i)*time.Hour)))
	}
	// Add one commit 3 days later to create the gap.
	events = append(events, makeTestEvent("b1", "feat: late", base.Add(72*time.Hour)))
	m := MetricFlow(events)
	if m.Passed {
		t.Errorf("expected not passed for spiky distribution, got value=%f", m.Value)
	}
}

// --- Aggregator Tests ---

func TestComputeMetrics_ReturnsAllFive(t *testing.T) {
	base := baseTime()
	events := []TimelineEvent{
		makeTestEvent("a1", "feat: one", base),
		makeTestEvent("a2", "feat: two", base.Add(time.Hour)),
		makeTestEvent("a3", "test: three", base.Add(2*time.Hour)),
	}
	metrics := ComputeMetrics(events)

	expected := []string{"velocity", "rework", "trust", "spirals", "flow"}
	for _, name := range expected {
		if _, ok := metrics[name]; !ok {
			t.Errorf("missing metric %q", name)
		}
	}

	if len(metrics) != 5 {
		t.Errorf("expected 5 metrics, got %d", len(metrics))
	}
}

func TestComputeOverallRating_AllPass(t *testing.T) {
	metrics := map[string]Metric{
		"velocity": {Name: "velocity", Value: 5, Threshold: 3, Passed: true},
		"rework":   {Name: "rework", Value: 10, Threshold: 30, Passed: true},
		"trust":    {Name: "trust", Value: 0.5, Threshold: 0.3, Passed: true},
		"spirals":  {Name: "spirals", Value: 0, Threshold: 0, Passed: true},
		"flow":     {Name: "flow", Value: 80, Threshold: 50, Passed: true},
	}
	score, grade := ComputeOverallRating(metrics)
	if score != 100 {
		t.Errorf("expected score 100, got %f", score)
	}
	if grade != "A" {
		t.Errorf("expected grade A, got %s", grade)
	}
}

func TestComputeOverallRating_AllFail(t *testing.T) {
	metrics := map[string]Metric{
		"velocity": {Name: "velocity", Value: 0, Threshold: 3, Passed: false},
		"rework":   {Name: "rework", Value: 90, Threshold: 30, Passed: false},
		"trust":    {Name: "trust", Value: 0, Threshold: 0.3, Passed: false},
		"spirals":  {Name: "spirals", Value: 3, Threshold: 0, Passed: false},
		"flow":     {Name: "flow", Value: 0, Threshold: 50, Passed: false},
	}
	score, grade := ComputeOverallRating(metrics)
	if score >= 40 {
		t.Errorf("expected score < 40, got %f", score)
	}
	_ = grade // Grade depends on exact partial credit
}

func TestComputeOverallRating_Empty(t *testing.T) {
	score, grade := ComputeOverallRating(nil)
	if score != 0 {
		t.Errorf("expected 0, got %f", score)
	}
	if grade != "F" {
		t.Errorf("expected F, got %s", grade)
	}
}

func TestFormatMetricsSummary(t *testing.T) {
	metrics := ComputeMetrics([]TimelineEvent{
		makeTestEvent("a1", "feat: one", baseTime()),
	})
	score, grade := ComputeOverallRating(metrics)
	summary := FormatMetricsSummary(metrics, score, grade)
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestComputeOverallRating_NonStandardMetricCount(t *testing.T) {
	// Use only 3 metrics instead of 5 to exercise the normalization path
	metrics := map[string]Metric{
		"velocity": {Name: "velocity", Value: 5, Threshold: 3, Passed: true},
		"trust":    {Name: "trust", Value: 0.5, Threshold: 0.3, Passed: true},
		"flow":     {Name: "flow", Value: 80, Threshold: 50, Passed: true},
	}
	score, grade := ComputeOverallRating(metrics)
	if score != 100 {
		t.Errorf("expected score 100 for all-passing metrics, got %f", score)
	}
	if grade != "A" {
		t.Errorf("expected grade A, got %s", grade)
	}
}

func TestMetricPartialCredit_ReworkHighValue(t *testing.T) {
	// Rework with value > threshold (100%) -- ratio should be negative, clamped to 0
	m := Metric{Name: "rework", Value: 100, Threshold: 30, Passed: false}
	credit := metricPartialCredit(m)
	if credit != 0 {
		t.Errorf("expected 0 credit for 100%% rework, got %f", credit)
	}
}

func TestMetricPartialCredit_ReworkModerateValue(t *testing.T) {
	// Rework at 50% with threshold 30%: ratio = (100-50)/(100-30) = 50/70 = ~0.714
	m := Metric{Name: "rework", Value: 50, Threshold: 30, Passed: false}
	credit := metricPartialCredit(m)
	expected := 50.0 / 70.0 * 20
	if credit < expected-0.1 || credit > expected+0.1 {
		t.Errorf("expected credit ~%.1f, got %f", expected, credit)
	}
}

func TestMetricPartialCredit_ReworkBelowThreshold(t *testing.T) {
	// Rework below threshold should get full 20 points
	m := Metric{Name: "rework", Value: 10, Threshold: 30, Passed: false}
	credit := metricPartialCredit(m)
	if credit != 20 {
		t.Errorf("expected 20 credit for rework below threshold, got %f", credit)
	}
}

func TestMetricPartialCredit_VelocityPartial(t *testing.T) {
	// Velocity halfway to threshold: value=1.5, threshold=3 -> ratio 0.5 -> credit 10
	m := Metric{Name: "velocity", Value: 1.5, Threshold: 3, Passed: false}
	credit := metricPartialCredit(m)
	if credit != 10 {
		t.Errorf("expected 10 credit for velocity at half threshold, got %f", credit)
	}
}

func TestMetricPartialCredit_VelocityAboveThreshold(t *testing.T) {
	// Value > threshold but somehow not passed: ratio clamped to 1
	m := Metric{Name: "velocity", Value: 5, Threshold: 3, Passed: false}
	credit := metricPartialCredit(m)
	if credit != 20 {
		t.Errorf("expected 20 credit (clamped), got %f", credit)
	}
}

func TestMetricPartialCredit_ZeroThreshold(t *testing.T) {
	// Zero threshold (like spirals) -- if not passed, value>0, so no credit
	m := Metric{Name: "spirals", Value: 2, Threshold: 0, Passed: false}
	credit := metricPartialCredit(m)
	if credit != 0 {
		t.Errorf("expected 0 credit for zero threshold, got %f", credit)
	}
}

func TestMetricPartialCredit_UnknownMetric(t *testing.T) {
	m := Metric{Name: "unknown-metric", Value: 5, Threshold: 10, Passed: false}
	credit := metricPartialCredit(m)
	if credit != 0 {
		t.Errorf("expected 0 credit for unknown metric, got %f", credit)
	}
}

func TestMetricPartialCredit_TrustAndFlow(t *testing.T) {
	// Trust partial credit
	m := Metric{Name: "trust", Value: 0.15, Threshold: 0.3, Passed: false}
	credit := metricPartialCredit(m)
	expected := 0.15 / 0.3 * 20 // = 10
	if credit < expected-0.1 || credit > expected+0.1 {
		t.Errorf("expected trust credit ~%.1f, got %f", expected, credit)
	}

	// Flow partial credit
	m = Metric{Name: "flow", Value: 25, Threshold: 50, Passed: false}
	credit = metricPartialCredit(m)
	expected = 25.0 / 50.0 * 20 // = 10
	if credit < expected-0.1 || credit > expected+0.1 {
		t.Errorf("expected flow credit ~%.1f, got %f", expected, credit)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{100, "A"},
		{80, "A"},
		{79, "B"},
		{60, "B"},
		{59, "C"},
		{40, "C"},
		{39, "D"},
		{20, "D"},
		{19, "F"},
		{0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestCountSpirals_DifferentComponents(t *testing.T) {
	base := baseTime()
	// Fix commits on different components should NOT form a spiral
	events := []TimelineEvent{
		makeTestEvent("a1", "fix(auth): token issue", base),
		makeTestEvent("a2", "fix(db): connection pool", base.Add(10*time.Minute)),
		makeTestEvent("a3", "fix(auth): another auth issue", base.Add(20*time.Minute)),
	}
	count := countSpirals(events)
	if count != 0 {
		t.Errorf("expected 0 spirals for different components, got %d", count)
	}
}

func TestCountSpirals_ChainBrokenByFeat(t *testing.T) {
	base := baseTime()
	// 3 fix commits interrupted by feat
	events := []TimelineEvent{
		makeTestEvent("a1", "fix(auth): issue 1", base),
		makeTestEvent("a2", "fix(auth): issue 2", base.Add(10*time.Minute)),
		makeTestEvent("a3", "feat: new feature", base.Add(20*time.Minute)),
		makeTestEvent("a4", "fix(auth): issue 3", base.Add(30*time.Minute)),
	}
	count := countSpirals(events)
	if count != 0 {
		t.Errorf("expected 0 spirals (chain broken by feat), got %d", count)
	}
}

func TestCountSpirals_ExactlyThree(t *testing.T) {
	base := baseTime()
	// Exactly 3 fix commits on same component followed by non-fix
	events := []TimelineEvent{
		makeTestEvent("a1", "fix(auth): issue 1", base),
		makeTestEvent("a2", "fix(auth): issue 2", base.Add(10*time.Minute)),
		makeTestEvent("a3", "fix(auth): issue 3", base.Add(20*time.Minute)),
		makeTestEvent("a4", "feat: done", base.Add(30*time.Minute)),
	}
	count := countSpirals(events)
	if count != 1 {
		t.Errorf("expected 1 spiral for exactly 3 fix commits, got %d", count)
	}
}

func TestCountSpirals_EndOfEventsFlush(t *testing.T) {
	base := baseTime()
	// 3 fix commits at end of events should flush as spiral
	events := []TimelineEvent{
		makeTestEvent("a1", "fix(auth): issue 1", base),
		makeTestEvent("a2", "fix(auth): issue 2", base.Add(10*time.Minute)),
		makeTestEvent("a3", "fix(auth): issue 3", base.Add(20*time.Minute)),
	}
	count := countSpirals(events)
	if count != 1 {
		t.Errorf("expected 1 spiral at end of events, got %d", count)
	}
}

func TestCountSpirals_ComponentSwitchFlush(t *testing.T) {
	base := baseTime()
	// 3 fix on auth, then switch to db (should flush auth spiral)
	events := []TimelineEvent{
		makeTestEvent("a1", "fix(auth): issue 1", base),
		makeTestEvent("a2", "fix(auth): issue 2", base.Add(10*time.Minute)),
		makeTestEvent("a3", "fix(auth): issue 3", base.Add(20*time.Minute)),
		makeTestEvent("a4", "fix(db): different component", base.Add(30*time.Minute)),
	}
	count := countSpirals(events)
	if count != 1 {
		t.Errorf("expected 1 spiral (flushed on component switch), got %d", count)
	}
}

func TestExtractComponent_Variations(t *testing.T) {
	tests := []struct {
		msg  string
		want string
	}{
		{"fix(auth): token issue", "auth"},
		{"fix: broken login page", "broken"},
		{"fix typo in readme", "typo"},
		{"fix(): empty scope", "fix()"},
		{"fix a b c", "unknown"}, // all short words
	}
	for _, tt := range tests {
		got := extractComponent(tt.msg)
		if got != tt.want {
			t.Errorf("extractComponent(%q) = %q, want %q", tt.msg, got, tt.want)
		}
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{-5, "-5"},
		{100, "100"},
	}
	for _, tt := range tests {
		got := itoa(tt.n)
		if got != tt.want {
			t.Errorf("itoa(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatMetricsSummary_MissingMetric(t *testing.T) {
	// Should handle missing metrics gracefully (just skip them)
	metrics := map[string]Metric{
		"velocity": {Name: "velocity", Value: 5, Threshold: 3, Passed: true},
	}
	summary := FormatMetricsSummary(metrics, 20, "D")
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestMetricTrust_AllTestCommits(t *testing.T) {
	// When all commits are test commits, codeCommits == 0, trust should be 1.0
	events := []TimelineEvent{
		{Message: "test: add unit tests"},
		{Message: "test(pool): add coverage"},
		{Message: "tests: fix flaky test"},
	}
	m := MetricTrust(events)
	if m.Value != 1.0 {
		t.Errorf("expected trust 1.0 for all test commits, got %f", m.Value)
	}
	if !m.Passed {
		t.Error("expected trust to pass for all test commits")
	}
}

func TestIsTestCommit_Keywords(t *testing.T) {
	// Exercise the keyword matching path in isTestCommit
	cases := []struct {
		msg  string
		want bool
	}{
		{"add test for edge case", true},
		{"update test expectations", true},
		{"fix test flakiness", true},
		{"write test for parser", true},
		{"testing new approach", true},
		{"refactor: clean up code", false},
		{"feat: add new feature", false},
	}
	for _, tc := range cases {
		got := isTestCommit(tc.msg)
		if got != tc.want {
			t.Errorf("isTestCommit(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

func TestComputeOverallRating_OverflowClamp(t *testing.T) {
	// Exercise the total > 100 and total < 0 clamping paths.
	// Create metrics where all pass (total = 5*20 = 100).
	metrics := map[string]Metric{
		"velocity": {Name: "velocity", Value: 10, Threshold: 3, Passed: true},
		"rework":   {Name: "rework", Value: 5, Threshold: 30, Passed: true},
		"trust":    {Name: "trust", Value: 0.5, Threshold: 0.3, Passed: true},
		"spirals":  {Name: "spirals", Value: 0, Threshold: 0, Passed: true},
		"flow":     {Name: "flow", Value: 3, Threshold: 2, Passed: true},
	}
	score, grade := ComputeOverallRating(metrics)
	if score > 100 {
		t.Errorf("score should be clamped to 100, got %f", score)
	}
	if grade != "A" {
		t.Errorf("expected grade A for all passed, got %s", grade)
	}
}

func TestComputeOverallRating_ThreeMetricsNormalized(t *testing.T) {
	// Exercise the count != 5 normalization path.
	metrics := map[string]Metric{
		"velocity": {Name: "velocity", Value: 10, Threshold: 3, Passed: true},
		"rework":   {Name: "rework", Value: 5, Threshold: 30, Passed: true},
		"trust":    {Name: "trust", Value: 0.5, Threshold: 0.3, Passed: true},
	}
	score, _ := ComputeOverallRating(metrics)
	// 3 metrics all passed: each 20 pts = 60, normalized: 60/3*5 = 100
	if score != 100 {
		t.Errorf("expected 100 for 3/3 passed metrics (normalized), got %f", score)
	}
}

func TestMetricPartialCredit_SpiralZeroThreshold(t *testing.T) {
	// Exercise the threshold == 0 path in metricPartialCredit
	m := Metric{Name: "spirals", Value: 2, Threshold: 0, Passed: false}
	credit := metricPartialCredit(m)
	if credit != 0 {
		t.Errorf("expected 0 partial credit for threshold=0, got %f", credit)
	}
}

func TestMetricPartialCredit_DefaultCase(t *testing.T) {
	// Exercise the default case in metricPartialCredit (unknown metric name)
	m := Metric{Name: "unknown", Value: 5, Threshold: 10, Passed: false}
	credit := metricPartialCredit(m)
	if credit != 0 {
		t.Errorf("expected 0 partial credit for unknown metric, got %f", credit)
	}
}

func TestMeanStddev_EmptySlice(t *testing.T) {
	// Exercise the len(xs) == 0 path in meanStddev
	mean, stddev := meanStddev(nil)
	if mean != 0 || stddev != 0 {
		t.Errorf("expected (0, 0) for empty slice, got (%f, %f)", mean, stddev)
	}
}

func TestComputeOverallRating_ClampAbove100(t *testing.T) {
	// Exercise the total > 100 clamping path (line 54-56).
	// Create 5 metrics that all pass plus have high values to push total > 100.
	// Each passed metric gets 20 points, and 5 * 20 = 100.
	// To exceed 100, use fewer than 5 metrics (e.g., 3 passed).
	// The normalization adjusts: total / count * 5 = 60 / 3 * 5 = 100.
	// That equals 100 exactly, not > 100. Need a different approach.
	//
	// Actually, with count != 5, total = total / count * 5.
	// If count = 2 and both pass: total = 40, normalized = 40/2*5 = 100.
	// If count = 1 and it passes: total = 20, normalized = 20/1*5 = 100.
	// These equal 100, not > 100. The only way to exceed 100 is if
	// partial credit + passed sums exceed expectations, which won't happen
	// with the capped partial credit formula.
	//
	// With count = 4, all pass: total=80, normalized = 80/4*5 = 100. Still exact.
	// With count = 3, all pass: total=60, normalized = 60/3*5 = 100. Still exact.
	// With count = 6, all pass: total=120, normalized = 120/6*5 = 100. Still exact.
	//
	// The > 100 and < 0 clamps seem unreachable with normal inputs.
	// Let's verify this by testing the boundary instead.

	// Boundary test: 5 metrics all pass => score exactly 100.
	metrics := map[string]Metric{
		"velocity": {Name: "velocity", Value: 5, Threshold: 3, Passed: true},
		"rework":   {Name: "rework", Value: 10, Threshold: 30, Passed: true},
		"trust":    {Name: "trust", Value: 80, Threshold: 50, Passed: true},
		"flow":     {Name: "flow", Value: 90, Threshold: 50, Passed: true},
		"spirals":  {Name: "spirals", Value: 0, Threshold: 5, Passed: true},
	}
	score, grade := ComputeOverallRating(metrics)
	if score != 100 {
		t.Errorf("expected score 100 for all passing metrics, got %f", score)
	}
	if grade != "A" {
		t.Errorf("expected grade A, got %s", grade)
	}
}

func TestMetricPartialCredit_ReworkValueAbove100(t *testing.T) {
	// Exercise the ratio < 0 path (line 79-81) in metricPartialCredit.
	// For "rework" with value >= threshold: ratio = (100 - value) / (100 - threshold)
	// If value = 110, threshold = 50: ratio = (100-110)/(100-50) = -10/50 = -0.2 < 0 => return 0
	m := Metric{
		Name:      "rework",
		Value:     110, // Above 100 -- makes ratio negative
		Threshold: 50,
		Passed:    false,
	}
	credit := metricPartialCredit(m)
	if credit != 0 {
		t.Errorf("expected 0 partial credit for rework value>100, got %f", credit)
	}
}

func TestMetricPartialCredit_VelocityNegativeThreshold(t *testing.T) {
	// Exercise the return 0 path (line 95) in metricPartialCredit.
	// For "velocity" with threshold <= 0 but != 0 (negative).
	// The top-level check `if m.Threshold == 0` returns early,
	// so a negative threshold falls through to the switch case,
	// where `if m.Threshold > 0` is false, reaching line 95.
	m := Metric{
		Name:      "velocity",
		Value:     3,
		Threshold: -1, // negative threshold triggers the else path
		Passed:    false,
	}
	credit := metricPartialCredit(m)
	if credit != 0 {
		t.Errorf("expected 0 partial credit for negative threshold, got %f", credit)
	}
}

// --- Benchmarks ---

func benchEvents(n int) []TimelineEvent {
	events := make([]TimelineEvent, n)
	base := time.Now().Add(-time.Duration(n) * time.Hour)
	for i := range n {
		events[i] = TimelineEvent{
			Timestamp:    base.Add(time.Duration(i) * time.Hour),
			SHA:          fmt.Sprintf("abc%04d", i),
			Author:       "bench",
			Message:      fmt.Sprintf("commit %d: refactor module", i),
			FilesChanged: 3,
			Insertions:   50,
			Deletions:    20,
		}
	}
	return events
}

func TestClampScore_NegativeClamps(t *testing.T) {
	score := clampScore(-10.0, 5)
	if score != 0 {
		t.Errorf("clampScore(-10, 5) = %v, want 0", score)
	}
}

func TestClampScore_Over100Clamps(t *testing.T) {
	score := clampScore(120.0, 5)
	if score != 100 {
		t.Errorf("clampScore(120, 5) = %v, want 100", score)
	}
}

func TestClampScore_NormalizeNon5Count(t *testing.T) {
	// 3 metrics each scoring 20 = 60 raw, normalized: 60/3*5 = 100
	score := clampScore(60.0, 3)
	if score != 100 {
		t.Errorf("clampScore(60, 3) = %v, want 100", score)
	}
}

func TestClampScore_PassThrough(t *testing.T) {
	score := clampScore(75.0, 5)
	if score != 75.0 {
		t.Errorf("clampScore(75, 5) = %v, want 75", score)
	}
}

func BenchmarkComputeMetrics(b *testing.B) {
	events := benchEvents(100)
	b.ResetTimer()
	for range b.N {
		ComputeMetrics(events)
	}
}

func BenchmarkComputeOverallRating(b *testing.B) {
	events := benchEvents(100)
	metrics := ComputeMetrics(events)
	b.ResetTimer()
	for range b.N {
		ComputeOverallRating(metrics)
	}
}

// --- dailyCommitCounts ---

func TestDailyCommitCounts(t *testing.T) {
	base := baseTime()
	events := []TimelineEvent{
		makeTestEvent("a1", "feat: day1", base),
		makeTestEvent("a2", "feat: day1b", base.Add(2*time.Hour)),
		makeTestEvent("a3", "feat: day2", base.Add(24*time.Hour)),
	}
	counts := dailyCommitCounts(events)
	if len(counts) != 2 {
		t.Errorf("expected 2 days, got %d", len(counts))
	}
	if counts[0] != 2 {
		t.Errorf("expected 2 commits on day 1, got %v", counts[0])
	}
	if counts[1] != 1 {
		t.Errorf("expected 1 commit on day 2, got %v", counts[1])
	}
}

func TestDailyCommitCounts_WithGap(t *testing.T) {
	base := baseTime()
	events := []TimelineEvent{
		makeTestEvent("a1", "feat: day1", base),
		makeTestEvent("a2", "feat: day3", base.Add(48*time.Hour)),
	}
	counts := dailyCommitCounts(events)
	if len(counts) != 3 {
		t.Errorf("expected 3 days (including gap), got %d", len(counts))
	}
	if counts[1] != 0 {
		t.Errorf("expected 0 commits on gap day, got %v", counts[1])
	}
}

// --- MetricVelocity single commit ---

func TestMetricVelocity_SingleCommit(t *testing.T) {
	events := []TimelineEvent{
		makeTestEvent("a1", "feat: only one", baseTime()),
	}
	m := MetricVelocity(events)
	if m.Value != 1 {
		t.Errorf("expected velocity 1 for single commit, got %f", m.Value)
	}
}

// --- MetricFlow single event ---

func TestMetricFlow_SingleEvent(t *testing.T) {
	m := MetricFlow([]TimelineEvent{
		makeTestEvent("a1", "feat: only", baseTime()),
	})
	if m.Passed {
		t.Error("expected not passed for single event")
	}
}

// --- MetricSpirals less than 3 events ---

func TestMetricSpirals_TwoEvents(t *testing.T) {
	m := MetricSpirals([]TimelineEvent{
		makeTestEvent("a1", "fix: a", baseTime()),
		makeTestEvent("a2", "fix: b", baseTime().Add(time.Minute)),
	})
	if !m.Passed {
		t.Error("expected passed for < 3 events")
	}
}

// --- MetricTrust edge case: all test commits ---

func TestMetricTrust_SingleTestCommit(t *testing.T) {
	m := MetricTrust([]TimelineEvent{
		{Message: "test: one"},
	})
	if m.Value != 1.0 {
		t.Errorf("expected 1.0 for single test commit, got %f", m.Value)
	}
}

// --- partialCreditHigherIsBetter ---

func TestPartialCreditHigherIsBetter(t *testing.T) {
	credit := partialCreditHigherIsBetter(0, 0)
	if credit != 0 {
		t.Errorf("expected 0 for zero threshold, got %f", credit)
	}

	credit = partialCreditHigherIsBetter(1.5, 3)
	if credit != 10 {
		t.Errorf("expected 10 for half-way, got %f", credit)
	}
}

// --- partialCreditLowerIsBetter ---

func TestPartialCreditLowerIsBetter(t *testing.T) {
	credit := partialCreditLowerIsBetter(10, 30)
	if credit != 20 {
		t.Errorf("expected 20 for value below threshold, got %f", credit)
	}

	credit = partialCreditLowerIsBetter(100, 30)
	if credit != 0 {
		t.Errorf("expected 0 for value at 100, got %f", credit)
	}
}
