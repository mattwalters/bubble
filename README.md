# Bubble

A local-first CLI tool that gives each parallel development task its own isolated set of backing services via Docker Compose. It manages port allocation, environment variable generation, service lifecycle, process supervision, and browser-accessible preview URLs with cookie isolation.

Built for multi-agent and multi-human workflows where multiple git worktrees run simultaneously on the same host without port collisions or session leakage.

## Architecture: Four Additive Layers

```
┌─────────────────────────────────────────────────────┐
│                    bubble                            │
│                                                     │
│  Layer 1: Identity + state (always)                  │
│    ID assignment · state tracking · concurrency cap   │
│                                                     │
│  Layer 2: Dependency setup (if [setup] configured)   │
│    clone dirs · install command                      │
│                                                     │
│  Layer 3: Compose services (if tier has compose)     │
│    up --wait · port discovery · env gen · down        │
│                                                     │
│  Layer 4: Host processes + proxy (if tier has procs)  │
│    start · logs · PID tracking · proxy routing · kill │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## Key Features

- **Ephemeral Port Allocation**: No hardcoded host ports or coordination tables. Docker assigns ephemeral kernel ports; Bubble discovers them and injects them into connection strings.
- **Cookie Isolation via `.localhost` Subdomains**: Each bubble is accessed at `http://<id>.localhost:19100/`. Cookies scoped to `pw-142.localhost` never leak to `pw-156.localhost`.
- **Self-Healing Doctor**: Detects port drift when Docker restarts (sleep, updates, crashes) and repairs `.env`, routes, and host processes via `bubble doctor <id> --fix`.
- **Robust Teardown**: `bubble down <id>` refuses to tear down if uncommitted changes exist (override with `--force`) and warns if branch has unpushed commits.
- **Orphan Sweeper**: `bubble sweep` enumerates Docker resources by the `bubble.managed=true` label, finding and cleaning containers even if state files were deleted.
- **Built-in Reverse Proxy**: Lightweight reverse proxy on port 19100 with WebSocket upgrade support, special dashboard at `localhost:19100/`, landing pages at `<id>.localhost:19100/_bubble/`, and helpful error pages for dead upstreams.
- **Zero-Service Capable**: For projects or tiers without backing services (e.g., lint/typecheck only), Bubble acts as a thin state and concurrency manager.

## Installation

```bash
go install github.com/mattwalters/bubble/cmd/bubble@latest
```

Or build locally:
```bash
go build -o /usr/local/bin/bubble ./cmd/bubble
```

## Configuration (`bubble.toml`)

### Full Example (Web App with Postgres, Redis, Multiple Tiers)

```toml
[bubble]
max = 5

[setup]
clone = ["node_modules", "app/*/node_modules", "lib/*/node_modules"]
install = "npm install --prefer-offline --no-audit --no-fund"

[compose]
file = "docker-compose.yml"

[compose.ports]
DATABASE_URL = "postgresql://postgres:password@127.0.0.1:{{postgres:5432}}/myapp"
REDIS_URL = "redis://127.0.0.1:{{redis:6379}}/0"
SMTP_HOST = "127.0.0.1"
SMTP_PORT = "{{mailpit:1025}}"
MAILPIT_URL = "http://127.0.0.1:{{mailpit:8025}}"

[open]
service = "web"

[env]
seed = ".env.example"
AUTH_URL = "http://{{id}}.localhost:19100"
NEXTAUTH_URL = "http://{{id}}.localhost:19100"

[tiers.lint]
# worktree + install only

[tiers.data]
compose = true
after = ["npm run db:generate", "npm run build:libs", "npm run db:deploy"]

[tiers.full]
compose = true
after = ["npm run db:generate", "npm run build:libs", "npm run db:deploy"]
processes = ["npm run dev:web", "npm run dev:worker"]
```

### Template Syntax
- `{{id}}`: Replaced with the bubble ID (e.g. `pw-142`).
- `{{service:port}}`: Replaced with Docker's allocated host port for that container port.
- `{{port}}`: Replaced with the ephemeral port allocated for the primary host process.

## Commands

```bash
# Start a bubble environment
bubble up <id> [--dir <path>] [--tier <name>]

# Tear down backing services and state
bubble down <id> [--force]

# List active bubbles and status
bubble list [--json]

# Diagnose and repair after Docker restarts
bubble doctor <id> [--fix] [--json]

# Kill and restart host processes with fresh env
bubble restart <id> [process]

# Tail or print process logs
bubble logs <id> [process] [-f]

# Open primary web service in browser
bubble open <id>

# Print generated .env
bubble env <id>

# Print discovered service ports
bubble ports <id> [--json]

# Find and clean orphaned Docker resources and stale routes
bubble sweep [--dry-run]

# Scaffold bubble.toml from existing docker-compose.yml
bubble init

# Configure macOS resolver for Safari (*.localhost)
bubble setup

# Reverse proxy management
bubble proxy start | stop | status
```

## License

MIT
