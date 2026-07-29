package graph

import (
	"strings"
	"testing"
)

// plain strips ANSI escapes so rendered output can be compared as text.
func plain(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEsc = true
		case inEsc && r == 'm':
			inEsc = false
		case !inEsc:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestRenderCompactNode(t *testing.T) {
	props := map[string]any{"name": "Neo", "age": 30}

	tests := []struct {
		name      string
		labels    []string
		props     map[string]any
		verbosity Verbosity
		want      string
	}{
		{"minimal hides props", []string{"Person"}, props, VerbosityMinimal, "(:Person)"},
		{"medium shows props", []string{"Person"}, props, VerbosityMedium, "(:Person {name: 'Neo', age: 30})"},
		{"multiple labels", []string{"Person", "Employee"}, nil, VerbosityMedium, "(:Person:Employee)"},
		{"no labels", nil, nil, VerbosityMedium, "((node))"},
		{"no props", []string{"Person"}, nil, VerbosityMedium, "(:Person)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := plain(RenderCompactNode(tt.labels, tt.props, tt.verbosity))
			if got != tt.want {
				t.Errorf("RenderCompactNode = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderCompactEdge(t *testing.T) {
	if got, want := plain(RenderCompactEdge("KNOWS", VerbosityMedium)), "-[:KNOWS]->"; got != want {
		t.Errorf("RenderCompactEdge = %q, want %q", got, want)
	}
}

// The branch variant drops the leading dash, which the tree connector supplies.
func TestRenderCompactEdgeAfterBranch(t *testing.T) {
	got := plain(RenderCompactEdgeAfterBranch("KNOWS", VerbosityMedium))
	if want := "[:KNOWS]->"; got != want {
		t.Errorf("RenderCompactEdgeAfterBranch = %q, want %q", got, want)
	}
	if strings.HasPrefix(got, "-") {
		t.Errorf("got %q, want no leading dash", got)
	}
}

// The clipboard forms carry no ANSI and no truncation.
func TestPlainCypherForms(t *testing.T) {
	node := PlainCypherNode([]string{"Person"}, map[string]any{"name": "O'Brien", "age": 30})
	if want := `(:Person {name: 'O\'Brien', age: 30})`; node != want {
		t.Errorf("PlainCypherNode = %q, want %q", node, want)
	}
	if node != plain(node) {
		t.Errorf("PlainCypherNode contains ANSI escapes: %q", node)
	}

	if got, want := PlainCypherEdge("KNOWS"), "-[:KNOWS]->"; got != want {
		t.Errorf("PlainCypherEdge = %q, want %q", got, want)
	}

	bare := PlainCypherNode([]string{"Person"}, nil)
	if want := "(:Person)"; bare != want {
		t.Errorf("PlainCypherNode with no props = %q, want %q", bare, want)
	}
}

// Only the first three properties are shown, with an ellipsis marker.
func TestRenderCompactNodeLimitsPropertyCount(t *testing.T) {
	props := map[string]any{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5}

	got := plain(RenderCompactNode([]string{"N"}, props, VerbosityMedium))
	if !strings.Contains(got, "...") {
		t.Errorf("got %q, want an ellipsis marking the elided properties", got)
	}
	if strings.Contains(got, "e: 5") {
		t.Errorf("got %q, want the property list truncated", got)
	}
}
