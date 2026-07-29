package graph

import (
	"fmt"
	"sort"
	"strings"
)

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
			result = append(result, fmt.Sprintf("%s: %s", key, displayValue(v)))
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
		result = append(result, fmt.Sprintf("%s: %s", k, displayValue(props[k])))
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

// MaxDisplayValueLen bounds a single property value in the summary views. A
// vector embedding or other blob otherwise decides the width of a node box or a
// compact line, pushing everything else off screen.
const MaxDisplayValueLen = 40

// displayValue renders a property value for the summary views, bounded in
// length. Long lists are summarised by their size rather than truncated
// mid-number, since the first few components of an embedding tell you nothing.
// Clipboard copies go through cypherValue and stay faithful.
func displayValue(v any) string {
	if list, ok := v.([]any); ok {
		if s := cypherValue(v); len([]rune(s)) <= MaxDisplayValueLen {
			return s
		}
		return fmt.Sprintf("[%d values]", len(list))
	}
	return truncateDisplayValue(cypherValue(v), MaxDisplayValueLen)
}

// truncateDisplayValue shortens a rendered value, keeping a quoted literal's
// quotes balanced so the result still reads as a value rather than a fragment.
func truncateDisplayValue(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if len(r) > 1 && r[0] == '\'' && r[len(r)-1] == '\'' {
		inner := r[1 : len(r)-1]
		keep := max - 3 // two quotes and the ellipsis
		if keep < 1 {
			keep = 1
		}
		if keep > len(inner) {
			keep = len(inner)
		}
		return "'" + string(inner[:keep]) + "…'"
	}
	keep := max - 1
	if keep < 1 {
		keep = 1
	}
	return string(r[:keep]) + "…"
}

func cypherValue(v any) string {
	switch val := v.(type) {
	case int, int64, float64, bool:
		return fmt.Sprintf("%v", v)
	case string:
		return "'" + escapeCypherString(val) + "'"
	default:
		return "'" + escapeCypherString(fmt.Sprintf("%v", v)) + "'"
	}
}

// escapeCypherString escapes a string for use inside a single-quoted Cypher
// literal. Backslashes are escaped first so the backslashes introduced for
// single quotes are not themselves re-escaped.
func escapeCypherString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}
