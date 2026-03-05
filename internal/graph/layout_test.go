package graph

import (
	"testing"
)

func TestLayout_SingleNode(t *testing.T) {
	g := NewGraph()
	g.Nodes[1] = &GraphNode{
		ID:           1,
		Labels:       []string{"Person"},
		DisplayLabel: ":Person",
		Properties:   map[string]any{},
	}

	Layout(g, StyleDetailed)

	node := g.Nodes[1]
	if node.Layer != 0 {
		t.Errorf("expected layer 0, got %d", node.Layer)
	}
	if node.X != 0 || node.Y != 0 {
		t.Errorf("expected position (0,0), got (%d,%d)", node.X, node.Y)
	}
}

func TestLayout_Chain(t *testing.T) {
	g := NewGraph()
	g.Nodes[1] = &GraphNode{ID: 1, DisplayLabel: ":A"}
	g.Nodes[2] = &GraphNode{ID: 2, DisplayLabel: ":B"}
	g.Nodes[3] = &GraphNode{ID: 3, DisplayLabel: ":C"}
	g.Edges = []*GraphEdge{
		{ID: 10, StartID: 1, EndID: 2, Type: "R"},
		{ID: 11, StartID: 2, EndID: 3, Type: "R"},
	}

	Layout(g, StyleDetailed)

	if g.Nodes[1].Layer != 0 {
		t.Errorf("node 1: expected layer 0, got %d", g.Nodes[1].Layer)
	}
	if g.Nodes[2].Layer != 1 {
		t.Errorf("node 2: expected layer 1, got %d", g.Nodes[2].Layer)
	}
	if g.Nodes[3].Layer != 2 {
		t.Errorf("node 3: expected layer 2, got %d", g.Nodes[3].Layer)
	}

	// Each layer should be below the previous
	if g.Nodes[2].Y <= g.Nodes[1].Y {
		t.Error("node 2 should be below node 1")
	}
	if g.Nodes[3].Y <= g.Nodes[2].Y {
		t.Error("node 3 should be below node 2")
	}
}

func TestLayout_Empty(t *testing.T) {
	g := NewGraph()
	Layout(g, StyleDetailed) // Should not panic
}

func TestLayout_SameLayerSiblings(t *testing.T) {
	g := NewGraph()
	g.Nodes[1] = &GraphNode{ID: 1, DisplayLabel: ":Root"}
	g.Nodes[2] = &GraphNode{ID: 2, DisplayLabel: ":Child1"}
	g.Nodes[3] = &GraphNode{ID: 3, DisplayLabel: ":Child2"}
	g.Edges = []*GraphEdge{
		{ID: 10, StartID: 1, EndID: 2, Type: "HAS"},
		{ID: 11, StartID: 1, EndID: 3, Type: "HAS"},
	}

	Layout(g, StyleDetailed)

	// Children should be on same layer
	if g.Nodes[2].Layer != g.Nodes[3].Layer {
		t.Errorf("siblings should be on same layer: %d vs %d", g.Nodes[2].Layer, g.Nodes[3].Layer)
	}
	// Children should be side by side (different X)
	if g.Nodes[2].X == g.Nodes[3].X {
		t.Error("siblings should have different X positions")
	}
}
