package engine

import (
	"fmt"
	"regexp"
)

// Pattern represents a prompt pattern to match and respond to.
type Pattern struct {
	Match     string
	Respond   string
	Hidden    bool
	Responder func() string // if set, called instead of using Respond
}

type MatchResult struct {
	Response string
	Hidden   bool
}

type compiledPattern struct {
	regex     *regexp.Regexp
	response  string
	hidden    bool
	responder func() string
}

type Matcher struct {
	patterns []compiledPattern
}

func NewMatcher(patterns []Pattern) (*Matcher, error) {
	compiled := make([]compiledPattern, len(patterns))
	for i, p := range patterns {
		re, err := regexp.Compile(p.Match)
		if err != nil {
			return nil, fmt.Errorf("compiling pattern %q: %w", p.Match, err)
		}
		compiled[i] = compiledPattern{
			regex:     re,
			response:  p.Respond,
			hidden:    p.Hidden,
			responder: p.Responder,
		}
	}
	return &Matcher{patterns: compiled}, nil
}

func (m *Matcher) Check(line string) *MatchResult {
	for _, p := range m.patterns {
		if p.regex.MatchString(line) {
			resp := p.response
			if p.responder != nil {
				resp = p.responder()
			}
			return &MatchResult{
				Response: resp,
				Hidden:   p.hidden,
			}
		}
	}
	return nil
}
