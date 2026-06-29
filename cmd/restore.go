package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/lifefinity/passauto/internal/crypto"
)

var restoreForce bool

var restoreCmd = &cobra.Command{
	Use:   "restore <directory>",
	Short: "Restore encryption key and data from a backup",
	Long: `Restores the encryption key and data file from a backup directory.
Will not overwrite existing data unless --force is specified.

Example:
  passauto restore /mnt/usb/passauto-backup
  passauto restore ~/Dropbox/passauto-backup --force`,
	Args: cobra.ExactArgs(1),
	RunE: runRestore,
}

func init() {
	restoreCmd.Flags().BoolVar(&restoreForce, "force", false, "overwrite existing data")
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	src := args[0]

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	passautoDir := filepath.Join(home, ".passauto")
	dataFile := filepath.Join(passautoDir, "data.json")

	// Check if already initialized
	if !restoreForce {
		if _, err := os.Stat(dataFile); err == nil {
			return fmt.Errorf("passauto already initialized. Use --force to overwrite")
		}
	}

	// Find data.json in backup
	srcData := filepath.Join(src, "data.json")
	if _, err := os.Stat(srcData); err != nil {
		return fmt.Errorf("backup data.json not found in %s", src)
	}

	// Find key file - look for known key names first
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("reading backup directory: %w", err)
	}

	knownKeys := []string{"passauto_key", "id_ed25519", "id_rsa", "id_ecdsa"}
	var keyFile string
	for _, name := range knownKeys {
		for _, e := range entries {
			if e.Name() == name {
				keyFile = name
				break
			}
		}
		if keyFile != "" {
			break
		}
	}
	// Fallback: any non-json non-pub file
	if keyFile == "" {
		for _, e := range entries {
			if !e.IsDir() && e.Name() != "data.json" && !isPublicKey(e.Name()) && filepath.Ext(e.Name()) != ".json" {
				keyFile = e.Name()
				fmt.Fprintf(os.Stderr, "Warning: no known key file found, using %q\n", keyFile)
				break
			}
		}
	}
	if keyFile == "" {
		return fmt.Errorf("no key file found in backup directory")
	}

	// Verify the key works with the data
	srcKey := filepath.Join(src, keyFile)
	if _, err := crypto.DeriveKey(srcKey, nil); err != nil {
		return fmt.Errorf("key verification failed: %w", err)
	}

	// Restore
	if err := os.MkdirAll(passautoDir, 0700); err != nil {
		return fmt.Errorf("creating passauto directory: %w", err)
	}

	destKey := filepath.Join(passautoDir, keyFile)
	if err := copyFile(srcKey, destKey); err != nil {
		return fmt.Errorf("restoring key: %w", err)
	}
	if err := copyFile(srcData, dataFile); err != nil {
		return fmt.Errorf("restoring data: %w", err)
	}

	fmt.Printf("Restored from: %s\n", src)
	fmt.Printf("Key: %s\n", destKey)
	fmt.Printf("Data: %s\n", dataFile)
	return nil
}

func isPublicKey(name string) bool {
	return len(name) > 4 && name[len(name)-4:] == ".pub"
}
