package evalsubstrate

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// IdentifierKind selects the grammar and storage policy for an eval identifier.
type IdentifierKind string

const (
	IdentifierTask  IdentifierKind = "task"
	IdentifierSuite IdentifierKind = "suite"
	IdentifierRun   IdentifierKind = "run"
	IdentifierModel IdentifierKind = "model"
)

var (
	taskSuiteIdentifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	runIdentifierPattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
	modelIdentifierPattern     = regexp.MustCompile(`^ms:[a-z0-9][a-z0-9._-]*$`)
)

// Identifier is a validated logical identifier. Its storage name is safe on
// every supported platform; notably, logical colons are encoded as %3A.
type Identifier struct {
	kind  IdentifierKind
	value string
}

func ParseIdentifier(kind IdentifierKind, value string) (Identifier, error) {
	if value == "" {
		return Identifier{}, fmt.Errorf("%s id is empty", kind)
	}
	if strings.TrimSpace(value) != value {
		return Identifier{}, fmt.Errorf("%s id %q contains leading or trailing whitespace", kind, value)
	}
	for _, r := range value {
		if r > unicode.MaxASCII {
			return Identifier{}, fmt.Errorf("%s id %q contains non-ASCII characters", kind, value)
		}
		if unicode.IsControl(r) {
			return Identifier{}, fmt.Errorf("%s id %q contains control characters", kind, value)
		}
	}
	if strings.ContainsAny(value, `/\`) {
		return Identifier{}, fmt.Errorf("%s id %q contains a path separator", kind, value)
	}
	if value == "." || value == ".." {
		return Identifier{}, fmt.Errorf("%s id %q is a dot path segment", kind, value)
	}
	if strings.HasPrefix(value, `\\`) || (len(value) >= 2 && value[1] == ':') {
		return Identifier{}, fmt.Errorf("%s id %q is an absolute or volume-qualified path", kind, value)
	}

	var pattern *regexp.Regexp
	switch kind {
	case IdentifierTask, IdentifierSuite:
		pattern = taskSuiteIdentifierPattern
	case IdentifierRun:
		pattern = runIdentifierPattern
	case IdentifierModel:
		pattern = modelIdentifierPattern
	default:
		return Identifier{}, fmt.Errorf("unknown identifier kind %q", kind)
	}
	if !pattern.MatchString(value) {
		return Identifier{}, fmt.Errorf("%s id %q does not match %s", kind, value, identifierGrammar(kind))
	}
	return Identifier{kind: kind, value: value}, nil
}

func identifierGrammar(kind IdentifierKind) string {
	switch kind {
	case IdentifierTask, IdentifierSuite:
		return "[a-z0-9][a-z0-9._-]*"
	case IdentifierRun:
		return "[A-Za-z0-9][A-Za-z0-9._:-]*"
	case IdentifierModel:
		return "ms:[a-z0-9][a-z0-9._-]*"
	default:
		return "<unknown>"
	}
}

func (id Identifier) Kind() IdentifierKind { return id.kind }
func (id Identifier) String() string       { return id.value }

// StorageName returns the canonical directory name for this logical ID.
func (id Identifier) StorageName() string {
	return strings.ReplaceAll(id.value, ":", "%3A")
}

// CompatibilityStorageNames returns the canonical storage name followed by
// the one bounded legacy spelling accepted for reads. New writes always use
// StorageName.
func (id Identifier) CompatibilityStorageNames() []string {
	names := []string{id.StorageName()}
	if names[0] != id.value {
		names = append(names, id.value)
	}
	return names
}

// ParseStorageIdentifier converts a canonical or bounded legacy directory
// name back to its logical ID. Arbitrary percent escapes are not accepted.
func ParseStorageIdentifier(kind IdentifierKind, name string) (Identifier, error) {
	if strings.Contains(name, "%") {
		if strings.Contains(strings.ReplaceAll(name, "%3A", ""), "%") {
			return Identifier{}, fmt.Errorf("%s storage id %q contains an unsupported escape", kind, name)
		}
		name = strings.ReplaceAll(name, "%3A", ":")
	}
	return ParseIdentifier(kind, name)
}
