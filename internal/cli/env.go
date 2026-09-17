package cli

import (
	"fmt"
	"os"

	"github.com/mattwalters/bubble/internal/state"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env <id>",
	Short: "Print the generated .env for a bubble",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		return runEnv(id)
	},
}

func runEnv(id string) error {
	route, err := state.ReadRoute(id)
	if err != nil {
		return fmt.Errorf("bubble %q not found: %w", id, err)
	}

	envPath := state.WorktreeEnvFile(route.Dir)
	data, err := os.ReadFile(envPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", envPath, err)
	}

	fmt.Print(string(data))
	return nil
}
