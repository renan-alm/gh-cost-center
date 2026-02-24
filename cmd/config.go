package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration",
	Long: `Display the current configuration and exit.

Shows enterprise, cost center IDs, PRU exception users, teams settings,
and other configuration details.

Examples:
  gh cost-center config
  gh cost-center config --config path/to/config.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Wire to business logic in later PRs
		fmt.Println("config command called")
		fmt.Printf("  config file: %s\n", cfgFile)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
