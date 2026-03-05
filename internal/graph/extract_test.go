package graph

import (
	"testing"

	n4j "github.com/jeremyadams/cypher-shell-browser/internal/neo4j"
)

func TestExtractGraph_Basic(t *testing.T) {
	result := &n4j.QueryResult{
		Nodes: []n4j.ResultNode{
			{ID: 1, Labels: []string{"Person"}, Properties: map[string]any{"name": "Alice"}},
			{ID: 2, Labels: []string{"Person"}, Properties: map[string]any{"name": "Bob"}},
		},
		Edges: []n4j.ResultEdge{
			{ID: 10, Type: "KNOWS", StartID: 1, EndID: 2},
		},
	}

	g, warning := ExtractGraph(result)

	if warning != "" {
		t.Errorf("unexpected warning: %s", warning)
	}
	if len(g.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(g.Edges))
	}
	if g.Nodes[1].DisplayLabel != ":Person" {
		t.Errorf("expected :Person label, got %s", g.Nodes[1].DisplayLabel)
	}
}

func TestExtractGraph_Trim(t *testing.T) {
	result := &n4j.QueryResult{}
	for i := int64(0); i < 150; i++ {
		result.Nodes = append(result.Nodes, n4j.ResultNode{
			ID:     i,
			Labels: []string{"Node"},
		})
	}

	g, warning := ExtractGraph(result)

	if warning == "" {
		t.Error("expected warning for large graph")
	}
	if len(g.Nodes) != MaxDisplayNodes {
		t.Errorf("expected %d nodes after trim, got %d", MaxDisplayNodes, len(g.Nodes))
	}
}

func TestExtractGraph_Empty(t *testing.T) {
	result := &n4j.QueryResult{}
	g, _ := ExtractGraph(result)

	if len(g.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(g.Nodes))
	}
}

func TestFormatNodeLabel(t *testing.T) {
	tests := []struct {
		labels []string
		want   string
	}{
		{nil, "(node)"},
		{[]string{"Person"}, ":Person"},
		{[]string{"Person", "Employee"}, ":Person:Employee"},
	}
	for _, tt := range tests {
		got := formatNodeLabel(tt.labels)
		if got != tt.want {
			t.Errorf("formatNodeLabel(%v) = %q, want %q", tt.labels, got, tt.want)
		}
	}
}
