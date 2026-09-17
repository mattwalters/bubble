package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// CloneDirectory copies a source directory or file to destDir using APFS clonefile on macOS or reflink on Linux.
func CloneDirectory(src, dest string) error {
	destParent := filepath.Dir(dest)
	if err := os.MkdirAll(destParent, 0755); err != nil {
		return fmt.Errorf("creating parent directory %s: %w", destParent, err)
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		// macOS: cp -Rc (APFS clonefile)
		// Note: to copy into destination, remove existing dest first if present
		_ = os.RemoveAll(dest)
		cmd = exec.Command("cp", "-Rc", src, dest)
	} else {
		// Linux: try cp -r --reflink=auto
		_ = os.RemoveAll(dest)
		cmd = exec.Command("cp", "-r", "--reflink=auto", src, dest)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		// Fallback to standard recursive copy if --reflink fails
		fallbackCmd := exec.Command("cp", "-R", src, dest)
		if fallbackOut, fallbackErr := fallbackCmd.CombinedOutput(); fallbackErr != nil {
			return fmt.Errorf("copying %s to %s: %s (fallback error: %s)", src, dest, string(out), string(fallbackOut))
		}
	}

	return nil
}

// CloneSetupDirectories clones all matching patterns from mainRepo to worktreeDir.
func CloneSetupDirectories(mainRepo, worktreeDir string, patterns []string) ([]string, error) {
	var cloned []string

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(mainRepo, pattern))
		if err != nil {
			continue
		}

		for _, match := range matches {
			rel, err := filepath.Rel(mainRepo, match)
			if err != nil {
				continue
			}

			dest := filepath.Join(worktreeDir, rel)
			if err := CloneDirectory(match, dest); err != nil {
				return cloned, fmt.Errorf("cloning %s: %w", rel, err)
			}
			cloned = append(cloned, rel)
		}
	}

	return cloned, nil
}
