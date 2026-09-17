package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "bubble",
	Short: "Per-worktree service isolation, port allocation, and preview URLs",
	Long: `Bubble is a local-first CLI tool that gives each parallel development task
its own isolated set of backing services via Docker Compose.

It manages ephemeral port allocation, environment variable generation,
service readiness, and cookie-safe browser preview URLs.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root bubble command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(envCmd)
	rootCmd.AddCommand(portsCmd)
	rootCmd.AddCommand(sweepCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(proxyCmd)
}
