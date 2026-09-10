package inputscope

import (
	"path"
	"sort"
	"strings"
)

// Select returns existing top-level check scripts changed by this commit or
// either merge parent. Inventory establishes existence, not selection.
func Select(changed, mergeChanged, inventory []string) []string {
	exists := make(map[string]bool)
	for _, name := range inventory {
		exists[name] = true
	}
	selected := make(map[string]bool)
	candidates := append(append([]string(nil), changed...), mergeChanged...)
	for _, name := range candidates {
		if exists[name] && path.Dir(name) == "scripts" && strings.HasPrefix(path.Base(name), "check-") && strings.HasSuffix(name, ".sh") {
			selected[name] = true
		}
	}
	result := make([]string, 0, len(selected))
	for name := range selected {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
