package engine

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"time"
)

type Options struct {
	Command  []string
	Patterns []Pattern
	Timeout  time.Duration
	Steps    []string
	Prompt   string
	Env      []string
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
}

func Run(opts Options) (int, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	matcher, err := NewMatcher(opts.Patterns)
	if err != nil {
		return 1, fmt.Errorf("creating matcher: %w", err)
	}

	return runWithPTY(opts, matcher)
}

func stripAnsi(s string) string {
	return ansiStripRe.ReplaceAllString(s, "")
}

var ansiStripRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b[^[\]].?`)
