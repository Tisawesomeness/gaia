package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCapitalizeFirstLetter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Single word lowercase",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "Single word uppercase",
			input:    "WORLD",
			expected: "WORLD",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CapitalizeFirstLetter(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToCapitalizedSpacedWords(t *testing.T) {
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
		{
			name:     "With underscore",
			input:    "pre_release",
			expected: "Pre Release",
		},
		{
			name:     "With dash",
			input:    "pre-release",
			expected: "Pre Release",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToCapitalizedSpacedWords(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTrimTrailingZeroes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Zero",
			input:    "0",
			expected: "0",
		},
		{
			name:     "Trailing zeroes",
			input:    "0.400",
			expected: "0.4",
		},
		{
			name:     "No trailing zeroes",
			input:    "0.401",
			expected: "0.401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimTrailingZeroes(tt.input)
			assert.Equal(t, tt.expected, result)
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
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoerce(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		min      int
		max      int
		orElse   int
		expected int
	}{
		{
			name:     "Value within range",
			value:    50,
			min:      0,
			max:      100,
			orElse:   -1,
			expected: 50,
		},
		{
			name:     "Value below minimum",
			value:    49,
			min:      50,
			max:      100,
			orElse:   0,
			expected: 0,
		},
		{
			name:     "Value above maximum",
			value:    101,
			min:      0,
			max:      100,
			orElse:   -999,
			expected: -999,
		},
		{
			name:     "Value below min with different orElse",
			value:    5,
			min:      10,
			max:      20,
			orElse:   0,
			expected: 0,
		},
		{
			name:     "Empty range (min > max)",
			value:    5,
			min:      20,
			max:      10,
			orElse:   -1,
			expected: -1,
		},
		{
			name:     "Single value range",
			value:    42,
			min:      42,
			max:      42,
			orElse:   -999,
			expected: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Coerce(tt.value, tt.min, tt.max, tt.orElse)
			assert.Equal(t, tt.expected, result)
		})
	}
}
