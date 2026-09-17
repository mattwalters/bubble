package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/mattwalters/bubble/internal/state"
	"github.com/mattwalters/bubble/internal/ui"
	"github.com/spf13/cobra"
)

var portsJSON bool

var portsCmd = &cobra.Command{
	Use:   "ports <id>",
	Short: "Print service to host port mappings for a bubble",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		return runPorts(id, portsJSON)
	},
}

func init() {
	portsCmd.Flags().BoolVar(&portsJSON, "json", false, "Output in JSON format")
}

func runPorts(id string, jsonOutput bool) error {
	route, err := state.ReadRoute(id)
	if err != nil {
		return fmt.Errorf("bubble %q not found: %w", id, err)
	}

	st, err := state.ReadWorktreeState(route.Dir)
	if err != nil {
		return fmt.Errorf("reading state for %s: %w", id, err)
	}

	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(st.DiscoveredPorts)
	}

	if len(st.DiscoveredPorts) == 0 {
		fmt.Println("No ports discovered for this bubble.")
		return nil
	}

	var keys []string
	for k := range st.DiscoveredPorts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	headers := []string{"SERVICE:PORT", "HOST PORT", "DIRECT URL"}
	var rows [][]string
	for _, k := range keys {
		hp := st.DiscoveredPorts[k]
		rows = append(rows, []string{k, strconv.Itoa(hp), fmt.Sprintf("http://127.0.0.1:%d", hp)})
	}

	ui.RenderTable(os.Stdout, headers, rows)
	return nil
}
