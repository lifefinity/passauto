package engine

import (
	"testing"
)

func TestStripAnsi_NoEscapes(t *testing.T) {
	input := "Enter password:"
	result := stripAnsi(input)
	if result != input {
		t.Errorf("expected %q, got %q", input, result)
	}
}

func TestStripAnsi_ColorCodes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bold red text",
			input:    "\x1b[1;31mError:\x1b[0m something failed",
			expected: "Error: something failed",
		},
		{
			name:     "green text",
			input:    "\x1b[32mSuccess\x1b[0m",
			expected: "Success",
		},
		{
			name:     "multiple colors",
			input:    "\x1b[1m\x1b[34mPassword:\x1b[0m ",
			expected: "Password: ",
		},
		{
			name:     "cursor movement",
			input:    "\x1b[2KEnter PIN:",
			expected: "Enter PIN:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := stripAnsi(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestStripAnsi_OSCSequences(t *testing.T) {
	// OSC (Operating System Command) sequences like terminal title
	input := "\x1b]0;user@host:~\x07$ "
	expected := "$ "
	result := stripAnsi(input)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestStripAnsi_EmptyString(t *testing.T) {
	result := stripAnsi("")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestStripAnsi_OnlyEscapes(t *testing.T) {
	input := "\x1b[1m\x1b[0m"
	result := stripAnsi(input)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}
