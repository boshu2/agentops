// practices: [ai-assisted-dev, pragmatic-programmer]
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGroupJSON_ParentCommandEmitsJSON(t *testing.T) {
	out, err := executeCommand("ratchet", "--json")
	if err != nil {
		t.Fatalf("ao ratchet --json returned error: %v", err)
	}
	var listing groupCommandListing
	if jerr := json.Unmarshal([]byte(out), &listing); jerr != nil {
		t.Fatalf("ao ratchet --json output is not valid JSON: %v\noutput: %s", jerr, out)
	}
	if !listing.IsGroup {
		t.Error("is_group should be true for a parent command")
	}
	if listing.Command != "ao ratchet" {
		t.Errorf("command = %q, want %q", listing.Command, "ao ratchet")
	}
	if len(listing.Subcommands) == 0 {
		t.Error("subcommands listing is empty")
	}
}

func TestGroupJSON_ParentCommandWithoutJSONStillShowsHelp(t *testing.T) {
	out, err := executeCommand("ratchet")
	if err != nil {
		t.Fatalf("ao ratchet returned error: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("ao ratchet (no --json) should print human help, not JSON: %s", out)
	}
}
