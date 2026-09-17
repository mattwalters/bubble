package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProcessLifecycle(t *testing.T) {
	tmpDir := t.TempDir()

	pid, err := StartProcess(tmpDir, "testproc", "echo hello from testproc; sleep 10", os.Environ())
	if err != nil {
		t.Fatalf("StartProcess failed: %v", err)
	}

	if pid <= 0 {
		t.Fatalf("unexpected pid: %d", pid)
	}

	if !IsAlive(pid) {
		t.Errorf("expected process %d to be alive", pid)
	}

	// Verify PID file
	pidFile := filepath.Join(tmpDir, ".bubble", "pids", "testproc")
	if _, err := os.Stat(pidFile); err != nil {
		t.Errorf("expected pid file to exist: %v", err)
	}

	// Verify log file
	time.Sleep(200 * time.Millisecond)
	logs, err := ReadLogs(tmpDir, "testproc")
	if err != nil {
		t.Fatalf("ReadLogs failed: %v", err)
	}
	if !strings.Contains(logs, "hello from testproc") {
		t.Errorf("expected log to contain hello from testproc, got: %q", logs)
	}

	// Verify ListProcesses
	procs, err := ListProcesses(tmpDir)
	if err != nil || len(procs) != 1 {
		t.Fatalf("ListProcesses failed: %v, procs: %+v", err, procs)
	}
	if procs[0].Name != "testproc" || !procs[0].Alive {
		t.Errorf("unexpected proc info: %+v", procs[0])
	}

	// Kill process
	if err := KillProcess(tmpDir, "testproc"); err != nil {
		t.Fatalf("KillProcess failed: %v", err)
	}

	if IsAlive(pid) {
		t.Errorf("expected process %d to be killed", pid)
	}

	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Errorf("expected pid file to be deleted")
	}
}
