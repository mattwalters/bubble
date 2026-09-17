package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// ProcessDef represents a configured host process with a name and command.
type ProcessDef struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

// TierConfig defines configuration for a tier.
type TierConfig struct {
	Compose   bool         `toml:"compose"`
	After     []string     `toml:"after"`
	Processes []ProcessDef `toml:"-"`
}

// BubbleConfig contains general bubble settings.
type BubbleConfig struct {
	Max int `toml:"max"`
}

// SetupConfig contains worktree preparation commands.
type SetupConfig struct {
	Clone   []string `toml:"clone"`
	Install string   `toml:"install"`
}

// ComposeConfig contains docker-compose settings.
type ComposeConfig struct {
	File     string                       `toml:"file"`
	Services []string                     `toml:"services"`
	Ports    map[string]string            `toml:"ports"`
	Env      map[string]map[string]string `toml:"env"`
}

// OpenConfig contains browser open settings.
type OpenConfig struct {
	Service string `toml:"service"`
}

// EnvConfig holds seed and extra environment variable definitions.
type EnvConfig struct {
	Seed string            `toml:"seed"`
	Vars map[string]string `toml:"-"`
}

// Config is the root configuration parsed from bubble.toml.
type Config struct {
	Bubble    BubbleConfig          `toml:"bubble"`
	Setup     SetupConfig           `toml:"setup"`
	Compose   ComposeConfig         `toml:"compose"`
	Open      OpenConfig            `toml:"open"`
	Env       EnvConfig             `toml:"env"`
	Tiers     map[string]TierConfig `toml:"-"`
	TierOrder []string              `toml:"-"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Bubble: BubbleConfig{
			Max: 5,
		},
		Compose: ComposeConfig{
			File:  "docker-compose.yml",
			Ports: make(map[string]string),
			Env:   make(map[string]map[string]string),
		},
		Env: EnvConfig{
			Vars: make(map[string]string),
		},
		Tiers:     make(map[string]TierConfig),
		TierOrder: make([]string, 0),
	}
}

var tierHeaderRegex = regexp.MustCompile(`^\s*\[tiers\.([a-zA-Z0-9_\-]+)\]`)

// Load reads and parses bubble.toml from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	cfg := DefaultConfig()

	// 1. Preserve tier declaration order
	cfg.TierOrder = extractTierOrder(string(data))

	// 2. Decode raw map to inspect sections
	var rawMap map[string]any
	if err := toml.Unmarshal(data, &rawMap); err != nil {
		return nil, fmt.Errorf("parsing TOML %s: %w", path, err)
	}

	// Unmarshal standard fields
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config %s: %w", path, err)
	}

	// Extract [env] variables beyond seed
	if rawEnv, ok := rawMap["env"].(map[string]any); ok {
		for k, v := range rawEnv {
			if k == "seed" {
				continue
			}
			if s, ok := v.(string); ok {
				cfg.Env.Vars[k] = s
			} else {
				cfg.Env.Vars[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	// Extract [tiers]
	if rawTiers, ok := rawMap["tiers"].(map[string]any); ok {
		for tierName, rawTierVal := range rawTiers {
			tierMap, ok := rawTierVal.(map[string]any)
			if !ok {
				continue
			}

			var tc TierConfig
			if comp, ok := tierMap["compose"].(bool); ok {
				tc.Compose = comp
			}
			if afters, ok := tierMap["after"].([]any); ok {
				for _, a := range afters {
					if s, ok := a.(string); ok {
						tc.After = append(tc.After, s)
					}
				}
			}

			// Parse processes: can be []any (list of strings) or map[string]any
			if procVal, exists := tierMap["processes"]; exists {
				tc.Processes = parseProcesses(procVal)
			}

			cfg.Tiers[tierName] = tc
		}
	}

	// Set default compose file if empty
	if cfg.Compose.File == "" {
		cfg.Compose.File = "docker-compose.yml"
	}

	// Set default bubble max if <= 0
	if cfg.Bubble.Max <= 0 {
		cfg.Bubble.Max = 5
	}

	return cfg, nil
}

func extractTierOrder(content string) []string {
	var order []string
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		matches := tierHeaderRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			tierName := matches[1]
			if !seen[tierName] {
				seen[tierName] = true
				order = append(order, tierName)
			}
		}
	}
	return order
}

// parseProcesses handles both array of command strings and map of name -> command.
func parseProcesses(val any) []ProcessDef {
	var procs []ProcessDef

	switch v := val.(type) {
	case []any:
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				continue
			}
			name := deriveProcessName(str, i+1)
			procs = append(procs, ProcessDef{
				Name:    name,
				Command: str,
			})
		}
	case map[string]any:
		for name, cmdVal := range v {
			if cmdStr, ok := cmdVal.(string); ok {
				procs = append(procs, ProcessDef{
					Name:    name,
					Command: cmdStr,
				})
			}
		}
	}

	return procs
}

// deriveProcessName derives a clean process identifier from a command string.
// Examples:
// "web: echo started" -> "web"
// "web=npm run dev" -> "web"
// "npm run dev:web" -> "web"
// "npm run dev" -> "dev"
// "go run ./cmd/server" -> "server"
func deriveProcessName(cmd string, fallbackIndex int) string {
	trimmed := strings.TrimSpace(cmd)

	// Format: "name: command" or "name=command"
	if idx := strings.Index(trimmed, ": "); idx > 0 && !strings.Contains(trimmed[:idx], " ") {
		return strings.TrimSpace(trimmed[:idx])
	}
	if idx := strings.Index(trimmed, "="); idx > 0 && !strings.Contains(trimmed[:idx], " ") {
		return strings.TrimSpace(trimmed[:idx])
	}

	// Common process names to look for
	lower := strings.ToLower(trimmed)
	for _, common := range []string{"web", "worker", "api", "server", "frontend", "backend", "client", "daemon"} {
		if strings.Contains(lower, common) {
			return common
		}
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return fmt.Sprintf("proc%d", fallbackIndex)
	}

	// Walk backwards looking for a non-numeric token
	for i := len(fields) - 1; i >= 0; i-- {
		token := fields[i]
		if _, err := strconv.Atoi(token); err == nil {
			continue // Skip numbers like sleep 10
		}
		if token == "&&" || token == "||" || token == ";" {
			continue
		}

		// If it contains colon (e.g. dev:web)
		if idx := strings.LastIndex(token, ":"); idx >= 0 && idx < len(token)-1 {
			return token[idx+1:]
		}
		// If it contains slash (e.g. ./cmd/web)
		if idx := strings.LastIndex(token, "/"); idx >= 0 && idx < len(token)-1 {
			return token[idx+1:]
		}

		clean := strings.Trim(token, "-_./\\")
		if clean != "" {
			return clean
		}
	}

	return fmt.Sprintf("proc%d", fallbackIndex)
}

// FindConfigFile walks up from the starting directory until it finds bubble.toml,
// stopping at the git root or filesystem root.
func FindConfigFile(startDir string) (string, error) {
	curr, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		target := filepath.Join(curr, "bubble.toml")
		if _, err := os.Stat(target); err == nil {
			return target, nil
		}

		gitDir := filepath.Join(curr, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			// At repo root; if bubble.toml is not here, stop
			return "", fmt.Errorf("bubble.toml not found in repository (checked up to %s)", curr)
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	return "", fmt.Errorf("bubble.toml not found")
}
