package verdictcheck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

func requireExactFields(raw map[string]any, required []string, label string) error {
	allowed := make(map[string]struct{}, len(required))
	for _, field := range required {
		allowed[field] = struct{}{}
		if _, ok := raw[field]; !ok {
			return fmt.Errorf("%s missing required field %q", label, field)
		}
	}
	var extras []string
	for field := range raw {
		if _, ok := allowed[field]; !ok {
			extras = append(extras, field)
		}
	}
	if len(extras) > 0 {
		sort.Strings(extras)
		return fmt.Errorf("%s contains unknown field %q", label, extras[0])
	}
	return nil
}

func verifyCanonicalArtifact(raw map[string]any, claimed, expected, label string) error {
	if !ValidDigest(expected) {
		return fmt.Errorf("%s filename digest is invalid", label)
	}
	if !ValidDigest(claimed) || claimed != expected {
		return fmt.Errorf("artifact_digest does not match filename")
	}
	identity := make(map[string]any, len(raw)-1)
	for key, value := range raw {
		if key != "artifact_digest" {
			identity[key] = value
		}
	}
	canonical, err := CanonicalJSON(identity)
	if err != nil {
		return fmt.Errorf("canonicalize %s: %w", label, err)
	}
	actual := sha256.Sum256(canonical)
	if hex.EncodeToString(actual[:]) != expected {
		return fmt.Errorf("canonical content digest does not match filename")
	}
	return nil
}

func stringValue(value any, label string, nonempty bool) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", label)
	}
	if nonempty && text == "" {
		return "", fmt.Errorf("%s must be a nonempty string", label)
	}
	return text, nil
}

func objectValue(value any, label string) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", label)
	}
	return object, nil
}

func arrayValue(value any, label string) ([]any, error) {
	array, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", label)
	}
	return array, nil
}

func integerValue(value any, label string) (*big.Int, error) {
	number, ok := value.(json.Number)
	if !ok {
		return nil, fmt.Errorf("%s must be an integer", label)
	}
	integer, ok := new(big.Int).SetString(number.String(), 10)
	if !ok {
		return nil, fmt.Errorf("%s must be an integer", label)
	}
	return integer, nil
}

func validID(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		valid := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			(index > 0 && strings.ContainsRune("._:-", character))
		if !valid {
			return false
		}
	}
	return true
}

func requireID(value any, label string) (string, error) {
	text, err := stringValue(value, label, true)
	if err != nil {
		return "", err
	}
	if !validID(text) {
		return "", fmt.Errorf("%s contains unsupported characters", label)
	}
	return text, nil
}

func validRepositoryRef(value string) bool {
	if value == "" || value == "." || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "//") || strings.HasSuffix(value, "/") {
		return false
	}
	if len(value) >= 2 &&
		((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) &&
		value[1] == ':' {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	for _, character := range value {
		if character < 32 || character == 127 {
			return false
		}
	}
	return true
}

func validObservationRootRef(value string) bool {
	return value == "." || validRepositoryRef(value)
}

func requireObservationRootRef(value any, label string) (string, error) {
	text, err := stringValue(value, label, true)
	if err != nil {
		return "", err
	}
	if !validObservationRootRef(text) {
		return "", fmt.Errorf("%s is not a canonical observation-root reference", label)
	}
	return text, nil
}

func requireRepositoryRef(value any, label string) (string, error) {
	text, err := stringValue(value, label, true)
	if err != nil {
		return "", err
	}
	if !validRepositoryRef(text) {
		return "", fmt.Errorf("%s is not a canonical repository-relative reference", label)
	}
	return text, nil
}

func requireDigest(value any, label string) (string, error) {
	text, err := stringValue(value, label, false)
	if err != nil || !ValidDigest(text) {
		return "", fmt.Errorf("%s is not a lowercase SHA-256 digest", label)
	}
	return text, nil
}

func validateStringArray(value any, label string) ([]string, error) {
	array, err := arrayValue(value, label)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(array))
	for index, item := range array {
		text, err := stringValue(item, fmt.Sprintf("%s[%d]", label, index), true)
		if err != nil {
			return nil, err
		}
		result = append(result, text)
	}
	return result, nil
}

func validateDigestArray(value any, label string, requireNonempty bool) ([]string, error) {
	array, err := arrayValue(value, label)
	if err != nil {
		return nil, err
	}
	if requireNonempty && len(array) == 0 {
		return nil, fmt.Errorf("%s must be nonempty", label)
	}
	result := make([]string, 0, len(array))
	seen := make(map[string]struct{}, len(array))
	for index, item := range array {
		digest, err := requireDigest(item, fmt.Sprintf("%s[%d]", label, index))
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[digest]; duplicate {
			return nil, fmt.Errorf("%s contains duplicate digests", label)
		}
		seen[digest] = struct{}{}
		result = append(result, digest)
	}
	return result, nil
}

func validateRFC3339(value any, label string) error {
	text, err := stringValue(value, label, true)
	if err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339, text); err != nil {
		return fmt.Errorf("%s must be an RFC3339 date-time: %w", label, err)
	}
	return nil
}

func runeLength(value string) int {
	return utf8.RuneCountInString(value)
}
