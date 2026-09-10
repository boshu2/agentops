package inputscope

import (
	"reflect"
	"testing"
)

func TestEvalMergeUnionAndExclusions(t *testing.T) {
	inventory := []string{"scripts/check-a.sh", "scripts/check-b.sh", "scripts/check-untouched.sh", "scripts/nested/check-deep.sh", "scripts/helper.sh", "docs/check-doc.sh", "scripts/check-data.json"}
	got := Select([]string{"scripts/check-b.sh", "scripts/check-deleted.sh"}, []string{"scripts/check-a.sh", "scripts/check-b.sh", "scripts/nested/check-deep.sh", "scripts/helper.sh", "docs/check-doc.sh", "scripts/check-data.json"}, inventory)
	want := []string{"scripts/check-a.sh", "scripts/check-b.sh"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changed-input union = %v, want %v; inventory is not the selected input", got, want)
	}
	if got := Select(nil, nil, inventory); len(got) != 0 {
		t.Fatalf("unaffected-input negative control selected %v", got)
	}
}

func TestEvalDoesNotMutateInputs(t *testing.T) {
	backing := []string{"scripts/check-a.sh", "unchanged", "sentinel"}
	Select(backing[:1], []string{"scripts/check-b.sh"}, []string{"scripts/check-a.sh", "scripts/check-b.sh"})
	if backing[1] != "unchanged" || backing[2] != "sentinel" {
		t.Fatal("selection mutated caller input backing array")
	}
}
