package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lifefinity/passauto/internal/crypto"
	"github.com/lifefinity/passauto/internal/data"
)

var (
	initNoPassphrase bool
	initKey          string
	initKeyCommand   string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize passauto (first-time setup)",
	Long: `Initialize passauto with an encryption key.

Key selection (in order of priority):
  1. --key <path>     Use a specific SSH private key
  2. ~/.ssh/          Auto-detect: id_ed25519 > id_rsa > id_ecdsa
  3. (none found)     Generate a dedicated key at ~/.passauto/passauto_key

If the selected key is passphrase-protected, you will be prompted once
to verify access. Subsequent 'passauto' runs will also prompt if needed.

Examples:
  passauto init                          # Auto-detect or generate
  passauto init --key ~/.ssh/id_rsa      # Use a specific key
  passauto init --no-passphrase          # Generate key without passphrase`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initNoPassphrase, "no-passphrase", false, "skip passphrase protection for generated key")
	initCmd.Flags().StringVar(&initKey, "key", "", "path to an existing SSH private key to use")
	initCmd.Flags().StringVar(&initKeyCommand, "key-command", "", "shell command that outputs key material (e.g., vault/KMS)")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	passautoDir := filepath.Join(home, ".passauto")
	cfgPath := filepath.Join(passautoDir, "config.json")

	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Println("passauto is already initialized. Config at:", cfgPath)
		return nil
	}
	// Also check legacy
	legacyPath := filepath.Join(passautoDir, "data.json")
	if _, err := os.Stat(legacyPath); err == nil {
		fmt.Println("passauto is already initialized (legacy format). Data at:", legacyPath)
		return nil
	}

	var keyFile string

	if initKeyCommand != "" {
		// Verify the command works
		fmt.Printf("Verifying key command: %s\n", initKeyCommand)
		if _, err := deriveKeyFromCommand(initKeyCommand); err != nil {
			return fmt.Errorf("key-command verification failed: %w", err)
		}

		d := &data.Data{
			Config:   data.Config{KeyCommand: initKeyCommand},
			Profiles: data.Profiles{Entries: make(map[string]data.Profile)},
		}
		if err := saveData(d); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Println("Initialized with key-command. Config at:", cfgPath)
		fmt.Println("Next: run 'passauto add <name>' to store a secret.")
		return nil
	}

	if initKey != "" {
		// User specified a key path
		keyFile = initKey
		fmt.Printf("Using specified key: %s\n", keyFile)
	} else {
		keyFile = findKeyFile(home)
	}

	if keyFile == "" {
		// No SSH key found — generate a dedicated passauto key
		keyFile = filepath.Join(passautoDir, "passauto_key")

		var passphrase []byte
		if !initNoPassphrase {
			passphrase, err = promptNewPassphrase()
			if err != nil {
				return err
			}
		}

		fmt.Println("Generating passauto encryption key...")
		if err := crypto.GenerateKey(keyFile, passphrase); err != nil {
			return fmt.Errorf("generating key: %w", err)
		}
		fmt.Printf("Created: %s\n", keyFile)
		if len(passphrase) == 0 {
			fmt.Println("⚠️  Key is unprotected (no passphrase). OK for full-disk encrypted machines.")
		}
	} else {
		fmt.Printf("Using key file: %s\n", keyFile)
	}

	// Verify we can read the key (may need passphrase)
	_, err = crypto.DeriveKey(keyFile, nil)
	if err != nil {
		fmt.Print("Enter SSH key passphrase: ")
		passphrase, readErr := term.ReadPassword(int(os.Stdin.Fd())) // #nosec G115
		fmt.Println()
		if readErr != nil {
			return fmt.Errorf("reading passphrase: %w", readErr)
		}

		_, err = crypto.DeriveKey(keyFile, passphrase)
		if err != nil {
			return fmt.Errorf("cannot derive key from SSH key: %w", err)
		}
	}

	d := &data.Data{
		Config:   data.Config{KeyFile: keyFile},
		Profiles: data.Profiles{Entries: make(map[string]data.Profile)},
	}

	if err := saveData(d); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println("Initialized. Config at:", cfgPath)
	fmt.Println("Next: run 'passauto add <name>' to store a secret.")
	return nil
}

func promptNewPassphrase() ([]byte, error) {
	fd := int(os.Stdin.Fd()) // #nosec G115
	if !term.IsTerminal(fd) {
		// Non-interactive (e.g. auto-init from loadData) — skip passphrase
		return nil, nil
	}

	fmt.Print("Set a passphrase to protect your encryption key (empty = no passphrase): ")
	p1, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return nil, fmt.Errorf("reading passphrase: %w", err)
	}

	if len(p1) == 0 {
		return nil, nil
	}

	fmt.Print("Confirm passphrase: ")
	p2, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return nil, fmt.Errorf("reading passphrase confirmation: %w", err)
	}

	if !bytes.Equal(p1, p2) {
		return nil, fmt.Errorf("passphrases do not match")
	}

	return p1, nil
}

func findKeyFile(home string) string {
	sshDir := filepath.Join(home, ".ssh")
	candidates := []string{"id_ed25519", "id_rsa", "id_ecdsa"}

	for _, name := range candidates {
		path := filepath.Join(sshDir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
