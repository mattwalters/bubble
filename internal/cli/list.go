package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mattwalters/bubble/internal/compose"
	"github.com/mattwalters/bubble/internal/process"
	"github.com/mattwalters/bubble/internal/proxy"
	"github.com/mattwalters/bubble/internal/state"
	"github.com/mattwalters/bubble/internal/ui"
	"github.com/spf13/cobra"
)

var listJSON bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active bubbles with status and health",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList(listJSON)
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output in JSON format")
}

type BubbleStatusItem struct {
	ID       string `json:"id"`
	Tier     string `json:"tier"`
	URL      string `json:"url"`
	Services string `json:"services"`
	Status   string `json:"status"`
	Dir      string `json:"dir"`
}

func runList(jsonOutput bool) error {
	routes, err := state.ListRoutes()
	if err != nil {
		return fmt.Errorf("listing routes: %w", err)
	}

	proxyPort := proxy.ConfiguredPort()
	var items []BubbleStatusItem

	for _, r := range routes {
		item := BubbleStatusItem{
			ID:     r.ID,
			Tier:   "-",
			URL:    fmt.Sprintf("http://%s.localhost:%d", r.ID, proxyPort),
			Status: "ok",
			Dir:    r.Dir,
		}

		// Check worktree dir
		if _, err := os.Stat(r.Dir); os.IsNotExist(err) {
			item.Status = "worktree missing — run: bubble sweep"
			items = append(items, item)
			continue
		}

		// Read worktree state
		st, err := state.ReadWorktreeState(r.Dir)
		if err != nil {
			item.Status = "state missing"
			items = append(items, item)
			continue
		}

		item.Tier = st.Tier

		// Collect services
		serviceSet := make(map[string]bool)
		for key := range st.DiscoveredPorts {
			parts := strings.Split(key, ":")
			if len(parts) > 0 {
				serviceSet[parts[0]] = true
			}
		}
		var svcs []string
		for s := range serviceSet {
			svcs = append(svcs, s)
		}
		sort.Strings(svcs)
		item.Services = strings.Join(svcs, " ")
		if item.Services == "" {
			item.Services = "-"
		}

		// Quick staleness check: verify recorded ports vs live containers
		if len(st.DiscoveredPorts) > 0 {
			drift := false
			// Check one or two ports for speed
			for key, recordedPort := range st.DiscoveredPorts {
				parts := strings.Split(key, ":")
				if len(parts) == 2 {
					svc := parts[0]
					var cp int
					_, _ = fmt.Sscanf(parts[1], "%d", &cp)
					livePort, err := compose.DiscoverPort(r.Dir, r.ComposeProject, "", "", "", svc, cp)
					if err != nil || livePort != recordedPort {
						drift = true
						break
					}
				}
			}
			if drift {
				item.Status = fmt.Sprintf("port drift — run: bubble doctor %s --fix", r.ID)
			}
		}

		// Quick PID check: verify recorded processes
		if item.Status == "ok" {
			procs, _ := process.ListProcesses(r.Dir)
			for _, p := range procs {
				if !p.Alive {
					item.Status = fmt.Sprintf("process %s dead — run: bubble restart %s", p.Name, r.ID)
					break
				}
			}
		}

		items = append(items, item)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}

	if len(items) == 0 {
		fmt.Println("No active bubbles found. Start one with: bubble up <id>")
		return nil
	}

	headers := []string{"ID", "TIER", "URL", "SERVICES", "STATUS"}
	var rows [][]string
	for _, it := range items {
		rows = append(rows, []string{it.ID, it.Tier, it.URL, it.Services, it.Status})
	}

	ui.RenderTable(os.Stdout, headers, rows)
	return nil
}
