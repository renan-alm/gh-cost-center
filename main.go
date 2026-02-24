// gh-cost-center is a GitHub CLI extension that automates cost center
// assignments for GitHub Copilot users in a GitHub Enterprise.
//
// Install:
//
//	gh extension install renan-alm/gh-cost-center
//
// Usage:
//
//	gh cost-center assign --mode plan
//	gh cost-center assign --mode apply --yes
package main

import "github.com/renan-alm/gh-cost-center/cmd"

func main() {
	cmd.Execute()
}
