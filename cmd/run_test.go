package cmd

import (
	"testing"
)

func TestSplitCommand_Simple(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"ssh user@host", []string{"ssh", "user@host"}},
		{"echo hello world", []string{"echo", "hello", "world"}},
		{"ls", []string{"ls"}},
		{"", nil},
	}

	for _, tc := range tests {
		result := splitCommand(tc.input)
		if len(result) != len(tc.expected) {
			t.Errorf("splitCommand(%q): got %v, want %v", tc.input, result, tc.expected)
			continue
		}
		for i := range result {
			if result[i] != tc.expected[i] {
				t.Errorf("splitCommand(%q)[%d]: got %q, want %q", tc.input, i, result[i], tc.expected[i])
			}
		}
	}
}

func TestSplitCommand_Quotes(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{`ssh -o "StrictHostKeyChecking no" user@host`, []string{"ssh", "-o", "StrictHostKeyChecking no", "user@host"}},
		{`echo 'hello world'`, []string{"echo", "hello world"}},
		{`cmd "arg with spaces" plain`, []string{"cmd", "arg with spaces", "plain"}},
		{`cmd 'single' "double"`, []string{"cmd", "single", "double"}},
	}

	for _, tc := range tests {
		result := splitCommand(tc.input)
		if len(result) != len(tc.expected) {
			t.Errorf("splitCommand(%q): got %v, want %v", tc.input, result, tc.expected)
			continue
		}
		for i := range result {
			if result[i] != tc.expected[i] {
				t.Errorf("splitCommand(%q)[%d]: got %q, want %q", tc.input, i, result[i], tc.expected[i])
			}
		}
	}
}

func TestSplitCommand_EdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		// Multiple spaces
		{"cmd   arg1   arg2", []string{"cmd", "arg1", "arg2"}},
		// Leading/trailing spaces
		{"  cmd arg  ", []string{"cmd", "arg"}},
		// Adjacent quotes
		{`cmd "a""b"`, []string{"cmd", "ab"}},
	}

	for _, tc := range tests {
		result := splitCommand(tc.input)
		if len(result) != len(tc.expected) {
			t.Errorf("splitCommand(%q): got %v, want %v", tc.input, result, tc.expected)
			continue
		}
		for i := range result {
			if result[i] != tc.expected[i] {
				t.Errorf("splitCommand(%q)[%d]: got %q, want %q", tc.input, i, result[i], tc.expected[i])
			}
		}
	}
}
