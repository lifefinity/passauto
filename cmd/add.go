package cmd

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lifefinity/passauto/internal/crypto"
	"github.com/lifefinity/passauto/internal/data"
)

var (
	addCommand       string
	addMatch         []string
	addPrompt        string
	addTimeout       string
	addCaseSensitive bool
	addSteps         []string
	addAfter         []string
	addDesc          string
	addKMSKeyID      string
	addTOTP          bool
	addTOTPMatch     []string
)

var addCmd = &cobra.Command{
	Use:   "add <profile>",
	Short: "Store a secret and create a profile",
	Long: `Store an encrypted secret and create a named profile that auto-answers
prompts. Run with 'passauto <profile>' afterwards.

Examples:
  # SSH server
  passauto add -c "ssh deploy@prod-server" -m "password:" prod

  # PostgreSQL (with prompt pattern for post-login commands)
  passauto add -c "psql -h db.example.com -U admin mydb" -m "password" -p "=>\s*$" mydb

  # MySQL
  passauto add -c "mysql -h db.example.com -u root -p" -m "password:" mysql-prod

  # Sudo
  passauto add -c "sudo apt upgrade -y" -m "password" apt-upgrade

  # Kerberos
  passauto add -c "kinit admin@EXAMPLE.COM" -m "password for" krb

  # Interactive mode (prompts for command and pattern)
  passauto add myservice

Post-login automation (use --then/--script when running):
  passauto mydb --then "SELECT now();" --then "\q"
  passauto mydb --script queries.sql

  The -p/--prompt flag in 'add' tells passauto what the shell prompt looks
  like, so it knows when to send the next --then command.

Post-exit commands (--after):
  Run commands in a new shell after the profile process exits successfully.
  Useful for non-interactive tools like kinit, ssh one-shot commands, etc.

  # kinit completes → verify ticket
  passauto add -c "kinit user@REALM" -m "Password:" --after "klist" krb

  # SSH session ends → sync local files
  passauto add -c "ssh deploy@prod" -m "password:" --after "echo 'session ended'" prod

  # Chain multiple post-exit commands
  passauto add -c "kinit user@REALM" -m "Password:" \
    --after "klist" --after "echo 'ticket acquired'" krb

Pattern matching tips:
  Patterns are regex and case-insensitive by default. A partial match is enough.
  # "password" matches "Password for user demo1:", "Enter password:", etc.
  passauto add -c "psql -U demo1 -h localhost" -m "password" mydb

  # Use regex for more control: match any username
  passauto add -c "psql -U admin -h db" -m "Password for user .+:" mydb

  # Match multiple different prompts
  passauto add -c "ssh host" -m "password" -m "passphrase" myserver`,
	Args: cobra.ExactArgs(1),
	RunE: runAdd,
}

