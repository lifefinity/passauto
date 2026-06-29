package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/lifefinity/passauto/internal/data"
	"github.com/spf13/cobra"
)

var importForce bool

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import profiles from a JSON file (merges with existing)",
	Args:  cobra.ExactArgs(1),
	RunE:  runImport,
}

func init() {
	importCmd.Flags().BoolVar(&importForce, "force", false, "Overwrite existing profiles on conflict")
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	raw, err := os.ReadFile(args[0]) // #nosec G304 -- user-provided import file
	if err != nil {
		return fmt.Errorf("reading import file: %w", err)
	}

	var incoming map[string]data.Profile
	if err := json.Unmarshal(raw, &incoming); err != nil {
		return fmt.Errorf("parsing import file: %w", err)
	}

	d, err := loadData()
	if err != nil {
		return err
	}

	var added, skipped int
	for name, p := range incoming {
		if _, exists := d.Entries[name]; exists && !importForce {
			skipped++
			continue
		}
		d.Entries[name] = p
		added++
	}

	if err := saveData(d); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Imported %d profiles (%d skipped)\n", added, skipped)
	return nil
}
