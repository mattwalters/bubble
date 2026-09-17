package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/mattwalters/bubble/internal/process"
	"github.com/mattwalters/bubble/internal/state"
	"github.com/spf13/cobra"
)

var logsFollow bool

var logsCmd = &cobra.Command{
	Use:   "logs <id> [process]",
	Short: "Print or follow captured logs from host processes",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		targetProc := ""
		if len(args) > 1 {
			targetProc = args[1]
		}
		return runLogs(id, targetProc, logsFollow)
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
}

func runLogs(id, targetProc string, follow bool) error {
	route, err := state.ReadRoute(id)
	if err != nil {
		return fmt.Errorf("bubble %q not found: %w", id, err)
	}

	worktreeDir := route.Dir
	logsDir := state.WorktreeLogsDir(worktreeDir)

	if targetProc == "" {
		// Find available log files
		entries, err := os.ReadDir(logsDir)
		if err != nil || len(entries) == 0 {
			return fmt.Errorf("no logs found in %s", logsDir)
		}

		var available []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
				name := strings.TrimSuffix(e.Name(), ".log")
				available = append(available, name)
			}
		}

		if len(available) == 1 {
			targetProc = available[0]
		} else {
			// If "web" exists, default to "web"
			foundWeb := false
			for _, a := range available {
				if a == "web" {
					targetProc = "web"
					foundWeb = true
					break
				}
			}
			if !foundWeb {
				return fmt.Errorf("multiple processes logged (%s). Specify one: bubble logs %s <process>", strings.Join(available, ", "), id)
			}
		}
	}

	logFile := filepath.Join(logsDir, fmt.Sprintf("%s.log", targetProc))
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return fmt.Errorf("log file %s does not exist", logFile)
	}

	if !follow {
		content, err := process.ReadLogs(worktreeDir, targetProc)
		if err != nil {
			return err
		}
		fmt.Print(content)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	return process.FollowLogs(ctx, worktreeDir, targetProc, os.Stdout)
}
