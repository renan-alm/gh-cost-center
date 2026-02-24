// Package cmd implements the CLI command tree for gh-cost-center.
package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/renan-alm/gh-cost-center/internal/config"
)

var (
	// Global flags
	cfgFile string
	verbose bool

	// cfgManager is the loaded configuration, available to all subcommands.
	cfgManager *config.Manager
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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Set up logger.
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		}
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
		slog.SetDefault(logger)

		// Load configuration.
		mgr, err := config.Load(cfgFile, logger)
		if err != nil {
			return fmt.Errorf("loading configuration: %w", err)
		}
		cfgManager = mgr
		cfgManager.CheckConfigWarnings()
		return nil
	},
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
