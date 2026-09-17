package state

import (
	"os"
	"path/filepath"
)

// GlobalDir returns the path to ~/.bubble or $BUBBLE_HOME.
func GlobalDir() string {
	if val := os.Getenv("BUBBLE_HOME"); val != "" {
		return val
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".bubble-global")
	}
	return filepath.Join(home, ".bubble")
}

// RoutesDir returns the path to ~/.bubble/routes.
func RoutesDir() string {
	return filepath.Join(GlobalDir(), "routes")
}

// ProxyPIDFile returns the path to ~/.bubble/proxy.pid.
func ProxyPIDFile() string {
	return filepath.Join(GlobalDir(), "proxy.pid")
}

// ProxyLogFile returns the path to ~/.bubble/proxy.log.
func ProxyLogFile() string {
	return filepath.Join(GlobalDir(), "proxy.log")
}

// WorktreeBubbleDir returns <dir>/.bubble.
func WorktreeBubbleDir(dir string) string {
	return filepath.Join(dir, ".bubble")
}

// WorktreeStateFile returns <dir>/.bubble/state.json.
func WorktreeStateFile(dir string) string {
	return filepath.Join(dir, ".bubble", "state.json")
}

// WorktreePidsDir returns <dir>/.bubble/pids.
func WorktreePidsDir(dir string) string {
	return filepath.Join(dir, ".bubble", "pids")
}

// WorktreeLogsDir returns <dir>/.bubble/logs.
func WorktreeLogsDir(dir string) string {
	return filepath.Join(dir, ".bubble", "logs")
}

// WorktreeOverrideFile returns <dir>/.bubble/compose-override.yml.
func WorktreeOverrideFile(dir string) string {
	return filepath.Join(dir, ".bubble", "compose-override.yml")
}

// WorktreeEnvFile returns <dir>/.env.
func WorktreeEnvFile(dir string) string {
	return filepath.Join(dir, ".env")
}
