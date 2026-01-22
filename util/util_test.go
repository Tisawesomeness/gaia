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

func TestWrapDegrees(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{
			name:     "Positive angle within range",
			input:    30,
			expected: 30,
		},
		{
			name:     "Positive angle exceeding 360",
			input:    390,
			expected: 30,
		},
		{
			name:     "Negative angle",
			input:    -30,
			expected: 330,
		},
		{
			name:     "Zero",
			input:    0,
			expected: 0,
		},
		{
			name:     "Multiple of 360",
			input:    720,
			expected: 0,
		},
		{
			name:     "Negative multiple of 360",
			input:    -720,
			expected: 0,
		},
		{
			name:     "Multiple of 360 (plus one)",
			input:    721,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapDegrees(tt.input)
			if result != tt.expected {
				t.Errorf("WrapDegrees(%d) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}
