package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/lifefinity/passauto/internal/data"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export <file>",
	Short: "Export profiles to a JSON file (secrets excluded)",
	Args:  cobra.ExactArgs(1),
	RunE:  runExport,
}

func init() {
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	d, err := loadData()
	if err != nil {
		return err
	}

	// Strip secrets before export
	clean := make(map[string]data.Profile, len(d.Entries))
	for name, p := range d.Entries {
		p.Secret = ""
		clean[name] = p
	}

	raw, err := json.MarshalIndent(clean, "", "  ") // #nosec G117 -- Secret field is intentionally empty in export
	if err != nil {
		return fmt.Errorf("marshaling profiles: %w", err)
	}

	if err := os.WriteFile(args[0], raw, 0600); err != nil {
		return fmt.Errorf("writing export file: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Exported %d profiles to %s\n", len(clean), args[0])
	return nil
}
