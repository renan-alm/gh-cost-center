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
	cfgFile   string
	verbose   bool
	tokenFlag string

	// cfgManager is the loaded configuration, available to all subcommands.
	cfgManager *config.Manager
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "gh-cost-center",
	Short: "GitHub Enterprise cost center management",
	Long: `gh cost-center automates cost center assignments for GitHub Copilot users.

The operational mode is determined by cost_center.mode in config.yaml:

  users (default):  Two-tier model based on Premium Request Unit exceptions.
                    All users go to a default cost center; exception users
                    go to a PRU-allowed cost center.

  teams:            Assigns users to cost centers based on GitHub team
                    membership, with auto or manual strategy.
                    Supports organization and enterprise scopes.

  repos:            Assigns repositories to cost centers based on
                    organization custom property values (explicit mappings).

  custom-prop:      Assigns repositories using custom property filters
                    with AND logic across multiple properties.

Examples:
  # Assign (mode from config)
  gh cost-center assign --mode plan
  gh cost-center assign --mode apply --yes

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
		cfgManager.Token = tokenFlag
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
	rootCmd.PersistentFlags().StringVar(&tokenFlag, "token", "", "GitHub personal access token (overrides GITHUB_TOKEN, GH_TOKEN, and gh auth)")
}
