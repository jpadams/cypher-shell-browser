package graph

import (
	"strings"
	"testing"
)

func TestRenderGraph_DetailedStyle(t *testing.T) {
	g := NewGraph()
	g.Nodes[1] = &GraphNode{
		ID:           1,
		DisplayLabel: ":Person",
		DisplayProps: []string{"name: 'Neo'"},
	}
	g.Nodes[2] = &GraphNode{
		ID:           2,
		DisplayLabel: ":Movie",
		DisplayProps: []string{"title: 'The Matrix'"},
	}
	g.Edges = []*GraphEdge{
		{ID: 10, Type: "ACTED_IN", StartID: 1, EndID: 2},
	}

	result := RenderGraph(g, StyleDetailed)

	if !strings.Contains(result, ":Person") {
		t.Error("should contain :Person label")
	}
	if !strings.Contains(result, ":Movie") {
		t.Error("should contain :Movie label")
	}
	if !strings.Contains(result, "name: 'Neo'") {
		t.Error("should contain name: 'Neo' property")
	}
	if !strings.Contains(result, "╭") {
		t.Error("should contain box drawing characters")
	}
	if !strings.Contains(result, "ACTED_IN") {
		t.Error("should contain edge label")
	}
	if !strings.Contains(result, "▼") {
		t.Error("should contain arrow for vertical edge")
	}
}

func TestRenderGraph_CompactStyle(t *testing.T) {
	g := NewGraph()
	g.Nodes[1] = &GraphNode{
		ID:           1,
		DisplayLabel: ":Person",
		DisplayProps: []string{"name: 'Neo'"},
	}
	g.Nodes[2] = &GraphNode{
		ID:           2,
		DisplayLabel: ":Movie",
		DisplayProps: []string{"title: 'The Matrix'"},
	}
	g.Edges = []*GraphEdge{
		{ID: 10, Type: "ACTED_IN", StartID: 1, EndID: 2},
	}

	result := RenderGraph(g, StyleCompact)

	// Should be inline Cypher-like
	if !strings.Contains(result, ":Person") {
		t.Error("should contain :Person label")
	}
	if !strings.Contains(result, ":Movie") {
		t.Error("should contain :Movie label")
	}
	if !strings.Contains(result, "ACTED_IN") {
		t.Error("should contain relationship type")
	}
	if !strings.Contains(result, "->") {
		t.Error("should contain arrow syntax")
	}
	// Should be on one line
	lines := strings.Split(result, "\n")
	if len(lines) != 1 {
		t.Errorf("compact style should render chain on one line, got %d lines:\n%s", len(lines), result)
	}
}

func TestRenderGraph_CompactChain(t *testing.T) {
	g := NewGraph()
	g.Nodes[1] = &GraphNode{ID: 1, DisplayLabel: ":A", DisplayProps: []string{"x: 1"}}
	g.Nodes[2] = &GraphNode{ID: 2, DisplayLabel: ":B"}
	g.Nodes[3] = &GraphNode{ID: 3, DisplayLabel: ":C"}
	g.Edges = []*GraphEdge{
		{ID: 10, Type: "R1", StartID: 1, EndID: 2},
		{ID: 11, Type: "R2", StartID: 2, EndID: 3},
	}

	result := RenderGraph(g, StyleCompact)

	// A->B->C should be on one line
	lines := strings.Split(result, "\n")
	if len(lines) != 1 {
		t.Errorf("chain should be one line, got %d:\n%s", len(lines), result)
	}
	if !strings.Contains(result, "R1") || !strings.Contains(result, "R2") {
		t.Error("should contain both relationship types")
	}
}

func TestRenderGraph_CompactDisconnected(t *testing.T) {
	g := NewGraph()
	g.Nodes[1] = &GraphNode{ID: 1, DisplayLabel: ":A"}
	g.Nodes[2] = &GraphNode{ID: 2, DisplayLabel: ":B"}
	// No edges

	result := RenderGraph(g, StyleCompact)

	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("two disconnected nodes should be 2 lines, got %d:\n%s", len(lines), result)
	}
}

func TestRenderGraph_Empty(t *testing.T) {
	g := NewGraph()
	result := RenderGraph(g, StyleDetailed)

	if !strings.Contains(result, "No graph data") {
		t.Errorf("expected no graph message, got: %s", result)
	}
}

func TestCanvas_SetAndRender(t *testing.T) {
	c := NewCanvas(5, 3)
	c.SetString(0, 0, "Hello", nodeLabelStyle)

	result := c.Render()
	if !strings.Contains(result, "H") {
		t.Error("canvas should contain set characters")
	}
}
