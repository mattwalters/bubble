package proxy

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mattwalters/bubble/internal/process"
	"github.com/mattwalters/bubble/internal/state"
	"github.com/pelletier/go-toml/v2"
)

// DefaultPort is the default proxy port.
const DefaultPort = 19100

// ConfiguredPort returns the proxy port from env, ~/.bubble/config.toml, or DefaultPort.
func ConfiguredPort() int {
	if val := os.Getenv("BUBBLE_PROXY_PORT"); val != "" {
		if p, err := strconv.Atoi(val); err == nil && p > 0 {
			return p
		}
	}

	cfgPath := filepath.Join(state.GlobalDir(), "config.toml")
	if data, err := os.ReadFile(cfgPath); err == nil {
		var cfg struct {
			ProxyPort int `toml:"proxy_port"`
		}
		if err := toml.Unmarshal(data, &cfg); err == nil && cfg.ProxyPort > 0 {
			return cfg.ProxyPort
		}
	}

	return DefaultPort
}

// IsProxyHealthy checks if the proxy is responding to /_bubble/health.
func IsProxyHealthy(port int) bool {
	if port <= 0 {
		port = ConfiguredPort()
	}

	client := http.Client{
		Timeout: 600 * time.Millisecond,
	}

	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/_bubble/health", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// StartDaemon starts the bubble proxy in the background if it is not already running.
func StartDaemon(port int) error {
	if port <= 0 {
		port = ConfiguredPort()
	}

	if IsProxyHealthy(port) {
		return nil
	}

	globalDir := state.GlobalDir()
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		return fmt.Errorf("creating global dir %s: %w", globalDir, err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	if strings.HasSuffix(exePath, ".test") {
		// In unit tests, run in-process background server
		srv := NewServer(port)
		go func() {
			_ = srv.Start(context.Background())
		}()
		deadline := time.Now().Add(1500 * time.Millisecond)
		for time.Now().Before(deadline) {
			if IsProxyHealthy(port) {
				return nil
			}
			time.Sleep(50 * time.Millisecond)
		}
		return nil
	}

	logPath := state.ProxyLogFile()
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("opening proxy log file %s: %w", logPath, err)
	}

	cmd := exec.Command(exePath, "proxy", "run", "--port", strconv.Itoa(port))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("starting proxy process: %w", err)
	}

	pid := cmd.Process.Pid
	pidFile := state.ProxyPIDFile()
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("writing proxy pid file %s: %w", pidFile, err)
	}

	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
	}()

	// Wait up to 3 seconds for healthcheck
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if IsProxyHealthy(port) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("proxy daemon started (PID %d) but failed to pass healthcheck within 3s", pid)
}

// StopDaemon stops the running proxy background process.
func StopDaemon() error {
	pidFile := state.ProxyPIDFile()
	data, err := os.ReadFile(pidFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err == nil && pid > 0 && process.IsAlive(pid) {
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		_ = syscall.Kill(pid, syscall.SIGTERM)

		deadline := time.Now().Add(1500 * time.Millisecond)
		for time.Now().Before(deadline) {
			if !process.IsAlive(pid) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if process.IsAlive(pid) {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}

	_ = os.Remove(pidFile)
	return nil
}

// DaemonStatus returns whether the proxy is healthy and its PID if known.
func DaemonStatus(port int) (bool, int) {
	if port <= 0 {
		port = ConfiguredPort()
	}

	pid := 0
	data, err := os.ReadFile(state.ProxyPIDFile())
	if err == nil {
		pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}

	healthy := IsProxyHealthy(port)
	return healthy, pid
}
