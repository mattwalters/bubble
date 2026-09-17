package config

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
)

var (
	idRegex          = regexp.MustCompile(`\{\{id\}\}`)
	servicePortRegex = regexp.MustCompile(`\{\{([a-zA-Z0-9_\-]+):([0-9]+)\}\}`)
	portRegex        = regexp.MustCompile(`\{\{port\}\}`)
)

// ServicePortRequirement represents a service and container port extracted from a template.
type ServicePortRequirement struct {
	Service       string
	ContainerPort int
}

// ExtractRequiredPorts scans all values in compose.ports, compose.env, and env for {{service:port}} templates.
func (c *Config) ExtractRequiredPorts() []ServicePortRequirement {
	var reqs []ServicePortRequirement
	seen := make(map[string]bool)

	add := func(val string) {
		matches := servicePortRegex.FindAllStringSubmatch(val, -1)
		for _, m := range matches {
			if len(m) == 3 {
				key := m[1] + ":" + m[2]
				if !seen[key] {
					seen[key] = true
					cp, _ := strconv.Atoi(m[2])
					reqs = append(reqs, ServicePortRequirement{
						Service:       m[1],
						ContainerPort: cp,
					})
				}
			}
		}
	}

	for _, v := range c.Compose.Ports {
		add(v)
	}
	for _, envMap := range c.Compose.Env {
		for _, v := range envMap {
			add(v)
		}
	}
	for _, v := range c.Env.Vars {
		add(v)
	}

	return reqs
}

// TemplateContext holds values used to resolve templates.
type TemplateContext struct {
	ID    string
	Ports map[string]int // key: "service:containerPort", value: hostPort
	Port  int            // host process port
}

// Resolve replaces {{id}}, {{service:port}}, and {{port}} templates in a string.
func Resolve(input string, ctx TemplateContext) string {
	res := idRegex.ReplaceAllString(input, ctx.ID)

	res = servicePortRegex.ReplaceAllStringFunc(res, func(match string) string {
		sub := servicePortRegex.FindStringSubmatch(match)
		if len(sub) == 3 {
			key := fmt.Sprintf("%s:%s", sub[1], sub[2])
			if hp, ok := ctx.Ports[key]; ok {
				return strconv.Itoa(hp)
			}
		}
		return match
	})

	if ctx.Port > 0 {
		res = portRegex.ReplaceAllString(res, strconv.Itoa(ctx.Port))
	}

	return res
}

// AllocateEphemeralPort finds and temporarily reserves an available TCP port on 127.0.0.1.
// Note: per design brief, pick-then-bind accepts the small TOCTOU window.
func AllocateEphemeralPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("allocating ephemeral port: %w", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}
