package graph

import (
	"testing"
)

func TestCypherValue(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"plain string", "Alice", "'Alice'"},
		{"single quote", "O'Brien", `'O\'Brien'`},
		{"backslash", `a\b`, `'a\\b'`},
		{"backslash before quote", `a\'b`, `'a\\\'b'`},
		{"int", 42, "42"},
		{"bool", true, "true"},
	}
	for _, tt := range tests {
		if got := cypherValue(tt.in); got != tt.want {
			t.Errorf("%s: cypherValue(%#v) = %q, want %q", tt.name, tt.in, got, tt.want)
		}
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
