package engine

import (
	"io"
	"regexp"
	"sync"
)

type Stepper struct {
	mu      sync.Mutex
	steps   []string
	current int
	prompt  *regexp.Regexp
	writer  io.Writer
	active  bool
	done    chan struct{}
}

func NewStepper(steps []string, prompt string, writer io.Writer) *Stepper {
	if len(steps) == 0 || prompt == "" {
		return nil
	}

	re, err := regexp.Compile(prompt)
	if err != nil {
		return nil
	}

	return &Stepper{
		steps:  steps,
		prompt: re,
		writer: writer,
		done:   make(chan struct{}),
	}
}

func (s *Stepper) Activate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.active = true
	s.mu.Unlock()
}

func (s *Stepper) Check(line string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}

	if s.current >= len(s.steps) {
		return
	}

	if s.prompt.MatchString(line) {
		cmd := s.steps[s.current]
		_, _ = s.writer.Write([]byte(cmd + "\r\n"))
		s.current++
		if s.current >= len(s.steps) {
			close(s.done)
		}
	}
}

func (s *Stepper) Done() <-chan struct{} {
	if s == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return s.done
}
