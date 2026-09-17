package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattwalters/bubble/internal/state"
)

func TestCLIInit(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create sample docker-compose.yml
	composeContent := `services:
  postgres:
    image: postgres:16
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
  redis:
    image: redis:7
    ports:
      - "6379:6379"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(composeContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := runInit(); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "bubble.toml"))
	if err != nil {
		t.Fatalf("bubble.toml not created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "DATABASE_URL = \"postgresql://") {
		t.Errorf("expected DATABASE_URL in scaffolded bubble.toml: %s", content)
	}
	if !strings.Contains(content, "REDIS_URL = \"redis://") {
		t.Errorf("expected REDIS_URL in scaffolded bubble.toml: %s", content)
	}
}

func TestCLIUpAndDownMinimal(t *testing.T) {
	tmpGlobal := t.TempDir()
	t.Setenv("BUBBLE_HOME", tmpGlobal)

	tmpWorktree := t.TempDir()

	// Create bubble.toml
	cfgContent := `[bubble]
max = 5

[tiers.lint]
# No compose, no processes
after = ["echo lint-passed"]
`
	if err := os.WriteFile(filepath.Join(tmpWorktree, "bubble.toml"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Run up
	if err := runUp("test-bubble-1", tmpWorktree, "lint"); err != nil {
		t.Fatalf("runUp failed: %v", err)
	}

	// Verify route file exists
	r, err := state.ReadRoute("test-bubble-1")
	if err != nil || r.ID != "test-bubble-1" {
		t.Fatalf("route file not found or invalid: %v, route: %+v", err, r)
	}

	// Verify state.json exists
	st, err := state.ReadWorktreeState(tmpWorktree)
	if err != nil || st.ID != "test-bubble-1" || st.Tier != "lint" {
		t.Fatalf("state.json not found or invalid: %v, state: %+v", err, st)
	}

	// 2. Run list
	if err := runList(false); err != nil {
		t.Fatalf("runList failed: %v", err)
	}

	// 3. Run down
	if err := runDown("test-bubble-1", false); err != nil {
		t.Fatalf("runDown failed: %v", err)
	}

	// Verify route file deleted
	routes, _ := state.ListRoutes()
	if len(routes) != 0 {
		t.Errorf("expected 0 routes after down, got %d", len(routes))
	}

	// Verify .bubble deleted
	if _, err := os.Stat(filepath.Join(tmpWorktree, ".bubble")); !os.IsNotExist(err) {
		t.Errorf("expected .bubble to be deleted")
	}
}

func TestCLIProcessLifecycle(t *testing.T) {
	tmpGlobal := t.TempDir()
	t.Setenv("BUBBLE_HOME", tmpGlobal)

	tmpWorktree := t.TempDir()

	cfgContent := `[bubble]
max = 5

[open]
service = "web"

[tiers.procs]
processes = ["echo started-web && sleep 10", "echo started-worker && sleep 10"]
`
	if err := os.WriteFile(filepath.Join(tmpWorktree, "bubble.toml"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Run up
	if err := runUp("test-proc-bubble", tmpWorktree, "procs"); err != nil {
		t.Fatalf("runUp with processes failed: %v", err)
	}

	// Verify logs command
	if err := runLogs("test-proc-bubble", "", false); err != nil {
		t.Errorf("runLogs failed: %v", err)
	}

	// Verify restart command
	if err := runRestart("test-proc-bubble", ""); err != nil {
		t.Errorf("runRestart failed: %v", err)
	}

	// 2. Run down
	if err := runDown("test-proc-bubble", false); err != nil {
		t.Fatalf("runDown failed: %v", err)
	}
}

func TestCLIDoctorPortsAndEnv(t *testing.T) {
	tmpGlobal := t.TempDir()
	t.Setenv("BUBBLE_HOME", tmpGlobal)

	tmpWorktree := t.TempDir()

	cfgContent := `[bubble]
max = 5

[compose.ports]
DATABASE_URL = "postgresql://postgres:password@127.0.0.1:{{postgres:5432}}/myapp"

[env]
seed = ".env.example"
AUTH_URL = "http://{{id}}.localhost:19100"

[tiers.data]
compose = false
`
	if err := os.WriteFile(filepath.Join(tmpWorktree, "bubble.toml"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpWorktree, ".env.example"), []byte("FOO=BAR\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := runUp("pw-test", tmpWorktree, "data"); err != nil {
		t.Fatalf("runUp failed: %v", err)
	}

	// Test env command
	if err := runEnv("pw-test"); err != nil {
		t.Errorf("runEnv failed: %v", err)
	}

	// Test ports command
	if err := runPorts("pw-test", false); err != nil {
		t.Errorf("runPorts failed: %v", err)
	}
	if err := runPorts("pw-test", true); err != nil {
		t.Errorf("runPorts JSON failed: %v", err)
	}

	// Test doctor report
	if err := runDoctor("pw-test", false, true); err != nil {
		t.Errorf("runDoctor failed: %v", err)
	}

	// Test doctor fix
	if err := runDoctor("pw-test", true, false); err != nil {
		t.Errorf("runDoctor fix failed: %v", err)
	}

	// Cleanup
	_ = runDown("pw-test", false)
}

func TestCLISweepOrphanedRoute(t *testing.T) {
	tmpGlobal := t.TempDir()
	t.Setenv("BUBBLE_HOME", tmpGlobal)

	orphanDir := filepath.Join(t.TempDir(), "deleted-worktree")

	r := state.Route{
		ID:             "orphan-1",
		Upstream:       "127.0.0.1:9999",
		Dir:            orphanDir, // doesn't exist
		ComposeProject: "orphan-1",
	}
	if err := state.WriteRoute(r); err != nil {
		t.Fatal(err)
	}

	// Dry run sweep
	if err := runSweep(true); err != nil {
		t.Fatalf("sweep dry-run failed: %v", err)
	}

	// Verify route still exists after dry run
	if _, err := state.ReadRoute("orphan-1"); err != nil {
		t.Errorf("route should not be deleted during dry-run")
	}

	// Real sweep
	if err := runSweep(false); err != nil {
		t.Fatalf("sweep failed: %v", err)
	}

	// Verify route was deleted
	if _, err := state.ReadRoute("orphan-1"); err == nil {
		t.Errorf("expected route to be deleted by sweep")
	}
}

