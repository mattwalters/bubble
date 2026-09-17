package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattwalters/bubble/internal/compose"
	"github.com/mattwalters/bubble/internal/config"
	"github.com/mattwalters/bubble/internal/process"
	"github.com/mattwalters/bubble/internal/proxy"
	"github.com/mattwalters/bubble/internal/state"
	"github.com/mattwalters/bubble/internal/ui"
	"github.com/spf13/cobra"
)

var (
	doctorFix  bool
	doctorJSON bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor <id>",
	Short: "Diagnose and repair a bubble after Docker restarts or crashes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		return runDoctor(id, doctorFix, doctorJSON)
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Automatically repair detected drift and restart crashed processes")
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Output diagnosis in JSON format")
}

type PortDriftInfo struct {
	Service       string `json:"service"`
	ContainerPort int    `json:"container_port"`
	RecordedHost  int    `json:"recorded_host"`
	LiveHost      int    `json:"live_host"`
}

type DoctorReport struct {
	ID              string          `json:"id"`
	WorktreeExists  bool            `json:"worktree_exists"`
	DockerRunning   bool            `json:"docker_running"`
	ContainersFound bool            `json:"containers_found"`
	PortDrifts      []PortDriftInfo `json:"port_drifts"`
	DeadProcesses   []string        `json:"dead_processes"`
	ProxyHealthy    bool            `json:"proxy_healthy"`
	RepairsApplied  []string        `json:"repairs_applied,omitempty"`
}

