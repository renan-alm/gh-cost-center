package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags.
var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  "Display the version of gh-cost-center.",
	Run: func(cmd *cobra.Command, args []string) {
		// If version is "dev", try to read from VERSION file
		v := version
		if v == "dev" {
			if data, err := os.ReadFile("VERSION"); err == nil {
				if trimmed := strings.TrimSpace(string(data)); trimmed != "" {
					v = trimmed
				}
			}
		}
		fmt.Printf("gh-cost-center version %s\n", v)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
