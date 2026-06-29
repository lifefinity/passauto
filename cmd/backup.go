package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup <directory>",
	Short: "Backup encryption key and data to a directory",
	Long: `Copies the encryption key and data file to the specified directory.
Store the backup securely — it contains your encrypted secrets and the key to decrypt them.

Example:
  passauto backup /mnt/usb/passauto-backup
  passauto backup ~/Dropbox/passauto-backup`,
	Args: cobra.ExactArgs(1),
	RunE: runBackup,
}

func init() {
	rootCmd.AddCommand(backupCmd)
}

func runBackup(cmd *cobra.Command, args []string) error {
	dest := args[0]

	d, err := loadData()
	if err != nil {
		return err
	}

	home, _ := os.UserHomeDir()
	passautoDir := filepath.Join(home, ".passauto")
	dataFile := filepath.Join(passautoDir, "data.json")

	// Resolve key path
	keyPath := d.KeyFile
	if len(keyPath) > 2 && keyPath[:2] == "~/" {
		keyPath = filepath.Join(home, keyPath[2:])
	}

	if err := os.MkdirAll(dest, 0700); err != nil {
		return fmt.Errorf("creating backup directory: %w", err)
	}

	// Copy data file
	if err := copyFile(dataFile, filepath.Join(dest, "data.json")); err != nil {
		return fmt.Errorf("backing up data: %w", err)
	}

	// Copy key file
	keyDest := filepath.Join(dest, filepath.Base(keyPath))
	if err := copyFile(keyPath, keyDest); err != nil {
		return fmt.Errorf("backing up key: %w", err)
	}

	fmt.Printf("Backed up to: %s\n", dest)
	fmt.Println("Files: data.json, " + filepath.Base(keyPath))
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) // #nosec G304 -- user-specified paths
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600) // #nosec G304 -- path is user-provided backup destination
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	return err
}
