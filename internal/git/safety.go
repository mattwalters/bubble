package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// SafetyReport contains the status of worktree changes and unpushed commits.
type SafetyReport struct {
	HasUncommittedChanges bool
	UncommittedSummary    string
	HasUnpushedCommits    bool
	UnpushedCount         int
	Branch                string
}

// CheckSafety checks whether the worktree has uncommitted changes or unpushed commits.
func CheckSafety(dir string) (*SafetyReport, error) {
	report := &SafetyReport{}

	// 1. Check for uncommitted changes (unstaged, staged, or untracked)
	// Ignore .bubble/ directory changes just in case
	cmdStatus := exec.Command("git", "status", "--porcelain")
	cmdStatus.Dir = dir
	statusOut, err := cmdStatus.Output()
	if err != nil {
		return nil, fmt.Errorf("checking git status in %s: %w", dir, err)
	}

	lines := strings.Split(string(statusOut), "\n")
	var dirtyLines []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		// Exclude .bubble and .env if generated
		if strings.Contains(trimmed, ".bubble") || strings.Contains(trimmed, ".env") {
			continue
		}
		dirtyLines = append(dirtyLines, l)
	}

	if len(dirtyLines) > 0 {
		report.HasUncommittedChanges = true
		report.UncommittedSummary = strings.Join(dirtyLines, "\n")
	}

	// 2. Check current branch
	cmdBranch := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmdBranch.Dir = dir
	branchOut, err := cmdBranch.Output()
	if err == nil {
		report.Branch = strings.TrimSpace(string(branchOut))
	}

	// 3. Check for upstream and unpushed commits
	cmdUpstream := exec.Command("git", "rev-parse", "--abbrev-ref", "@{upstream}")
	cmdUpstream.Dir = dir
	if err := cmdUpstream.Run(); err == nil {
		// Upstream exists, check commit count ahead
		cmdRevList := exec.Command("git", "rev-list", "--count", "@{upstream}..HEAD")
		cmdRevList.Dir = dir
		countOut, err := cmdRevList.Output()
		if err == nil {
			countStr := strings.TrimSpace(string(countOut))
			count, err := strconv.Atoi(countStr)
			if err == nil && count > 0 {
				report.HasUnpushedCommits = true
				report.UnpushedCount = count
			}
		}
	}

	return report, nil
}

// FindMainRepo determines the root repository directory (handling worktrees).
func FindMainRepo(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// Fallback to toplevel
		cmdTop := exec.Command("git", "rev-parse", "--show-toplevel")
		cmdTop.Dir = dir
		topOut, errTop := cmdTop.Output()
		if errTop != nil {
			return "", fmt.Errorf("locating git repository for %s: %w", dir, err)
		}
		return strings.TrimSpace(string(topOut)), nil
	}

	commonDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Clean(filepath.Join(dir, commonDir))
	}

	// The common dir is typically "/path/to/repo/.git". Its parent is the main checkout.
	parent := filepath.Dir(commonDir)
	return parent, nil
}

// RunCommand runs an arbitrary command inside dir and returns its output.
func RunCommand(dir, command string, env []string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = env
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	return buf.String(), err
}
