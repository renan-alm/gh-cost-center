// Package cmd implements the CLI command tree for gh-cost-center.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	cfgFile string
	verbose bool
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "gh-cost-center",
	Short: "GitHub Enterprise cost center management",
	Long: `gh cost-center automates cost center assignments for GitHub Copilot users.

Three operational modes:

  PRU-Based (default):  Two-tier model based on Premium Request Unit exceptions.
                        All users go to a default cost center; exception users
                        go to a PRU-allowed cost center.

  Teams-Based:          Assigns users to cost centers based on GitHub team
                        membership, with auto or manual mapping modes.
                        Supports organization and enterprise scopes.

  Repository-Based:     Assigns repositories to cost centers based on
                        organization custom property values.

Examples:
  # PRU-based mode (default)
  gh cost-center assign --mode plan
  gh cost-center assign --mode apply --yes

  # Teams-based mode
  gh cost-center assign --teams --mode plan
  gh cost-center assign --teams --mode apply --yes

  # Repository-based mode
  gh cost-center assign --repo --mode plan

  # View configuration
  gh cost-center config

  # List Copilot users
  gh cost-center list-users

  # Generate summary report
  gh cost-center report`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config/config.yaml", "configuration file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose (debug) logging")
}
