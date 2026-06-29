package engine

import (
	"bytes"
	"testing"
)

func TestStepper_NilWhenNoSteps(t *testing.T) {
	s := NewStepper(nil, "\\$", &bytes.Buffer{})
	if s != nil {
		t.Fatal("expected nil stepper when no steps")
	}
}

func TestStepper_NilWhenNoPrompt(t *testing.T) {
	s := NewStepper([]string{"ls"}, "", &bytes.Buffer{})
	if s != nil {
		t.Fatal("expected nil stepper when no prompt")
	}
}

func TestStepper_NilWhenInvalidRegex(t *testing.T) {
	s := NewStepper([]string{"ls"}, "[invalid", &bytes.Buffer{})
	if s != nil {
		t.Fatal("expected nil stepper when regex is invalid")
	}
}

func TestStepper_InactiveByDefault(t *testing.T) {
	var buf bytes.Buffer
	s := NewStepper([]string{"cmd1"}, "\\$\\s*$", &buf)

	// Should not respond when inactive
	s.Check("user@host:~$ ")
	if buf.Len() != 0 {
		t.Fatalf("expected no output when inactive, got %q", buf.String())
	}
}

func TestStepper_ActivateAndExecuteSteps(t *testing.T) {
	var buf bytes.Buffer
	steps := []string{"SELECT 1;", "\\q"}
	s := NewStepper(steps, "=>\\s*$", &buf)

	s.Activate()

	// First prompt match
	s.Check("mydb=> ")
	if buf.String() != "SELECT 1;\r\n" {
		t.Fatalf("expected 'SELECT 1;\\r\\n', got %q", buf.String())
	}

	buf.Reset()

	// Second prompt match
	s.Check("mydb=> ")
	if buf.String() != "\\q\r\n" {
		t.Fatalf("expected '\\q\\r\\n', got %q", buf.String())
	}

	// Done channel should be closed
	select {
	case <-s.Done():
		// OK
	default:
		t.Fatal("expected Done channel to be closed after all steps")
	}
}

func TestStepper_IgnoresNonMatchingLines(t *testing.T) {
	var buf bytes.Buffer
	s := NewStepper([]string{"cmd1"}, "\\$\\s*$", &buf)
	s.Activate()

	// These should not trigger
	s.Check("Loading...")
	s.Check("Connected to server")
	s.Check("Welcome message")

	if buf.Len() != 0 {
		t.Fatalf("expected no output for non-matching lines, got %q", buf.String())
	}

	// This should trigger
	s.Check("user@host:~$ ")
	if buf.String() != "cmd1\r\n" {
		t.Fatalf("expected 'cmd1\\r\\n', got %q", buf.String())
	}
}

func TestStepper_StopsAfterAllSteps(t *testing.T) {
	var buf bytes.Buffer
	s := NewStepper([]string{"only-one"}, "\\$", &buf)
	s.Activate()

	s.Check("$ ")
	buf.Reset()

	// Additional prompts should not produce output
	s.Check("$ ")
	if buf.Len() != 0 {
		t.Fatalf("expected no output after all steps consumed, got %q", buf.String())
	}
}

func TestStepper_NilSafety(t *testing.T) {
	// All methods should be safe to call on nil
	var s *Stepper
	s.Activate()
	s.Check("anything")

	// Done should return a closed channel
	select {
	case <-s.Done():
		// OK
	default:
		t.Fatal("nil stepper Done() should return closed channel")
	}
}
