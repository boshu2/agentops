// Tests for small pure helpers in beads_audit_cluster.go that are referenced
// by the cluster/audit pipeline but had 0% coverage. Each helper drives a
// branch in the user-visible cluster output (epic vs leaf representative,
// shared-keyword summarization, deterministic JSON ordering), so a regression
// would manifest as garbled output rather than a panic.

package main

import (
	"reflect"
	"testing"
)

func TestRepresentativeIsEpic_FoundEpic(t *testing.T) {
	cluster := BeadCluster{
		Representative: "ag-1",
		Beads: []ClusterBead{
			{ID: "ag-2", Title: "leaf", IsEpic: false},
			{ID: "ag-1", Title: "epic", IsEpic: true},
		},
	}
	if !representativeIsEpic(cluster) {
		t.Fatalf("representativeIsEpic = false, want true (representative ag-1 IsEpic=true)")
	}
}

func TestRepresentativeIsEpic_FoundLeaf(t *testing.T) {
	cluster := BeadCluster{
		Representative: "ag-2",
		Beads: []ClusterBead{
			{ID: "ag-1", Title: "epic", IsEpic: true},
			{ID: "ag-2", Title: "leaf", IsEpic: false},
		},
	}
	if representativeIsEpic(cluster) {
		t.Fatalf("representativeIsEpic = true, want false (representative ag-2 IsEpic=false)")
	}
}

func TestRepresentativeIsEpic_RepresentativeMissing(t *testing.T) {
	// When the recorded representative ID is not in the bead slice (which
	// shouldn't happen in production but can occur during partial cluster
	// merges), the helper must return false rather than panic.
	cluster := BeadCluster{
		Representative: "ag-99",
		Beads: []ClusterBead{
			{ID: "ag-1", Title: "epic", IsEpic: true},
		},
	}
	if representativeIsEpic(cluster) {
		t.Fatalf("representativeIsEpic = true, want false when representative not in Beads slice")
	}
}

func TestRepresentativeIsEpic_EmptyCluster(t *testing.T) {
	cluster := BeadCluster{Representative: "any", Beads: nil}
	if representativeIsEpic(cluster) {
		t.Fatalf("representativeIsEpic = true on empty cluster")
	}
}

func TestFirstNNonEmptyLines_BasicTake(t *testing.T) {
	in := "alpha\nbravo\ncharlie\ndelta"
	got := firstNNonEmptyLines(in, 2)
	want := "alpha\nbravo"
	if got != want {
		t.Fatalf("firstNNonEmptyLines = %q, want %q", got, want)
	}
}

func TestFirstNNonEmptyLines_SkipsEmptyAndWhitespaceLines(t *testing.T) {
	in := "\n  \nalpha\n\n   \nbravo\n\ncharlie"
	got := firstNNonEmptyLines(in, 2)
	want := "alpha\nbravo"
	if got != want {
		t.Fatalf("firstNNonEmptyLines (with empty/ws) = %q, want %q", got, want)
	}
}

func TestFirstNNonEmptyLines_TrimsLineWhitespace(t *testing.T) {
	in := "   alpha   \n\tbravo\t"
	got := firstNNonEmptyLines(in, 2)
	want := "alpha\nbravo"
	if got != want {
		t.Fatalf("firstNNonEmptyLines (trim) = %q, want %q", got, want)
	}
}

func TestFirstNNonEmptyLines_FewerLinesThanRequested(t *testing.T) {
	got := firstNNonEmptyLines("only-line", 5)
	want := "only-line"
	if got != want {
		t.Fatalf("firstNNonEmptyLines (n>lines) = %q, want %q", got, want)
	}
}

func TestFirstNNonEmptyLines_EmptyInput(t *testing.T) {
	got := firstNNonEmptyLines("", 3)
	if got != "" {
		t.Fatalf("firstNNonEmptyLines(\"\") = %q, want empty", got)
	}
	got = firstNNonEmptyLines("\n\n  \n\t\n", 3)
	if got != "" {
		t.Fatalf("firstNNonEmptyLines(only-blank) = %q, want empty", got)
	}
}

func TestSortedMapKeys_DeterministicOrder(t *testing.T) {
	m := map[string]bool{"c": true, "a": false, "b": true}
	got := sortedMapKeys(m)
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedMapKeys = %v, want %v", got, want)
	}
}

func TestSortedMapKeys_EmptyMap(t *testing.T) {
	got := sortedMapKeys(map[string]bool{})
	if len(got) != 0 {
		t.Fatalf("sortedMapKeys({}) = %v, want empty slice", got)
	}
}

func TestSortedMapKeys_IgnoresValues(t *testing.T) {
	// Sorted by KEY regardless of bool value
	m := map[string]bool{"zeta": false, "alpha": false, "mu": true}
	got := sortedMapKeys(m)
	want := []string{"alpha", "mu", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedMapKeys (mixed values) = %v, want %v", got, want)
	}
}
