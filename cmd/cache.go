package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	cacheStats   bool
	cacheClear   bool
	cacheCleanup bool
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the cost center cache",
	Long: `View, clear, or clean up the cost center cache.

The cache stores cost center lookups to reduce API calls on repeated runs.
Cache entries expire after 24 hours.

Examples:
  # Show cache statistics
  gh cost-center cache --stats

  # Clear the entire cache
  gh cost-center cache --clear

  # Remove only expired entries
  gh cost-center cache --cleanup`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !cacheStats && !cacheClear && !cacheCleanup {
			return cmd.Help()
		}

		// TODO: Wire to business logic in later PRs
		if cacheStats {
			fmt.Println("cache stats requested")
		}
		if cacheClear {
			fmt.Println("cache clear requested")
		}
		if cacheCleanup {
			fmt.Println("cache cleanup requested")
		}
		return nil
	},
}

func init() {
	cacheCmd.Flags().BoolVar(&cacheStats, "stats", false, "show cache statistics")
	cacheCmd.Flags().BoolVar(&cacheClear, "clear", false, "clear the entire cache")
	cacheCmd.Flags().BoolVar(&cacheCleanup, "cleanup", false, "remove expired cache entries")

	rootCmd.AddCommand(cacheCmd)
}
