// practices: [sre, resilience-patterns]
package main

import "testing"

func TestExitCodes_Values(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"CodeMoreWork", CodeMoreWork, 0},
		{"CodeError", CodeError, 1},
		{"CodeBeadClaimed", CodeBeadClaimed, 2},
		{"CodeQuestComplete", CodeQuestComplete, 42},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	cases := []struct {
		name string
		code int
		want bool
	}{
		{"complete is terminal", CodeQuestComplete, true},
		{"error is terminal", CodeError, true},
		{"more-work is not terminal", CodeMoreWork, false},
		{"bead-claimed is not terminal", CodeBeadClaimed, false},
		{"unknown code 99 is not terminal", 99, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTerminal(tc.code); got != tc.want {
				t.Errorf("IsTerminal(%d) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}
}
