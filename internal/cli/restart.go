package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mattwalters/bubble/internal/config"
	"github.com/mattwalters/bubble/internal/process"
	"github.com/mattwalters/bubble/internal/state"
	"github.com/mattwalters/bubble/internal/ui"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart <id> [process]",
	Short: "Kill and restart host processes with fresh environment",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		targetProc := ""
		if len(args) > 1 {
			targetProc = args[1]
		}
		return runRestart(id, targetProc)
	},
}

func runRestart(id, targetProc string) error {
	route, err := state.ReadRoute(id)
	if err != nil {
		return fmt.Errorf("bubble %q not found: %w", id, err)
	}

	worktreeDir := route.Dir
	st, err := state.ReadWorktreeState(worktreeDir)
	if err != nil {
		return fmt.Errorf("reading worktree state for %s: %w", id, err)
	}

	cfgPath, err := config.FindConfigFile(worktreeDir)
	if err != nil {
		return fmt.Errorf("locating bubble.toml: %w", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading bubble.toml: %w", err)
	}

	tier, ok := cfg.Tiers[st.Tier]
	if !ok {
		return fmt.Errorf("tier %q not found in config", st.Tier)
	}

	envFilePath := state.WorktreeEnvFile(worktreeDir)
	envBytes, err := os.ReadFile(envFilePath)
	if err != nil {
		return fmt.Errorf("reading .env file %s: %w", envFilePath, err)
	}
	envSlice := append(os.Environ(), strings.Split(string(envBytes), "\n")...)

	restartedCount := 0
	for _, proc := range tier.Processes {
		if targetProc != "" && proc.Name != targetProc {
			continue
		}

		_ = process.KillProcess(worktreeDir, proc.Name)
		pid, err := process.StartProcess(worktreeDir, proc.Name, proc.Command, envSlice)
		if err != nil {
			return fmt.Errorf("restarting %s: %w", proc.Name, err)
		}
		ui.PrintStep("process", fmt.Sprintf("restarted %s (PID %d)", proc.Name, pid))
		restartedCount++
	}

	if restartedCount == 0 && targetProc != "" {
		return fmt.Errorf("process %q not found in tier %q", targetProc, st.Tier)
	}

	return nil
}
