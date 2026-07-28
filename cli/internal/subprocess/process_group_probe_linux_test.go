//go:build linux

package subprocess

import "testing"

func TestParseLinuxProcessStat(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		stat  string
		state string
		group int
	}{
		{
			name:  "ordinary",
			stat:  "42 (bash) S 7 42 42 0 -1",
			state: "S",
			group: 42,
		},
		{
			name:  "command contains closing parenthesis",
			stat:  "84 (odd) command)) Z 7 84 84 0 -1",
			state: "Z",
			group: 84,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			state, group, err := parseLinuxProcessStat(test.stat)
			if err != nil {
				t.Fatalf("parseLinuxProcessStat: %v", err)
			}
			if state != test.state || group != test.group {
				t.Fatalf("state/group = %q/%d, want %q/%d", state, group, test.state, test.group)
			}
		})
	}
}

func TestParseLinuxProcessStatRejectsMalformedInput(t *testing.T) {
	t.Parallel()
	for _, stat := range []string{"", "42 bash S 7 42", "42 (bash) S 7 nope"} {
		if _, _, err := parseLinuxProcessStat(stat); err == nil {
			t.Fatalf("parseLinuxProcessStat(%q) succeeded", stat)
		}
	}
}
