package cli

import (
	"fmt"
	"os"

	"github.com/mattwalters/bubble/internal/compose"
	"github.com/mattwalters/bubble/internal/git"
	"github.com/mattwalters/bubble/internal/process"
	"github.com/mattwalters/bubble/internal/proxy"
	"github.com/mattwalters/bubble/internal/state"
	"github.com/mattwalters/bubble/internal/ui"
	"github.com/spf13/cobra"
)

var downForce bool

var downCmd = &cobra.Command{
	Use:   "down <id>",
	Short: "Tear down backing services, host processes, and state for a bubble",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		return runDown(id, downForce)
	},
}

func init() {
	downCmd.Flags().BoolVarP(&downForce, "force", "f", false, "Force teardown even if worktree has uncommitted changes")
}

func runDown(id string, force bool) error {
	// Find worktree directory
	worktreeDir := ""
	if route, err := state.ReadRoute(id); err == nil && route.Dir != "" {
		worktreeDir = route.Dir
	} else if currSt, err := state.ReadWorktreeState("."); err == nil && currSt.ID == id {
		worktreeDir = "."
	}

	// 1. Safety check on worktree
	if worktreeDir != "" {
		if _, err := os.Stat(worktreeDir); err == nil {
			safety, err := git.CheckSafety(worktreeDir)
			if err == nil {
				if safety.HasUncommittedChanges && !force {
					ui.PrintError(fmt.Sprintf("cannot tear down bubble %q: worktree has uncommitted changes:\n%s\n\nCommit or clean up your changes, or pass --force to discard.", id, safety.UncommittedSummary))
					return fmt.Errorf("worktree has uncommitted changes")
				}
				if safety.HasUnpushedCommits {
					ui.PrintWarning(fmt.Sprintf("branch %q has %d unpushed commit(s) relative to upstream", safety.Branch, safety.UnpushedCount))
				} else {
					ui.PrintStep("worktree", "clean, branch pushed ✓")
				}
			}
		}
	}

	// 2. Kill host processes
	if worktreeDir != "" {
		procs, _ := process.ListProcesses(worktreeDir)
		_ = process.KillAllProcesses(worktreeDir)
		if len(procs) > 0 {
			ui.PrintStep("processes", fmt.Sprintf("%d process(es) stopped", len(procs)))
		} else {
			ui.PrintStep("processes", "none running")
		}
	}

	// 3. docker compose down --volumes --remove-orphans
	_ = compose.Down(id, worktreeDir)
	ui.PrintStep("compose", fmt.Sprintf("%s stopped, volumes removed", id))

	// 4. Remove .bubble/ from worktree
	if worktreeDir != "" {
		_ = state.RemoveWorktreeBubbleDir(worktreeDir)
	}

	// 5. Remove route file
	_ = state.DeleteRoute(id)
	ui.PrintStep("state", "cleaned up")

	// 6. Stop proxy if no routes remain
	remaining, _ := state.ListRoutes()
	if len(remaining) == 0 {
		_ = proxy.StopDaemon()
	}

	return nil
}
