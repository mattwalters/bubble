package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mattwalters/bubble/internal/ui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure system resolver for .localhost subdomains (Safari on macOS)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSetup()
	},
}

func runSetup() error {
	if runtime.GOOS != "darwin" {
		ui.PrintSuccess("On Linux and Windows, Chrome, Firefox, and Edge resolve *.localhost to 127.0.0.1 natively. No system configuration required.")
		return nil
	}

	resolverPath := "/etc/resolver/localhost"
	if data, err := os.ReadFile(resolverPath); err == nil {
		if strings.Contains(string(data), "127.0.0.1") {
			ui.PrintSuccess("/etc/resolver/localhost is already configured for *.localhost subdomains.")
			return nil
		}
	}

	fmt.Println("Configuring macOS resolver so Safari can resolve *.localhost subdomains.")
	fmt.Println("This requires administrative privileges to create /etc/resolver/localhost.")

	setupScript := `sudo mkdir -p /etc/resolver && echo "nameserver 127.0.0.1" | sudo tee /etc/resolver/localhost > /dev/null`
	cmd := exec.Command("sh", "-c", setupScript)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("setting up /etc/resolver/localhost: %w", err)
	}

	ui.PrintSuccess("Configured /etc/resolver/localhost successfully. Safari will now resolve *.localhost to 127.0.0.1.")
	return nil
}
