package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mattwalters/bubble/internal/state"
)

// ProcessInfo holds information about a recorded process.
type ProcessInfo struct {
	Name  string
	PID   int
	Alive bool
}

// StartProcess spawns a background process, redirects output to .bubble/logs/<name>.log,
// and records its PID in .bubble/pids/<name>.
func StartProcess(dir, name, command string, env []string) (int, error) {
	logsDir := state.WorktreeLogsDir(dir)
	pidsDir := state.WorktreePidsDir(dir)

	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return 0, fmt.Errorf("creating logs directory: %w", err)
	}
	if err := os.MkdirAll(pidsDir, 0755); err != nil {
		return 0, fmt.Errorf("creating pids directory: %w", err)
	}

	logPath := filepath.Join(logsDir, fmt.Sprintf("%s.log", name))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("opening log file %s: %w", logPath, err)
	}

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Put in its own process group so we can terminate child subprocesses cleanly
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return 0, fmt.Errorf("starting process %s (%s): %w", name, command, err)
	}

	pid := cmd.Process.Pid

	// Write PID file
	pidPath := filepath.Join(pidsDir, name)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return pid, fmt.Errorf("writing PID file %s: %w", pidPath, err)
	}

	// Detach process in Go runtime so it runs independently
	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
	}()

	return pid, nil
}

// IsAlive checks if a process with the given PID is currently running.
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds, so we send signal 0
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// KillProcess terminates a process by name, along with its process group.
func KillProcess(dir, name string) error {
	pidsDir := state.WorktreePidsDir(dir)
	pidPath := filepath.Join(pidsDir, name)

	data, err := os.ReadFile(pidPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err == nil && pid > 0 && IsAlive(pid) {
		// Attempt to kill process group
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		_ = syscall.Kill(pid, syscall.SIGTERM)

		// Wait briefly for graceful exit
		deadline := time.Now().Add(1500 * time.Millisecond)
		for time.Now().Before(deadline) {
			if !IsAlive(pid) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Force kill if still alive
		if IsAlive(pid) {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}

	_ = os.Remove(pidPath)
	return nil
}

// KillAllProcesses terminates all recorded processes in .bubble/pids/.
func KillAllProcesses(dir string) error {
	pidsDir := state.WorktreePidsDir(dir)
	entries, err := os.ReadDir(pidsDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading pids dir %s: %w", pidsDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		_ = KillProcess(dir, entry.Name())
	}

	return nil
}

// ListProcesses returns the status of all recorded processes in .bubble/pids/.
func ListProcesses(dir string) ([]ProcessInfo, error) {
	pidsDir := state.WorktreePidsDir(dir)
	entries, err := os.ReadDir(pidsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var procs []ProcessInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		data, err := os.ReadFile(filepath.Join(pidsDir, name))
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			continue
		}
		procs = append(procs, ProcessInfo{
			Name:  name,
			PID:   pid,
			Alive: IsAlive(pid),
		})
	}

	return procs, nil
}
