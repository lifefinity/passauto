package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionBash(t *testing.T) {
	buf := new(bytes.Buffer)
	if err := rootCmd.GenBashCompletion(buf); err != nil {
		t.Fatalf("GenBashCompletion failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected bash completion output, got empty")
	}
	if !strings.Contains(buf.String(), "passauto") {
		t.Fatal("bash completion does not reference passauto")
	}
}

func TestCompletionZsh(t *testing.T) {
	buf := new(bytes.Buffer)
	if err := rootCmd.GenZshCompletion(buf); err != nil {
		t.Fatalf("GenZshCompletion failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected zsh completion output, got empty")
	}
	if !strings.Contains(buf.String(), "#compdef passauto") {
		t.Fatal("zsh completion missing compdef header")
	}
}

func TestCompletionFish(t *testing.T) {
	buf := new(bytes.Buffer)
	if err := rootCmd.GenFishCompletion(buf, true); err != nil {
		t.Fatalf("GenFishCompletion failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected fish completion output, got empty")
	}
	if !strings.Contains(buf.String(), "passauto") {
		t.Fatal("fish completion does not reference passauto")
	}
}

func TestCompletionPowershell(t *testing.T) {
	buf := new(bytes.Buffer)
	if err := rootCmd.GenPowerShellCompletionWithDesc(buf); err != nil {
		t.Fatalf("GenPowerShellCompletionWithDesc failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected powershell completion output, got empty")
	}
}

func TestCompleteProfileNames(t *testing.T) {
	// completeProfileNames returns nil gracefully when no data file exists
	result := completeProfileNames("")
	_ = result

	result = completeProfileNames("nonexistent")
	_ = result
}
