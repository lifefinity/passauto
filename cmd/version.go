package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// These variables are set at build time via -ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("passauto %s\n", Version)
		fmt.Printf("  commit:  %s\n", Commit)
		fmt.Printf("  built:   %s\n", BuildDate)
		fmt.Printf("  go:      %s\n", runtime.Version())
		fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.SetVersionTemplate(fmt.Sprintf(
		"passauto %s\n  commit:  %s\n  built:   %s\n  go:      %s\n  os/arch: %s/%s\n",
		Version, Commit, BuildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH,
	))
}
