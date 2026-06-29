package engine

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func buildMockPrompt(t *testing.T) string {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	srcPath := filepath.Join(filepath.Dir(filename), "..", "testutil", "mockprompt.go")
	binPath := filepath.Join(t.TempDir(), "mockprompt")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", binPath, srcPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building mockprompt: %v\n%s", err, output)
	}
	return binPath
}

func TestEngine_MultiPrompt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ConPTY integration test requires manual verification on Windows (use 'passauto run' with a real interactive program)")
	}

	bin := buildMockPrompt(t)

	patterns := []Pattern{
		{Match: "(?i)password:", Respond: "correct-password", Hidden: true},
		{Match: "\\(yes/no\\)", Respond: "yes", Hidden: false},
	}

	var stdout bytes.Buffer

	exitCode, err := Run(Options{
		Command:  []string{bin},
		Patterns: patterns,
		Timeout:  5 * time.Second,
		Stdin:    &bytes.Buffer{},
		Stdout:   &stdout,
	})

	if err != nil {
		if strings.Contains(err.Error(), "Setctty") || strings.Contains(err.Error(), "starting PTY") {
			t.Skipf("skipping: PTY not available in this environment: %v", err)
		}
		t.Fatalf("Run failed: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nOutput: %s", exitCode, stdout.String())
	}

	output := stdout.String()
	if !bytes.Contains([]byte(output), []byte("Access granted")) {
		t.Fatalf("expected 'Access granted' in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Done!")) {
		t.Fatalf("expected 'Done!' in output, got: %s", output)
	}
}
