package evalsubstrate

import "testing"

func TestParseIdentifierKindGrammars(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		kind  IdentifierKind
		value string
		want  string
	}{
		{name: "task", kind: IdentifierTask, value: "task-1.alpha", want: "task-1.alpha"},
		{name: "suite", kind: IdentifierSuite, value: "suite_1", want: "suite_1"},
		{name: "run uppercase and colon", kind: IdentifierRun, value: "Run:2026-01", want: "Run%3A2026-01"},
		{name: "model", kind: IdentifierModel, value: "ms:qwen-2.5", want: "ms%3Aqwen-2.5"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			id, err := ParseIdentifier(tc.kind, tc.value)
			if err != nil {
				t.Fatalf("ParseIdentifier: %v", err)
			}
			got := id.StorageName()
			if got != tc.want {
				t.Fatalf("StorageName = %q, want %q", got, tc.want)
			}
			roundTrip, err := ParseStorageIdentifier(tc.kind, got)
			if err != nil {
				t.Fatalf("ParseStorageIdentifier: %v", err)
			}
			if roundTrip.String() != tc.value {
				t.Fatalf("round trip = %q, want %q", roundTrip.String(), tc.value)
			}
		})
	}
}

func TestParseIdentifierRejectsHostileValues(t *testing.T) {
	t.Parallel()
	hostile := []string{
		"",
		".",
		"..",
		"../escape",
		`..\escape`,
		"/absolute",
		`\\server\share`,
		`C:\escape`,
		" leading",
		"trailing ",
		"line\nbreak",
		"café",
		"%2e%2e",
	}
	for _, kind := range []IdentifierKind{IdentifierTask, IdentifierSuite, IdentifierRun, IdentifierModel} {
		for _, value := range hostile {
			t.Run(string(kind)+"/"+value, func(t *testing.T) {
				t.Parallel()
				if _, err := ParseIdentifier(kind, value); err == nil {
					t.Fatalf("ParseIdentifier(%q, %q) unexpectedly succeeded", kind, value)
				}
			})
		}
	}
}

func TestParseIdentifierRejectsCrossKindNearMisses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind  IdentifierKind
		value string
	}{
		{IdentifierTask, "Task-1"},
		{IdentifierSuite, "suite:1"},
		{IdentifierRun, "-run"},
		{IdentifierModel, "model-1"},
		{IdentifierModel, "MS:model"},
		{IdentifierModel, "ms:Model"},
	}
	for _, tc := range tests {
		if _, err := ParseIdentifier(tc.kind, tc.value); err == nil {
			t.Fatalf("ParseIdentifier(%q, %q) unexpectedly succeeded", tc.kind, tc.value)
		}
	}
}

func TestParseStorageIdentifierAllowsOnlyCanonicalColonEscape(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"Run%3a1", "Run%2F1", "Run%253A1"} {
		if _, err := ParseStorageIdentifier(IdentifierRun, value); err == nil {
			t.Fatalf("ParseStorageIdentifier(%q) unexpectedly succeeded", value)
		}
	}
}
