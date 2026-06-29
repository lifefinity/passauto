package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <profile>",
	Short: "Delete a profile and its secret",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeProfileNames(toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	d, err := loadData()
	if err != nil {
		return err
	}

	if err := d.RemoveProfile(name, serviceFlag); err != nil {
		return err
	}

	if err := saveData(d); err != nil {
		return fmt.Errorf("saving data: %w", err)
	}

	fmt.Printf("Removed %q\n", name)
	return nil
}
