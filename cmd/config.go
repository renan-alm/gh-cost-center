package cmd

import (
	"fmt"
	"sort"
	"strings"

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
		summary := cfgManager.Summary()

		// Print in sorted key order for deterministic output.
		keys := make([]string, 0, len(summary))
		for k := range summary {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		fmt.Println("Current configuration:")
		fmt.Println(strings.Repeat("-", 50))
		for _, k := range keys {
			fmt.Printf("  %-35s %v\n", k+":", summary[k])
		}
		fmt.Println(strings.Repeat("-", 50))
		fmt.Printf("  config file: %s\n", cfgFile)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
