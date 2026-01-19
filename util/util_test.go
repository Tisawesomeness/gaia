package util

import (
	"testing"
)

func TestToSpacedWords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple camel case",
			input:    "someSampleText",
			expected: "Some Sample Text",
		},
		{
			name:     "Single word lowercase",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "Single word uppercase",
			input:    "WORLD",
			expected: "W O R L D",
		},
		{
			name:     "Mixed case with multiple words",
			input:    "thisIsATest",
			expected: "This Is A Test",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "String with numbers",
			input:    "test123Case",
			expected: "Test123 Case",
		},
		{
			name:     "With spaces",
			input:    "hello world",
			expected: "Hello World",
		},
		{
			name:     "With spaces uppercase",
			input:    "Hello World",
			expected: "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToCapitalizedSpacedWords(tt.input)
			if result != tt.expected {
				t.Errorf("CamelCaseToUppercase(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}
