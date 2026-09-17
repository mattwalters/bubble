package compose

import (
	"net"
	"strings"
	"testing"
	"time"
)

const sampleBaseCompose = `services:
  postgres:
    image: postgres:16
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
  redis:
    image: redis:7
    ports:
      - "6379:6379"
`

func TestGenerateOverrideYAML(t *testing.T) {
	envDirectives := map[string]map[string]string{
		"postgres": {
			"POSTGRES_DB": "myapp_{{id}}",
		},
	}

	override, err := GenerateOverrideYAML(sampleBaseCompose, "pw-142", envDirectives)
	if err != nil {
		t.Fatalf("GenerateOverrideYAML failed: %v", err)
	}

	if !strings.Contains(override, "bubble.managed: \"true\"") {
		t.Errorf("expected bubble.managed label in override: %s", override)
	}
	if !strings.Contains(override, "ports: !override") {
		t.Errorf("expected ports: !override in override: %s", override)
	}
	if !strings.Contains(override, "- \"5432\"") {
		t.Errorf("expected - \"5432\" in override: %s", override)
	}
	if !strings.Contains(override, "- \"6379\"") {
		t.Errorf("expected - \"6379\" in override: %s", override)
	}
	if !strings.Contains(override, "POSTGRES_DB: \"myapp_pw-142\"") {
		t.Errorf("expected POSTGRES_DB: \"myapp_pw-142\" in override: %s", override)
	}
}

func TestParseHostPort(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"0.0.0.0:55123", 55123},
		{"127.0.0.1:55124\n", 55124},
		{":::55125", 55125},
		{"[::]:55126", 55126},
	}

	for _, tc := range tests {
		got, err := parseHostPort(tc.input)
		if err != nil || got != tc.want {
			t.Errorf("parseHostPort(%q) = %d, err %v; want %d", tc.input, got, err, tc.want)
		}
	}
}

func TestServicesLackingHealthcheck(t *testing.T) {
	lacking, err := ServicesLackingHealthcheck(sampleBaseCompose)
	if err != nil {
		t.Fatalf("ServicesLackingHealthcheck failed: %v", err)
	}
	if len(lacking) != 1 || lacking[0] != "redis" {
		t.Errorf("expected redis to lack healthcheck, got: %v", lacking)
	}
}

func TestWaitForTCPPorts(t *testing.T) {
	// Start a local listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	err = WaitForTCPPorts([]int{port}, 2*time.Second)
	if err != nil {
		t.Errorf("WaitForTCPPorts failed unexpectedly: %v", err)
	}
}
