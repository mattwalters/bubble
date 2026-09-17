package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattwalters/bubble/internal/compose"
	"github.com/mattwalters/bubble/internal/ui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a new bubble.toml configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func runInit() error {
	if _, err := os.Stat("bubble.toml"); err == nil {
		return fmt.Errorf("bubble.toml already exists in current directory")
	}

	// 1. Detect compose file
	composeFiles := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}
	var foundCompose string
	for _, f := range composeFiles {
		if _, err := os.Stat(f); err == nil {
			foundCompose = f
			break
		}
	}

	var sb strings.Builder
	sb.WriteString("[bubble]\nmax = 5\n\n")

	// Detect setup (package.json)
	if _, err := os.Stat("package.json"); err == nil {
		sb.WriteString("[setup]\n")
		sb.WriteString("clone = [\"node_modules\", \"apps/*/node_modules\", \"packages/*/node_modules\"]\n")
		sb.WriteString("install = \"npm install --prefer-offline --no-audit --no-fund\"\n\n")
	}

	hasCompose := false
	if foundCompose != "" {
		hasCompose = true
		sb.WriteString("[compose]\n")
		sb.WriteString(fmt.Sprintf("file = %q\n\n", foundCompose))

		// Parse compose file
		data, err := os.ReadFile(foundCompose)
		if err == nil {
			var base compose.BaseCompose
			if err := yaml.Unmarshal(data, &base); err == nil {
				sb.WriteString("[compose.ports]\n")

				for svcName := range base.Services {
					lower := strings.ToLower(svcName)
					// Guess convention
					if strings.Contains(lower, "postgres") || strings.Contains(lower, "pg") {
						sb.WriteString(fmt.Sprintf("DATABASE_URL = \"postgresql://postgres:password@127.0.0.1:{{%s:5432}}/myapp\"\n", svcName))
					} else if strings.Contains(lower, "redis") {
						sb.WriteString(fmt.Sprintf("REDIS_URL = \"redis://127.0.0.1:{{%s:6379}}/0\"\n", svcName))
					} else if strings.Contains(lower, "mailpit") {
						sb.WriteString("SMTP_HOST = \"127.0.0.1\"\n")
						sb.WriteString(fmt.Sprintf("SMTP_PORT = \"{{%s:1025}}\"\n", svcName))
						sb.WriteString(fmt.Sprintf("MAILPIT_URL = \"http://127.0.0.1:{{%s:8025}}\"\n", svcName))
					} else if strings.Contains(lower, "mysql") {
						sb.WriteString(fmt.Sprintf("DATABASE_URL = \"mysql://root:password@127.0.0.1:{{%s:3306}}/myapp\"\n", svcName))
					} else if strings.Contains(lower, "mongo") {
						sb.WriteString(fmt.Sprintf("MONGO_URL = \"mongodb://127.0.0.1:{{%s:27017}}/myapp\"\n", svcName))
					}
				}
				sb.WriteString("\n")

				// Check healthchecks
				lacking, _ := compose.ServicesLackingHealthcheck(string(data))
				for _, l := range lacking {
					ui.PrintWarning(fmt.Sprintf("service %q lacks a healthcheck in %s. Bubble's 'up --wait' works best with healthchecks.", l, foundCompose))
				}
			}
		}
	}

	// Detect seed
	seedFile := ".env.example"
	if _, err := os.Stat(".env.example"); err != nil {
		if _, err := os.Stat(".env.sample"); err == nil {
			seedFile = ".env.sample"
		}
	}

	sb.WriteString("[env]\n")
	sb.WriteString(fmt.Sprintf("seed = %q\n", seedFile))
	sb.WriteString("AUTH_URL = \"http://{{id}}.localhost:19100\"\n\n")

	sb.WriteString("[open]\nservice = \"web\"\n\n")

	// Scaffold tiers
	sb.WriteString("[tiers.lint]\n# worktree + install only\n\n")

	if hasCompose {
		sb.WriteString("[tiers.data]\ncompose = true\n# after = [\"npm run db:migrate\"]\n\n")
		sb.WriteString("[tiers.full]\ncompose = true\n# after = [\"npm run db:migrate\"]\n# processes = [\"npm run dev\"]\n")
	}

	if err := os.WriteFile("bubble.toml", []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("writing bubble.toml: %w", err)
	}

	ui.PrintSuccess("Created bubble.toml successfully.")
	absPath, _ := filepath.Abs("bubble.toml")
	fmt.Printf("Review and edit %s to match your project needs.\n", absPath)

	return nil
}
