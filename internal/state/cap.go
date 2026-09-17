package state

import (
	"fmt"
)

// CheckConcurrencyCap verifies that creating or activating bubbleID does not exceed maxBubbles.
// If the bubbleID already has an active route file, it does not count as a new bubble.
func CheckConcurrencyCap(bubbleID string, maxBubbles int) error {
	if maxBubbles <= 0 {
		maxBubbles = 5
	}

	routes, err := ListRoutes()
	if err != nil {
		return fmt.Errorf("checking active bubbles: %w", err)
	}

	activeCount := 0
	alreadyActive := false

	for _, r := range routes {
		if r.ID == bubbleID {
			alreadyActive = true
		}
		activeCount++
	}

	if !alreadyActive && activeCount >= maxBubbles {
		return fmt.Errorf("concurrency cap reached: %d active bubbles (max: %d). Run 'bubble down <id>' on an unused bubble, or increase [bubble].max in bubble.toml", activeCount, maxBubbles)
	}

	return nil
}
