package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lifefinity/passauto/internal/crypto"
	"github.com/lifefinity/passauto/internal/data"
)

var (
	updateCommand       string
	updateMatch         []string
	updatePrompt        string
	updateTimeout       string
	updateSecret        bool
	updateCaseSensitive bool
	updateSteps         []string
	updateAfter         []string
	updateDesc          string
	updateKMSKeyID      string
	updateTOTPSecret    bool
	updateTOTPMatch     []string
)

var updateCmd = &cobra.Command{
	Use:   "update <profile>",
	Short: "Update an existing profile",
	Long: `Update fields of an existing profile. Only specified flags are changed;
unspecified fields remain unchanged.

Examples:
  # Update only the secret
  passauto update myserver --secret

  # Update the command
  passauto update myserver -c "ssh newuser@host"

  # Update match pattern and timeout
  passauto update myserver -m "password:" -t 60s

  # Update multiple fields at once
  passauto update myserver -c "ssh newuser@host" -m "password:" --secret`,
	Args: cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeProfileNames(toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().StringVarP(&updateCommand, "command", "c", "", "update the command")
	updateCmd.Flags().StringVarP(&updateDesc, "desc", "d", "", "update the description")
	updateCmd.Flags().StringArrayVarP(&updateMatch, "match", "m", nil, "update the prompt pattern (regex, can specify multiple)")
	updateCmd.Flags().StringVarP(&updatePrompt, "prompt", "p", "", "update the shell prompt pattern for post-login steps")
	updateCmd.Flags().StringVarP(&updateTimeout, "timeout", "t", "", "update the timeout (e.g. 30s, 1m)")
	updateCmd.Flags().BoolVar(&updateSecret, "secret", false, "prompt for a new secret")
	updateCmd.Flags().BoolVar(&updateCaseSensitive, "case-sensitive", false, "match pattern with exact case (default: case-insensitive)")
	updateCmd.Flags().StringArrayVar(&updateSteps, "then", nil, "update post-login commands (replaces existing)")
	updateCmd.Flags().StringArrayVar(&updateAfter, "after", nil, "update post-exit commands (replaces existing)")
	updateCmd.Flags().StringVar(&updateKMSKeyID, "kms-key-id", "", "set AWS KMS key ID for envelope encryption")
	updateCmd.Flags().BoolVar(&updateTOTPSecret, "totp-secret", false, "prompt for a new TOTP secret seed")
	updateCmd.Flags().StringArrayVar(&updateTOTPMatch, "totp-match", nil, "update TOTP prompt patterns (replaces existing)")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	profileName := args[0]

	d, err := loadData()
	if err != nil {
		return err
	}

	key, profile, lookupErr := d.LookupProfile(profileName, serviceFlag)
	if lookupErr != nil {
		return lookupErr
	}
	profileKey := key

	// Check if any flag was provided
	anyChange := cmd.Flags().Changed("command") ||
		cmd.Flags().Changed("desc") ||
		cmd.Flags().Changed("match") ||
		cmd.Flags().Changed("prompt") ||
		cmd.Flags().Changed("timeout") ||
		cmd.Flags().Changed("case-sensitive") ||
		cmd.Flags().Changed("then") ||
		cmd.Flags().Changed("after") ||
		cmd.Flags().Changed("kms-key-id") ||
		cmd.Flags().Changed("totp-match") ||
		updateSecret ||
		updateTOTPSecret

	if !anyChange {
		return fmt.Errorf("no flags specified. Use --help to see available options")
	}

	// Update command
	if cmd.Flags().Changed("command") {
		profile.Command = updateCommand
	}

	// Update description
	if cmd.Flags().Changed("desc") {
		profile.Description = updateDesc
	}

	// Update match pattern
	if cmd.Flags().Changed("match") {
		cs := updateCaseSensitive
		if !cmd.Flags().Changed("case-sensitive") && len(profile.Patterns) > 0 {
			cs = profile.Patterns[0].CaseSensitive
		}
		patterns := make([]data.Pattern, len(updateMatch))
		for i, m := range updateMatch {
			patterns[i] = data.Pattern{Match: m, Hidden: true, CaseSensitive: cs}
		}
		profile.Patterns = patterns
	} else if cmd.Flags().Changed("case-sensitive") && len(profile.Patterns) > 0 {
		for i := range profile.Patterns {
			profile.Patterns[i].CaseSensitive = updateCaseSensitive
		}
	}

	// Update prompt
	if cmd.Flags().Changed("prompt") {
		profile.Prompt = updatePrompt
	}

	// Update timeout
	if cmd.Flags().Changed("timeout") {
		timeout, parseErr := time.ParseDuration(updateTimeout)
		if parseErr != nil {
			return fmt.Errorf("invalid timeout %q: %w", updateTimeout, parseErr)
		}
		profile.Timeout = data.Duration{Duration: timeout}
	}

	// Update steps
	if cmd.Flags().Changed("then") {
		profile.Steps = updateSteps
	}

	// Update after
	if cmd.Flags().Changed("after") {
		profile.After = updateAfter
	}

	// Update KMS key ID
	if cmd.Flags().Changed("kms-key-id") {
		profile.KMSKeyID = updateKMSKeyID
	}

	// Update secret
	if updateSecret {
		kmsKey := profile.KMSKeyID
		if cmd.Flags().Changed("kms-key-id") {
			kmsKey = updateKMSKeyID
		}

		fmt.Printf("Enter new secret (will be hidden): ")
		secret, readErr := term.ReadPassword(int(os.Stdin.Fd())) // #nosec G115
		fmt.Println()
		if readErr != nil {
			return fmt.Errorf("reading secret: %w", readErr)
		}
		defer func() {
			for i := range secret {
				secret[i] = 0
			}
		}()

		if kmsKey != "" {
			sealed, encErr := crypto.KMSEncrypt(cmd.Context(), kmsKey, secret, []byte(profileKey))
			if encErr != nil {
				return fmt.Errorf("KMS encrypting secret: %w", encErr)
			}
			profile.Secret = base64.StdEncoding.EncodeToString(sealed)
		} else {
			key, keyErr := deriveKey()
			if keyErr != nil {
				return keyErr
			}
			ciphertext, encErr := crypto.Encrypt(key, secret, []byte(profileKey))
			if encErr != nil {
				return fmt.Errorf("encrypting secret: %w", encErr)
			}
			profile.Secret = base64.StdEncoding.EncodeToString(ciphertext)
		}
	}

	// Update TOTP patterns
	if cmd.Flags().Changed("totp-match") {
		// Remove existing TOTP patterns
		var nonTOTP []data.Pattern
		for _, p := range profile.Patterns {
			if !p.TOTP {
				nonTOTP = append(nonTOTP, p)
			}
		}
		// Add new TOTP patterns
		for _, tp := range updateTOTPMatch {
			nonTOTP = append(nonTOTP, data.Pattern{Match: tp, Hidden: true, CaseSensitive: updateCaseSensitive, TOTP: true})
		}
		profile.Patterns = nonTOTP
	}

	// Update TOTP secret
	if updateTOTPSecret {
		kmsKey := profile.KMSKeyID
		if cmd.Flags().Changed("kms-key-id") {
			kmsKey = updateKMSKeyID
		}

		fmt.Printf("Enter new TOTP secret seed (base32, will be hidden): ")
		totpSeed, readErr := term.ReadPassword(int(os.Stdin.Fd())) // #nosec G115
		fmt.Println()
		if readErr != nil {
			return fmt.Errorf("reading TOTP seed: %w", readErr)
		}
		defer func() {
			for i := range totpSeed {
				totpSeed[i] = 0
			}
		}()

		if kmsKey != "" {
			sealed, encErr := crypto.KMSEncrypt(cmd.Context(), kmsKey, totpSeed, []byte(profileKey))
			if encErr != nil {
				return fmt.Errorf("KMS encrypting TOTP seed: %w", encErr)
			}
			profile.TOTPSecret = base64.StdEncoding.EncodeToString(sealed)
		} else {
			key, keyErr := deriveKey()
			if keyErr != nil {
				return keyErr
			}
			ciphertext, encErr := crypto.Encrypt(key, totpSeed, []byte(profileKey))
			if encErr != nil {
				return fmt.Errorf("encrypting TOTP seed: %w", encErr)
			}
			profile.TOTPSecret = base64.StdEncoding.EncodeToString(ciphertext)
		}
	}

	d.Entries[profileKey] = profile

	if err := saveData(d); err != nil {
		return fmt.Errorf("saving data: %w", err)
	}

	fmt.Printf("Updated profile %q\n", profileName)
	return nil
}