func init() {
	addCmd.Flags().StringVarP(&addCommand, "command", "c", "", "command to run for this profile")
	addCmd.Flags().StringVarP(&addDesc, "desc", "d", "", "short description for this profile")
	addCmd.Flags().StringArrayVarP(&addMatch, "match", "m", nil, "prompt pattern to match (regex, can specify multiple)")
	addCmd.Flags().StringVarP(&addPrompt, "prompt", "p", "", "shell prompt pattern for post-login steps (regex)")
	addCmd.Flags().StringVarP(&addTimeout, "timeout", "t", "30s", "timeout for pattern matching")
	addCmd.Flags().BoolVar(&addCaseSensitive, "case-sensitive", false, "match pattern with exact case (default: case-insensitive)")
	addCmd.Flags().StringArrayVar(&addSteps, "then", nil, "command to run after login (can specify multiple)")
	addCmd.Flags().StringArrayVar(&addAfter, "after", nil, "command to run in new shell after profile exits (can specify multiple)")
	addCmd.Flags().StringVar(&addKMSKeyID, "kms-key-id", "", "AWS KMS key ID for envelope encryption (overrides SSH key derivation)")
	addCmd.Flags().BoolVar(&addTOTP, "totp", false, "prompt for a TOTP secret seed (for 2FA auto-fill)")
	addCmd.Flags().StringArrayVar(&addTOTPMatch, "totp-match", nil, "prompt pattern that triggers TOTP code (regex, can specify multiple)")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	name := args[0]

	// If -c not provided, prompt interactively
	command := addCommand
	if command == "" {
		fmt.Printf("Command to run (e.g. \"ssh user@host\"): ")
		command = readLine()
		if command == "" {
			return fmt.Errorf("command is required")
		}
	}

	// If -m not provided, prompt interactively
	matches := addMatch
	if len(matches) == 0 {
		fmt.Printf("Prompt to match (e.g. \"password:\") [default: password:]: ")
		m := readLine()
		if m == "" {
			m = "password:"
		}
		matches = []string{m}
	}

	var key []byte
	if addKMSKeyID == "" {
		var keyErr error
		key, keyErr = deriveKey()
		if keyErr != nil {
			return keyErr
		}
	}

	fmt.Printf("Enter secret (will be hidden): ")
	secret, err := term.ReadPassword(int(os.Stdin.Fd())) // #nosec G115
	fmt.Println()
	if err != nil {
		return fmt.Errorf("reading secret: %w", err)
	}
	defer func() {
		for i := range secret {
			secret[i] = 0
		}
	}()

	var encryptedB64 string
	profileKey := data.ProfileKey(name, serviceFlag)
	if addKMSKeyID != "" {
		sealed, kmsErr := crypto.KMSEncrypt(cmd.Context(), addKMSKeyID, secret, []byte(profileKey))
		if kmsErr != nil {
			return fmt.Errorf("KMS encrypting secret: %w", kmsErr)
		}
		encryptedB64 = base64.StdEncoding.EncodeToString(sealed)
	} else {
		ciphertext, encErr := crypto.Encrypt(key, secret, []byte(profileKey))
		if encErr != nil {
			return fmt.Errorf("encrypting secret: %w", encErr)
		}
		encryptedB64 = base64.StdEncoding.EncodeToString(ciphertext)
	}

	timeout, _ := time.ParseDuration(addTimeout)
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	d, err := loadData()
	if err != nil {
		return err
	}

	patterns := make([]data.Pattern, len(matches))
	for i, m := range matches {
		patterns[i] = data.Pattern{Match: m, Hidden: true, CaseSensitive: addCaseSensitive}
	}

	// Add TOTP patterns
	for _, tp := range addTOTPMatch {
		patterns = append(patterns, data.Pattern{Match: tp, Hidden: true, CaseSensitive: addCaseSensitive, TOTP: true})
	}

	// Encrypt TOTP seed if --totp or --totp-match provided
	var totpEncB64 string
	if addTOTP || len(addTOTPMatch) > 0 {
		fmt.Printf("Enter TOTP secret seed (base32, will be hidden): ")
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

		profileKey := data.ProfileKey(name, serviceFlag)
		if addKMSKeyID != "" {
			sealed, kmsErr := crypto.KMSEncrypt(cmd.Context(), addKMSKeyID, totpSeed, []byte(profileKey))
			if kmsErr != nil {
				return fmt.Errorf("KMS encrypting TOTP seed: %w", kmsErr)
			}
			totpEncB64 = base64.StdEncoding.EncodeToString(sealed)
		} else {
			var encKey []byte
			if key != nil {
				encKey = key
			} else {
				encKey, err = deriveKey()
				if err != nil {
					return err
				}
			}
			ciphertext, encErr := crypto.Encrypt(encKey, totpSeed, []byte(profileKey))
			if encErr != nil {
				return fmt.Errorf("encrypting TOTP seed: %w", encErr)
			}
			totpEncB64 = base64.StdEncoding.EncodeToString(ciphertext)
		}

		// If --totp without --totp-match, use same match patterns as TOTP
		if addTOTP && len(addTOTPMatch) == 0 {
			for i := range patterns {
				patterns[i].TOTP = true
			}
		}
	}

	profile := data.Profile{
		Command:     command,
		Description: addDesc,
		Service:     serviceFlag,
		Patterns:    patterns,
		Secret:      encryptedB64,
		TOTPSecret:  totpEncB64,
		Prompt:      addPrompt,
		Timeout:     data.Duration{Duration: timeout},
		Steps:       addSteps,
		After:       addAfter,
		KMSKeyID:    addKMSKeyID,
	}

	if err := d.AddProfile(name, profile); err != nil {
		return err
	}

	if err := saveData(d); err != nil {
		return fmt.Errorf("saving data: %w", err)
	}

	fmt.Println()
	fmt.Printf("Done! Run with: passauto %s\n", name)
	return nil
}

func readLine() string {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimRight(scanner.Text(), "\r\n")
	}
	return ""
}
