package cmd

import (
	"strings"
	"testing"

	"github.com/lifefinity/passauto/internal/data"
	"github.com/lifefinity/passauto/internal/engine"
)

// buildEnginePatterns mirrors the logic in runProfileWithSteps for testability.
func buildEnginePatterns(patterns []data.Pattern, secret string) []engine.Pattern {
	enginePatterns := make([]engine.Pattern, len(patterns))
	for i, p := range patterns {
		match := p.Match
		if !p.CaseSensitive && !strings.HasPrefix(match, "(?i)") {
			match = "(?i)" + match
		}
		enginePatterns[i] = engine.Pattern{
			Match:   match,
			Respond: secret,
			Hidden:  p.Hidden,
		}
	}
	return enginePatterns
}

func TestBuildEnginePatterns_DefaultCaseInsensitive(t *testing.T) {
	patterns := []data.Pattern{{Match: "password:", Hidden: true}}
	result := buildEnginePatterns(patterns, "secret")

	if result[0].Match != "(?i)password:" {
		t.Errorf("expected (?i) prefix, got %q", result[0].Match)
	}
}

func TestBuildEnginePatterns_CaseSensitive(t *testing.T) {
	patterns := []data.Pattern{{Match: "Password:", Hidden: true, CaseSensitive: true}}
	result := buildEnginePatterns(patterns, "secret")

	if result[0].Match != "Password:" {
		t.Errorf("expected no prefix, got %q", result[0].Match)
	}
}

func TestBuildEnginePatterns_AlreadyHasPrefix(t *testing.T) {
	patterns := []data.Pattern{{Match: "(?i)password:", Hidden: true}}
	result := buildEnginePatterns(patterns, "secret")

	if result[0].Match != "(?i)password:" {
		t.Errorf("expected no double prefix, got %q", result[0].Match)
	}
}

func TestBuildEnginePatterns_CaseInsensitiveMatches(t *testing.T) {
	patterns := []data.Pattern{{Match: "password:", Hidden: true}}
	result := buildEnginePatterns(patterns, "secret")

	m, err := engine.NewMatcher([]engine.Pattern{result[0]})
	if err != nil {
		t.Fatal(err)
	}

	// Should match regardless of case
	for _, line := range []string{"Password:", "PASSWORD:", "password:", "PaSsWoRd:"} {
		if m.Check(line) == nil {
			t.Errorf("expected match for %q", line)
		}
	}
}

func TestBuildEnginePatterns_CaseSensitiveRejects(t *testing.T) {
	patterns := []data.Pattern{{Match: "Password:", Hidden: true, CaseSensitive: true}}
	result := buildEnginePatterns(patterns, "secret")

	m, err := engine.NewMatcher([]engine.Pattern{result[0]})
	if err != nil {
		t.Fatal(err)
	}

	if m.Check("PASSWORD:") != nil {
		t.Error("case-sensitive should not match different case")
	}
	if m.Check("Password:") == nil {
		t.Error("case-sensitive should match exact case")
	}
}
