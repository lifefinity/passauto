package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lifefinity/passauto/internal/data"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured profiles",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	d, err := loadData()
	if err != nil {
		return err
	}

	keys := d.ListProfiles()
	if len(keys) == 0 {
		fmt.Println("No profiles configured.")
		fmt.Println()
		printExamples()
		return nil
	}

	// Calculate column widths
	maxName, maxSvc, maxCmd := len("NAME"), len("SERVICE"), len("COMMAND")
	for _, key := range keys {
		name, svc := data.ParseProfileKey(key)
		profile := d.Entries[key]
		if len(name) > maxName {
			maxName = len(name)
		}
		if len(svc) > maxSvc {
			maxSvc = len(svc)
		}
		if len(profile.Command) > maxCmd {
			maxCmd = len(profile.Command)
		}
	}

	// Only show SERVICE column if any profile has a service
	hasService := false
	for _, key := range keys {
		if _, svc := data.ParseProfileKey(key); svc != "" {
			hasService = true
			break
		}
	}

	if hasService {
		header := fmt.Sprintf("  %-*s  %-*s  %-*s  %s", maxName, "NAME", maxSvc, "SERVICE", maxCmd, "COMMAND", "DESCRIPTION")
		fmt.Println(header)
		fmt.Println("  " + strings.Repeat("-", len(header)-2))
		for _, key := range keys {
			name, svc := data.ParseProfileKey(key)
			profile := d.Entries[key]
			desc := profileDesc(profile)
			fmt.Printf("  %-*s  %-*s  %-*s  %s\n", maxName, name, maxSvc, svc, maxCmd, profile.Command, desc)
		}
	} else {
		header := fmt.Sprintf("  %-*s  %-*s  %s", maxName, "NAME", maxCmd, "COMMAND", "DESCRIPTION")
		fmt.Println(header)
		fmt.Println("  " + strings.Repeat("-", len(header)-2))
		for _, key := range keys {
			profile := d.Entries[key]
			desc := profileDesc(profile)
			fmt.Printf("  %-*s  %-*s  %s\n", maxName, key, maxCmd, profile.Command, desc)
		}
	}
	fmt.Println()
	fmt.Println("Run: passauto <name>")
	fmt.Println()

	return nil
}

func profileDesc(profile data.Profile) string {
	if profile.Description != "" {
		return profile.Description
	}
	prompts := []string{}
	for _, p := range profile.Patterns {
		prompts = append(prompts, friendlyPattern(p.Match))
	}
	return strings.Join(prompts, ", ")
}

func friendlyPattern(match string) string {
	// Strip common regex prefixes to show a readable prompt description
	s := match
	if len(s) > 4 && s[:4] == "(?i)" {
		s = s[4:]
	}
	// Remove regex escapes for display
	replacer := strings.NewReplacer(`\(`, "(", `\)`, ")", `\[`, "[", `\]`, "]")
	s = replacer.Replace(s)
	return s
}

func printExamples() {
	fmt.Println("Get started with 'passauto add <name>'. Examples:")
	fmt.Println()
	fmt.Println("  passauto add myserver     # Create a new profile")
	fmt.Println("  passauto add myserver     # SSH to a host")
	fmt.Println("  passauto add mysudo       # Sudo commands")
	fmt.Println("  passauto add docker       # Docker login")
	fmt.Println("  passauto add aws-sso      # AWS SSO login")
	fmt.Println("  passauto add mysql        # MySQL client")
	fmt.Println("  passauto add git-push     # Git HTTPS credential")
	fmt.Println()
	fmt.Println("Or with flags:")
	fmt.Println("  passauto add -c \"ssh user@host\" -m \"password:\" myserver")
	fmt.Println()
	fmt.Println("Then run: passauto <name>")
}
