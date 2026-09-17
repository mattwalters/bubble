package compose

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mattwalters/bubble/internal/config"
)

// DiscoverPort queries docker compose port for a single service and container port.
func DiscoverPort(dir, projectID, composeFile, overrideFile, envFile, service string, containerPort int) (int, error) {
	args := []string{"compose"}
	if projectID != "" {
		args = append(args, "-p", projectID)
	}
	if composeFile != "" {
		args = append(args, "-f", composeFile)
	}
	if overrideFile != "" {
		args = append(args, "-f", overrideFile)
	}
	if envFile != "" {
		args = append(args, "--env-file", envFile)
	}
	args = append(args, "port", service, strconv.Itoa(containerPort))

	cmd := exec.Command("docker", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("docker compose port %s %d: %w (%s)", service, containerPort, err, strings.TrimSpace(string(out)))
	}

	return parseHostPort(string(out))
}

// DiscoverAllPorts queries docker compose port for all required service ports.
func DiscoverAllPorts(dir, projectID, composeFile, overrideFile, envFile string, reqs []config.ServicePortRequirement) (map[string]int, error) {
	ports := make(map[string]int)

	for _, req := range reqs {
		hp, err := DiscoverPort(dir, projectID, composeFile, overrideFile, envFile, req.Service, req.ContainerPort)
		if err != nil {
			return nil, err
		}
		key := fmt.Sprintf("%s:%d", req.Service, req.ContainerPort)
		ports[key] = hp
	}

	return ports, nil
}

func parseHostPort(output string) (int, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Typically "0.0.0.0:55123" or ":::55123" or "127.0.0.1:55123"
		colonIdx := strings.LastIndex(line, ":")
		if colonIdx >= 0 && colonIdx < len(line)-1 {
			portStr := line[colonIdx+1:]
			p, err := strconv.Atoi(portStr)
			if err == nil && p > 0 {
				return p, nil
			}
		}
	}
	return 0, fmt.Errorf("could not parse host port from output: %q", output)
}
