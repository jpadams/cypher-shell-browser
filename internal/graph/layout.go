package graph

import (
	"sort"
)

const (
	NodePadX            = 4 // horizontal space between nodes
	NodePadY            = 4 // vertical space between layers (room for edge labels + arrow)
	DetailedBoxMinWidth = 14
)

// Layout assigns character coordinates to each node in the graph using a layered approach.
func Layout(g *Graph, style RenderStyle) {
	if len(g.Nodes) == 0 {
		return
	}

	// Build adjacency
	children := make(map[int64][]int64)
	parents := make(map[int64][]int64)
	for _, e := range g.Edges {
		if _, ok := g.Nodes[e.StartID]; !ok {
			continue
		}
		if _, ok := g.Nodes[e.EndID]; !ok {
			continue
		}
		children[e.StartID] = append(children[e.StartID], e.EndID)
		parents[e.EndID] = append(parents[e.EndID], e.StartID)
	}

	// Step 1: Assign layers via BFS from root nodes
	assignLayers(g, children, parents)

	// Step 2: Order nodes within layers to reduce crossings
	orderNodes(g, children)

	// Step 3: Compute node dimensions and convert to character coords
	computePositions(g, style)
}

func assignLayers(g *Graph, children, parents map[int64][]int64) {
	// Find root nodes (no incoming edges among graph nodes)
	roots := make([]int64, 0)
	for id := range g.Nodes {
		if len(parents[id]) == 0 {
			roots = append(roots, id)
		}
	}

	// If no roots (cycle), pick node with lowest ID
	if len(roots) == 0 {
		minID := int64(0)
		first := true
		for id := range g.Nodes {
			if first || id < minID {
				minID = id
				first = false
			}
		}
		roots = append(roots, minID)
	}

	sort.Slice(roots, func(i, j int) bool { return roots[i] < roots[j] })

	assigned := make(map[int64]bool)
	queue := make([]int64, 0, len(g.Nodes))

	for _, r := range roots {
		g.Nodes[r].Layer = 0
		assigned[r] = true
		queue = append(queue, r)
	}

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		layer := g.Nodes[id].Layer

		for _, child := range children[id] {
			if !assigned[child] {
				g.Nodes[child].Layer = layer + 1
				assigned[child] = true
				queue = append(queue, child)
			}
		}
	}

	// Assign unvisited nodes (disconnected components)
	for id, node := range g.Nodes {
		if !assigned[id] {
			node.Layer = 0
			assigned[id] = true
		}
	}
}

func orderNodes(g *Graph, children map[int64][]int64) {
	// Group nodes by layer
	layers := make(map[int][]int64)
	for id, node := range g.Nodes {
		layers[node.Layer] = append(layers[node.Layer], id)
	}

	// Sort each layer by ID first (stable baseline)
	for _, ids := range layers {
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	}

	// Find max layer
	maxLayer := 0
	for l := range layers {
		if l > maxLayer {
			maxLayer = l
		}
	}

	// Barycenter heuristic: order nodes by average position of parents
	for l := 1; l <= maxLayer; l++ {
		ids := layers[l]
		type nodePos struct {
			id       int64
			bary     float64
			hasParen bool
		}
		positions := make([]nodePos, len(ids))
		for i, id := range ids {
			// Find parents of this node
			sum := 0.0
			count := 0
			for _, e := range g.Edges {
				if e.EndID == id {
					if parent, ok := g.Nodes[e.StartID]; ok && parent.Layer == l-1 {
						sum += float64(parent.Order)
						count++
					}
				}
			}
			if count > 0 {
				positions[i] = nodePos{id: id, bary: sum / float64(count), hasParen: true}
			} else {
				positions[i] = nodePos{id: id, bary: float64(i), hasParen: false}
			}
		}
		sort.SliceStable(positions, func(i, j int) bool {
			return positions[i].bary < positions[j].bary
		})
		for i, p := range positions {
			g.Nodes[p.id].Order = i
		}
		// Update layers slice to match new order
		for i, p := range positions {
			ids[i] = p.id
		}
	}

	// Assign order for layer 0
	for i, id := range layers[0] {
		g.Nodes[id].Order = i
	}
}

func computePositions(g *Graph, style RenderStyle) {
	// Calculate node dimensions
	for _, node := range g.Nodes {
		switch style {
		case StyleDetailed:
			w := len(node.DisplayLabel) + 4
			if w < DetailedBoxMinWidth {
				w = DetailedBoxMinWidth
			}
			for _, p := range node.DisplayProps {
				pw := len(p) + 4
				if pw > w {
					w = pw
				}
			}
			node.Width = w
			node.Height = 3 + len(node.DisplayProps) // top border + label + props + bottom border
			if len(node.DisplayProps) > 0 {
				node.Height = 2 + 1 + len(node.DisplayProps) // borders + label + props
			}
		case StyleCompact:
			node.Width = len(node.DisplayLabel) + 2 // parens
			if len(node.DisplayProps) > 0 {
				propW := len(node.DisplayProps[0])
				if propW > node.Width {
					node.Width = propW
				}
			}
			node.Height = 1
			if len(node.DisplayProps) > 0 {
				node.Height = 2
			}
		}
	}

	// Group by layer
	layers := make(map[int][]*GraphNode)
	for _, node := range g.Nodes {
		layers[node.Layer] = append(layers[node.Layer], node)
	}
	for _, nodes := range layers {
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Order < nodes[j].Order })
	}

	maxLayer := 0
	for l := range layers {
		if l > maxLayer {
			maxLayer = l
		}
	}

	// Assign Y positions per layer
	y := 0
	for l := 0; l <= maxLayer; l++ {
		nodes := layers[l]
		maxH := 0
		x := 0
		for _, node := range nodes {
			node.X = x
			node.Y = y
			x += node.Width + NodePadX
			if node.Height > maxH {
				maxH = node.Height
			}
		}
		y += maxH + NodePadY
	}
}
