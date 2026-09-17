package config

import (
	"os"
	"path/filepath"
	"testing"
)

const fullConfigToml = `[bubble]
max = 5

[setup]
clone = ["node_modules", "app/*/node_modules", "lib/*/node_modules"]
install = "npm install --prefer-offline --no-audit --no-fund"

[compose]
file = "docker-compose.yml"

[compose.ports]
DATABASE_URL = "postgresql://postgres:password@127.0.0.1:{{postgres:5432}}/myapp"
REDIS_URL = "redis://127.0.0.1:{{redis:6379}}/0"
SMTP_HOST = "127.0.0.1"
SMTP_PORT = "{{mailpit:1025}}"
MAILPIT_URL = "http://127.0.0.1:{{mailpit:8025}}"

[compose.env.postgres]
POSTGRES_DB = "myapp_{{id}}"

[open]
service = "web"

[env]
seed = ".env.example"
AUTH_URL = "http://{{id}}.localhost:19100"
NEXTAUTH_URL = "http://{{id}}.localhost:19100"

[tiers.lint]
# worktree + install only

[tiers.data]
compose = true
after = ["npm run db:generate", "npm run build:libs", "npm run db:deploy"]

[tiers.full]
compose = true
after = ["npm run db:generate", "npm run build:libs", "npm run db:deploy"]
processes = ["npm run dev:web", "npm run dev:worker"]
`

func TestLoadFullConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "bubble.toml")
	if err := os.WriteFile(cfgPath, []byte(fullConfigToml), 0644); err != nil {
		t.Fatalf("writing test toml: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Bubble.Max != 5 {
		t.Errorf("expected max 5, got %d", cfg.Bubble.Max)
	}

	if len(cfg.TierOrder) != 3 {
		t.Fatalf("expected 3 tiers in order, got %d: %v", len(cfg.TierOrder), cfg.TierOrder)
	}
	if cfg.TierOrder[0] != "lint" || cfg.TierOrder[1] != "data" || cfg.TierOrder[2] != "full" {
		t.Errorf("tier order mismatch: %v", cfg.TierOrder)
	}

	fullTier := cfg.Tiers["full"]
	if !fullTier.Compose {
		t.Errorf("expected compose = true for full tier")
	}
	if len(fullTier.After) != 3 {
		t.Errorf("expected 3 after hooks, got %d", len(fullTier.After))
	}
	if len(fullTier.Processes) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(fullTier.Processes))
	}
	if fullTier.Processes[0].Name != "web" || fullTier.Processes[0].Command != "npm run dev:web" {
		t.Errorf("process 0 mismatch: %+v", fullTier.Processes[0])
	}
	if fullTier.Processes[1].Name != "worker" || fullTier.Processes[1].Command != "npm run dev:worker" {
		t.Errorf("process 1 mismatch: %+v", fullTier.Processes[1])
	}

	if cfg.Compose.Env["postgres"]["POSTGRES_DB"] != "myapp_{{id}}" {
		t.Errorf("expected compose.env.postgres.POSTGRES_DB to be myapp_{{id}}")
	}

	if cfg.Env.Seed != ".env.example" {
		t.Errorf("expected env.seed to be .env.example, got %s", cfg.Env.Seed)
	}
	if cfg.Env.Vars["AUTH_URL"] != "http://{{id}}.localhost:19100" {
		t.Errorf("expected AUTH_URL with {{id}}, got %s", cfg.Env.Vars["AUTH_URL"])
	}
}

func TestTemplateExtractionAndResolution(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "bubble.toml")
	if err := os.WriteFile(cfgPath, []byte(fullConfigToml), 0644); err != nil {
		t.Fatalf("writing test toml: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	reqs := cfg.ExtractRequiredPorts()
	if len(reqs) < 4 {
		t.Errorf("expected at least 4 port requirements, got %d", len(reqs))
	}

	ctx := TemplateContext{
		ID: "pw-142",
		Ports: map[string]int{
			"postgres:5432": 55123,
			"redis:6379":    55124,
			"mailpit:1025":  55125,
			"mailpit:8025":  55126,
		},
		Port: 3000,
	}

	dbURL := Resolve(cfg.Compose.Ports["DATABASE_URL"], ctx)
	expectedDB := "postgresql://postgres:password@127.0.0.1:55123/myapp"
	if dbURL != expectedDB {
		t.Errorf("Resolve dbURL expected %s, got %s", expectedDB, dbURL)
	}

	authURL := Resolve(cfg.Env.Vars["AUTH_URL"], ctx)
	expectedAuth := "http://pw-142.localhost:19100"
	if authURL != expectedAuth {
		t.Errorf("Resolve authURL expected %s, got %s", expectedAuth, authURL)
	}

	portStr := Resolve("PORT={{port}}", ctx)
	if portStr != "PORT=3000" {
		t.Errorf("Resolve port expected PORT=3000, got %s", portStr)
	}
}

func TestAllocateEphemeralPort(t *testing.T) {
	port, err := AllocateEphemeralPort()
	if err != nil {
		t.Fatalf("AllocateEphemeralPort failed: %v", err)
	}
	if port <= 1024 || port > 65535 {
		t.Errorf("allocated unexpected port number: %d", port)
	}
}
