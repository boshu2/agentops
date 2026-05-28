// practices: [ai-assisted-dev, pragmatic-programmer]
package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"json", "json", 0},
		{"jsno", "json", 2},
		{"jsn", "json", 1},
		{"verbsoe", "verbose", 2},
		{"", "json", 4},
	}
	for _, c := range cases {
		if got := levenshtein(c.a, c.b); got != c.want {
			t.Errorf("levenshtein(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParseUnknownFlag(t *testing.T) {
	cases := []struct {
		msg  string
		want string
	}{
		{"unknown flag: --jsno", "jsno"},
		{"unknown flag: --verbose extra", "verbose"},
		{"required flag(s) \"kind\" not set", ""},
		{"some other error", ""},
	}
	for _, c := range cases {
		if got := parseUnknownFlag(c.msg); got != c.want {
			t.Errorf("parseUnknownFlag(%q) = %q, want %q", c.msg, got, c.want)
		}
	}
}

func TestFlagErrorWithSuggestion_SuggestsClosestFlag(t *testing.T) {
	out, err := executeCommand("status", "--jsno")
	if err == nil {
		t.Fatal("expected error for unknown flag --jsno")
	}
	if !strings.Contains(out, "Did you mean --json?") {
		t.Errorf("expected typo suggestion for --json, got: %s", out)
	}
}

func TestFlagErrorWithSuggestion_NoSuggestionForGibberish(t *testing.T) {
	out, err := executeCommand("status", "--xyzzy123notaflag")
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if strings.Contains(out, "Did you mean") {
		t.Errorf("should not suggest a flag for gibberish, got: %s", out)
	}
	if !strings.Contains(out, "--help") {
		t.Errorf("expected a --help fallback hint, got: %s", out)
	}
}

func TestWriteRequiredFlagHint(t *testing.T) {
	var buf bytes.Buffer
	writeRequiredFlagHint(&buf, citationCmd, errors.New("required flag(s) \"kind\" not set"))
	got := buf.String()
	if !strings.Contains(got, "Usage:") {
		t.Errorf("expected a Usage line, got: %s", got)
	}
}

func TestWriteRequiredFlagHint_IgnoresUnrelatedErrors(t *testing.T) {
	var buf bytes.Buffer
	writeRequiredFlagHint(&buf, rootCmd, errors.New("some unrelated failure"))
	if buf.Len() != 0 {
		t.Errorf("expected no output for unrelated error, got: %s", buf.String())
	}
}
