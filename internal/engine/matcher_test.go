package engine

import (
	"testing"
)

func TestMatcher_SimpleMatch(t *testing.T) {
	patterns := []Pattern{
		{Match: "(?i)password:", Respond: "secret123", Hidden: true},
	}

	m, err := NewMatcher(patterns)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}

	resp := m.Check("Enter password:")
	if resp == nil {
		t.Fatal("expected a match")
	}
	if resp.Response != "secret123" {
		t.Fatalf("expected 'secret123', got %q", resp.Response)
	}
	if !resp.Hidden {
		t.Fatal("expected hidden=true")
	}
}

func TestMatcher_NoMatch(t *testing.T) {
	patterns := []Pattern{
		{Match: "(?i)password:", Respond: "secret123", Hidden: true},
	}

	m, _ := NewMatcher(patterns)

	resp := m.Check("Loading configuration...")
	if resp != nil {
		t.Fatal("expected no match")
	}
}

func TestMatcher_FirstMatchWins(t *testing.T) {
	patterns := []Pattern{
		{Match: "password", Respond: "first", Hidden: true},
		{Match: "pass", Respond: "second", Hidden: false},
	}

	m, _ := NewMatcher(patterns)

	resp := m.Check("enter password now")
	if resp == nil {
		t.Fatal("expected a match")
	}
	if resp.Response != "first" {
		t.Fatalf("expected first match to win, got %q", resp.Response)
	}
}

func TestMatcher_InvalidRegex(t *testing.T) {
	patterns := []Pattern{
		{Match: "[broken", Respond: "x", Hidden: false},
	}

	_, err := NewMatcher(patterns)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestMatcher_Responder(t *testing.T) {
	callCount := 0
	patterns := []Pattern{
		{Match: "(?i)verification code:", Hidden: true, Responder: func() string {
			callCount++
			return "123456"
		}},
	}

	m, err := NewMatcher(patterns)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}

	resp := m.Check("Verification code:")
	if resp == nil {
		t.Fatal("expected a match")
	}
	if resp.Response != "123456" {
		t.Fatalf("expected '123456', got %q", resp.Response)
	}
	if callCount != 1 {
		t.Fatalf("expected Responder called once, got %d", callCount)
	}
}

func TestMatcher_ResponderOverridesRespond(t *testing.T) {
	patterns := []Pattern{
		{Match: "code:", Respond: "static", Responder: func() string {
			return "dynamic"
		}},
	}

	m, _ := NewMatcher(patterns)
	resp := m.Check("Enter code:")
	if resp.Response != "dynamic" {
		t.Fatalf("Responder should override Respond, got %q", resp.Response)
	}
}
