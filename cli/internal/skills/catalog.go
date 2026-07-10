// catalog.go — load and query the generated skills/catalog.json.
//
// The catalog is the queryable inventory emitted by
// scripts/generate-skill-catalog.sh (slice 1 of soc-vuu6.4). This file is
// slice 2: a pure, table-testable query engine plus a thin disk loader so the
// `ao skills list|consumers|producers|graph` commands never re-parse SKILL.md
// frontmatter — they read the committed catalog, which CI keeps in sync.
//
// Source of truth for the JSON shape is schemas/skill-catalog.schema.json.
package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Catalog is the top-level skills/catalog.json document.
type Catalog struct {
	SchemaVersion string         `json:"schema_version"`
	GeneratedAt   string         `json:"generated_at"`
	SkillCount    int            `json:"skill_count"`
	Skills        []CatalogEntry `json:"skills"`
}

// CatalogEntry is one skill's generated metadata. Field tags mirror
// schemas/skill-catalog.schema.json exactly.
type CatalogEntry struct {
	Name                 string       `json:"name"`
	Description          string       `json:"description"`
	HexagonalRole        string       `json:"hexagonal_role"`
	Consumes             []string     `json:"consumes"`
	Produces             []string     `json:"produces"`
	Dependencies         []string     `json:"dependencies"`
	ContextRel           []ContextRel `json:"context_rel"`
	Practices            []string     `json:"practices"`
	UserInvocable        bool         `json:"user_invocable"`
	GraphRoot            bool         `json:"graph_root"`
	CodexOverridePresent bool         `json:"codex_override_present"`
	ReferencesCount      int          `json:"references_count"`
}

// ContextRel is one hex relationship (customer-of, shared-kernel, alias-of).
type ContextRel struct {
	Kind string `json:"kind"`
	With string `json:"with"`
}

// LoadCatalog reads and decodes skills/catalog.json from the given skills
// directory. It returns a clear error if the file is missing (the caller
// should suggest regenerating it) or malformed.
func LoadCatalog(skillsDir string) (*Catalog, error) {
	path := filepath.Join(skillsDir, "catalog.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cat Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cat, nil
}

// ListFilter narrows a List query. A zero-value filter matches every entry.
// String filters are case-insensitive exact matches; UserInvocable is a
// tri-state via a pointer (nil = don't filter).
type ListFilter struct {
	Role          string // hexagonal_role
	Produces      string // an entry in produces[]
	Consumes      string // an entry in consumes[]
	Practice      string // an entry in practices[]
	UserInvocable *bool  // user_invocable, tri-state
}

// List returns the entries matching filter, sorted by name ascending. The
// result is never nil (empty slice on no match) so JSON callers emit `[]`.
func List(entries []CatalogEntry, filter ListFilter) []CatalogEntry {
	out := make([]CatalogEntry, 0, len(entries))
	for _, e := range entries {
		if filter.Role != "" && !strings.EqualFold(e.HexagonalRole, filter.Role) {
			continue
		}
		if filter.Produces != "" && !containsFold(e.Produces, filter.Produces) {
			continue
		}
		if filter.Consumes != "" && !containsFold(e.Consumes, filter.Consumes) {
			continue
		}
		if filter.Practice != "" && !containsFold(e.Practices, filter.Practice) {
			continue
		}
		if filter.UserInvocable != nil && e.UserInvocable != *filter.UserInvocable {
			continue
		}
		out = append(out, e)
	}
	sortByName(out)
	return out
}

// Consumers returns the names of skills that declare `name` in their
// consumes[] list — i.e. who depends on this skill. Sorted, never nil.
func Consumers(entries []CatalogEntry, name string) []string {
	out := make([]string, 0)
	for _, e := range entries {
		if containsFold(e.Consumes, name) {
			out = append(out, e.Name)
		}
	}
	sort.Strings(out)
	return out
}

// Producers returns the names of skills that declare `output` in their
// produces[] list — i.e. who writes this port/artifact. Sorted, never nil.
func Producers(entries []CatalogEntry, output string) []string {
	out := make([]string, 0)
	for _, e := range entries {
		if containsFold(e.Produces, output) {
			out = append(out, e.Name)
		}
	}
	sort.Strings(out)
	return out
}

// Mermaid renders the execution-dependency graph as a Mermaid flowchart. Each
// skill is a node; an edge A-->B means A delegates to or requires B. Output is
// edges are emitted in sorted order. Only edges whose target is a known skill
// are drawn, so the diagram stays inside the catalog.
func Mermaid(entries []CatalogEntry) string {
	known := make(map[string]bool, len(entries))
	for _, e := range entries {
		known[e.Name] = true
	}
	sorted := append([]CatalogEntry(nil), entries...)
	sortByName(sorted)

	var b strings.Builder
	b.WriteString("graph LR\n")
	for _, e := range sorted {
		fmt.Fprintf(&b, "  %s[%s]\n", mermaidID(e.Name), e.Name)
	}
	for _, e := range sorted {
		deps := append([]string(nil), e.Dependencies...)
		sort.Strings(deps)
		for _, dep := range deps {
			if !known[dep] {
				continue
			}
			fmt.Fprintf(&b, "  %s --> %s\n", mermaidID(e.Name), mermaidID(dep))
		}
	}
	return b.String()
}

// mermaidID sanitizes a skill name into a Mermaid-safe node id (Mermaid node
// ids may not safely contain dots, hyphens, or colons unescaped).
func mermaidID(name string) string {
	r := strings.NewReplacer("-", "_", ".", "_", ":", "_")
	return "s_" + r.Replace(name)
}

// containsFold reports whether haystack contains target (case-insensitive).
func containsFold(haystack []string, target string) bool {
	for _, h := range haystack {
		if strings.EqualFold(h, target) {
			return true
		}
	}
	return false
}

// sortByName sorts entries in place by Name ascending.
func sortByName(entries []CatalogEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
}
