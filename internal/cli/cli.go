// Package cli provides agent-native CLI conventions for Cobra-based tools.
// It handles structured output (JSON/plain), leveled logging to stderr,
// consistent error codes, and test helpers for CLI integration testing.
package cli

import "github.com/spf13/cobra"

// RegisterOutputFlags adds --output/-o (plain|json) to a command's persistent flags.
// Agents should always call this on the root command so all subcommands inherit it.
func RegisterOutputFlags(cmd *cobra.Command, target *string) {
	cmd.PersistentFlags().StringVarP(target, "output", "o", "plain",
		"Output format: plain, json")
}

// RegisterLogFlags adds --verbose/-v and --quiet/-q to a command's persistent flags.
func RegisterLogFlags(cmd *cobra.Command, verbose, quiet *bool) {
	cmd.PersistentFlags().BoolVarP(verbose, "verbose", "v", false,
		"Enable verbose diagnostic output (stderr)")
	cmd.PersistentFlags().BoolVarP(quiet, "quiet", "q", false,
		"Suppress all diagnostic output")
}
