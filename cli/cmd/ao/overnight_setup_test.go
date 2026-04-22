package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestIsValidDailyTime(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{in: "00:00", want: true},
		{in: "23:59", want: true},
		{in: "09:05", want: true},
		{in: "24:00", want: false},
		{in: "23:60", want: false},
		{in: "1:00", want: false},  // hour not zero-padded
		{in: "01:5", want: false},  // minute not zero-padded
		{in: "ab:cd", want: false},
		{in: "11-30", want: false},
		{in: "", want: false},
		{in: "11:30:00", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := isValidDailyTime(tc.in); got != tc.want {
				t.Fatalf("isValidDailyTime(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitDailyTime(t *testing.T) {
	tests := []struct {
		in                 string
		wantHour, wantMin  string
	}{
		{in: "09:30", wantHour: "09", wantMin: "30"},
		{in: " 08:15 ", wantHour: "08", wantMin: "15"},
		{in: "23:59", wantHour: "23", wantMin: "59"},
		{in: "24:00", wantHour: "", wantMin: ""},
		{in: "00:60", wantHour: "", wantMin: ""},
		{in: "bad", wantHour: "", wantMin: ""},
		{in: "1:00", wantHour: "", wantMin: ""},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			h, m := splitDailyTime(tc.in)
			if h != tc.wantHour || m != tc.wantMin {
				t.Fatalf("splitDailyTime(%q) = (%q, %q), want (%q, %q)",
					tc.in, h, m, tc.wantHour, tc.wantMin)
			}
		})
	}
}

func TestXMLEscape(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{in: "plain text", want: "plain text"},
		{in: "a & b", want: "a &amp; b"},
		{in: "<tag>", want: "&lt;tag&gt;"},
		{in: `"quoted"`, want: "&quot;quoted&quot;"},
		{in: "it's", want: "it&apos;s"},
		{in: "", want: ""},
		{in: "<&>", want: "&lt;&amp;&gt;"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := xmlEscape(tc.in); got != tc.want {
				t.Fatalf("xmlEscape(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeDreamRunnerList(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "comma-split and lowercase",
			in:   []string{"Codex,Claude"},
			want: []string{"claude", "codex"},
		},
		{
			name: "dedupes",
			in:   []string{"codex", "codex", "Claude"},
			want: []string{"claude", "codex"},
		},
		{
			name: "trims whitespace",
			in:   []string{"  codex , claude  "},
			want: []string{"claude", "codex"},
		},
		{
			name: "ignores empty parts",
			in:   []string{",,codex,,"},
			want: []string{"codex"},
		},
		{
			name:  "empty input",
			in:    nil,
			want:  []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeDreamRunnerList(tc.in)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("normalizeDreamRunnerList(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsDreamSchedulerModeValid(t *testing.T) {
	valid := []string{"manual", "launchd", "cron", "systemd", "task-scheduler"}
	for _, v := range valid {
		if !isDreamSchedulerModeValid(v) {
			t.Fatalf("expected %q to be valid", v)
		}
	}
	invalid := []string{"", "auto", "LAUNCHD", "other", "unknown"}
	for _, v := range invalid {
		if isDreamSchedulerModeValid(v) {
			t.Fatalf("expected %q to be invalid", v)
		}
	}
}

func TestDreamBoolPtr(t *testing.T) {
	trueP := dreamBoolPtr(true)
	if trueP == nil || *trueP != true {
		t.Fatalf("dreamBoolPtr(true) = %v, want pointer to true", trueP)
	}
	falseP := dreamBoolPtr(false)
	if falseP == nil || *falseP != false {
		t.Fatalf("dreamBoolPtr(false) = %v, want pointer to false", falseP)
	}
	// Distinct instances so future mutation doesn't cross-talk.
	if trueP == falseP {
		t.Fatalf("dreamBoolPtr should return fresh pointers each call")
	}
}

func TestRenderDreamCronLine(t *testing.T) {
	got := renderDreamCronLine("/repo", "09:05")
	// Format: "<min> <hour> * * * cd <cwd> && ao overnight start >> <log> 2>&1"
	if !strings.HasPrefix(got, "05 09 * * * ") {
		t.Fatalf("cron line should start with '05 09 * * * '; got %q", got)
	}
	if !strings.Contains(got, "ao overnight start") {
		t.Fatalf("cron line missing command: %q", got)
	}
	if !strings.HasSuffix(got, "2>&1\n") {
		t.Fatalf("cron line should end with stderr redirect+newline, got %q", got)
	}
}

func TestRenderDreamSystemdTimer(t *testing.T) {
	got := renderDreamSystemdTimer("07:30")
	if !strings.Contains(got, "OnCalendar=*-*-* 07:30:00") {
		t.Fatalf("systemd timer missing OnCalendar line: %q", got)
	}
	if !strings.Contains(got, "Persistent=true") {
		t.Fatalf("systemd timer missing Persistent line: %q", got)
	}
	if !strings.Contains(got, "[Install]") {
		t.Fatalf("systemd timer missing [Install] section: %q", got)
	}
}

func TestRenderDreamLaunchdPlist_EscapesXMLAndSetsTimes(t *testing.T) {
	got := renderDreamLaunchdPlist("/path/with<special>&", "06:45")
	if !strings.Contains(got, "<key>Hour</key>\n    <integer>06</integer>") {
		t.Fatalf("launchd plist missing Hour=06, got %q", got)
	}
	if !strings.Contains(got, "<key>Minute</key>\n    <integer>45</integer>") {
		t.Fatalf("launchd plist missing Minute=45, got %q", got)
	}
	if !strings.Contains(got, "/path/with&lt;special&gt;&amp;") {
		t.Fatalf("launchd plist did not XML-escape cwd, got %q", got)
	}
}
