package goals

import (
	"strconv"
	"strings"
	"testing"
)

// blockOpsGoals builds a GOALS.md with the given directive titles (numbered
// 1..N) followed by a non-directive "## Gates" section + a claim comment, so
// edge-case tests can assert both directive behavior and byte-preservation of
// trailing content.
func blockOpsGoals(titles ...string) string {
	var b strings.Builder
	b.WriteString("# Fitness Goals\n\n## Directives\n\n")
	for i, t := range titles {
		b.WriteString("### ")
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(". ")
		b.WriteString(t)
		b.WriteString("\n\nBody of ")
		b.WriteString(t)
		b.WriteString(".\n\n**Steer:** increase\n\n")
	}
	b.WriteString("## Gates\n\n<!-- agentops:claim:AOP-CLAIM-EDGE -->\n| ID | Check |\n|----|-------|\n| g1 | x.sh |\n")
	return b.String()
}

func patch(t *testing.T, content string) *GoalsPatcher {
	t.Helper()
	p, err := NewGoalsPatcher([]byte(content))
	if err != nil {
		t.Fatalf("NewGoalsPatcher: %v", err)
	}
	return p
}

func TestAppendDirective_IntoZeroDirectiveSection(t *testing.T) {
	p := patch(t, blockOpsGoals()) // "## Directives" present, no directives
	num, err := p.AppendDirective("First", "Body.", "increase")
	if err != nil {
		t.Fatalf("AppendDirective: %v", err)
	}
	if num != 1 {
		t.Errorf("num = %d, want 1", num)
	}
	got := string(p.Bytes())
	if !strings.Contains(got, "### 1. First") {
		t.Errorf("new directive missing:\n%s", got)
	}
	for _, must := range []string{"## Gates", "AOP-CLAIM-EDGE", "| g1 | x.sh |"} {
		if !strings.Contains(got, must) {
			t.Errorf("non-directive content dropped: %q\n%s", must, got)
		}
	}
}

func TestAppendDirective_RenumbersFromMaxNotCount(t *testing.T) {
	// Non-contiguous numbering: a hand-edited GOALS with directives 1, 2, 5.
	src := "# G\n\n## Directives\n\n### 1. A\n\n**Steer:** increase\n\n### 2. B\n\n**Steer:** increase\n\n### 5. C\n\n**Steer:** increase\n"
	p := patch(t, src)
	num, err := p.AppendDirective("D", "body", "hold")
	if err != nil {
		t.Fatalf("AppendDirective: %v", err)
	}
	if num != 6 { // max(1,2,5)+1, not count+1
		t.Errorf("num = %d, want 6 (max+1)", num)
	}
	if !strings.Contains(string(p.Bytes()), "### 6. D") {
		t.Errorf("appended at wrong number:\n%s", string(p.Bytes()))
	}
}

func TestRemoveDirective_OnlyDirective(t *testing.T) {
	p := patch(t, blockOpsGoals("Solo"))
	if err := p.RemoveDirective(1); err != nil {
		t.Fatalf("RemoveDirective: %v", err)
	}
	got := string(p.Bytes())
	if strings.Contains(got, "Solo") {
		t.Errorf("directive not removed:\n%s", got)
	}
	if !strings.Contains(got, "## Gates") || !strings.Contains(got, "AOP-CLAIM-EDGE") {
		t.Errorf("trailing section dropped after removing only directive:\n%s", got)
	}
}

func TestRemoveDirective_Last(t *testing.T) {
	p := patch(t, blockOpsGoals("A", "B", "C"))
	if err := p.RemoveDirective(3); err != nil {
		t.Fatalf("RemoveDirective: %v", err)
	}
	got := string(p.Bytes())
	if strings.Contains(got, "### 3. C") || strings.Contains(got, "Body of C") {
		t.Errorf("last directive not removed:\n%s", got)
	}
	if !strings.Contains(got, "### 1. A") || !strings.Contains(got, "### 2. B") {
		t.Errorf("survivors lost/misnumbered:\n%s", got)
	}
}

func TestRemoveDirective_MiddleRenumbers(t *testing.T) {
	p := patch(t, blockOpsGoals("A", "B", "C"))
	if err := p.RemoveDirective(2); err != nil {
		t.Fatalf("RemoveDirective: %v", err)
	}
	got := string(p.Bytes())
	if !strings.Contains(got, "### 1. A") || !strings.Contains(got, "### 2. C") {
		t.Errorf("C not renumbered 3->2 after removing B:\n%s", got)
	}
	if strings.Contains(got, "### 3.") {
		t.Errorf("stale #3 heading remains:\n%s", got)
	}
}

func TestRemoveDirective_NotFound(t *testing.T) {
	p := patch(t, blockOpsGoals("A"))
	if err := p.RemoveDirective(9); err == nil {
		t.Error("expected error removing nonexistent directive")
	}
}

func TestMoveDirective_SamePositionIsIdempotent(t *testing.T) {
	src := blockOpsGoals("A", "B", "C")
	p := patch(t, src)
	if err := p.MoveDirective(2, 2); err != nil {
		t.Fatalf("MoveDirective: %v", err)
	}
	got := string(p.Bytes())
	for _, want := range []string{"### 1. A", "### 2. B", "### 3. C"} {
		if !strings.Contains(got, want) {
			t.Errorf("same-position move changed order: %q missing\n%s", want, got)
		}
	}
}

func TestMoveDirective_ToLast(t *testing.T) {
	p := patch(t, blockOpsGoals("A", "B", "C"))
	if err := p.MoveDirective(1, 3); err != nil { // move A to the end
		t.Fatalf("MoveDirective: %v", err)
	}
	got := string(p.Bytes())
	for _, want := range []string{"### 1. B", "### 2. C", "### 3. A"} {
		if !strings.Contains(got, want) {
			t.Errorf("move-to-last produced wrong order: %q missing\n%s", want, got)
		}
	}
}

func TestMoveDirective_OutOfRange(t *testing.T) {
	p := patch(t, blockOpsGoals("A", "B"))
	if err := p.MoveDirective(1, 9); err == nil {
		t.Error("expected error for out-of-range position")
	}
}

func TestAppendDirective_RejectsEmpty(t *testing.T) {
	p := patch(t, blockOpsGoals("A"))
	if _, err := p.AppendDirective("", "body", "increase"); err == nil {
		t.Error("expected error for empty title")
	}
	if _, err := p.AppendDirective("T", "", "increase"); err == nil {
		t.Error("expected error for empty description")
	}
}
