package compose

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// CheckDockerRunning checks if the Docker daemon is accessible.
func CheckDockerRunning() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker is not running. Start Docker Desktop and try again.")
	}
	return nil
}

// RunCompose executes a docker compose command with the specified arguments.
func RunCompose(dir string, args ...string) (string, error) {
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stdout.String(), fmt.Errorf("docker compose %s failed: %w (stderr: %s)", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}

// Down tears down containers and volumes for a given project name.
func Down(projectID, dir string) error {
	cmd := exec.Command("docker", "compose", "-p", projectID, "down", "--volumes", "--remove-orphans")
	if dir != "" {
		cmd.Dir = dir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errStr := stderr.String()
		// If project or containers are already gone, swallow error
		if strings.Contains(errStr, "not found") || strings.Contains(errStr, "no such") {
			return nil
		}
		return fmt.Errorf("docker compose down for %s: %w (%s)", projectID, err, errStr)
	}
	return nil
}
