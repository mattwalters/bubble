package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WorktreeState holds metadata about the active bubble in the worktree.
type WorktreeState struct {
	ID              string         `json:"id"`
	Tier            string         `json:"tier"`
	Dir             string         `json:"dir"`
	CreatedAt       time.Time      `json:"created_at"`
	DiscoveredPorts map[string]int `json:"discovered_ports"` // e.g. "postgres:5432": 55123
	ComposeProject  string         `json:"compose_project"`
	Upstream        string         `json:"upstream"`
}

// WriteWorktreeState writes .bubble/state.json in the worktree.
func WriteWorktreeState(dir string, st WorktreeState) error {
	bubbleDir := WorktreeBubbleDir(dir)
	if err := os.MkdirAll(bubbleDir, 0755); err != nil {
		return fmt.Errorf("creating .bubble directory %s: %w", bubbleDir, err)
	}

	if st.CreatedAt.IsZero() {
		st.CreatedAt = time.Now().UTC()
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state for %s: %w", st.ID, err)
	}

	statePath := filepath.Join(bubbleDir, "state.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("writing state file %s: %w", statePath, err)
	}

	return nil
}

// ReadWorktreeState reads .bubble/state.json from the worktree.
func ReadWorktreeState(dir string) (*WorktreeState, error) {
	statePath := filepath.Join(WorktreeBubbleDir(dir), "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err
	}

	var st WorktreeState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parsing state %s: %w", statePath, err)
	}

	return &st, nil
}

// RemoveWorktreeBubbleDir removes the .bubble directory from the worktree.
func RemoveWorktreeBubbleDir(dir string) error {
	bubbleDir := WorktreeBubbleDir(dir)
	if err := os.RemoveAll(bubbleDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", bubbleDir, err)
	}
	return nil
}
