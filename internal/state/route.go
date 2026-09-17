package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Route represents the route data stored in ~/.bubble/routes/<id>.json.
type Route struct {
	ID             string    `json:"id"`
	Upstream       string    `json:"upstream"`
	Dir            string    `json:"dir"`
	ComposeProject string    `json:"compose_project"`
	CreatedAt      time.Time `json:"created_at"`
}

// WriteRoute saves or updates the route file for a bubble in ~/.bubble/routes/<id>.json.
func WriteRoute(r Route) error {
	routesDir := RoutesDir()
	if err := os.MkdirAll(routesDir, 0755); err != nil {
		return fmt.Errorf("creating routes directory %s: %w", routesDir, err)
	}

	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling route for %s: %w", r.ID, err)
	}

	filePath := filepath.Join(routesDir, fmt.Sprintf("%s.json", r.ID))
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("writing route file %s: %w", filePath, err)
	}

	return nil
}

// ReadRoute loads a route file from ~/.bubble/routes/<id>.json.
func ReadRoute(id string) (*Route, error) {
	filePath := filepath.Join(RoutesDir(), fmt.Sprintf("%s.json", id))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var r Route
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parsing route %s: %w", filePath, err)
	}

	return &r, nil
}

// DeleteRoute removes ~/.bubble/routes/<id>.json if it exists.
func DeleteRoute(id string) error {
	filePath := filepath.Join(RoutesDir(), fmt.Sprintf("%s.json", id))
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting route file %s: %w", filePath, err)
	}
	return nil
}

// ListRoutes returns all routes currently found in ~/.bubble/routes/.
func ListRoutes() ([]Route, error) {
	routesDir := RoutesDir()
	entries, err := os.ReadDir(routesDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading routes dir %s: %w", routesDir, err)
	}

	var routes []Route
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(routesDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var r Route
		if err := json.Unmarshal(data, &r); err == nil && r.ID != "" {
			routes = append(routes, r)
		}
	}

	return routes, nil
}
