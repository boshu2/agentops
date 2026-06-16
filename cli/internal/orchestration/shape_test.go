package orchestration

import (
	"reflect"
	"testing"
)

func TestWriteSetsOverlap(t *testing.T) {
	tests := []struct {
		name string
		sets [][]string
		want bool
	}{
		{"empty", nil, false},
		{"single lane", [][]string{{"cli/a.go", "cli/b.go"}}, false},
		{"two disjoint lanes", [][]string{{"cli/a.go"}, {"docs/b.md"}}, false},
		{"two overlapping lanes", [][]string{{"cli/a.go", "schemas/x.json"}, {"docs/b.md", "schemas/x.json"}}, true},
		{"repeat within one lane is not overlap", [][]string{{"cli/a.go", "cli/a.go"}, {"docs/b.md"}}, false},
		{"blank entries ignored", [][]string{{"", "  "}, {"  ", ""}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := writeSetsOverlap(tt.sets); got != tt.want {
				t.Fatalf("writeSetsOverlap(%v) = %v, want %v", tt.sets, got, tt.want)
			}
		})
	}
}

func TestValidateShape(t *testing.T) {
	tests := []struct {
		name           string
		in             ShapeInputs
		wantShape      string
		wantOverridden bool
		wantFired      []string
	}{
		{
			name:           "default stays single-agent with no signals",
			in:             ShapeInputs{Proposed: ShapeSingleAgent},
			wantShape:      ShapeSingleAgent,
			wantOverridden: false,
			wantFired:      []string{},
		},
		{
			name:           "two writers with overlapping write-sets force am-only",
			in:             ShapeInputs{Proposed: ShapeSingleAgent, LiveWriters: 2, WriteSets: [][]string{{"schemas/x.json"}, {"schemas/x.json"}}},
			wantShape:      ShapeAMOnly,
			wantOverridden: true,
			wantFired:      []string{"contention:live-writers-overlap", "override:under-escalated-contention"},
		},
		{
			name:           "two writers with DISJOINT write-sets are not contention (partition beats lock)",
			in:             ShapeInputs{Proposed: ShapeAMOnly, LiveWriters: 2, WriteSets: [][]string{{"cli/a.go"}, {"docs/b.md"}}},
			wantShape:      ShapeSingleAgent,
			wantOverridden: true,
			wantFired:      []string{"override:over-escalated-contention"},
		},
		{
			name:           "unattended need forces atm-only without contention",
			in:             ShapeInputs{Proposed: ShapeSingleAgent, UnattendedNeed: true},
			wantShape:      ShapeATMOnly,
			wantOverridden: true,
			wantFired:      []string{"durability:unattended", "override:under-escalated-durability"},
		},
		{
			name:           "contention plus unattended compose to both",
			in:             ShapeInputs{Proposed: ShapeSingleAgent, LiveWriters: 3, WriteSets: [][]string{{"x"}, {"x"}}, UnattendedNeed: true},
			wantShape:      ShapeBoth,
			wantOverridden: true,
			wantFired:      []string{"contention:live-writers-overlap", "durability:unattended", "override:under-escalated-contention", "override:under-escalated-durability"},
		},
		{
			name:           "over-escalated both is stripped to single-agent (de-mandate)",
			in:             ShapeInputs{Proposed: ShapeBoth},
			wantShape:      ShapeSingleAgent,
			wantOverridden: true,
			wantFired:      []string{"override:over-escalated-contention", "override:over-escalated-durability"},
		},
		{
			name:           "correct am-only proposal under real contention is not overridden",
			in:             ShapeInputs{Proposed: ShapeAMOnly, LiveWriters: 2, WriteSets: [][]string{{"x"}, {"x"}}},
			wantShape:      ShapeAMOnly,
			wantOverridden: false,
			wantFired:      []string{"contention:live-writers-overlap"},
		},
		{
			name:           "lone writer is never contention even with a write-set",
			in:             ShapeInputs{Proposed: ShapeSingleAgent, LiveWriters: 1, WriteSets: [][]string{{"x"}}},
			wantShape:      ShapeSingleAgent,
			wantOverridden: false,
			wantFired:      []string{},
		},
		{
			name:           "unrecognized proposal still resolves required shape, no override classification",
			in:             ShapeInputs{Proposed: "swarm", LiveWriters: 2, WriteSets: [][]string{{"x"}, {"x"}}},
			wantShape:      ShapeAMOnly,
			wantOverridden: false,
			wantFired:      []string{"contention:live-writers-overlap"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateShape(tt.in)
			if got.Shape != tt.wantShape {
				t.Fatalf("Shape = %q, want %q", got.Shape, tt.wantShape)
			}
			if got.Overridden != tt.wantOverridden {
				t.Fatalf("Overridden = %v, want %v", got.Overridden, tt.wantOverridden)
			}
			if !reflect.DeepEqual(got.PredicatesFired, tt.wantFired) {
				t.Fatalf("PredicatesFired = %v, want %v", got.PredicatesFired, tt.wantFired)
			}
			if got.Rationale == "" {
				t.Fatalf("Rationale is empty for shape %q", got.Shape)
			}
		})
	}
}
