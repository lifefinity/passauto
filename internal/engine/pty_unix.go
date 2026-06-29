//go:build !windows

package engine

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func runWithPTY(opts Options, matcher *Matcher) (int, error) {
	cmd := exec.Command(opts.Command[0], opts.Command[1:]...) // #nosec G204 -- user-provided command is by design
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return 1, fmt.Errorf("starting PTY: %w", err)
	}
	defer func() { _ = ptmx.Close() }()

	// Handle terminal resize
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	go func() {
		for range sigCh {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	sigCh <- syscall.SIGWINCH // Initial resize

	// Set stdin to raw mode if it's a terminal
	if fd := int(os.Stdin.Fd()); term.IsTerminal(fd) { // #nosec G115 -- safe on all supported platforms
		oldState, err := term.MakeRaw(fd)
		if err == nil {
			defer func() { _ = term.Restore(fd, oldState) }()
		}
	}

	// Create stepper for post-login step execution
	var ptmxMu sync.Mutex
	stepper := NewStepper(opts.Steps, opts.Prompt, &syncWriter{w: ptmx, mu: &ptmxMu})

	var wg sync.WaitGroup

	// Forward user input to PTY (mutex-protected writes)
	// Not in WaitGroup: stdin.Read blocks until user types; we don't wait for it on exit.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := opts.Stdin.Read(buf)
			if n > 0 {
				ptmxMu.Lock()
				_, _ = ptmx.Write(buf[:n])
				ptmxMu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()

	// Read PTY output, match patterns, forward to stdout
	wg.Add(1)
	go func() {
		defer wg.Done()

		type readResult struct {
			data []byte
			err  error
		}
		ch := make(chan readResult, 1)

		// Continuous reader
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := ptmx.Read(buf)
				if n > 0 {
					tmp := make([]byte, n)
					copy(tmp, buf[:n])
					ch <- readResult{data: tmp}
				}
				if err != nil {
					ch <- readResult{err: err}
					return
				}
			}
		}()

		lineBuf := ""
		timer := time.NewTimer(100 * time.Millisecond)
		timer.Stop()

		for {
			select {
			case res := <-ch:
				if len(res.data) > 0 {
					_, _ = opts.Stdout.Write(res.data)
					lineBuf += string(res.data)

					for {
						idx := findNewline(lineBuf)
						if idx < 0 {
							timer.Reset(100 * time.Millisecond)
							break
						}
						line := lineBuf[:idx+1]
						lineBuf = lineBuf[idx+1:]
						processLineUnix(matcher, stepper, line, ptmx, &ptmxMu)
					}
				}
				if res.err != nil {
					if lineBuf != "" {
						processLineUnix(matcher, stepper, lineBuf, ptmx, &ptmxMu)
					}
					return
				}
			case <-timer.C:
				if lineBuf != "" {
					processLineUnix(matcher, stepper, lineBuf, ptmx, &ptmxMu)
					lineBuf = ""
				}
			}
		}
	}()

	err = cmd.Wait()
	signal.Stop(sigCh)
	close(sigCh)
	_ = ptmx.Close() // unblock reader goroutine
	wg.Wait()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return 1, fmt.Errorf("waiting for command: %w", err)
		}
	}

	return exitCode, nil
}

func processLineUnix(matcher *Matcher, stepper *Stepper, line string, ptmx *os.File, mu *sync.Mutex) {
	clean := stripAnsi(line)
	result := matcher.Check(clean)
	if result != nil {
		mu.Lock()
		_, _ = ptmx.Write([]byte(result.Response + "\n"))
		mu.Unlock()
		stepper.Activate()
		return
	}
	stepper.Check(clean)
}

func findNewline(s string) int {
	for i, c := range s {
		if c == '\n' || c == '\r' {
			return i
		}
	}
	return -1
}

type syncWriter struct {
	w  io.Writer
	mu *sync.Mutex
}

func (sw *syncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(p)
}
