package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mattwalters/bubble/internal/compose"
	"github.com/mattwalters/bubble/internal/proxy"
	"github.com/mattwalters/bubble/internal/state"
	"github.com/mattwalters/bubble/internal/ui"
	"github.com/spf13/cobra"
)

var sweepDryRun bool

var sweepCmd = &cobra.Command{
	Use:   "sweep",
	Short: "Find and clean orphaned Docker resources and stale route files",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSweep(sweepDryRun)
	},
}

func init() {
	sweepCmd.Flags().BoolVar(&sweepDryRun, "dry-run", false, "Simulate sweep and print orphaned resources without deleting")
}

func runSweep(dryRun bool) error {
	routes, err := state.ListRoutes()
	if err != nil {
		return fmt.Errorf("listing routes: %w", err)
	}

	activeRoutes := make(map[string]state.Route)
	for _, r := range routes {
		activeRoutes[r.ID] = r
	}

	orphansFound := 0

	// Pass 1: By label: docker ps -a --filter label=bubble.managed=true
	if err := compose.CheckDockerRunning(); err == nil {
		cmd := exec.Command("docker", "ps", "-a", "--filter", "label=bubble.managed=true", "--format", "{{.Label \"com.docker.compose.project\"}}\t{{.Names}}")
		out, err := cmd.Output()
		if err == nil {
			projectContainers := make(map[string][]string)
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				parts := strings.Split(line, "\t")
				if len(parts) >= 2 && parts[0] != "" {
					projectContainers[parts[0]] = append(projectContainers[parts[0]], parts[1])
				}
			}

			for project, containers := range projectContainers {
				route, routeExists := activeRoutes[project]
				worktreeExists := false
				if routeExists {
					if _, err := os.Stat(route.Dir); err == nil {
						worktreeExists = true
					}
				}

				if !routeExists || !worktreeExists {
					orphansFound++
					if dryRun {
						fmt.Printf("[dry-run] Would clean orphaned Docker compose project %q (containers: %s)\n", project, strings.Join(containers, ", "))
					} else {
						ui.PrintStep("sweep", fmt.Sprintf("cleaning orphaned compose project %q...", project))
						_ = compose.Down(project, "")
					}
				}
			}
		}
	}

	// Pass 2: By route file
	for _, route := range routes {
		worktreeMissing := false
		if _, err := os.Stat(route.Dir); os.IsNotExist(err) {
			worktreeMissing = true
		}

		if worktreeMissing {
			orphansFound++
			if dryRun {
				fmt.Printf("[dry-run] Would remove orphaned route %q (worktree %s does not exist)\n", route.ID, route.Dir)
			} else {
				ui.PrintStep("sweep", fmt.Sprintf("cleaning Docker for deleted worktree %s...", route.ID))
				_ = compose.Down(route.ComposeProject, "")
				_ = state.DeleteRoute(route.ID)
			}
		}
	}

	if !dryRun {
		remaining, _ := state.ListRoutes()
		if len(remaining) == 0 {
			_ = proxy.StopDaemon()
		}
	}

	if orphansFound == 0 {
		ui.PrintSuccess("No orphaned resources found.")
	} else if dryRun {
		fmt.Printf("\nFound %d orphaned resource(s). Run 'bubble sweep' to remove them.\n", orphansFound)
	} else {
		ui.PrintSuccess(fmt.Sprintf("Swept %d orphaned resource(s).", orphansFound))
	}

	return nil
}
