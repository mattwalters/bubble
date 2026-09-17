package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateAndRouteLifecycle(t *testing.T) {
	tmpGlobal := t.TempDir()
	t.Setenv("BUBBLE_HOME", tmpGlobal)

	tmpWorktree := t.TempDir()

	// Verify paths
	if GlobalDir() != tmpGlobal {
		t.Errorf("expected GlobalDir %s, got %s", tmpGlobal, GlobalDir())
	}

	// 1. Concurrency cap check initially
	if err := CheckConcurrencyCap("b1", 2); err != nil {
		t.Fatalf("concurrency cap check failed unexpectedly: %v", err)
	}

	// 2. Write route
	r1 := Route{
		ID:             "b1",
		Upstream:       "127.0.0.1:3000",
		Dir:            tmpWorktree,
		ComposeProject: "b1",
		CreatedAt:      time.Now().UTC(),
	}
	if err := WriteRoute(r1); err != nil {
		t.Fatalf("WriteRoute failed: %v", err)
	}

	routes, err := ListRoutes()
	if err != nil || len(routes) != 1 {
		t.Fatalf("ListRoutes expected 1, got %d, err: %v", len(routes), err)
	}

	readR1, err := ReadRoute("b1")
	if err != nil || readR1.Upstream != "127.0.0.1:3000" {
		t.Fatalf("ReadRoute failed: %v", err)
	}

	// Re-checking cap for b1 should pass even if max=1
	if err := CheckConcurrencyCap("b1", 1); err != nil {
		t.Fatalf("re-checking b1 should pass cap: %v", err)
	}

	// Check cap for b2 with max=1 should fail
	if err := CheckConcurrencyCap("b2", 1); err == nil {
		t.Fatalf("expected cap error for b2 when max=1")
	}

	// 3. Write Worktree State
	st := WorktreeState{
		ID:   "b1",
		Tier: "data",
		Dir:  tmpWorktree,
		DiscoveredPorts: map[string]int{
			"postgres:5432": 55123,
		},
		ComposeProject: "b1",
		Upstream:       "127.0.0.1:3000",
	}
	if err := WriteWorktreeState(tmpWorktree, st); err != nil {
		t.Fatalf("WriteWorktreeState failed: %v", err)
	}

	readSt, err := ReadWorktreeState(tmpWorktree)
	if err != nil || readSt.DiscoveredPorts["postgres:5432"] != 55123 {
		t.Fatalf("ReadWorktreeState failed: %v", err)
	}

	// 4. Cleanup
	if err := DeleteRoute("b1"); err != nil {
		t.Fatalf("DeleteRoute failed: %v", err)
	}
	routes, _ = ListRoutes()
	if len(routes) != 0 {
		t.Fatalf("expected 0 routes after delete")
	}

	if err := RemoveWorktreeBubbleDir(tmpWorktree); err != nil {
		t.Fatalf("RemoveWorktreeBubbleDir failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpWorktree, ".bubble")); !os.IsNotExist(err) {
		t.Fatalf("expected .bubble to be deleted")
	}
}
