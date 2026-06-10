package hytale

import (
	"testing"
)

func TestValidateAndFormatUUID(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedUUID  string
		expectedValid bool
	}{
		{
			name:          "Valid UUID with dashes",
			input:         "12345678-1234-1234-1234-123456789abc",
			expectedUUID:  "12345678-1234-1234-1234-123456789abc",
			expectedValid: true,
		},
		{
			name:          "Valid UUID without dashes",
			input:         "12345678123412341234123456789abc",
			expectedUUID:  "12345678-1234-1234-1234-123456789abc",
			expectedValid: true,
		},
		{
			name:          "Valid UUID mixed case",
			input:         "AbCdEf12-1234-1234-1234-123456789abc",
			expectedUUID:  "AbCdEf12-1234-1234-1234-123456789abc",
			expectedValid: true,
		},
		{
			name:          "Invalid: Too short",
			input:         "12345",
			expectedUUID:  "",
			expectedValid: false,
		},
		{
			name:          "Invalid: Too long",
			input:         "123456789012345678901234567890123",
			expectedUUID:  "",
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid, valid := ValidateUUID(tt.input)
			if valid != tt.expectedValid {
				t.Errorf("validateAndFormatUUID(%q) valid = %v, want %v", tt.input, valid, tt.expectedValid)
				return
			}
			if uuid != tt.expectedUUID {
				t.Errorf("validateAndFormatUUID(%q) = %q, want %q", tt.input, uuid, tt.expectedUUID)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedValid bool
	}{
		{
			name:          "Valid 3 chars",
			input:         "abc",
			expectedValid: true,
		},
		{
			name:          "Valid 16 chars",
			input:         "abcdefghijklmn_0",
			expectedValid: true,
		},
		{
			name:          "Valid with numbers",
			input:         "User123_45",
			expectedValid: true,
		},
		{
			name:          "Too short 2 chars",
			input:         "ab",
			expectedValid: false,
		},
		{
			name:          "Too long 17 chars",
			input:         "abcdefghijklmnopqr",
			expectedValid: false,
		},
		{
			name:          "Contains space",
			input:         "user name",
			expectedValid: false,
		},
		{
			name:          "Contains special char",
			input:         "user@name",
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := ValidateUsername(tt.input)
			if valid != tt.expectedValid {
				t.Errorf("validateUsername(%q) = %v, want %v", tt.input, valid, tt.expectedValid)
			}
		})
	}
}
