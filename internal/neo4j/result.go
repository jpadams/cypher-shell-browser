package neo4j

import (
	"context"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/dbtype"
)

type QueryResult struct {
	Columns  []string
	Rows     [][]string
	Nodes    []ResultNode
	Edges    []ResultEdge
	Summary  string
	RowPaths [][]RowPathItem
}

// RowPathItem represents a single node or edge in a result row's path.
type RowPathItem struct {
	IsNode     bool
	Labels     []string
	Type       string // edge type
	Properties map[string]any
}

type ResultNode struct {
	ID         int64
	Labels     []string
	Properties map[string]any
}

type ResultEdge struct {
	ID         int64
	Type       string
	StartID    int64
	EndID      int64
	Properties map[string]any
}

func collectResult(ctx context.Context, result neo4j.ResultWithContext) (*QueryResult, error) {
	qr := &QueryResult{}

	keys, err := result.Keys()
	if err != nil {
		return nil, err
	}
	qr.Columns = keys

	nodesSeen := make(map[int64]bool)
	edgesSeen := make(map[int64]bool)

	for result.Next(ctx) {
		record := result.Record()
		row := make([]string, len(record.Values))
		var rowPath []RowPathItem
		for i, val := range record.Values {
			row[i] = formatValue(val)
			extractEntities(val, qr, nodesSeen, edgesSeen)
			rowPath = append(rowPath, extractRowPath(val)...)
		}
		qr.Rows = append(qr.Rows, row)
		qr.RowPaths = append(qr.RowPaths, rowPath)
	}

	if err := result.Err(); err != nil {
		return nil, err
	}

	summary, err := result.Consume(ctx)
	if err == nil && summary != nil {
		counters := summary.Counters()
		parts := []string{}
		if n := counters.NodesCreated(); n > 0 {
			parts = append(parts, fmt.Sprintf("%d nodes created", n))
		}
		if n := counters.NodesDeleted(); n > 0 {
			parts = append(parts, fmt.Sprintf("%d nodes deleted", n))
		}
		if n := counters.RelationshipsCreated(); n > 0 {
			parts = append(parts, fmt.Sprintf("%d relationships created", n))
		}
		if n := counters.RelationshipsDeleted(); n > 0 {
			parts = append(parts, fmt.Sprintf("%d relationships deleted", n))
		}
		if n := counters.PropertiesSet(); n > 0 {
			parts = append(parts, fmt.Sprintf("%d properties set", n))
		}
		if len(parts) > 0 {
			qr.Summary = strings.Join(parts, ", ")
		}
		if qr.Summary == "" {
			if len(qr.Rows) > 0 {
				qr.Summary = fmt.Sprintf("%d rows returned", len(qr.Rows))
			} else {
				qr.Summary = "no changes, no records"
			}
		}
	}

	return qr, nil
}

func extractRowPath(val any) []RowPathItem {
	switch v := val.(type) {
	case dbtype.Node:
		return []RowPathItem{{IsNode: true, Labels: v.Labels, Properties: v.Props}}
	case dbtype.Relationship:
		return []RowPathItem{{IsNode: false, Type: v.Type, Properties: v.Props}}
	case dbtype.Path:
		var items []RowPathItem
		for i, node := range v.Nodes {
			items = append(items, RowPathItem{IsNode: true, Labels: node.Labels, Properties: node.Props})
			if i < len(v.Relationships) {
				rel := v.Relationships[i]
				items = append(items, RowPathItem{IsNode: false, Type: rel.Type, Properties: rel.Props})
			}
		}
		return items
	}
	return nil
}

func extractEntities(val any, qr *QueryResult, nodesSeen, edgesSeen map[int64]bool) {
	switch v := val.(type) {
	case dbtype.Node:
		if !nodesSeen[v.Id] {
			nodesSeen[v.Id] = true
			qr.Nodes = append(qr.Nodes, ResultNode{
				ID:         v.Id,
				Labels:     v.Labels,
				Properties: v.Props,
			})
		}
	case dbtype.Relationship:
		if !edgesSeen[v.Id] {
			edgesSeen[v.Id] = true
			qr.Edges = append(qr.Edges, ResultEdge{
				ID:         v.Id,
				Type:       v.Type,
				StartID:    v.StartId,
				EndID:      v.EndId,
				Properties: v.Props,
			})
		}
		// Also extract start/end nodes if not seen
	case dbtype.Path:
		for _, node := range v.Nodes {
			if !nodesSeen[node.Id] {
				nodesSeen[node.Id] = true
				qr.Nodes = append(qr.Nodes, ResultNode{
					ID:         node.Id,
					Labels:     node.Labels,
					Properties: node.Props,
				})
			}
		}
		for _, rel := range v.Relationships {
			if !edgesSeen[rel.Id] {
				edgesSeen[rel.Id] = true
				qr.Edges = append(qr.Edges, ResultEdge{
					ID:         rel.Id,
					Type:       rel.Type,
					StartID:    rel.StartId,
					EndID:      rel.EndId,
					Properties: rel.Props,
				})
			}
		}
	case []any:
		for _, item := range v {
			extractEntities(item, qr, nodesSeen, edgesSeen)
		}
	case map[string]any:
		for _, item := range v {
			extractEntities(item, qr, nodesSeen, edgesSeen)
		}
	}
}

func formatValue(val any) string {
	if val == nil {
		return "null"
	}
	switch v := val.(type) {
	case dbtype.Node:
		labels := strings.Join(v.Labels, ":")
		return fmt.Sprintf("(:%s %s)", labels, formatProps(v.Props))
	case dbtype.Relationship:
		return fmt.Sprintf("-[:%s %s]->", v.Type, formatProps(v.Props))
	case dbtype.Path:
		parts := []string{}
		for i, node := range v.Nodes {
			parts = append(parts, formatValue(node))
			if i < len(v.Relationships) {
				parts = append(parts, formatValue(v.Relationships[i]))
			}
		}
		return strings.Join(parts, "")
	case []any:
		items := make([]string, len(v))
		for i, item := range v {
			items[i] = formatValue(item)
		}
		return "[" + strings.Join(items, ", ") + "]"
	case map[string]any:
		return formatProps(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatProps(props map[string]any) string {
	if len(props) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(props))
	for k, v := range props {
		parts = append(parts, fmt.Sprintf("%s: %v", k, v))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
