package graph

import (
	"fmt"
	"sort"
	"strings"

	n4j "github.com/jeremyadams/cypher-shell-browser/internal/neo4j"
)

const MaxDisplayNodes = 100

func ExtractGraph(result *n4j.QueryResult) (*Graph, string) {
	g := NewGraph()

	for _, node := range result.Nodes {
		gn := &GraphNode{
			ID:         node.ID,
			Labels:     node.Labels,
			Properties: node.Properties,
		}
		gn.DisplayLabel = formatNodeLabel(node.Labels)
		gn.DisplayProps = formatNodeProps(node.Properties)
		g.Nodes[node.ID] = gn
	}

	for _, edge := range result.Edges {
		ge := &GraphEdge{
			ID:         edge.ID,
			Type:       edge.Type,
			StartID:    edge.StartID,
			EndID:      edge.EndID,
			Properties: edge.Properties,
		}
		g.Edges = append(g.Edges, ge)
	}

	var warning string
	if len(g.Nodes) > MaxDisplayNodes {
		warning = fmt.Sprintf("Showing first %d of %d nodes", MaxDisplayNodes, len(g.Nodes))
		trimGraph(g)
	}

	return g, warning
}

func formatNodeLabel(labels []string) string {
	if len(labels) == 0 {
		return "(node)"
	}
	return ":" + strings.Join(labels, ":")
}

func formatNodeProps(props map[string]any) []string {
	if len(props) == 0 {
		return nil
	}
	result := make([]string, 0, len(props))
	// Show name/title first if present
	for _, key := range []string{"name", "title", "id"} {
		if v, ok := props[key]; ok {
			result = append(result, fmt.Sprintf("%s: %s", key, cypherValue(v)))
		}
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		if k != "name" && k != "title" && k != "id" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		result = append(result, fmt.Sprintf("%s: %s", k, cypherValue(props[k])))
	}
	// Limit displayed props
	if len(result) > 3 {
		result = result[:3]
		result = append(result, "...")
	}
	return result
}

// formatAllNodeProps returns all properties without truncation, for clipboard copy.
func formatAllNodeProps(props map[string]any) []string {
	if len(props) == 0 {
		return nil
	}
	result := make([]string, 0, len(props))
	for _, key := range []string{"name", "title", "id"} {
		if v, ok := props[key]; ok {
			result = append(result, fmt.Sprintf("%s: %s", key, cypherValue(v)))
		}
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		if k != "name" && k != "title" && k != "id" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		result = append(result, fmt.Sprintf("%s: %s", k, cypherValue(props[k])))
	}
	return result
}

func cypherValue(v any) string {
	switch v.(type) {
	case int, int64, float64, bool:
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("'%v'", v)
	}
}

func trimGraph(g *Graph) {
	// Keep first MaxDisplayNodes nodes by ID
	ids := make([]int64, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	keep := make(map[int64]bool)
	for i := 0; i < MaxDisplayNodes && i < len(ids); i++ {
		keep[ids[i]] = true
	}

	for id := range g.Nodes {
		if !keep[id] {
			delete(g.Nodes, id)
		}
	}

	filtered := make([]*GraphEdge, 0)
	for _, e := range g.Edges {
		if keep[e.StartID] && keep[e.EndID] {
			filtered = append(filtered, e)
		}
	}
	g.Edges = filtered
}
