package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/mattwalters/bubble/internal/proxy"
	"github.com/mattwalters/bubble/internal/state"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open <id>",
	Short: "Open the primary web service in the browser",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		return runOpen(id)
	},
}

func runOpen(id string) error {
	if _, err := state.ReadRoute(id); err != nil {
		return fmt.Errorf("bubble %q not found: %w", id, err)
	}

	proxyPort := proxy.ConfiguredPort()
	url := fmt.Sprintf("http://%s.localhost:%d/", id, proxyPort)

	var openExec string
	switch runtime.GOOS {
	case "darwin":
		openExec = "open"
	case "windows":
		openExec = "explorer"
	default:
		openExec = "xdg-open"
	}

	fmt.Printf("Opening %s\n", url)
	cmd := exec.Command(openExec, url)
	return cmd.Start()
}
