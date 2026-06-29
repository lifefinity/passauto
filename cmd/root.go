package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "passauto",
	Short: "Auto-fill passwords and prompts for CLI tools",
	Long: `Store secrets encrypted, auto-type them when commands ask for passwords or codes.

Examples:
  1. Auto-fill SSH password
     1) passauto add -c "ssh user@host" -m "password:" myserver
     2) passauto myserver

  2. Run SQL after database login
     1) passauto add -c "psql -h db -U admin mydb" -m "password" mydb
     2) passauto mydb --then "SELECT now();" --then "\q"

  3. Password + TOTP two-factor auth
     1) passauto add -c "ssh admin@secure" -m "password:" --totp-match "code:" prod
     2) passauto prod

  4. Multiple services on the same server
     1) passauto add -c "ssh admin@prod" -m "password:" -s ssh prod
     2) passauto add -c "psql -h prod -U app" -m "password" -s pg prod
     3) passauto prod -s ssh
     4) passauto prod -s pg`,
	Version: Version,
	Args:    cobra.ArbitraryArgs,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeProfileNames(toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		profileName, runOpts := parseProfileArgs(args)
		return runProfileWithSteps(profileName, runOpts)
	},
}

var serviceFlag string

func Execute() {
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "bypass keychain cache for key derivation")
	rootCmd.PersistentFlags().StringVarP(&serviceFlag, "service", "s", "", "service name for profile disambiguation")
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type profileRunOpts struct {
	then       []string
	scriptFile string
	prompt     string
	quiet      bool
	dryRun     bool
	env        []string
	after      []string
}

func parseProfileArgs(args []string) (string, profileRunOpts) {
	profileName := args[0]
	var opts profileRunOpts

	i := 1
	for i < len(args) {
		switch args[i] {
		case "--then":
			if i+1 < len(args) {
				opts.then = append(opts.then, args[i+1])
				i += 2
			} else {
				i++
			}
		case "--after":
			if i+1 < len(args) {
				opts.after = append(opts.after, args[i+1])
				i += 2
			} else {
				i++
			}
		case "--script":
			if i+1 < len(args) {
				opts.scriptFile = args[i+1]
				i += 2
			} else {
				i++
			}
		case "--prompt":
			if i+1 < len(args) {
				opts.prompt = args[i+1]
				i += 2
			} else {
				i++
			}
		case "--env", "-e":
			if i+1 < len(args) {
				opts.env = append(opts.env, args[i+1])
				i += 2
			} else {
				i++
			}
		case "--quiet", "-q":
			opts.quiet = true
			i++
		case "--dry-run":
			opts.dryRun = true
			i++
		default:
			i++
		}
	}

	return profileName, opts
}

func loadSteps(opts profileRunOpts) ([]string, error) {
	var steps []string

	steps = append(steps, opts.then...)

	if opts.scriptFile != "" {
		lines, err := readScriptFile(opts.scriptFile)
		if err != nil {
			return nil, err
		}
		steps = append(steps, lines...)
	}

	return steps, nil
}

func readScriptFile(path string) ([]string, error) {
	f, err := os.Open(path) // #nosec G304 -- path is user-provided script file
	if err != nil {
		return nil, fmt.Errorf("opening script file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading script file: %w", err)
	}
	return lines, nil
}

func completeProfileNames(prefix string) []string {
	d, err := loadData()
	if err != nil {
		return nil
	}
	names := d.ListProfiles()
	if prefix == "" {
		return names
	}
	var filtered []string
	for _, name := range names {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			filtered = append(filtered, name)
		}
	}
	return filtered
}
