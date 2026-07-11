package cliapp

import "testing"

func TestParseProfileRejectsUnknown(t *testing.T) {
	for _, input := range []string{"", "default ", "DEFAULT", "factory", "flywheel,legacy"} {
		if _, err := ParseProfile(input); err == nil {
			t.Errorf("ParseProfile(%q) error = nil", input)
		}
	}
	for _, want := range []Profile{ProfileDefault, ProfileFlywheel, ProfileLegacy, ProfileCombined} {
		got, err := ParseProfile(string(want))
		if err != nil || got != want {
			t.Errorf("ParseProfile(%q) = %q, %v; want %q, nil", want, got, err, want)
		}
	}
}
