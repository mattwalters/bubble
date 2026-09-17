package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitSafetyAndCloning(t *testing.T) {
	// Create a temporary git repo
	tmpRepo := t.TempDir()

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpRepo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s", args, string(out))
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")

	// Commit an initial file
	fPath := filepath.Join(tmpRepo, "initial.txt")
	if err := os.WriteFile(fPath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "initial.txt")
	runGit("commit", "-m", "initial commit")

	// 1. Check safety on clean repo
	safety, err := CheckSafety(tmpRepo)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}
	if safety.HasUncommittedChanges {
		t.Errorf("expected clean repo, got uncommitted: %s", safety.UncommittedSummary)
	}

	// 2. Add an untracked file -> dirty
	untracked := filepath.Join(tmpRepo, "untracked.txt")
	if err := os.WriteFile(untracked, []byte("dirt"), 0644); err != nil {
		t.Fatal(err)
	}

	safety, err = CheckSafety(tmpRepo)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}
	if !safety.HasUncommittedChanges {
		t.Errorf("expected dirty status for untracked file")
	}

	// Remove untracked
	_ = os.Remove(untracked)

	// 3. Test CloneDirectory
	srcDir := filepath.Join(tmpRepo, "node_modules")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	destWorktree := t.TempDir()
	cloned, err := CloneSetupDirectories(tmpRepo, destWorktree, []string{"node_modules"})
	if err != nil {
		t.Fatalf("CloneSetupDirectories failed: %v", err)
	}
	if len(cloned) != 1 || cloned[0] != "node_modules" {
		t.Errorf("cloned unexpected: %v", cloned)
	}
	if _, err := os.Stat(filepath.Join(destWorktree, "node_modules", "package.json")); err != nil {
		t.Errorf("expected cloned package.json to exist: %v", err)
	}
}