func runDoctor(id string, fix, jsonOutput bool) error {
	report := DoctorReport{
		ID: id,
	}

	route, err := state.ReadRoute(id)
	if err != nil {
		return fmt.Errorf("bubble %q not found: %w", id, err)
	}

	worktreeDir := route.Dir
	if _, err := os.Stat(worktreeDir); err == nil {
		report.WorktreeExists = true
	} else {
		report.WorktreeExists = false
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(report)
		}
		ui.PrintError(fmt.Sprintf("Worktree directory %s does not exist", worktreeDir))
		return nil
	}

	st, err := state.ReadWorktreeState(worktreeDir)
	if err != nil {
		return fmt.Errorf("reading worktree state for %s: %w", id, err)
	}

	// 1. Check Docker running
	if err := compose.CheckDockerRunning(); err == nil {
		report.DockerRunning = true
	} else {
		report.DockerRunning = false
	}

	// 2. Check compose ports & drift
	if report.DockerRunning && len(st.DiscoveredPorts) > 0 {
		for key, recordedPort := range st.DiscoveredPorts {
			parts := strings.Split(key, ":")
			if len(parts) == 2 {
				svc := parts[0]
				var cp int
				_, _ = fmt.Sscanf(parts[1], "%d", &cp)

				livePort, err := compose.DiscoverPort(worktreeDir, route.ComposeProject, "", "", "", svc, cp)
				if err == nil {
					report.ContainersFound = true
					if livePort != recordedPort {
						report.PortDrifts = append(report.PortDrifts, PortDriftInfo{
							Service:       svc,
							ContainerPort: cp,
							RecordedHost:  recordedPort,
							LiveHost:      livePort,
						})
					}
				}
			}
		}
	}

	// 3. Check processes
	procs, _ := process.ListProcesses(worktreeDir)
	for _, p := range procs {
		if !p.Alive {
			report.DeadProcesses = append(report.DeadProcesses, p.Name)
		}
	}

	// 4. Check proxy
	proxyPort := proxy.ConfiguredPort()
	report.ProxyHealthy = proxy.IsProxyHealthy(proxyPort)

	// If --fix requested, apply repairs
	if fix {
		// Fix port drift
		if len(report.PortDrifts) > 0 {
			// Update state discovered ports
			for _, pd := range report.PortDrifts {
				key := fmt.Sprintf("%s:%d", pd.Service, pd.ContainerPort)
				st.DiscoveredPorts[key] = pd.LiveHost
			}

			// Re-render .env
			cfgPath, err := config.FindConfigFile(worktreeDir)
			if err == nil {
				cfg, err := config.Load(cfgPath)
				if err == nil {
					// Read existing PORT if present
					currentPort := 0
					envFilePath := state.WorktreeEnvFile(worktreeDir)
					if envBytes, err := os.ReadFile(envFilePath); err == nil {
						for _, line := range strings.Split(string(envBytes), "\n") {
							if strings.HasPrefix(line, "PORT=") {
								_, _ = fmt.Sscanf(line, "PORT=%d", &currentPort)
							}
						}
					}
					if currentPort == 0 && len(cfg.Tiers[st.Tier].Processes) > 0 {
						currentPort, _ = config.AllocateEphemeralPort()
					}

					templateCtx := config.TemplateContext{
						ID:    id,
						Ports: st.DiscoveredPorts,
						Port:  currentPort,
					}

					var envLines []string
					if cfg.Env.Seed != "" {
						seedPath := cfg.Env.Seed
						if !filepath.IsAbs(seedPath) {
							seedPath = filepath.Join(filepath.Dir(cfgPath), seedPath)
						}
						if seedData, err := os.ReadFile(seedPath); err == nil {
							for _, line := range strings.Split(string(seedData), "\n") {
								trimmed := strings.TrimSpace(line)
								if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
									envLines = append(envLines, line)
								}
							}
						}
					}
					for k, v := range cfg.Compose.Ports {
						resolved := config.Resolve(v, templateCtx)
						envLines = append(envLines, fmt.Sprintf("%s=%s", k, resolved))
					}
					for k, v := range cfg.Env.Vars {
						resolved := config.Resolve(v, templateCtx)
						envLines = append(envLines, fmt.Sprintf("%s=%s", k, resolved))
					}
					if currentPort > 0 {
						envLines = append(envLines, fmt.Sprintf("PORT=%d", currentPort))
					}

					_ = os.WriteFile(envFilePath, []byte(strings.Join(envLines, "\n")+"\n"), 0644)
					report.RepairsApplied = append(report.RepairsApplied, "rewrote .env with live ports")

					// Update route file upstream if needed
					if currentPort > 0 {
						route.Upstream = fmt.Sprintf("127.0.0.1:%d", currentPort)
						_ = state.WriteRoute(*route)
						st.Upstream = route.Upstream
						report.RepairsApplied = append(report.RepairsApplied, "updated route upstream")
					}

					_ = state.WriteWorktreeState(worktreeDir, *st)

					// Restart host processes (they hold stale env in memory)
					tier := cfg.Tiers[st.Tier]
					if len(tier.Processes) > 0 {
						_ = process.KillAllProcesses(worktreeDir)
						envSlice := append(os.Environ(), envLines...)
						for _, proc := range tier.Processes {
							_, _ = process.StartProcess(worktreeDir, proc.Name, proc.Command, envSlice)
						}
						report.RepairsApplied = append(report.RepairsApplied, "restarted host processes with fresh env")
					}
				}
			}
		}

		// Restart dead processes if no port drift restart already occurred
		if len(report.DeadProcesses) > 0 && len(report.PortDrifts) == 0 {
			cfgPath, err := config.FindConfigFile(worktreeDir)
			if err == nil {
				cfg, err := config.Load(cfgPath)
				if err == nil {
					tier := cfg.Tiers[st.Tier]
					envFilePath := state.WorktreeEnvFile(worktreeDir)
					envBytes, _ := os.ReadFile(envFilePath)
					envSlice := append(os.Environ(), strings.Split(string(envBytes), "\n")...)

					for _, proc := range tier.Processes {
						for _, deadName := range report.DeadProcesses {
							if proc.Name == deadName {
								_, _ = process.StartProcess(worktreeDir, proc.Name, proc.Command, envSlice)
								report.RepairsApplied = append(report.RepairsApplied, fmt.Sprintf("restarted dead process %s", proc.Name))
							}
						}
					}
				}
			}
		}

		// Restart proxy if not healthy
		if !report.ProxyHealthy {
			if err := proxy.StartDaemon(proxyPort); err == nil {
				report.RepairsApplied = append(report.RepairsApplied, "restarted proxy daemon")
			}
		}
	}

	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(report)
	}

	// Formatted human output
	fmt.Printf("Diagnosing bubble %s:\n", id)
	if !report.DockerRunning {
		ui.PrintError("Docker is not running. Start Docker Desktop and try again.")
		return nil
	}

	if len(report.PortDrifts) > 0 {
		var driftStrs []string
		for _, pd := range report.PortDrifts {
			driftStrs = append(driftStrs, fmt.Sprintf("%s %d → %d", pd.Service, pd.RecordedHost, pd.LiveHost))
		}
		ui.PrintStep("ports", strings.Join(driftStrs, ", "))
	} else {
		ui.PrintStep("ports", "all live ports match recorded state ✓")
	}

	if len(report.DeadProcesses) > 0 {
		ui.PrintWarning(fmt.Sprintf("dead process(es): %s", strings.Join(report.DeadProcesses, ", ")))
	}

	if !report.ProxyHealthy {
		ui.PrintWarning("proxy is not running or not responding")
	}

	if fix {
		if len(report.RepairsApplied) > 0 {
			ui.PrintSuccess("Repairs applied:")
			for _, rep := range report.RepairsApplied {
				fmt.Printf("  • %s\n", rep)
			}
		} else {
			ui.PrintSuccess("No repairs needed. Bubble is healthy.")
		}
	} else if len(report.PortDrifts) > 0 || len(report.DeadProcesses) > 0 || !report.ProxyHealthy {
		fmt.Printf("\nRun 'bubble doctor %s --fix' to automatically resolve these issues.\n", id)
	}

	return nil
}
