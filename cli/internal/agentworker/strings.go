package agentworker

import "strings"

// firstNonEmpty returns the first trimmed-non-empty value, or "" if all are blank.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
