package compose

import (
	"fmt"
	"net"
	"time"

	"gopkg.in/yaml.v3"
)

// ServicesLackingHealthcheck returns names of services in compose file that have no healthcheck.
func ServicesLackingHealthcheck(baseComposeContent string) ([]string, error) {
	var base BaseCompose
	if err := yaml.Unmarshal([]byte(baseComposeContent), &base); err != nil {
		return nil, err
	}

	var lacking []string
	for name, svc := range base.Services {
		if svc.Healthcheck == nil {
			lacking = append(lacking, name)
		}
	}
	return lacking, nil
}

// WaitForTCPPorts attempts TCP connections to 127.0.0.1:<hostPort> until successful or timeout.
func WaitForTCPPorts(ports []int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for _, port := range ports {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		connected := false

		for time.Now().Before(deadline) {
			conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				connected = true
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

		if !connected {
			return fmt.Errorf("timeout waiting for service to accept TCP connections on %s", addr)
		}
	}

	return nil
}
