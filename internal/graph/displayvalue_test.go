package graph

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// embeddingString mimics a vector stored as a numpy-style string.
func embeddingString(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("%.8f", float64(i)/10000)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// embeddingList mimics a vector stored as a LIST OF FLOAT.
func embeddingList(n int) []any {
	list := make([]any, n)
	for i := range list {
		list[i] = float64(i) / 10000
	}
	return list
}

func TestDisplayValueCapsLongString(t *testing.T) {
	got := displayValue(embeddingString(1024))

	if n := len([]rune(got)); n > MaxDisplayValueLen {
		t.Errorf("value is %d runes, want at most %d: %q", n, MaxDisplayValueLen, got)
	}
	if !strings.HasPrefix(got, "'") || !strings.HasSuffix(got, "'") {
		t.Errorf("value = %q, want the quotes kept balanced", got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("value = %q, want an ellipsis marking the truncation", got)
	}
	if !strings.HasPrefix(got, "'[0.00000000") {
		t.Errorf("value = %q, want it to start with the real content", got)
	}
}

// A long list is summarised by size: the first few components of an embedding
// tell you nothing, but its dimensionality does.
func TestDisplayValueSummarisesLongList(t *testing.T) {
	if got, want := displayValue(embeddingList(1024)), "[1024 values]"; got != want {
		t.Errorf("displayValue = %q, want %q", got, want)
	}
}

func TestDisplayValueKeepsShortValues(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"short string", "video-context-graph", "'video-context-graph'"},
		{"int", 42, "42"},
		{"float", 0.5, "0.5"},
		{"bool", true, "true"},
		{"short list", []any{1.0, 2.0, 3.0}, cypherValue([]any{1.0, 2.0, 3.0})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := displayValue(tc.in); got != tc.want {
				t.Errorf("displayValue = %q, want %q", got, tc.want)
			}
		})
	}
}

// Truncation must not split a multi-byte rune.
func TestDisplayValueRuneSafe(t *testing.T) {
	got := displayValue(strings.Repeat("日本語テキスト", 40))
	if !utf8.ValidString(got) {
		t.Errorf("value is not valid UTF-8: %q", got)
	}
	if n := len([]rune(got)); n > MaxDisplayValueLen {
		t.Errorf("value is %d runes, want at most %d", n, MaxDisplayValueLen)
	}
}

// Clipboard copies must stay faithful — the cap is a display concern only.
func TestClipboardKeepsFullValue(t *testing.T) {
	embedding := embeddingString(1024)
	props := map[string]any{"id": "seg#0", "embedding": embedding}

	full := PlainCypherNode([]string{"Segment"}, props)
	if !strings.Contains(full, embedding) {
		t.Error("PlainCypherNode dropped or truncated the embedding; copies must be faithful")
	}
	if strings.Contains(full, "…") {
		t.Errorf("PlainCypherNode contains an ellipsis: %.120s", full)
	}

	list := embeddingList(1024)
	fullList := PlainCypherNode([]string{"Segment"}, map[string]any{"embedding": list})
	if strings.Contains(fullList, "1024 values") {
		t.Error("PlainCypherNode summarised a list; copies must be faithful")
	}
}

// A blob property must not set the width of a rendered line.
func TestLongPropertyDoesNotBlowUpLineWidth(t *testing.T) {
	props := map[string]any{
		"id":        "seg#0",
		"embedding": embeddingString(1024),
	}

	for _, p := range formatNodeProps(props) {
		if n := len([]rune(p)); n > MaxDisplayValueLen+40 {
			t.Errorf("display prop is %d runes: %q", n, p)
		}
	}

	// The live compact/tree rendering path must stay comfortably inside a normal
	// terminal, rather than running to thousands of columns.
	rendered := RenderCompactNode([]string{"Segment"}, props, VerbosityMedium)
	if n := len([]rune(plain(rendered))); n > 200 {
		t.Errorf("rendered node is %d runes, want it bounded: %q", n, rendered)
	}
}
