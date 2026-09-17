package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mattwalters/bubble/internal/compose"
	"github.com/mattwalters/bubble/internal/config"
	"github.com/mattwalters/bubble/internal/git"
	"github.com/mattwalters/bubble/internal/process"
	"github.com/mattwalters/bubble/internal/proxy"
	"github.com/mattwalters/bubble/internal/state"
	"github.com/mattwalters/bubble/internal/ui"
	"github.com/spf13/cobra"
)

var (
	upDir  string
	upTier string
)

var upCmd = &cobra.Command{
	Use:   "up <id>",
	Short: "Start backing services and optional host processes for a bubble",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		return runUp(id, upDir, upTier)
	},
}

func init() {
	upCmd.Flags().StringVar(&upDir, "dir", ".", "Worktree directory to operate on")
	upCmd.Flags().StringVar(&upTier, "tier", "", "Tier name to launch (defaults to first defined)")
}

func runUp(id, worktreeDir, tierName string) error {
	absDir, err := filepath.Abs(worktreeDir)
	if err != nil {
		return fmt.Errorf("resolving dir %s: %w", worktreeDir, err)
	}

	// 1. Locate bubble.toml
	cfgPath, err := config.FindConfigFile(absDir)
	if err != nil {
		return fmt.Errorf("locating configuration: %w", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading %s: %w", cfgPath, err)
	}

	// 2. Check concurrency cap
	if err := state.CheckConcurrencyCap(id, cfg.Bubble.Max); err != nil {
		return err
	}

	// 3. If bubble already exists: clean -> tear down and recreate; uncommitted changes -> refuse
	if existingRoute, _ := state.ReadRoute(id); existingRoute != nil {
		safety, err := git.CheckSafety(absDir)
		if err == nil && safety.HasUncommittedChanges {
			return fmt.Errorf("cannot recreate bubble %q: worktree has uncommitted changes. Commit or stash them first", id)
		}
		// Tear down existing resources cleanly
		_ = process.KillAllProcesses(absDir)
		_ = compose.Down(id, absDir)
	}

	// 4. Run [setup].clone
	mainRepo, err := git.FindMainRepo(absDir)
	if err == nil && mainRepo != absDir && len(cfg.Setup.Clone) > 0 {
		cloned, err := git.CloneSetupDirectories(mainRepo, absDir, cfg.Setup.Clone)
		if err != nil {
			return fmt.Errorf("setup clone: %w", err)
		}
		if len(cloned) > 0 {
			ui.PrintStep("setup", fmt.Sprintf("cloning %s... done (APFS clonefile)", strings.Join(cloned, ", ")))
		}
	}

	// 5. Run [setup].install
	if cfg.Setup.Install != "" {
		ui.PrintStep("setup", fmt.Sprintf("%s...", cfg.Setup.Install))
		out, err := git.RunCommand(absDir, cfg.Setup.Install, os.Environ())
		if err != nil {
			return fmt.Errorf("setup install failed: %w\n%s", err, out)
		}
		ui.PrintStep("setup", fmt.Sprintf("%s... done", cfg.Setup.Install))
	}

	// 6. Resolve tier
	if tierName == "" {
		if len(cfg.TierOrder) > 0 {
			tierName = cfg.TierOrder[0]
		} else {
			tierName = "default"
		}
	}

	tier, exists := cfg.Tiers[tierName]
	if !exists {
		// If user defined no tiers, use empty tier
		if len(cfg.Tiers) == 0 {
			tier = config.TierConfig{}
		} else {
			var available []string
			for k := range cfg.Tiers {
				available = append(available, k)
			}
			return fmt.Errorf("tier %q not found in bubble.toml. Available tiers: %s", tierName, strings.Join(available, ", "))
		}
	}

	discoveredPorts := make(map[string]int)
	bubbleDir := state.WorktreeBubbleDir(absDir)
	_ = os.MkdirAll(bubbleDir, 0755)

	// 7. If tier has compose = true
	if tier.Compose {
		if err := compose.CheckDockerRunning(); err != nil {
			return err
		}

		ui.PrintStep("compose", fmt.Sprintf("starting %s... waiting for healthy", id))

		// 7a. Write env file
		envFile := filepath.Join(bubbleDir, "compose.env")
		envContent := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s\nBUBBLE_ID=%s\n", id, id)
		if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
			return fmt.Errorf("writing compose env file: %w", err)
		}

		// 7b. Generate compose override
		baseComposePath := cfg.Compose.File
		if !filepath.IsAbs(baseComposePath) {
			baseComposePath = filepath.Join(filepath.Dir(cfgPath), baseComposePath)
		}
		baseData, err := os.ReadFile(baseComposePath)
		if err != nil {
			return fmt.Errorf("reading base compose file %s: %w", baseComposePath, err)
		}

		overrideContent, err := compose.GenerateOverrideYAML(string(baseData), id, cfg.Compose.Env)
		if err != nil {
			return fmt.Errorf("generating compose override: %w", err)
		}
		overrideFile := state.WorktreeOverrideFile(absDir)
		if err := compose.WriteOverrideFile(overrideFile, overrideContent); err != nil {
			return fmt.Errorf("writing compose override %s: %w", overrideFile, err)
		}

		reqs := cfg.ExtractRequiredPorts()
		var targetServices []string
		if len(cfg.Compose.Services) > 0 {
			targetServices = cfg.Compose.Services
		} else if len(reqs) > 0 {
			svcSet := make(map[string]bool)
			for _, r := range reqs {
				svcSet[r.Service] = true
			}
			for s := range svcSet {
				targetServices = append(targetServices, s)
			}
			sort.Strings(targetServices)
		}

		// 7c. docker compose up -d --wait
		composeArgs := []string{
			"-p", id,
			"-f", baseComposePath,
			"-f", overrideFile,
			"--env-file", envFile,
			"up", "-d", "--wait",
		}
		composeArgs = append(composeArgs, targetServices...)

		start := time.Now()
		if _, err := compose.RunCompose(absDir, composeArgs...); err != nil {
			return fmt.Errorf("docker compose up: %w", err)
		}

		// Fallback TCP wait loop for services without healthchecks
		lacking, _ := compose.ServicesLackingHealthcheck(string(baseData))

		// 7d. Discover host ports
		ports, err := compose.DiscoverAllPorts(absDir, id, baseComposePath, overrideFile, envFile, reqs)
		if err != nil {
			return fmt.Errorf("discovering ports: %w", err)
		}
		discoveredPorts = ports

		// Run fallback TCP connect check on services lacking healthchecks
		var fallbackTCPPorts []int
		for _, req := range reqs {
			for _, l := range lacking {
				if req.Service == l {
					key := fmt.Sprintf("%s:%d", req.Service, req.ContainerPort)
					if hp, ok := ports[key]; ok {
						fallbackTCPPorts = append(fallbackTCPPorts, hp)
					}
				}
			}
		}
		if len(fallbackTCPPorts) > 0 {
			_ = compose.WaitForTCPPorts(fallbackTCPPorts, 15*time.Second)
		}

		elapsed := time.Since(start).Round(100 * time.Millisecond)
		ui.PrintStep("compose", fmt.Sprintf("services ready (%s)", elapsed))

		// Display discovered ports
		for key, hp := range discoveredPorts {
			ui.PrintSubStep(fmt.Sprintf("%-10s → 127.0.0.1:%d", key, hp))
		}
	}

	// Ephemeral port for host primary web process
	allocPort := 0
	if len(tier.Processes) > 0 {
		p, err := config.AllocateEphemeralPort()
		if err != nil {
			return fmt.Errorf("allocating host process port: %w", err)
		}
		allocPort = p
	}

	// 8. Write .env
	templateCtx := config.TemplateContext{
		ID:    id,
		Ports: discoveredPorts,
		Port:  allocPort,
	}

	var envLines []string
	if cfg.Env.Seed != "" {
		seedPath := cfg.Env.Seed
		if !filepath.IsAbs(seedPath) {
			seedPath = filepath.Join(filepath.Dir(cfgPath), seedPath)
		}
		if seedData, err := os.ReadFile(seedPath); err == nil {
			lines := strings.Split(string(seedData), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
					envLines = append(envLines, line)
				}
			}
		}
	}

	// Append compose port vars
	for k, v := range cfg.Compose.Ports {
		resolved := config.Resolve(v, templateCtx)
		envLines = append(envLines, fmt.Sprintf("%s=%s", k, resolved))
	}

	// Append env vars
	for k, v := range cfg.Env.Vars {
		resolved := config.Resolve(v, templateCtx)
		envLines = append(envLines, fmt.Sprintf("%s=%s", k, resolved))
	}

	if allocPort > 0 {
		envLines = append(envLines, fmt.Sprintf("PORT=%d", allocPort))
	}

	envFilePath := state.WorktreeEnvFile(absDir)
	if err := os.WriteFile(envFilePath, []byte(strings.Join(envLines, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("writing .env file %s: %w", envFilePath, err)
	}
	seedDesc := ".env.example"
	if cfg.Env.Seed != "" {
		seedDesc = cfg.Env.Seed
	}
	ui.PrintStep("env", fmt.Sprintf(".env written (seed: %s + bubble vars)", seedDesc))

	// Construct environment for hooks and processes
	envSlice := os.Environ()
	envSlice = append(envSlice, envLines...)

	// 9. Run tier's after hooks
	for _, hook := range tier.After {
		ui.PrintStep("after", fmt.Sprintf("%s...", hook))
		out, err := git.RunCommand(absDir, hook, envSlice)
		if err != nil {
			return fmt.Errorf("after hook %q failed: %w\n%s", hook, err, out)
		}
		ui.PrintStep("after", fmt.Sprintf("%s... done", hook))
	}

	// 10. Start tier processes
	for _, proc := range tier.Processes {
		procEnv := envSlice
		pid, err := process.StartProcess(absDir, proc.Name, proc.Command, procEnv)
		if err != nil {
			return fmt.Errorf("starting process %s: %w", proc.Name, err)
		}
		ui.PrintStep("process", fmt.Sprintf("%s started (PID %d)", proc.Name, pid))
	}

	// 11. Write route file
	upstream := ""
	if allocPort > 0 {
		upstream = fmt.Sprintf("127.0.0.1:%d", allocPort)
	} else if cfg.Open.Service != "" {
		// If open service is a compose service
		for key, hp := range discoveredPorts {
			if strings.HasPrefix(key, cfg.Open.Service+":") {
				upstream = fmt.Sprintf("127.0.0.1:%d", hp)
				break
			}
		}
	}

	route := state.Route{
		ID:             id,
		Upstream:       upstream,
		Dir:            absDir,
		ComposeProject: id,
		CreatedAt:      time.Now().UTC(),
	}
	if err := state.WriteRoute(route); err != nil {
		return fmt.Errorf("writing route file: %w", err)
	}

	// 12. Start proxy if not running
	proxyPort := proxy.ConfiguredPort()
	if err := proxy.StartDaemon(proxyPort); err != nil {
		ui.PrintWarning(fmt.Sprintf("could not auto-start proxy: %v", err))
	}
	ui.PrintStep("proxy", fmt.Sprintf("http://%s.localhost:%d", id, proxyPort))

	// 13. Write .bubble/state.json
	st := state.WorktreeState{
		ID:              id,
		Tier:            tierName,
		Dir:             absDir,
		CreatedAt:       time.Now().UTC(),
		DiscoveredPorts: discoveredPorts,
		ComposeProject:  id,
		Upstream:        upstream,
	}
	if err := state.WriteWorktreeState(absDir, st); err != nil {
		return fmt.Errorf("writing worktree state: %w", err)
	}

	return nil
}
