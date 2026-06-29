package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lifefinity/passauto/internal/crypto"
)

var changeKeyCmd = &cobra.Command{
	Use:   "change-key <new-key-path>",
	Short: "Switch to a new SSH key for encryption",
	Long: `Re-encrypt all secrets with a new SSH key.

This decrypts all stored secrets using the current key, then re-encrypts
them with the new key. Both keys may be passphrase-protected.

Examples:
  passauto change-key ~/.ssh/id_ed25519_new
  passauto change-key /path/to/other_key`,
	Args: cobra.ExactArgs(1),
	RunE: runChangeKey,
}

func init() {
	rootCmd.AddCommand(changeKeyCmd)
}

func runChangeKey(cmd *cobra.Command, args []string) error {
	newKeyPath := args[0]

	// Verify new key file exists
	if _, err := os.Stat(newKeyPath); err != nil {
		return fmt.Errorf("new key not found: %w", err)
	}

	d, err := loadData()
	if err != nil {
		return err
	}

	// Derive old key
	fmt.Printf("Enter passphrase for current key file (%s): ", d.KeyFile)
	oldKey, err := deriveKeyFromPath(d.KeyFile)
	if err != nil {
		return fmt.Errorf("deriving current key: %w", err)
	}
	fmt.Println("✓ Current key loaded")

	// Derive new key
	fmt.Printf("Enter passphrase for new key (%s): ", newKeyPath)
	newKey, err := deriveKeyFromPath(newKeyPath)
	if err != nil {
		return fmt.Errorf("deriving new key: %w", err)
	}
	fmt.Println("✓ New key loaded")

	// Re-encrypt all secrets
	count := 0
	for name, profile := range d.Entries {
		if profile.Secret == "" {
			continue
		}

		ciphertext, err := base64.StdEncoding.DecodeString(profile.Secret)
		if err != nil {
			return fmt.Errorf("decoding secret for %q: %w", name, err)
		}

		plaintext, err := crypto.Decrypt(oldKey, ciphertext, []byte(name))
		if err != nil {
			return fmt.Errorf("decrypting secret for %q: %w", name, err)
		}

		newCiphertext, err := crypto.Encrypt(newKey, plaintext, []byte(name))
		if err != nil {
			return fmt.Errorf("re-encrypting secret for %q: %w", name, err)
		}

		profile.Secret = base64.StdEncoding.EncodeToString(newCiphertext)
		d.Entries[name] = profile
		count++
	}

	// Update SSH key path (store as ~/... if under home)
	home, _ := os.UserHomeDir()
	absPath, _ := filepath.Abs(newKeyPath)
	if rel, err := filepath.Rel(home, absPath); err == nil && len(rel) > 0 && rel[0] != '.' {
		d.KeyFile = "~/" + rel
	} else {
		d.KeyFile = absPath
	}

	if err := saveData(d); err != nil {
		return fmt.Errorf("saving data: %w", err)
	}

	fmt.Printf("✓ Re-encrypted %d profile(s) with new key\n", count)
	fmt.Printf("  Key: %s\n", d.KeyFile)
	return nil
}

func deriveKeyFromPath(keyPath string) ([]byte, error) {
	home, _ := os.UserHomeDir()
	if len(keyPath) > 2 && keyPath[:2] == "~/" {
		keyPath = filepath.Join(home, keyPath[2:])
	}

	// Try without passphrase first
	key, err := crypto.DeriveKey(keyPath, nil)
	if err == nil {
		return key, nil
	}

	// Need passphrase
	passphrase, readErr := term.ReadPassword(int(os.Stdin.Fd())) // #nosec G115
	fmt.Println()
	if readErr != nil {
		return nil, fmt.Errorf("reading passphrase: %w", readErr)
	}

	return crypto.DeriveKey(keyPath, passphrase)
}
