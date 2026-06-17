package orchestration

import "strings"

func normalizeModels(models []string) []string {
	out := make([]string, 0, len(models))
	for _, m := range models {
		m = strings.ToLower(strings.TrimSpace(m))
		if m != "" {
			out = append(out, m)
		}
	}
	return out
}
