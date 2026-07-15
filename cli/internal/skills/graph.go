package skills

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const graphSchemaVersion = "skill-graph.v1"

type GraphNode struct {
	Name          string `json:"name"`
	GraphRoot     bool   `json:"graph_root"`
	UserInvocable bool   `json:"user_invocable"`
}

type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type GraphDiagnostics struct {
	DuplicateNodes      []string    `json:"duplicate_nodes"`
	DanglingEdges       []GraphEdge `json:"dangling_edges"`
	DependencyCycles    [][]string  `json:"dependency_cycles"`
	UnreachableNonRoots []string    `json:"unreachable_non_roots"`
	ZeroInboundRoots    []string    `json:"zero_inbound_roots"`
	EntryPoints         []string    `json:"entry_points"`
}

type SkillGraph struct {
	SchemaVersion string           `json:"schema_version"`
	Nodes         []GraphNode      `json:"nodes"`
	Edges         []GraphEdge      `json:"edges"`
	Diagnostics   GraphDiagnostics `json:"diagnostics"`
}

func BuildGraph(entries []CatalogEntry) SkillGraph {
	g := SkillGraph{
		SchemaVersion: graphSchemaVersion,
		Nodes:         []GraphNode{},
		Edges:         []GraphEdge{},
		Diagnostics: GraphDiagnostics{
			DuplicateNodes: []string{}, DanglingEdges: []GraphEdge{}, DependencyCycles: [][]string{},
			UnreachableNonRoots: []string{}, ZeroInboundRoots: []string{}, EntryPoints: []string{},
		},
	}
	counts := make(map[string]int, len(entries))
	byName := make(map[string]CatalogEntry, len(entries))
	for _, entry := range entries {
		counts[entry.Name]++
		if _, exists := byName[entry.Name]; !exists {
			byName[entry.Name] = entry
			g.Nodes = append(g.Nodes, GraphNode{Name: entry.Name, GraphRoot: entry.GraphRoot, UserInvocable: entry.UserInvocable})
		}
	}
	for name, count := range counts {
		if count > 1 {
			g.Diagnostics.DuplicateNodes = append(g.Diagnostics.DuplicateNodes, name)
		}
	}
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].Name < g.Nodes[j].Name })
	sort.Strings(g.Diagnostics.DuplicateNodes)

	known := make(map[string]bool, len(byName))
	inbound := make(map[string]int, len(byName))
	deps := make(map[string][]string, len(byName))
	for name := range byName {
		known[name] = true
	}
	for _, node := range g.Nodes {
		entry := byName[node.Name]
		for _, dep := range uniqueSorted(entry.Dependencies) {
			edge := GraphEdge{From: node.Name, To: dep, Kind: "dependency"}
			g.Edges = append(g.Edges, edge)
			if known[dep] {
				deps[node.Name] = append(deps[node.Name], dep)
				inbound[dep]++
			} else {
				g.Diagnostics.DanglingEdges = append(g.Diagnostics.DanglingEdges, edge)
			}
		}
		for _, rel := range entry.ContextRel {
			edge := GraphEdge{From: node.Name, To: rel.With, Kind: "context:" + rel.Kind}
			g.Edges = append(g.Edges, edge)
			if !known[rel.With] {
				g.Diagnostics.DanglingEdges = append(g.Diagnostics.DanglingEdges, edge)
			}
		}
		if entry.GraphRoot {
			g.Diagnostics.EntryPoints = append(g.Diagnostics.EntryPoints, node.Name)
		}
	}
	sortEdges(g.Edges)
	sortEdges(g.Diagnostics.DanglingEdges)
	sort.Strings(g.Diagnostics.EntryPoints)

	reachable := make(map[string]bool, len(byName))
	stack := append([]string(nil), g.Diagnostics.EntryPoints...)
	for len(stack) > 0 {
		last := len(stack) - 1
		name := stack[last]
		stack = stack[:last]
		if reachable[name] {
			continue
		}
		reachable[name] = true
		stack = append(stack, deps[name]...)
	}
	for _, node := range g.Nodes {
		if node.GraphRoot && inbound[node.Name] == 0 {
			g.Diagnostics.ZeroInboundRoots = append(g.Diagnostics.ZeroInboundRoots, node.Name)
		}
		if !node.GraphRoot && !reachable[node.Name] {
			g.Diagnostics.UnreachableNonRoots = append(g.Diagnostics.UnreachableNonRoots, node.Name)
		}
	}
	sort.Strings(g.Diagnostics.ZeroInboundRoots)
	sort.Strings(g.Diagnostics.UnreachableNonRoots)
	g.Diagnostics.DependencyCycles = dependencyCycles(g.Nodes, deps)
	return g
}

func (g SkillGraph) Validate() error {
	var problems []string
	if len(g.Diagnostics.DuplicateNodes) > 0 {
		problems = append(problems, "duplicate nodes: "+strings.Join(g.Diagnostics.DuplicateNodes, ", "))
	}
	if len(g.Diagnostics.DanglingEdges) > 0 {
		parts := make([]string, 0, len(g.Diagnostics.DanglingEdges))
		for _, edge := range g.Diagnostics.DanglingEdges {
			parts = append(parts, edge.From+" -> "+edge.To+" ("+edge.Kind+")")
		}
		problems = append(problems, "dangling edges: "+strings.Join(parts, ", "))
	}
	if len(g.Diagnostics.DependencyCycles) > 0 {
		parts := make([]string, 0, len(g.Diagnostics.DependencyCycles))
		for _, cycle := range g.Diagnostics.DependencyCycles {
			parts = append(parts, strings.Join(cycle, " -> "))
		}
		problems = append(problems, "dependency cycles: "+strings.Join(parts, "; "))
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid skill graph: %s", strings.Join(problems, "; "))
	}
	return nil
}

func GraphJSON(entries []CatalogEntry) ([]byte, error) {
	g := BuildGraph(entries)
	return json.MarshalIndent(g, "", "  ")
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func sortEdges(edges []GraphEdge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Kind < edges[j].Kind
	})
}

func dependencyCycles(nodes []GraphNode, deps map[string][]string) [][]string {
	state := make(map[string]int, len(nodes))
	path := make([]string, 0, len(nodes))
	seenCycles := map[string]bool{}
	cycles := make([][]string, 0)
	var visit func(string)
	visit = func(name string) {
		state[name] = 1
		path = append(path, name)
		for _, dep := range uniqueSorted(deps[name]) {
			switch state[dep] {
			case 0:
				visit(dep)
			case 1:
				start := 0
				for start < len(path) && path[start] != dep {
					start++
				}
				cycle := append([]string(nil), path[start:]...)
				cycle = append(cycle, dep)
				key := strings.Join(cycle, "\x00")
				if !seenCycles[key] {
					seenCycles[key] = true
					cycles = append(cycles, cycle)
				}
			}
		}
		path = path[:len(path)-1]
		state[name] = 2
	}
	for _, node := range nodes {
		if state[node.Name] == 0 {
			visit(node.Name)
		}
	}
	sort.Slice(cycles, func(i, j int) bool { return strings.Join(cycles[i], "\x00") < strings.Join(cycles[j], "\x00") })
	return cycles
}
