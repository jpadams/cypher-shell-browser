package graph

type GraphNode struct {
	ID           int64
	Labels       []string
	Properties   map[string]any
	DisplayLabel string
	DisplayProps []string
}

type GraphEdge struct {
	ID         int64
	Type       string
	StartID    int64
	EndID      int64
	Properties map[string]any
}

type Graph struct {
	Nodes map[int64]*GraphNode
	Edges []*GraphEdge
}

func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[int64]*GraphNode),
	}
}

type Verbosity int

const (
	VerbosityMinimal Verbosity = iota
	VerbosityMedium
)
