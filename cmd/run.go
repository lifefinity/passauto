package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/lifefinity/passauto/internal/crypto"
	"github.com/lifefinity/passauto/internal/engine"
	"github.com/lifefinity/passauto/internal/totp"
)

func runProfileWithSteps(profileName string, runOpts profileRunOpts) error {
	d, err := loadData()
	if err != nil {
		return err
	}

	key, profile, err := d.LookupProfile(profileName, serviceFlag)
	if err != nil {
		return err
	}
	profileKey := key

	command := splitCommand(profile.Command)
	timeout := profile.Timeout.Duration
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Decrypt the profile's secret
	var secret string
	if profile.Secret != "" {
		ciphertext, err := base64.StdEncoding.DecodeString(profile.Secret)
		if err != nil {
			return fmt.Errorf("decoding secret: %w", err)
		}

		var plaintext []byte
		if profile.KMSKeyID != "" {
			plaintext, err = crypto.KMSDecrypt(context.Background(), ciphertext, []byte(profileKey))
		} else {
			key, keyErr := deriveKeyForProfile(profileKey)
			if keyErr != nil {
				return keyErr
			}
			plaintext, err = crypto.Decrypt(key, ciphertext, []byte(profileKey))
		}
		if err != nil {
			return fmt.Errorf("decrypting secret: %w", err)
		}
		defer func() {
			for i := range plaintext {
				plaintext[i] = 0
			}
		}()
		secret = string(plaintext)
	}

	// Decrypt TOTP secret if present
	var totpSeed string
	if profile.TOTPSecret != "" {
		ciphertext, err := base64.StdEncoding.DecodeString(profile.TOTPSecret)
		if err != nil {
			return fmt.Errorf("decoding TOTP secret: %w", err)
		}

		var plaintext []byte
		if profile.KMSKeyID != "" {
			plaintext, err = crypto.KMSDecrypt(context.Background(), ciphertext, []byte(profileKey))
		} else {
			key, keyErr := deriveKeyForProfile(profileKey)
			if keyErr != nil {
				return keyErr
			}
			plaintext, err = crypto.Decrypt(key, ciphertext, []byte(profileKey))
		}
		if err != nil {
			return fmt.Errorf("decrypting TOTP secret: %w", err)
		}
		totpSeed = string(plaintext)
		defer func() {
			for i := range plaintext {
				plaintext[i] = 0
			}
		}()
	}

	// Build engine patterns: each pattern's response is the decrypted secret
	enginePatterns := make([]engine.Pattern, len(profile.Patterns))
	for i, p := range profile.Patterns {
		match := p.Match
		if !p.CaseSensitive && !strings.HasPrefix(match, "(?i)") {
			match = "(?i)" + match
		}
		ep := engine.Pattern{
			Match:   match,
			Respond: secret,
			Hidden:  p.Hidden,
		}
		if p.TOTP {
			seed := totpSeed
			ep.Respond = ""
			ep.Responder = func() string {
				code, err := totp.Generate(seed)
				if err != nil {
					return ""
				}
				return code
			}
		}
		enginePatterns[i] = ep
	}

	// Load post-login steps: profile steps first, then runtime --then/--script
	steps, err := loadSteps(runOpts)
	if err != nil {
		return err
	}
	if len(profile.Steps) > 0 {
		steps = append(profile.Steps, steps...)
	}

	// Dry-run: print config and exit without running
	if runOpts.dryRun {
		fmt.Printf("Command: %s\n", profile.Command)
		fmt.Printf("Timeout: %s\n", timeout)
		fmt.Println("Patterns:")
		for _, p := range enginePatterns {
			fmt.Printf("  - %s\n", p.Match)
		}
		if len(steps) > 0 {
			fmt.Println("Steps:")
			for _, s := range steps {
				fmt.Printf("  - %s\n", s)
			}
		}
		return nil
	}

	// Determine prompt pattern (CLI override > profile > empty)
	prompt := runOpts.prompt
	if prompt == "" {
		prompt = profile.Prompt
	}

	exitCode, err := engine.Run(engine.Options{
		Command:  command,
		Patterns: enginePatterns,
		Timeout:  timeout,
		Steps:    steps,
		Prompt:   prompt,
		Env:      runOpts.env,
		Stdout:   quietWriter(runOpts.quiet),
	})

	if err != nil {
		return err
	}

	// Run --after commands if main process succeeded
	if exitCode == 0 {
		afterCmds := append(profile.After, runOpts.after...)
		for _, cmd := range afterCmds {
			afterCmd := exec.Command("sh", "-c", cmd) // #nosec G204 -- user-provided post-exit command is by design
			afterCmd.Stdin = os.Stdin
			afterCmd.Stdout = os.Stdout
			afterCmd.Stderr = os.Stderr
			if runErr := afterCmd.Run(); runErr != nil {
				fmt.Fprintf(os.Stderr, "after command failed: %v\n", runErr)
			}
		}
	}

	os.Exit(exitCode)
	return nil
}

func splitCommand(cmd string) []string {
	fields := []string{}
	current := ""
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		if inQuote {
			if c == quoteChar {
				inQuote = false
			} else {
				current += string(c)
			}
		} else if c == '"' || c == '\'' {
			inQuote = true
			quoteChar = c
		} else if c == ' ' {
			if current != "" {
				fields = append(fields, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		fields = append(fields, current)
	}
	return fields
}

func quietWriter(quiet bool) io.Writer {
	if quiet {
		return io.Discard
	}
	return nil // engine defaults to os.Stdout
}
