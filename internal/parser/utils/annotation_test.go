package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractAnnotation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		expected []Annotation
	}{
		{
			name:     "no input",
			input:    nil,
			expected: []Annotation{},
		},
		{
			name:     "empty string",
			input:    []string{""},
			expected: []Annotation{},
		},
		{
			name:     "blank lines only",
			input:    []string{"\n\n   \n\t\n"},
			expected: []Annotation{},
		},
		{
			name:     "non annotation lines",
			input:    []string{"this is a comment\n// another comment"},
			expected: []Annotation{},
		},
		{
			name:     "annotation without parens",
			input:    []string{"@SnAPI.deprecated"},
			expected: []Annotation{{Name: "deprecated"}},
		},
		{
			name:     "annotation with empty parens",
			input:    []string{"@SnAPI.get()"},
			expected: []Annotation{{Name: "get"}},
		},
		{
			name:     "annotation with single quoted arg",
			input:    []string{`@SnAPI.get("/users")`},
			expected: []Annotation{{Name: "get", Args: []string{"/users"}}},
		},
		{
			name:     "annotation with multiple args",
			input:    []string{`@SnAPI.status(200, "OK")`},
			expected: []Annotation{{Name: "status", Args: []string{"200", "OK"}}},
		},
		{
			name:     "annotation with dotted identifier arg",
			input:    []string{`@SnAPI.request(model.User)`},
			expected: []Annotation{{Name: "request", Args: []string{"model.User"}}},
		},
		{
			name:     "case insensitive name",
			input:    []string{"@snapi.GET()"},
			expected: []Annotation{{Name: "GET"}},
		},
		{
			name:  "leading whitespace and indentation",
			input: []string{"   \t@SnAPI.tags(\"a\", \"b\")"},
			expected: []Annotation{
				{Name: "tags", Args: []string{"a", "b"}},
			},
		},
		{
			name: "multiple annotations in one block",
			input: []string{
				"@SnAPI.get(\"/x\")\n@SnAPI.deprecated\n@SnAPI.tags(\"t1\")",
			},
			expected: []Annotation{
				{Name: "get", Args: []string{"/x"}},
				{Name: "deprecated"},
				{Name: "tags", Args: []string{"t1"}},
			},
		},
		{
			name: "multiple blocks",
			input: []string{
				"@SnAPI.get(\"/x\")",
				"@SnAPI.post(\"/y\")",
			},
			expected: []Annotation{
				{Name: "get", Args: []string{"/x"}},
				{Name: "post", Args: []string{"/y"}},
			},
		},
		{
			name:     "trailing content invalidates",
			input:    []string{`@SnAPI.get("/x") extra`},
			expected: []Annotation{},
		},
		{
			name:     "unterminated parens",
			input:    []string{`@SnAPI.get("/x"`},
			expected: []Annotation{},
		},
		{
			name:     "ignores empty arg entries",
			input:    []string{`@SnAPI.tags("a", , "b")`},
			expected: []Annotation{{Name: "tags", Args: []string{"a", "b"}}},
		},
		{
			name:     "name with digits not matched",
			input:    []string{"@SnAPI.get1()"},
			expected: []Annotation{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractAnnotation(tc.input...)
			assert.Equal(t, tc.expected, got)
		})
	}
}
